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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/ffresty"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/httpserver"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-common/pkg/wsclient"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/events"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/internal/ws"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengines"
	"github.com/hyperledger/firefly/pkg/core"
)

type Manager interface {
	Start() error
	Close()
}

type manager struct {
	ctx           context.Context
	cancelCtx     func()
	connector     ffcapi.API
	confirmations confirmations.Manager
	policyEngine  policyengine.PolicyEngine
	apiServer     httpserver.HTTPServer
	ffCoreClient  *resty.Client
	wsClient      wsclient.WSClient
	wsChannels    ws.WebSocketChannels
	persistence   persistence.Persistence

	mux                 sync.Mutex
	nextNonces          map[string]uint64
	lockedNonces        map[string]*lockedNonce
	pendingOpsByID      map[string]*pendingState
	eventStreams        map[fftypes.UUID]events.Stream
	streamsByName       map[string]*fftypes.UUID
	changeEventLoopDone chan struct{}
	firstFullScanDone   chan error
	policyLoopDone      chan struct{}
	fullScanLoopDone    chan struct{}
	fullScanRequests    chan bool
	started             bool
	apiServerDone       chan error

	name                  string
	opTypes               []string
	startupScanMaxRetries int
	fullScanPageSize      int64
	fullScanMinDelay      time.Duration
	policyLoopInterval    time.Duration
	errorHistoryCount     int
	enableChangeListener  bool
}

func NewManager(ctx context.Context, connector ffcapi.API) (Manager, error) {
	var err error
	events.InitDefaults()
	m := &manager{
		connector:        connector,
		ffCoreClient:     ffresty.New(ctx, tmconfig.FFCoreConfig),
		fullScanRequests: make(chan bool, 1),
		nextNonces:       make(map[string]uint64),
		lockedNonces:     make(map[string]*lockedNonce),
		apiServerDone:    make(chan error),
		pendingOpsByID:   make(map[string]*pendingState),
		eventStreams:     make(map[fftypes.UUID]events.Stream),
		streamsByName:    make(map[string]*fftypes.UUID),

		name:                  config.GetString(tmconfig.ManagerName),
		opTypes:               config.GetStringSlice(tmconfig.OperationsTypes),
		startupScanMaxRetries: config.GetInt(tmconfig.OperationsFullScanStartupMaxRetries),
		fullScanPageSize:      config.GetInt64(tmconfig.OperationsFullScanPageSize),
		fullScanMinDelay:      config.GetDuration(tmconfig.OperationsFullScanMinimumDelay),
		policyLoopInterval:    config.GetDuration(tmconfig.PolicyLoopInterval),
		errorHistoryCount:     config.GetInt(tmconfig.OperationsErrorHistoryCount),
		enableChangeListener:  config.GetBool(tmconfig.OperationsChangeListenerEnabled),
	}
	m.ctx, m.cancelCtx = context.WithCancel(ctx)
	if m.name == "" {
		return nil, i18n.NewError(ctx, tmmsgs.MsgConfigParamNotSet, tmconfig.ManagerName)
	}
	m.confirmations, err = confirmations.NewBlockConfirmationManager(ctx, m.connector)
	if err != nil {
		return nil, err
	}
	m.policyEngine, err = policyengines.NewPolicyEngine(ctx, tmconfig.PolicyEngineBaseConfig, config.GetString(tmconfig.PolicyEngineName))
	if err != nil {
		return nil, err
	}
	wsconfig := wsclient.GenerateConfig(tmconfig.FFCoreConfig)
	m.wsClient, err = wsclient.New(m.ctx, wsconfig, nil, m.startChangeListener)
	if err != nil {
		return nil, err
	}
	m.apiServer, err = httpserver.NewHTTPServer(ctx, "api", m.router(), m.apiServerDone, tmconfig.APIConfig, tmconfig.CorsConfig)
	if err != nil {
		return nil, err
	}
	if err = m.initPersistence(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

type pendingState struct {
	mtx                     *policyengine.ManagedTXOutput
	confirmed               bool
	removed                 bool
	trackingTransactionHash string
}

func (m *manager) requestFullScan() {
	select {
	case m.fullScanRequests <- true:
		log.L(m.ctx).Debugf("Full scan of pending ops requested")
	default:
		log.L(m.ctx).Debugf("Full scan of pending ops already queued")
	}
}

func (m *manager) initPersistence(ctx context.Context) (err error) {
	pType := config.GetString(tmconfig.PersistenceType)
	switch pType {
	case "leveldb":
		if m.persistence, err = persistence.NewLevelDBPersistence(ctx); err != nil {
			return err
		}
		return nil
	default:
		return i18n.NewError(ctx, tmmsgs.MsgUnknownPersistence, pType)
	}
}

func (m *manager) waitScanDelay(lastFullScan *fftypes.FFTime) {
	scanDelay := m.fullScanMinDelay - time.Since(*lastFullScan.Time())
	log.L(m.ctx).Debugf("Delaying %dms before next full scan", scanDelay.Milliseconds())
	timer := time.NewTimer(scanDelay)
	select {
	case <-timer.C:
	case <-m.ctx.Done():
		log.L(m.ctx).Infof("Full scan loop exiting waiting for retry")
		return
	}
}

func (m *manager) fullScanLoop() {
	defer close(m.fullScanLoopDone)
	firstFullScanDone := m.firstFullScanDone
	var lastFullScan *fftypes.FFTime
	errorCount := 0
	for {
		select {
		case <-m.fullScanRequests:
			if lastFullScan != nil {
				m.waitScanDelay(lastFullScan)
			}
			lastFullScan = fftypes.Now()
			err := m.fullScan()
			if err != nil {
				errorCount++
				if firstFullScanDone != nil && errorCount > m.startupScanMaxRetries {
					firstFullScanDone <- err
					return
				}
				log.L(m.ctx).Errorf("Full scan failed (will be retried) count=%d: %s", errorCount, err)
				m.requestFullScan()
				continue
			}
			errorCount = 0
			// On startup we need to know the first scan has completed to populate the nonces,
			// before we complete startup
			if firstFullScanDone != nil {
				firstFullScanDone <- nil
				firstFullScanDone = nil
			}
		case <-m.ctx.Done():
			log.L(m.ctx).Infof("Full scan loop exiting")
			return
		}
	}
}

func (m *manager) fullScan() error {
	log.L(m.ctx).Debugf("Reading all operations after connect")
	var page int64
	var read, added int
	var lastOp *core.Operation
	for {
		ops, err := m.readOperationPage(lastOp)
		if err != nil {
			return err
		}
		if len(ops) == 0 {
			log.L(m.ctx).Debugf("Finished reading all operations - %d read, %d added", read, added)
			return nil
		}
		lastOp = ops[len(ops)-1]
		read += len(ops)
		for _, op := range ops {
			added++
			m.trackIfManaged(op)
		}
		page++
	}
}

func (m *manager) trackIfManaged(op *core.Operation) {
	outputJSON := []byte(op.Output.String())
	var mtx policyengine.ManagedTXOutput
	err := json.Unmarshal(outputJSON, &mtx)
	if err != nil {
		log.L(m.ctx).Warnf("Failed to parse output from operation %s", err)
		return
	}
	if mtx.FFTMName != m.name {
		log.L(m.ctx).Debugf("Operation %s is not managed by us (fftm=%s)", op.ID, mtx.FFTMName)
		return
	}
	if fmt.Sprintf("%s:%s", op.Namespace, op.ID) != mtx.ID {
		log.L(m.ctx).Warnf("Operation %s contains an invalid ID %s in the output", op.ID, mtx.ID)
		return
	}
	if mtx.Request == nil {
		log.L(m.ctx).Warnf("Operation %s contains a nil request in the output", op.ID)
		return
	}
	m.trackManaged(&mtx)
}

func (m *manager) trackManaged(mtx *policyengine.ManagedTXOutput) {
	m.mux.Lock()
	defer m.mux.Unlock()
	_, existing := m.pendingOpsByID[mtx.ID]
	if !existing {
		nextNonce, ok := m.nextNonces[mtx.Request.From]
		nonce := mtx.Nonce.Uint64()
		if !ok || nextNonce <= nonce {
			log.L(m.ctx).Debugf("Nonce %d in-flight. Next nonce: %d", nonce, nonce+1)
			m.nextNonces[mtx.Request.From] = nonce + 1
		}
		m.pendingOpsByID[mtx.ID] = &pendingState{
			mtx: mtx,
		}
	}
}

func (m *manager) markCancelledIfTracked(nsOpID string) {
	m.mux.Lock()
	pending, existing := m.pendingOpsByID[nsOpID]
	if existing {
		pending.removed = true
	}
	m.mux.Unlock()

}

func (m *manager) Start() error {
	m.fullScanRequests <- true
	m.firstFullScanDone = make(chan error)
	m.fullScanLoopDone = make(chan struct{})
	go m.fullScanLoop()
	err := m.waitForFirstScanAndStart()
	if err != nil {
		return err
	}
	return m.restoreStreams()
}

func (m *manager) waitForFirstScanAndStart() error {
	log.L(m.ctx).Infof("Waiting for first full scan of operations to build state")
	select {
	case err := <-m.firstFullScanDone:
		if err != nil {
			return err
		}
	case <-m.ctx.Done():
		log.L(m.ctx).Infof("Cancelled before startup completed")
		return nil
	}
	log.L(m.ctx).Infof("Scan complete. Completing startup")
	m.policyLoopDone = make(chan struct{})
	go m.receiptPollingLoop()
	go m.runAPIServer()
	go m.confirmations.Start()
	err := m.startWS()
	if err == nil {
		m.started = true
	}
	return err
}

func (m *manager) Close() {
	m.cancelCtx()
	if m.started {
		m.started = false
		<-m.apiServerDone
		<-m.fullScanLoopDone
		<-m.policyLoopDone
		m.waitWSStop()

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
