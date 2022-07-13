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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/policyenginemocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
	"github.com/hyperledger/firefly/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	sampleTXHash  = "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"
	sampleTXHash2 = "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df"
)

func TestPolicyLoopE2EOk(t *testing.T) {

	mtx := &policyengine.ManagedTXOutput{
		ID:              "ns1:" + fftypes.NewUUID().String(),
		FirstSubmit:     fftypes.Now(),
		TransactionHash: sampleTXHash,
		Request:         &apitypes.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {
			var op core.Operation
			err := json.NewDecoder(r.Body).Decode(&op)
			assert.NoError(t, err)
			assert.Equal(t, core.OpStatusSucceeded, op.Status)
			w.WriteHeader(200)
		},
	)
	defer cancel()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		return n.NotificationType == confirmations.NewTransaction
	})).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		n.Transaction.Receipt(&ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			Success:          true,
		})
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		return n.NotificationType == confirmations.RemovedTransaction
	})).Return(nil)

	m.trackManaged(mtx)
	m.policyLoopCycle()

	err := m.execPolicy(m.pendingOpsByID[mtx.ID])
	assert.NoError(t, err)
	assert.Empty(t, m.pendingOpsByID)

	mc.AssertExpectations(t)
}

func TestPolicyLoopE2EOkReverted(t *testing.T) {

	mtx := &policyengine.ManagedTXOutput{
		ID:              "ns1:" + fftypes.NewUUID().String(),
		FirstSubmit:     fftypes.Now(),
		TransactionHash: sampleTXHash,
		Request:         &apitypes.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {
			var op core.Operation
			err := json.NewDecoder(r.Body).Decode(&op)
			assert.NoError(t, err)
			assert.Equal(t, core.OpStatusFailed, op.Status)
			w.WriteHeader(200)
		},
	)
	defer cancel()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		return n.NotificationType == confirmations.NewTransaction
	})).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		n.Transaction.Receipt(&ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			Success:          false,
		})
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		return n.NotificationType == confirmations.RemovedTransaction
	})).Return(nil)

	m.trackManaged(mtx)
	m.policyLoopCycle()

	err := m.execPolicy(m.pendingOpsByID[mtx.ID])
	assert.NoError(t, err)
	assert.Empty(t, m.pendingOpsByID)

	mc.AssertExpectations(t)
}

func TestPolicyLoopUpdateFFCoreWithError(t *testing.T) {

	mtx := &policyengine.ManagedTXOutput{
		ID:              "ns1:" + fftypes.NewUUID().String(),
		FirstSubmit:     fftypes.Now(),
		TransactionHash: sampleTXHash,
		Request:         &apitypes.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {
			var op core.Operation
			err := json.NewDecoder(r.Body).Decode(&op)
			assert.NoError(t, err)
			assert.Equal(t, core.OpStatusPending, op.Status)
			w.WriteHeader(200)
		},
	)
	defer cancel()

	m.policyEngine = &policyenginemocks.PolicyEngine{}
	pc := m.policyEngine.(*policyenginemocks.PolicyEngine)
	pc.On("Execute", mock.Anything, mock.Anything, mtx).Return(false, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	m.trackManaged(mtx)
	m.policyLoopCycle()

	err := m.execPolicy(m.pendingOpsByID[mtx.ID])
	assert.NoError(t, err)
	assert.NotEmpty(t, m.pendingOpsByID)
}

func TestPolicyLoopUpdateOpFail(t *testing.T) {

	mtx := &policyengine.ManagedTXOutput{
		ID:              "ns1:" + fftypes.NewUUID().String(),
		FirstSubmit:     fftypes.Now(),
		TransactionHash: sampleTXHash,
		Request:         &apitypes.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {
			errRes := fftypes.RESTError{Error: "pop"}
			b, err := json.Marshal(&errRes)
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			w.Write(b)
		},
	)
	defer cancel()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		return n.NotificationType == confirmations.NewTransaction
	})).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		n.Transaction.Receipt(&ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			Success:          true,
		})
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil)

	m.trackManaged(mtx)
	m.policyLoopCycle()

	err := m.execPolicy(m.pendingOpsByID[mtx.ID])
	assert.Regexp(t, "FF21017.*pop", err)
	assert.NotEmpty(t, m.pendingOpsByID)

	mc.AssertExpectations(t)
}

func TestPolicyLoopResubmitNewTXID(t *testing.T) {

	mtx := &policyengine.ManagedTXOutput{
		ID:      "ns1:" + fftypes.NewUUID().String(),
		Request: &apitypes.TransactionRequest{},
	}

	opUpdateCount := 0
	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {
			var op core.Operation
			err := json.NewDecoder(r.Body).Decode(&op)
			assert.NoError(t, err)
			opUpdateCount++
			if opUpdateCount == 1 {
				assert.Equal(t, core.OpStatusPending, op.Status)
			} else {
				assert.Equal(t, core.OpStatusSucceeded, op.Status)
			}
			w.WriteHeader(200)
		},
	)
	defer cancel()

	mFFC := m.connector.(*ffcapimocks.API)

	mFFC.On("TransactionPrepare", mock.Anything, mock.Anything).Return(&ffcapi.TransactionPrepareResponse{
		Gas:             fftypes.NewFFBigInt(12345),
		TransactionData: "0x12345",
	}, ffcapi.ErrorReason(""), nil)

	mFFC.On("TransactionSend", mock.Anything, mock.Anything).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: sampleTXHash2,
	}, ffcapi.ErrorReason(""), nil)

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		// First we get notified to remove the old TX hash
		return n.NotificationType == confirmations.RemovedTransaction &&
			n.Transaction.TransactionHash == sampleTXHash
	})).Return(nil)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		// Then we get the new TX hash, which we confirm
		return n.NotificationType == confirmations.NewTransaction &&
			n.Transaction.TransactionHash == sampleTXHash2
	})).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		n.Transaction.Receipt(&ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			Success:          true,
		})
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		// Then we're done
		return n.NotificationType == confirmations.RemovedTransaction &&
			n.Transaction.TransactionHash == sampleTXHash2
	})).Return(nil)

	m.trackManaged(mtx)
	pending := m.pendingOpsByID[mtx.ID]
	pending.trackingTransactionHash = sampleTXHash

	m.policyLoopCycle()

	err := m.execPolicy(m.pendingOpsByID[mtx.ID])
	assert.NoError(t, err)
	assert.Empty(t, m.pendingOpsByID)

	mc.AssertExpectations(t)
}

func TestPolicyLoopCycleCleanupRemoved(t *testing.T) {

	mtx := &policyengine.ManagedTXOutput{
		ID:      "ns1:" + fftypes.NewUUID().String(),
		Request: &apitypes.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.Anything).Return(nil).Once()

	m.trackManaged(mtx)
	m.markCancelledIfTracked(mtx.ID)

	err := m.execPolicy(m.pendingOpsByID[mtx.ID])
	assert.NoError(t, err)
	assert.Empty(t, m.pendingOpsByID)
}

func TestNotifyConfirmationMgrFail(t *testing.T) {

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.Anything).Return(fmt.Errorf("pop"))

	m.trackSubmittedTransaction(&pendingState{
		mtx: &policyengine.ManagedTXOutput{
			TransactionHash: sampleSendTX,
		},
	})

}
