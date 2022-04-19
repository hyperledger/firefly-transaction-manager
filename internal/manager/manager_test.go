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
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hyperledger/firefly-transaction-manager/internal/policyengines"
	"github.com/hyperledger/firefly-transaction-manager/internal/policyengines/simple"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/ffresty"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/httpserver"
	"github.com/stretchr/testify/assert"
)

const testManagerName = "unittest"

func newTestManager(t *testing.T, cAPIHandler http.HandlerFunc, ffCoreHandler http.HandlerFunc) (string, *manager, func()) {
	tmconfig.Reset()
	policyengines.RegisterEngine(tmconfig.PolicyEngineBasePrefix, &simple.PolicyEngineFactory{})

	cAPIServer := httptest.NewServer(cAPIHandler)
	tmconfig.ConnectorPrefix.Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", cAPIServer.Listener.Addr()))

	ffCoreServer := httptest.NewServer(ffCoreHandler)
	tmconfig.FFCorePrefix.Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", ffCoreServer.Listener.Addr()))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	managerPort := strings.Split(ln.Addr().String(), ":")[1]
	ln.Close()
	tmconfig.APIPrefix.Set(httpserver.HTTPConfPort, managerPort)
	tmconfig.APIPrefix.Set(httpserver.HTTPConfAddress, "127.0.0.1")

	config.Set(tmconfig.ManagerName, testManagerName)
	config.Set(tmconfig.ReceiptsPollingInterval, "1ms")
	tmconfig.PolicyEngineBasePrefix.SubPrefix("simple").Set(simple.FixedGas, "223344556677")

	mm, err := NewManager(context.Background())
	assert.NoError(t, err)
	m := mm.(*manager)
	m.confirmations = &confirmationsmocks.Manager{}

	return fmt.Sprintf("http://127.0.0.1:%s", managerPort),
		m,
		func() {
			cAPIServer.Close()
			ffCoreServer.Close()
			_ = m.WaitStop()
		}

}

func newTestOperation(t *testing.T, mtx *fftm.ManagedTXOutput, status fftypes.OpStatus) *fftypes.Operation {
	b, err := json.Marshal(&mtx)
	assert.NoError(t, err)
	op := &fftypes.Operation{
		ID:     mtx.ID,
		Status: status,
	}
	err = json.Unmarshal(b, &op.Output)
	assert.NoError(t, err)
	return op
}

func TestNewManagerMissingName(t *testing.T) {

	tmconfig.Reset()
	config.Set(tmconfig.ManagerName, "")

	_, err := NewManager(context.Background())
	assert.Regexp(t, "FF201018", err)

}

func TestNewManagerBadHttpConfig(t *testing.T) {

	tmconfig.Reset()
	config.Set(tmconfig.ManagerName, "test")
	tmconfig.APIPrefix.Set(httpserver.HTTPConfAddress, "::::")

	policyengines.RegisterEngine(tmconfig.PolicyEngineBasePrefix, &simple.PolicyEngineFactory{})
	tmconfig.PolicyEngineBasePrefix.SubPrefix("simple").Set(simple.FixedGas, "223344556677")

	_, err := NewManager(context.Background())
	assert.Regexp(t, "FF10104", err)

}

func TestNewManagerBadConfirmationsCacheSize(t *testing.T) {

	tmconfig.Reset()
	config.Set(tmconfig.ManagerName, "test")
	config.Set(tmconfig.ConfirmationsBlockCacheSize, -1)

	_, err := NewManager(context.Background())
	assert.Regexp(t, "FF201015", err)

}

func TestNewManagerBadPolicyEngine(t *testing.T) {

	tmconfig.Reset()
	config.Set(tmconfig.ManagerName, "test")
	config.Set(tmconfig.PolicyEngineName, "wrong")

	_, err := NewManager(context.Background())
	assert.Regexp(t, "FF201019", err)

}

func TestChangeEventsNewTracked(t *testing.T) {

	ce := &fftypes.ChangeEvent{
		ID:         fftypes.NewUUID(),
		Type:       fftypes.ChangeEventTypeUpdated,
		Collection: "operations",
		Namespace:  "ns1",
	}

	var m *manager
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, fmt.Sprintf("/admin/api/v1/operations/%s", ce.ID), r.URL.Path)
			b, err := json.Marshal(newTestOperation(t, &fftm.ManagedTXOutput{
				ID:       ce.ID,
				FFTMName: testManagerName,
			}, fftypes.OpStatusPending))
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write(b)
			// Cancel context here so loop ends
			m.cancelCtx()
		},
	)
	defer cancel()

	m.changeEvents = make(chan *fftypes.ChangeEvent, 1)
	m.changeEventLoopDone = make(chan struct{})
	m.changeEvents <- ce

	m.changeEventLoop()

	assert.Equal(t, ce.ID, m.pendingOpsByID[*ce.ID].mtx.ID)

}

func TestChangeEventsNewBadOutput(t *testing.T) {

	ce := &fftypes.ChangeEvent{
		ID:         fftypes.NewUUID(),
		Type:       fftypes.ChangeEventTypeUpdated,
		Collection: "operations",
		Namespace:  "ns1",
	}

	var m *manager
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, fmt.Sprintf("/admin/api/v1/operations/%s", ce.ID), r.URL.Path)
			b, err := json.Marshal(&fftypes.Operation{
				ID:     ce.ID,
				Status: fftypes.OpStatusPending,
				Output: fftypes.JSONObject{
					"id": "!not a UUID",
				},
			})
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write(b)
		},
	)
	defer cancel()

	m.handleEvent(ce)
	assert.Empty(t, m.pendingOpsByID)

}

func TestChangeEventsWrongName(t *testing.T) {

	ce := &fftypes.ChangeEvent{
		ID:         fftypes.NewUUID(),
		Type:       fftypes.ChangeEventTypeUpdated,
		Collection: "operations",
		Namespace:  "ns1",
	}

	var m *manager
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, fmt.Sprintf("/admin/api/v1/operations/%s", ce.ID), r.URL.Path)
			b, err := json.Marshal(newTestOperation(t, &fftm.ManagedTXOutput{
				ID:       ce.ID,
				FFTMName: "wrong",
			}, fftypes.OpStatusPending))
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write(b)
		},
	)
	defer cancel()

	m.handleEvent(ce)
	assert.Empty(t, m.pendingOpsByID)

}

func TestChangeEventsWrongID(t *testing.T) {

	ce := &fftypes.ChangeEvent{
		ID:         fftypes.NewUUID(),
		Type:       fftypes.ChangeEventTypeUpdated,
		Collection: "operations",
		Namespace:  "ns1",
	}

	var m *manager
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, fmt.Sprintf("/admin/api/v1/operations/%s", ce.ID), r.URL.Path)
			op := newTestOperation(t, &fftm.ManagedTXOutput{
				ID:       fftypes.NewUUID(),
				FFTMName: testManagerName,
			}, fftypes.OpStatusPending)
			op.ID = fftypes.NewUUID()
			b, err := json.Marshal(&op)
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write(b)
		},
	)
	defer cancel()

	m.handleEvent(ce)
	assert.Empty(t, m.pendingOpsByID)

}

func TestChangeEventsQueryFail(t *testing.T) {

	ce := &fftypes.ChangeEvent{
		ID:         fftypes.NewUUID(),
		Type:       fftypes.ChangeEventTypeUpdated,
		Collection: "operations",
		Namespace:  "ns1",
	}

	var m *manager
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, fmt.Sprintf("/admin/api/v1/operations/%s", ce.ID), r.URL.Path)
			w.WriteHeader(404)
		},
	)
	defer cancel()

	m.fullScanRequests = make(chan bool, 1)

	m.handleEvent(ce)
	assert.Empty(t, m.pendingOpsByID)

	// Full scan should have been requested after this failure
	<-m.fullScanRequests

}

func TestChangeEventsMarkForCleanup(t *testing.T) {

	ce := &fftypes.ChangeEvent{
		ID:         fftypes.NewUUID(),
		Type:       fftypes.ChangeEventTypeUpdated,
		Collection: "operations",
		Namespace:  "ns1",
	}

	op := newTestOperation(t, &fftm.ManagedTXOutput{
		ID:       fftypes.NewUUID(),
		FFTMName: testManagerName,
	}, fftypes.OpStatusFailed)

	var m *manager
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, fmt.Sprintf("/admin/api/v1/operations/%s", ce.ID), r.URL.Path)
			b, err := json.Marshal(&op)
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write(b)
		},
	)
	defer cancel()

	m.trackIfManaged(op)
	m.handleEvent(ce)
	assert.True(t, m.pendingOpsByID[*op.ID].removed)

}

func TestStartupScanMultiPageOK(t *testing.T) {

	op1 := newTestOperation(t, &fftm.ManagedTXOutput{
		ID:       fftypes.NewUUID(),
		FFTMName: testManagerName,
	}, fftypes.OpStatusPending)
	t1 := fftypes.FFTime(time.Now().Add(-10 * time.Minute))
	op1.Created = &t1
	op2 := newTestOperation(t, &fftm.ManagedTXOutput{
		ID:       fftypes.NewUUID(),
		FFTMName: testManagerName,
	}, fftypes.OpStatusPending)
	t2 := fftypes.FFTime(time.Now().Add(-5 * time.Minute))
	op2.Created = &t2
	op3 := newTestOperation(t, &fftm.ManagedTXOutput{
		ID:       fftypes.NewUUID(),
		FFTMName: testManagerName,
	}, fftypes.OpStatusPending)
	t3 := fftypes.FFTime(time.Now().Add(-1 * time.Minute))
	op3.Created = &t3

	call := 0

	var m *manager
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/admin/api/v1/operations", r.URL.Path)
			status := 200
			var res interface{}
			switch call {
			case 0:
				res = &fftypes.RESTError{Error: "not ready yet"}
				status = 500
			case 1:
				res = []*fftypes.Operation{op1, op2}
				assert.Equal(t, "", r.URL.Query().Get("created"))
			case 2:
				res = []*fftypes.Operation{op2 /* simulate overlap */, op3}
				assert.Equal(t, fmt.Sprintf(">=%d", op2.Created.Time().UnixNano()), r.URL.Query().Get("created"))
			case 3:
				res = []*fftypes.Operation{}
				assert.Equal(t, fmt.Sprintf(">=%d", op3.Created.Time().UnixNano()), r.URL.Query().Get("created"))
			default:
				assert.Fail(t, "should have stopped after empty page")
			}
			call++
			b, err := json.Marshal(res)
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			w.Write(b)
		},
	)
	defer cancel()
	m.fullScanMinDelay = 1 * time.Microsecond

	err := m.Start()
	assert.NoError(t, err)

	assert.Len(t, m.pendingOpsByID, 3)

}

func TestStartupScanFail(t *testing.T) {

	var m *manager
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	cancel() // close servers
	m.ctx = context.Background()
	m.startupScanMaxRetries = 2
	m.fullScanMinDelay = 1 * time.Microsecond

	err := m.Start()
	assert.Regexp(t, "FF201017", err)

}

func TestRequestFullScanNonBlocking(t *testing.T) {

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	m.requestFullScan()
	m.requestFullScan()
	m.requestFullScan()

}

func TestRequestFullScanCancelledBeforeStart(t *testing.T) {

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	m.cancelCtx()
	m.waitForFirstScanAndStart()

}

func TestStartupCancelledDuringRetry(t *testing.T) {

	var m *manager
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	cancel() // close servers
	m.startupScanMaxRetries = 2
	m.fullScanMinDelay = 1 * time.Second

	m.waitScanDelay(fftypes.Now())

}
