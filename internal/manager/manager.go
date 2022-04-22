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

package manager

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/policyengines"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/ffresty"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/httpserver"
	"github.com/hyperledger/firefly/pkg/i18n"
	"github.com/hyperledger/firefly/pkg/log"
	"github.com/hyperledger/firefly/pkg/wsclient"
)

type Manager interface {
	Start() error
	Stop()
	WaitStop() error
}

type manager struct {
	ctx           context.Context
	cancelCtx     func()
	connectorAPI  ffcapi.API
	confirmations confirmations.Manager
	policyEngine  policyengine.PolicyEngine
	apiServer     httpserver.HTTPServer
	ffCoreClient  *resty.Client
	wsClient      wsclient.WSClient

	mux                 sync.Mutex
	nextNonces          map[string]uint64
	lockedNonces        map[string]*lockedNonce
	pendingOpsByID      map[fftypes.UUID]*pendingState
	changeEventLoopDone chan struct{}
	firstFullScanDone   chan error
	receiptPollerDone   chan struct{}
	fullScanLoopDone    chan struct{}
	fullScanRequests    chan bool
	started             bool
	apiServerDone       chan error

	wsDisabled              bool
	name                    string
	opTypes                 []string
	startupScanMaxRetries   int
	fullScanPageSize        int64
	fullScanMinDelay        time.Duration
	receiptsPollingInterval time.Duration
	errorHistoryCount       int
}

func NewManager(ctx context.Context) (Manager, error) {
	var err error
	m := &manager{
		connectorAPI:     ffcapi.NewFFCAPI(ctx),
		ffCoreClient:     ffresty.New(ctx, tmconfig.FFCorePrefix),
		fullScanRequests: make(chan bool, 1),
		nextNonces:       make(map[string]uint64),
		lockedNonces:     make(map[string]*lockedNonce),
		apiServerDone:    make(chan error),
		pendingOpsByID:   make(map[fftypes.UUID]*pendingState),

		name:                    config.GetString(tmconfig.ManagerName),
		opTypes:                 config.GetStringSlice(tmconfig.OperationsTypes),
		startupScanMaxRetries:   config.GetInt(tmconfig.OperationsFullScanStartupMaxRetries),
		fullScanPageSize:        config.GetInt64(tmconfig.OperationsFullScanPageSize),
		fullScanMinDelay:        config.GetDuration(tmconfig.OperationsFullScanMinimumDelay),
		receiptsPollingInterval: config.GetDuration(tmconfig.ReceiptsPollingInterval),
		errorHistoryCount:       config.GetInt(tmconfig.OperationsErrorHistoryCount),
	}
	m.ctx, m.cancelCtx = context.WithCancel(ctx)
	if m.name == "" {
		return nil, i18n.NewError(ctx, tmmsgs.MsgConfigParamNotSet, tmconfig.ManagerName)
	}
	m.confirmations, err = confirmations.NewBlockConfirmationManager(ctx, m.connectorAPI)
	if err != nil {
		return nil, err
	}
	m.policyEngine, err = policyengines.NewPolicyEngine(ctx, tmconfig.PolicyEngineBasePrefix, config.GetString(tmconfig.PolicyEngineName))
	if err != nil {
		return nil, err
	}
	wsconfig := wsclient.GenerateConfigFromPrefix(tmconfig.FFCorePrefix)
	m.wsClient, err = wsclient.New(m.ctx, wsconfig, nil, m.startChangeListener)
	if err != nil {
		return nil, err
	}
	m.apiServer, err = httpserver.NewHTTPServer(ctx, "api", m.router(), m.apiServerDone, tmconfig.APIPrefix)
	if err != nil {
		return nil, err
	}
	return m, nil
}

type pendingState struct {
	mtx                  *fftm.ManagedTXOutput
	confirmed            bool
	removed              bool
	lastReceiptBlockHash string
}

func (m *manager) startChangeListener(ctx context.Context, w wsclient.WSClient) error {
	cmd := fftypes.WSChangeEventCommand{
		Type:        fftypes.WSChangeEventCommandTypeStart,
		Collections: []string{"operations"},
		Filter: fftypes.ChangeEventFilter{
			Types: []fftypes.ChangeEventType{fftypes.ChangeEventTypeUpdated},
		},
	}
	b, _ := json.Marshal(&cmd)
	log.L(m.ctx).Infof("Change listener connected. Sent: %s", b)
	return w.Send(ctx, b)
}

func (m *manager) requestFullScan() {
	select {
	case m.fullScanRequests <- true:
		log.L(m.ctx).Debugf("Full scan of pending ops requested")
	default:
		log.L(m.ctx).Debugf("Full scan of pending ops already queued")
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
	var lastOp *fftypes.Operation
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

func (m *manager) trackIfManaged(op *fftypes.Operation) {
	outputJSON := []byte(op.Output.String())
	var mtx fftm.ManagedTXOutput
	err := json.Unmarshal(outputJSON, &mtx)
	if err != nil {
		log.L(m.ctx).Warnf("Failed to parse output from operation %s", err)
		return
	}
	if mtx.FFTMName != m.name {
		log.L(m.ctx).Debugf("Operation %s is not managed by us (fftm=%s)", op.ID, mtx.FFTMName)
		return
	}
	if !op.ID.Equals(mtx.ID) {
		log.L(m.ctx).Warnf("Operation %s contains an invalid ID %s in the output", op.ID, mtx.ID)
		return
	}
	if mtx.Request == nil {
		log.L(m.ctx).Warnf("Operation %s contains a nil request in the output", op.ID)
		return
	}
	m.trackManaged(&mtx)
}

func (m *manager) trackManaged(mtx *fftm.ManagedTXOutput) {
	m.mux.Lock()
	defer m.mux.Unlock()
	_, existing := m.pendingOpsByID[*mtx.ID]
	if !existing {
		nextNonce, ok := m.nextNonces[mtx.Request.From]
		nonce := mtx.Nonce.Uint64()
		if !ok || nextNonce <= nonce {
			log.L(m.ctx).Debugf("Nonce %d in-flight. Next nonce: %d", nonce, nonce+1)
			m.nextNonces[mtx.Request.From] = nonce + 1
		}
		m.pendingOpsByID[*mtx.ID] = &pendingState{
			mtx: mtx,
		}
	}
}

func (m *manager) markCancelledIfTracked(opID *fftypes.UUID) {
	m.mux.Lock()
	pending, existing := m.pendingOpsByID[*opID]
	if existing {
		pending.removed = true
	}
	m.mux.Unlock()

}

func (m *manager) handleEvent(ce *fftypes.ChangeEvent) {
	log.L(m.ctx).Debugf("%s:%s/%s operation change event received", ce.Namespace, ce.ID, ce.Type)
	// Note that we only subscribe the events on update (this check is just belt and braces).
	// The operation gets created before any connector is called, so the first event should be
	// after we do the update from the prepare.
	if ce.Collection == "operations" && ce.Type == fftypes.ChangeEventTypeUpdated {
		m.mux.Lock()
		_, knownID := m.pendingOpsByID[*ce.ID]
		m.mux.Unlock()
		if !knownID {
			m.queryAndAddPending(ce.ID)
		}
	}
}

func (m *manager) changeEventLoop() {
	defer close(m.changeEventLoopDone)
	for {
		select {
		case b := <-m.wsClient.Receive():
			var ce *fftypes.ChangeEvent
			_ = json.Unmarshal(b, &ce)
			m.handleEvent(ce)
		case <-m.ctx.Done():
			log.L(m.ctx).Infof("Change event loop exiting")
			return
		}
	}
}

func (m *manager) Start() error {
	m.fullScanRequests <- true
	m.firstFullScanDone = make(chan error)
	m.fullScanLoopDone = make(chan struct{})
	go m.fullScanLoop()
	return m.waitForFirstScanAndStart()
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
	m.receiptPollerDone = make(chan struct{})
	go m.receiptPollingLoop()
	go m.runAPIServer()
	go m.confirmations.Start()
	err := m.startWS()
	if err == nil {
		m.started = true
	}
	return err
}

func (m *manager) startWS() error {
	if !m.wsDisabled {
		m.changeEventLoopDone = make(chan struct{})
		if err := m.wsClient.Connect(); err != nil {
			return err
		}
		go m.changeEventLoop()
	}
	return nil
}

func (m *manager) waitWSStop() {
	if !m.wsDisabled {
		<-m.changeEventLoopDone
	}
}

func (m *manager) Stop() {
	m.cancelCtx()
}

func (m *manager) WaitStop() (err error) {
	if m.started {
		m.started = false
		err = <-m.apiServerDone
		<-m.fullScanLoopDone
		<-m.receiptPollerDone
		m.waitWSStop()
	}
	return err
}
