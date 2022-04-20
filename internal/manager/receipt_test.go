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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/policyenginemocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCheckReceiptE2EOk(t *testing.T) {

	mtx := &fftm.ManagedTXOutput{
		ID:          fftypes.NewUUID(),
		FirstSubmit: fftypes.Now(),
		Request:     &fftm.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		testFFCAPIHandler(t, func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int) {
			assert.Equal(t, ffcapi.RequestTypeGetReceipt, reqType)
			return &ffcapi.GetReceiptResponse{
				BlockNumber:      fftypes.NewFFBigInt(12345),
				TransactionIndex: fftypes.NewFFBigInt(10),
				BlockHash:        fftypes.NewRandB32().String(),
				Success:          true,
			}, 200
		}),
		func(w http.ResponseWriter, r *http.Request) {
			var op fftypes.Operation
			err := json.NewDecoder(r.Body).Decode(&op)
			assert.NoError(t, err)
			assert.Equal(t, mtx.ID, op.ID)
			assert.Equal(t, fftypes.OpStatusSucceeded, op.Status)
			w.WriteHeader(200)
		},
	)
	defer cancel()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.Anything).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		assert.Equal(t, confirmations.NewTransaction, n.NotificationType)
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil).Once()
	mc.On("Notify", mock.Anything).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		assert.Equal(t, confirmations.RemovedTransaction, n.NotificationType)
	}).Return(nil).Once()

	m.trackManaged(mtx)
	m.checkReceipts()

	err := m.checkReceiptCycle(m.pendingOpsByID[*mtx.ID])
	assert.NoError(t, err)
	assert.Empty(t, m.pendingOpsByID)

	mc.AssertExpectations(t)
}

func TestCheckReceiptE2EOkReverted(t *testing.T) {

	mtx := &fftm.ManagedTXOutput{
		ID:          fftypes.NewUUID(),
		FirstSubmit: fftypes.Now(),
		Request:     &fftm.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		testFFCAPIHandler(t, func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int) {
			assert.Equal(t, ffcapi.RequestTypeGetReceipt, reqType)
			return &ffcapi.GetReceiptResponse{
				BlockNumber:      fftypes.NewFFBigInt(12345),
				TransactionIndex: fftypes.NewFFBigInt(10),
				BlockHash:        fftypes.NewRandB32().String(),
				Success:          false,
			}, 200
		}),
		func(w http.ResponseWriter, r *http.Request) {
			var op fftypes.Operation
			err := json.NewDecoder(r.Body).Decode(&op)
			assert.NoError(t, err)
			assert.Equal(t, mtx.ID, op.ID)
			assert.Equal(t, fftypes.OpStatusFailed, op.Status)
			w.WriteHeader(200)
		},
	)
	defer cancel()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.Anything).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		assert.Equal(t, confirmations.NewTransaction, n.NotificationType)
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil).Once()
	mc.On("Notify", mock.Anything).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		assert.Equal(t, confirmations.RemovedTransaction, n.NotificationType)
	}).Return(nil).Once()

	m.trackManaged(mtx)
	m.checkReceipts()

	err := m.checkReceiptCycle(m.pendingOpsByID[*mtx.ID])
	assert.NoError(t, err)
	assert.Empty(t, m.pendingOpsByID)

	mc.AssertExpectations(t)
}

func TestCheckReceiptUpdateFFCoreWithError(t *testing.T) {

	mtx := &fftm.ManagedTXOutput{
		ID:      fftypes.NewUUID(),
		Request: &fftm.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {
			var op fftypes.Operation
			err := json.NewDecoder(r.Body).Decode(&op)
			assert.NoError(t, err)
			assert.Equal(t, mtx.ID, op.ID)
			assert.Equal(t, fftypes.OpStatusPending, op.Status)
			w.WriteHeader(200)
		},
	)
	defer cancel()

	m.policyEngine = &policyenginemocks.PolicyEngine{}
	pc := m.policyEngine.(*policyenginemocks.PolicyEngine)
	pc.On("Execute", mock.Anything, mock.Anything, mtx).Return(false, fmt.Errorf("pop"))

	m.trackManaged(mtx)
	m.checkReceipts()

	err := m.checkReceiptCycle(m.pendingOpsByID[*mtx.ID])
	assert.NoError(t, err)
	assert.NotEmpty(t, m.pendingOpsByID)
}

func TestCheckReceiptUpdateOpFail(t *testing.T) {

	mtx := &fftm.ManagedTXOutput{
		ID:          fftypes.NewUUID(),
		FirstSubmit: fftypes.Now(),
		Request:     &fftm.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		testFFCAPIHandler(t, func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int) {
			assert.Equal(t, ffcapi.RequestTypeGetReceipt, reqType)
			return &ffcapi.GetReceiptResponse{
				BlockNumber:      fftypes.NewFFBigInt(12345),
				TransactionIndex: fftypes.NewFFBigInt(10),
				BlockHash:        fftypes.NewRandB32().String(),
			}, 200
		}),
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
	mc.On("Notify", mock.Anything).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		assert.Equal(t, confirmations.NewTransaction, n.NotificationType)
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil).Once()

	m.trackManaged(mtx)
	m.checkReceipts()

	err := m.checkReceiptCycle(m.pendingOpsByID[*mtx.ID])
	assert.Regexp(t, "FF201017.*pop", err)
	assert.NotEmpty(t, m.pendingOpsByID)

	mc.AssertExpectations(t)
}

func TestCheckReceiptGetReceiptFail(t *testing.T) {

	mtx := &fftm.ManagedTXOutput{
		ID:          fftypes.NewUUID(),
		FirstSubmit: fftypes.Now(),
		Request:     &fftm.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		testFFCAPIHandler(t, func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int) {
			assert.Equal(t, ffcapi.RequestTypeGetReceipt, reqType)
			return &ffcapi.ErrorResponse{Error: "pop"}, 500
		}),
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	m.trackManaged(mtx)
	m.checkReceipts()

	err := m.checkReceiptCycle(m.pendingOpsByID[*mtx.ID])
	assert.Regexp(t, "FF201012.*pop", err)
	assert.NotEmpty(t, m.pendingOpsByID)
}

func TestCheckReceiptGetReceiptForkRemoved(t *testing.T) {

	mtx := &fftm.ManagedTXOutput{
		ID:          fftypes.NewUUID(),
		FirstSubmit: fftypes.Now(),
		Request:     &fftm.TransactionRequest{},
		Receipt: &ffcapi.GetReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
		},
	}

	_, m, cancel := newTestManager(t,
		testFFCAPIHandler(t, func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int) {
			assert.Equal(t, ffcapi.RequestTypeGetReceipt, reqType)
			return &ffcapi.ErrorResponse{Error: "not found", Reason: ffcapi.ErrorReasonNotFound}, 404
		}),
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.Anything).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		assert.Equal(t, confirmations.RemovedTransaction, n.NotificationType)
	}).Return(nil).Once()

	m.trackManaged(mtx)

	err := m.checkReceiptCycle(m.pendingOpsByID[*mtx.ID])
	assert.NoError(t, err)
	assert.NotEmpty(t, m.pendingOpsByID)

	mc.AssertExpectations(t)
}

func TestCheckReceiptCycleCleanupRemoved(t *testing.T) {

	mtx := &fftm.ManagedTXOutput{
		ID:      fftypes.NewUUID(),
		Request: &fftm.TransactionRequest{},
	}

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.Anything).Return(nil).Once()

	m.trackManaged(mtx)
	m.markCancelledIfTracked(mtx.ID)

	err := m.checkReceiptCycle(m.pendingOpsByID[*mtx.ID])
	assert.NoError(t, err)
	assert.Empty(t, m.pendingOpsByID)
}
