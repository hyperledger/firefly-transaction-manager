// Copyright © 2023 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fftm

import (
	"context"
	"net/http"
	"sync"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/httpserver"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/blocklistener"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/events"
	"github.com/hyperledger/firefly-transaction-manager/internal/metrics"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/internal/ws"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhandler"
	txRegistry "github.com/hyperledger/firefly-transaction-manager/pkg/txhandler/registry"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhistory"
)

type Manager interface {
	Start() error
	Close()
}

type manager struct {
	ctx           context.Context
	cancelCtx     func()
	confirmations confirmations.Manager
	txHandler     txhandler.TransactionHandler
	apiServer     httpserver.HTTPServer
	metricsServer httpserver.HTTPServer
	wsServer      ws.WebSocketServer
	persistence   persistence.Persistence

	txhistory txhistory.Manager
	connector ffcapi.API
	toolkit   *txhandler.Toolkit

	mux               sync.Mutex
	eventStreams      map[fftypes.UUID]events.Stream
	streamsByName     map[string]*fftypes.UUID
	blockListenerDone chan struct{}
	txHandlerDone     <-chan struct{}
	started           bool
	apiServerDone     chan error
	metricsServerDone chan error
	metricsEnabled    bool
	metricsManager    metrics.Metrics
	debugServer       *http.Server
	debugServerDone   chan struct{}
}

func InitConfig() {
	tmconfig.Reset()
	events.InitDefaults()
}

func NewManager(ctx context.Context, connector ffcapi.API) (Manager, error) {
	var err error
	m := newManager(ctx, connector)
	if err = m.initPersistence(ctx); err != nil {
		return nil, err
	}
	if err = m.initServices(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

func newManager(ctx context.Context, connector ffcapi.API) *manager {
	m := &manager{
		connector:         connector,
		apiServerDone:     make(chan error),
		metricsServerDone: make(chan error),
		metricsEnabled:    config.GetBool(tmconfig.MetricsEnabled),
		eventStreams:      make(map[fftypes.UUID]events.Stream),
		streamsByName:     make(map[string]*fftypes.UUID),
		metricsManager:    metrics.NewMetricsManager(ctx),
		txhistory:         txhistory.NewTxHistoryManager(ctx),
	}
	m.toolkit = &txhandler.Toolkit{
		Connector:      m.connector,
		TXHistory:      m.txhistory,
		MetricsManager: m.metricsManager,
	}
	m.ctx, m.cancelCtx = context.WithCancel(ctx)
	return m
}

func (m *manager) initServices(ctx context.Context) (err error) {
	m.confirmations = confirmations.NewBlockConfirmationManager(ctx, m.connector, "receipts")
	m.wsServer = ws.NewWebSocketServer(ctx)
	m.apiServer, err = httpserver.NewHTTPServer(ctx, "api", m.router(), m.apiServerDone, tmconfig.APIConfig, tmconfig.CorsConfig)
	if err != nil {
		return err
	}

	// check whether a policy engine name is provided
	if config.GetString(tmconfig.DeprecatedPolicyEngineName) != "" {
		log.L(ctx).Warnf("The 'policyengine.name' config key has been deprecated. Please use 'transactions.handler.name' instead")
		m.txHandler, err = txRegistry.NewTransactionHandler(ctx, tmconfig.DeprecatedPolicyEngineBaseConfig, config.GetString(tmconfig.DeprecatedPolicyEngineName))
	} else {
		// if not, fall back to use the deprecated policy engine
		m.txHandler, err = txRegistry.NewTransactionHandler(ctx, tmconfig.TransactionHandlerBaseConfig, config.GetString(tmconfig.TransactionHandlerName))
	}

	if err != nil {
		return err
	}
	m.toolkit.EventHandler = NewManagedTransactionEventHandler(ctx, m.confirmations, m.wsServer, m.txHandler)
	m.txHandler.Init(ctx, m.toolkit)

	// metrics service must be initialized after transaction handler
	// in case the transaction handler has logic in the Init function
	// to add more metrics
	if m.metricsEnabled {
		m.metricsServer, err = httpserver.NewHTTPServer(ctx, "metrics", m.createMetricsMuxRouter(), m.metricsServerDone, tmconfig.MetricsConfig, tmconfig.CorsConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *manager) initPersistence(ctx context.Context) (err error) {
	pType := config.GetString(tmconfig.PersistenceType)
	switch pType {
	case "leveldb":
		if m.persistence, err = persistence.NewLevelDBPersistence(ctx); err != nil {
			return i18n.NewError(ctx, tmmsgs.MsgPersistenceInitFail, pType, err)
		}
		m.toolkit.TXPersistence = m.persistence
		return nil
	default:
		return i18n.NewError(ctx, tmmsgs.MsgUnknownPersistence, pType)
	}
}

func (m *manager) Start() error {
	if err := m.restoreStreams(); err != nil {
		return err
	}

	blReq := &ffcapi.NewBlockListenerRequest{ListenerContext: m.ctx, ID: fftypes.NewUUID()}
	blReq.BlockListener, m.blockListenerDone = blocklistener.BufferChannel(m.ctx, m.confirmations)
	_, _, err := m.connector.NewBlockListener(m.ctx, blReq)
	if err != nil {
		return err
	}

	m.debugServerDone = make(chan struct{})
	go m.runDebugServer()
	go m.runAPIServer()
	if m.metricsEnabled {
		go m.runMetricsServer()
	}
	go m.confirmations.Start()

	m.txHandlerDone, err = m.txHandler.Start(m.ctx)
	if err != nil {
		return err
	}
	m.started = true
	return nil
}

func (m *manager) Close() {
	m.cancelCtx()
	if m.started {
		m.started = false
		if m.debugServer != nil {
			m.debugServer.Close()
		}
		<-m.apiServerDone
		if m.metricsEnabled {
			<-m.metricsServerDone
		}
		<-m.txHandlerDone
		<-m.blockListenerDone
		<-m.debugServerDone

		streams := []events.Stream{}
		m.mux.Lock()
		for _, s := range m.eventStreams {
			streams = append(streams, s)
		}
		m.mux.Unlock()
		for _, s := range streams {
			_ = s.Stop(m.ctx)
		}
	}
	m.persistence.Close(m.ctx)
}
