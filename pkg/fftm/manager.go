// Copyright © 2022 Kaleido, Inc.
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
	"time"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/httpserver"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/retry"
	"github.com/hyperledger/firefly-transaction-manager/internal/blocklistener"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/events"
	"github.com/hyperledger/firefly-transaction-manager/internal/metrics"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/internal/ws"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengines"
)

type Manager interface {
	Start() error
	Close()
}

type policyEngineAPIRequestType int

const (
	policyEngineAPIRequestTypeDelete policyEngineAPIRequestType = iota
)

// policyEngineAPIRequest requests are queued to the policy engine thread for processing against a given Transaction
type policyEngineAPIRequest struct {
	requestType policyEngineAPIRequestType
	txID        string
	startTime   time.Time
	response    chan policyEngineAPIResponse
}

type policyEngineAPIResponse struct {
	tx     *apitypes.ManagedTX
	err    error
	status int // http status code (200 Ok vs. 202 Accepted) - only set for success cases
}

type manager struct {
	ctx            context.Context
	cancelCtx      func()
	retry          *retry.Retry
	connector      ffcapi.API
	confirmations  confirmations.Manager
	policyEngine   policyengine.PolicyEngine
	apiServer      httpserver.HTTPServer
	metricsServer  httpserver.HTTPServer
	wsServer       ws.WebSocketServer
	persistence    persistence.Persistence
	inflightStale  chan bool
	inflightUpdate chan bool
	inflight       []*pendingState

	mux                     sync.Mutex
	policyEngineAPIRequests []*policyEngineAPIRequest
	lockedNonces            map[string]*lockedNonce
	eventStreams            map[fftypes.UUID]events.Stream
	streamsByName           map[string]*fftypes.UUID
	policyLoopDone          chan struct{}
	blockListenerDone       chan struct{}
	started                 bool
	apiServerDone           chan error
	metricsServerDone       chan error
	metricsEnabled          bool
	metricsManager          metrics.Manager
	debugServer             *http.Server
	debugServerDone         chan struct{}

	policyLoopInterval time.Duration
	nonceStateTimeout  time.Duration
	errorHistoryCount  int
	maxInFlight        int
}

func InitConfig() {
	tmconfig.Reset()
	events.InitDefaults()

	if config.GetBool(tmconfig.MetricsEnabled) {
		metrics.Registry()
	}
}

func NewManager(ctx context.Context, connector ffcapi.API) (Manager, error) {
	var err error
	m := newManager(ctx, connector)
	if err = m.initServices(ctx); err != nil {
		return nil, err
	}
	if err = m.initPersistence(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

func newManager(ctx context.Context, connector ffcapi.API) *manager {
	m := &manager{
		connector:          connector,
		lockedNonces:       make(map[string]*lockedNonce),
		apiServerDone:      make(chan error),
		metricsServerDone:  make(chan error),
		metricsEnabled:     config.GetBool(tmconfig.MetricsEnabled),
		eventStreams:       make(map[fftypes.UUID]events.Stream),
		streamsByName:      make(map[string]*fftypes.UUID),
		metricsManager:     metrics.NewMetricsManager(ctx),
		policyLoopInterval: config.GetDuration(tmconfig.PolicyLoopInterval),
		errorHistoryCount:  config.GetInt(tmconfig.TransactionsErrorHistoryCount),
		maxInFlight:        config.GetInt(tmconfig.TransactionsMaxInFlight),
		nonceStateTimeout:  config.GetDuration(tmconfig.TransactionsNonceStateTimeout),
		inflightStale:      make(chan bool, 1),
		inflightUpdate:     make(chan bool, 1),
		retry: &retry.Retry{
			InitialDelay: config.GetDuration(tmconfig.PolicyLoopRetryInitDelay),
			MaximumDelay: config.GetDuration(tmconfig.PolicyLoopRetryMaxDelay),
			Factor:       config.GetFloat64(tmconfig.PolicyLoopRetryFactor),
		},
	}
	m.ctx, m.cancelCtx = context.WithCancel(ctx)
	return m
}

type pendingState struct {
	mtx                     *apitypes.ManagedTX
	lastPolicyCycle         time.Time
	confirmed               bool
	remove                  bool
	trackingTransactionHash string
}

func (m *manager) initServices(ctx context.Context) (err error) {
	m.confirmations = confirmations.NewBlockConfirmationManager(ctx, m.connector, "receipts")
	m.policyEngine, err = policyengines.NewPolicyEngine(ctx, tmconfig.PolicyEngineBaseConfig, config.GetString(tmconfig.PolicyEngineName))
	if err != nil {
		return err
	}
	m.wsServer = ws.NewWebSocketServer(ctx)
	m.apiServer, err = httpserver.NewHTTPServer(ctx, "api", m.router(), m.apiServerDone, tmconfig.APIConfig, tmconfig.CorsConfig)
	if err != nil {
		return err
	}

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
	m.policyLoopDone = make(chan struct{})
	m.markInflightStale()
	go m.policyLoop()
	go m.confirmations.Start()

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
		<-m.policyLoopDone
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
