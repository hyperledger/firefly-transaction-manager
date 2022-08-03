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
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/persistencemocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/policyenginemocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
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
		n.Transaction.Receipt(&ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			Success:          true,
		})
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil)

	// Run the policy once to do the send
	<-m.inflightStale // from sending the TX
	m.policyLoopCycle(true)
	assert.Equal(t, mtx.ID, m.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, m.inflight[0].mtx.Status)

	// A second time will mark it complete for flush
	m.policyLoopCycle(false)

	<-m.inflightStale // policy loop should have marked us stale, to clean up the TX
	m.policyLoopCycle(true)
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
		n.Transaction.Receipt(&ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			Success:          false,
		})
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil)

	// Run the policy once to do the send
	<-m.inflightStale // from sending the TX
	m.policyLoopCycle(true)
	assert.Equal(t, mtx.ID, m.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, m.inflight[0].mtx.Status)

	// A second time will mark it complete for flush
	m.policyLoopCycle(false)

	<-m.inflightStale // policy loop should have marked us stale, to clean up the TX
	m.policyLoopCycle(true)
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
		n.Transaction.Receipt(&ffcapi.TransactionReceiptResponse{
			BlockNumber:      fftypes.NewFFBigInt(12345),
			TransactionIndex: fftypes.NewFFBigInt(10),
			BlockHash:        fftypes.NewRandB32().String(),
			Success:          true,
		})
		n.Transaction.Confirmed([]confirmations.BlockInfo{})
	}).Return(nil)

	// Run the policy once to do the send with the first hash
	<-m.inflightStale // from sending the TX
	m.policyLoopCycle(true)
	assert.Len(t, m.inflight, 1)
	assert.Equal(t, mtx.ID, m.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, m.inflight[0].mtx.Status)

	// Run again to confirm it does not change anything, when the state is the same
	m.policyLoopCycle(true)
	assert.Len(t, m.inflight, 1)
	assert.Equal(t, mtx.ID, m.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, m.inflight[0].mtx.Status)

	// Reset the transaction so the policy manager resubmits it
	m.inflight[0].mtx.FirstSubmit = nil
	m.policyLoopCycle(false)
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

	m.policyLoopCycle(true)

	mc.AssertExpectations(t)
	mfc.AssertExpectations(t)

}

func TestInflightSetListFailCancel(t *testing.T) {

	_, m, cancel := newTestManager(t)
	cancel()

	mp := &persistencemocks.Persistence{}
	m.persistence = mp
	mp.On("ListTransactionsPending", m.ctx, (*fftypes.UUID)(nil), m.maxInFlight, persistence.SortDirectionAscending).
		Return(nil, fmt.Errorf("pop"))
	mp.On("Close", mock.Anything).Return(nil).Maybe()

	m.policyLoopCycle(true)

	mp.AssertExpectations(t)

}

func TestPolicyLoopUpdateFail(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()

	m.inflight = []*pendingState{
		{
			confirmed: true,
			mtx: &apitypes.ManagedTX{
				ID:          fmt.Sprintf("ns1/%s", fftypes.NewUUID()),
				Created:     fftypes.Now(),
				SequenceID:  apitypes.UUIDVersion1(),
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

	mp := &persistencemocks.Persistence{}
	m.persistence = mp
	mp.On("WriteTransaction", m.ctx, mock.Anything, false).Return(fmt.Errorf("pop"))
	mp.On("Close", mock.Anything).Return(nil).Maybe()

	m.policyLoopCycle(false)

	mp.AssertExpectations(t)

}

func TestPolicyEngineFail(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()

	mpe := &policyenginemocks.PolicyEngine{}
	m.policyEngine = mpe
	mpe.On("Execute", mock.Anything, mock.Anything, mock.Anything).
		Return(false, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	_ = sendSampleTX(t, m, "0xaaaaa", 12345)

	m.policyLoopCycle(true)

	mpe.AssertExpectations(t)

}

func TestMarkInflightStaleDoesNotBlock(t *testing.T) {

	_, m, cancel := newTestManager(t)
	defer cancel()

	m.markInflightStale()
	m.markInflightStale()

}
