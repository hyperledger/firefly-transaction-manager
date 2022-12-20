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
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/persistencemocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/policyenginemocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func sendSampleTX(t *testing.T, m *manager, signer string, nonce int64) *apitypes.ManagedTX {

	txInput := ffcapi.TransactionInput{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: signer,
		},
	}

	mfc := m.connector.(*ffcapimocks.API)
	mfc.On("NextNonceForSigner", m.ctx, &ffcapi.NextNonceForSignerRequest{
		Signer: signer,
	}).Return(&ffcapi.NextNonceForSignerResponse{
		Nonce: fftypes.NewFFBigInt(nonce),
	}, ffcapi.ErrorReason(""), nil).Once()
	mfc.On("TransactionPrepare", m.ctx, &ffcapi.TransactionPrepareRequest{
		TransactionInput: txInput,
	}).Return(&ffcapi.TransactionPrepareResponse{
		Gas:             fftypes.NewFFBigInt(100000),
		TransactionData: "0xabce1234",
	}, ffcapi.ErrorReason(""), nil).Once()

	mtx, err := m.sendManagedTransaction(m.ctx, &apitypes.TransactionRequest{
		TransactionInput: txInput,
	})
	assert.NoError(t, err)
	return mtx
}

func TestPolicyLoopE2EOk(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()

	mtx := sendSampleTX(t, m, "0xaaaaa", 12345)
	txHash := "0x" + fftypes.NewRandB32().String()

	mfc := m.connector.(*ffcapimocks.API)
	mfc.On("TransactionSend", m.ctx, mock.MatchedBy(func(r *ffcapi.TransactionSendRequest) bool {
		return r.Nonce.Equals(fftypes.NewFFBigInt(12345))
	})).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: txHash,
	}, ffcapi.ErrorReason(""), nil)

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		return n.NotificationType == confirmations.NewTransaction
	})).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		n.Transaction.Receipt(context.Background(), &ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			ProtocolID:       fmt.Sprintf("%.12d/%.6d", fftypes.NewFFBigInt(12345).Int64(), fftypes.NewFFBigInt(10).Int64()),
			Success:          true,
		})
		n.Transaction.Confirmed(context.Background(), []confirmations.BlockInfo{})
	}).Return(nil)

	// Run the policy once to do the send
	<-m.inflightStale // from sending the TX
	m.policyLoopCycle(m.ctx, true)
	assert.Equal(t, mtx.ID, m.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, m.inflight[0].mtx.Status)

	// A second time will mark it complete for flush
	m.policyLoopCycle(m.ctx, false)

	<-m.inflightStale // policy loop should have marked us stale, to clean up the TX
	m.policyLoopCycle(m.ctx, true)
	assert.Empty(t, m.inflight)

	// Check the update is persisted
	rtx, err := m.persistence.GetTransactionByID(m.ctx, mtx.ID)
	assert.NoError(t, err)
	assert.Equal(t, apitypes.TxStatusSucceeded, rtx.Status)

	mc.AssertExpectations(t)
	mfc.AssertExpectations(t)
}

func TestPolicyLoopE2EReverted(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()

	mtx := sendSampleTX(t, m, "0xaaaaa", 12345)
	txHash := "0x" + fftypes.NewRandB32().String()

	mfc := m.connector.(*ffcapimocks.API)
	mfc.On("TransactionSend", m.ctx, mock.MatchedBy(func(r *ffcapi.TransactionSendRequest) bool {
		return r.Nonce.Equals(fftypes.NewFFBigInt(12345))
	})).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: txHash,
	}, ffcapi.ErrorReason(""), nil)

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		return n.NotificationType == confirmations.NewTransaction
	})).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		n.Transaction.Receipt(context.Background(), &ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			ProtocolID:       fmt.Sprintf("%.12d/%.6d", fftypes.NewFFBigInt(12345).Int64(), fftypes.NewFFBigInt(10).Int64()),
			Success:          false,
		})
		n.Transaction.Confirmed(context.Background(), []confirmations.BlockInfo{})
	}).Return(nil)

	// Run the policy once to do the send
	<-m.inflightStale // from sending the TX
	m.policyLoopCycle(m.ctx, true)
	assert.Equal(t, mtx.ID, m.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, m.inflight[0].mtx.Status)

	// A second time will mark it complete for flush
	m.policyLoopCycle(m.ctx, false)

	<-m.inflightStale // policy loop should have marked us stale, to clean up the TX
	m.policyLoopCycle(m.ctx, true)
	assert.Empty(t, m.inflight)

	// Check the update is persisted
	rtx, err := m.persistence.GetTransactionByID(m.ctx, mtx.ID)
	assert.NoError(t, err)
	assert.Equal(t, apitypes.TxStatusFailed, rtx.Status)

	mc.AssertExpectations(t)
	mfc.AssertExpectations(t)
}

func TestPolicyLoopResubmitNewTXID(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()

	mtx := sendSampleTX(t, m, "0xaaaaa", 12345)
	txHash1 := "0x" + fftypes.NewRandB32().String()
	txHash2 := "0x" + fftypes.NewRandB32().String()

	mfc := m.connector.(*ffcapimocks.API)

	mfc.On("TransactionPrepare", mock.Anything, mock.Anything).Return(&ffcapi.TransactionPrepareResponse{
		Gas:             fftypes.NewFFBigInt(12345),
		TransactionData: "0x12345",
	}, ffcapi.ErrorReason(""), nil)

	mfc.On("TransactionSend", mock.Anything, mock.Anything).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: txHash1,
	}, ffcapi.ErrorReason(""), nil).Once()
	mfc.On("TransactionSend", mock.Anything, mock.Anything).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: txHash2,
	}, ffcapi.ErrorReason(""), nil).Once()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		// First we get notified to add the old TX hash
		return n.NotificationType == confirmations.NewTransaction &&
			n.Transaction.TransactionHash == txHash1
	})).Return(nil)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		// Then we get notified to remove the old TX hash
		return n.NotificationType == confirmations.RemovedTransaction &&
			n.Transaction.TransactionHash == txHash1
	})).Return(nil)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		// Then we get the new TX hash, which we confirm
		return n.NotificationType == confirmations.NewTransaction &&
			n.Transaction.TransactionHash == txHash2
	})).Run(func(args mock.Arguments) {
		n := args[0].(*confirmations.Notification)
		n.Transaction.Receipt(context.Background(), &ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			ProtocolID:       fmt.Sprintf("%.12d/%.6d", fftypes.NewFFBigInt(12345).Int64(), fftypes.NewFFBigInt(10).Int64()),
			Success:          true,
		})
		n.Transaction.Confirmed(context.Background(), []confirmations.BlockInfo{})
	}).Return(nil)

	// Run the policy once to do the send with the first hash
	<-m.inflightStale // from sending the TX
	m.policyLoopCycle(m.ctx, true)
	assert.Len(t, m.inflight, 1)
	assert.Equal(t, mtx.ID, m.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, m.inflight[0].mtx.Status)

	// Run again to confirm it does not change anything, when the state is the same
	m.policyLoopCycle(m.ctx, true)
	assert.Len(t, m.inflight, 1)
	assert.Equal(t, mtx.ID, m.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, m.inflight[0].mtx.Status)

	// Reset the transaction so the policy manager resubmits it
	m.inflight[0].mtx.FirstSubmit = nil
	m.policyLoopCycle(m.ctx, false)
	assert.Equal(t, mtx.ID, m.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, m.inflight[0].mtx.Status)

	mc.AssertExpectations(t)
	mfc.AssertExpectations(t)
}

func TestNotifyConfirmationMgrFail(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()

	_ = sendSampleTX(t, m, "0xaaaaa", 12345)
	txHash := "0x" + fftypes.NewRandB32().String()

	mfc := m.connector.(*ffcapimocks.API)
	mfc.On("TransactionSend", mock.Anything, mock.Anything).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: txHash,
	}, ffcapi.ErrorReason(""), nil).Once()

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.Anything).Return(fmt.Errorf("pop"))

	m.policyLoopCycle(m.ctx, true)

	mc.AssertExpectations(t)
	mfc.AssertExpectations(t)

}

func TestInflightSetListFailCancel(t *testing.T) {

	_, m, close := newTestManagerMockPersistence(t)
	close()

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("ListTransactionsPending", m.ctx, (*fftypes.UUID)(nil), m.maxInFlight, persistence.SortDirectionAscending).
		Return(nil, fmt.Errorf("pop"))

	m.policyLoopCycle(m.ctx, true)

	mp.AssertExpectations(t)

}

func TestPolicyLoopUpdateFail(t *testing.T) {

	_, m, close := newTestManagerMockPersistence(t)
	defer close()

	m.inflight = []*pendingState{
		{
			confirmed: true,
			mtx: &apitypes.ManagedTX{
				ID:          fmt.Sprintf("ns1/%s", fftypes.NewUUID()),
				Created:     fftypes.Now(),
				SequenceID:  apitypes.NewULID(),
				Nonce:       fftypes.NewFFBigInt(1000),
				Status:      apitypes.TxStatusSucceeded,
				FirstSubmit: fftypes.Now(),
				Receipt:     &ffcapi.TransactionReceiptResponse{},
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x12345",
				},
			},
		},
	}

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("WriteTransaction", m.ctx, mock.Anything, false).Return(fmt.Errorf("pop"))
	mp.On("Close", mock.Anything).Return(nil).Maybe()

	m.policyLoopCycle(m.ctx, false)

	mp.AssertExpectations(t)

}

func TestPolicyEngineFailStaleThenUpdated(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()
	m.policyLoopInterval = 1 * time.Hour

	mpe := &policyenginemocks.PolicyEngine{}
	m.policyEngine = mpe
	done1 := make(chan struct{})
	mpe.On("Execute", mock.Anything, mock.Anything, mock.Anything).
		Return(policyengine.UpdateNo, ffcapi.ErrorReason(""), fmt.Errorf("pop")).
		Once().
		Run(func(args mock.Arguments) {
			close(done1)
			m.markInflightUpdate()
		})

	done2 := make(chan struct{})
	mpe.On("Execute", mock.Anything, mock.Anything, mock.Anything).
		Return(policyengine.UpdateNo, ffcapi.ErrorReason(""), fmt.Errorf("pop")).
		Once().
		Run(func(args mock.Arguments) {
			close(done2)
		})

	_ = sendSampleTX(t, m, "0xaaaaa", 12345)

	m.Start()

	<-done1

	<-done2

	mpe.AssertExpectations(t)

}

func TestMarkInflightStaleDoesNotBlock(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()

	m.markInflightStale()
	m.markInflightStale()

}

func TestMarkInflightUpdateDoesNotBlock(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()

	m.markInflightUpdate()
	m.markInflightUpdate()

}

func TestExecPolicyDeleteFail(t *testing.T) {

	_, m, cancel := newTestManagerMockPersistence(t)
	defer cancel()

	mpe := &policyenginemocks.PolicyEngine{}
	m.policyEngine = mpe
	mpe.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return(policyengine.UpdateDelete, ffcapi.ErrorReason(""), nil).Maybe()

	tx := genTestTxn("0xabcd1234", 12345, apitypes.TxStatusPending)

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("GetTransactionByID", m.ctx, tx.ID).Return(tx, nil)
	mp.On("DeleteTransaction", m.ctx, tx.ID).Return(fmt.Errorf("pop"))

	req := &policyEngineAPIRequest{
		requestType: policyEngineAPIRequestTypeDelete,
		txID:        tx.ID,
		response:    make(chan policyEngineAPIResponse, 1),
	}
	m.policyEngineAPIRequests = append(m.policyEngineAPIRequests, req)

	m.processPolicyAPIRequests(m.ctx)

	res := <-req.response
	assert.Regexp(t, "pop", res.err)

	mp.AssertExpectations(t)

}

func TestExecPolicyDeleteInflightSync(t *testing.T) {

	_, m, cancel := newTestManagerMockPersistence(t)
	defer cancel()

	mpe := &policyenginemocks.PolicyEngine{}
	m.policyEngine = mpe
	mpe.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return(policyengine.UpdateDelete, ffcapi.ErrorReason(""), nil).Maybe()

	tx := genTestTxn("0xabcd1234", 12345, apitypes.TxStatusPending)
	m.inflight = []*pendingState{{mtx: tx}}

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("DeleteTransaction", m.ctx, tx.ID).Return(nil)

	req := &policyEngineAPIRequest{
		requestType: policyEngineAPIRequestTypeDelete,
		txID:        tx.ID,
		response:    make(chan policyEngineAPIResponse, 1),
	}
	m.policyEngineAPIRequests = append(m.policyEngineAPIRequests, req)

	m.processPolicyAPIRequests(m.ctx)

	res := <-req.response
	assert.NoError(t, res.err)
	assert.Equal(t, http.StatusOK, res.status)
	assert.True(t, m.inflight[0].remove)

	mp.AssertExpectations(t)

}

func TestExecPolicyDeleteNotFound(t *testing.T) {

	_, m, cancel := newTestManagerMockPersistence(t)
	defer cancel()

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("GetTransactionByID", m.ctx, "bad-id").Return(nil, nil)

	req := &policyEngineAPIRequest{
		requestType: policyEngineAPIRequestTypeDelete,
		txID:        "bad-id",
		response:    make(chan policyEngineAPIResponse, 1),
	}
	m.policyEngineAPIRequests = append(m.policyEngineAPIRequests, req)

	m.processPolicyAPIRequests(m.ctx)

	res := <-req.response
	assert.Regexp(t, "FF21067", res.err)

	mp.AssertExpectations(t)

}

func TestBadPolicyAPIRequest(t *testing.T) {

	_, m, cancel := newTestManagerMockPersistence(t)
	defer cancel()

	tx := genTestTxn("0xabcd1234", 12345, apitypes.TxStatusPending)
	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("GetTransactionByID", m.ctx, tx.ID).Return(tx, nil)

	req := &policyEngineAPIRequest{
		requestType: policyEngineAPIRequestType(999),
		txID:        tx.ID,
		response:    make(chan policyEngineAPIResponse, 1),
	}
	m.policyEngineAPIRequests = append(m.policyEngineAPIRequests, req)

	m.processPolicyAPIRequests(m.ctx)

	res := <-req.response
	assert.Regexp(t, "FF21069", res.err)

	mp.AssertExpectations(t)

}

func TestBadPolicyAPITimeout(t *testing.T) {

	_, m, cancel := newTestManagerMockPersistence(t)
	defer cancel()

	ctx, cancelCtx := context.WithCancel(context.Background())
	cancelCtx()

	res := m.policyEngineAPIRequest(ctx, &policyEngineAPIRequest{})
	assert.Regexp(t, "FF21068", res.err)

}

func TestExecPolicyUpdateNewInfo(t *testing.T) {

	_, m, cancel := newTestManagerMockPersistence(t)
	defer cancel()

	mpe := &policyenginemocks.PolicyEngine{}
	m.policyEngine = mpe
	mpe.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return(
		policyengine.UpdateNo, // Even though we return no, our update gets passed through
		ffcapi.ErrorReason(""), nil).Maybe()

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("WriteTransaction", mock.Anything, mock.Anything, false).Return(nil)
	mp.On("Close", mock.Anything).Return(nil).Maybe()

	ctx, cancelCtx := context.WithCancel(context.Background())
	cancelCtx()

	err := m.execPolicy(ctx, &pendingState{
		mtx: &apitypes.ManagedTX{
			ID: "id1",
		},
	}, false)
	assert.NoError(t, err)

}
