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

package simple

import (
	"context"
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/wsmocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func sendSampleTX(t *testing.T, sth *simpleTransactionHandler, signer string, nonce int64) *apitypes.ManagedTX {

	txInput := ffcapi.TransactionInput{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: signer,
		},
	}

	mfc := sth.tkAPI.Connector.(*ffcapimocks.API)
	mfc.On("NextNonceForSigner", sth.ctx, &ffcapi.NextNonceForSignerRequest{
		Signer: signer,
	}).Return(&ffcapi.NextNonceForSignerResponse{
		Nonce: fftypes.NewFFBigInt(nonce),
	}, ffcapi.ErrorReason(""), nil).Once()
	mfc.On("TransactionPrepare", sth.ctx, &ffcapi.TransactionPrepareRequest{
		TransactionInput: txInput,
	}).Return(&ffcapi.TransactionPrepareResponse{
		Gas:             fftypes.NewFFBigInt(100000),
		TransactionData: "0xabce1234",
	}, ffcapi.ErrorReason(""), nil).Once()

	mtx, err := sth.RegisterNewTransaction(sth.ctx, &apitypes.TransactionRequest{
		TransactionInput: txInput,
	})
	assert.NoError(t, err)
	return mtx
}

func TestPolicyLoopE2EOk(t *testing.T) {

	f, tk, _, _, conf, cleanup := newTestTransactionHandlerFactoryWithFilePersistence(t)
	defer cleanup()
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	th, err := f.NewTransactionHandler(context.Background(), conf)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	sth.Init(sth.ctx, tk)
	txHash := "0x" + fftypes.NewRandB32().String()

	mfc := sth.tkAPI.Connector.(*ffcapimocks.API)
	mfc.On("TransactionSend", sth.ctx, mock.MatchedBy(func(r *ffcapi.TransactionSendRequest) bool {
		return r.Nonce.Equals(fftypes.NewFFBigInt(12345))
	})).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: txHash,
	}, ffcapi.ErrorReason(""), nil)

	eh := &fftm.ManagedTransactionEventHandler{
		Ctx:       context.Background(),
		TxHandler: sth,
	}

	mc := &confirmationsmocks.Manager{}
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
			ContractLocation: fftypes.JSONAnyPtr(`{"address": "0x24746b95d118b2b4e8d07b06b1bad988fbf9415d"}`),
		})
		n.Transaction.Confirmed(context.Background(), []confirmations.BlockInfo{})
	}).Return(nil)
	eh.ConfirmationManager = mc
	mws := &wsmocks.WebSocketServer{}
	mws.On("SendReply", mock.Anything).Return(nil).Maybe()

	eh.WsServer = mws
	sth.transactionEventHandler = eh

	mtx := sendSampleTX(t, sth, "0xaaaaa", 12345)
	// Run the policy once to do the send
	<-sth.inflightStale // from sending the TX
	sth.policyLoopCycle(sth.ctx, true)
	assert.Equal(t, mtx.ID, sth.inflight[0].mtx.ID)
	assert.Equal(t, apitypes.TxStatusPending, sth.inflight[0].mtx.Status)

	// A second time will mark it complete for flush
	sth.policyLoopCycle(sth.ctx, false)

	<-sth.inflightStale // policy loop should have marked us stale, to clean up the TX
	sth.policyLoopCycle(sth.ctx, true)
	assert.Empty(t, sth.inflight)

	// Check the update is persisted
	rtx, err := sth.tkAPI.Persistence.GetTransactionByID(sth.ctx, mtx.ID)
	assert.NoError(t, err)
	assert.Equal(t, apitypes.TxStatusSucceeded, rtx.Status)

	mc.AssertExpectations(t)
	mfc.AssertExpectations(t)
}

// func TestPolicyLoopE2EReverted(t *testing.T) {

// 	_, m, cancel := newTestManager(t)
// 	defer cancel()

// 	mtx := sendSampleTX(t, m, "0xaaaaa", 12345)
// 	txHash := "0x" + fftypes.NewRandB32().String()

// 	mfc := sth.connector.(*ffcapimocks.API)
// 	mfc.On("TransactionSend", sth.ctx, mock.MatchedBy(func(r *ffcapi.TransactionSendRequest) bool {
// 		return r.Nonce.Equals(fftypes.NewFFBigInt(12345))
// 	})).Return(&ffcapi.TransactionSendResponse{
// 		TransactionHash: txHash,
// 	}, ffcapi.ErrorReason(""), nil)

// 	mc := sth.confirmations.(*confirmationsmocks.Manager)
// 	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
// 		return n.NotificationType == confirmations.NewTransaction
// 	})).Run(func(args mock.Arguments) {
// 		n := args[0].(*confirmations.Notification)
// 		n.Transaction.Receipt(context.Background(), &ffcapi.TransactionReceiptResponse{
// 			BlockNumber:      fftypes.NewFFBigInt(12345),
// 			TransactionIndex: fftypes.NewFFBigInt(10),
// 			BlockHash:        fftypes.NewRandB32().String(),
// 			ProtocolID:       fmt.Sprintf("%.12d/%.6d", fftypes.NewFFBigInt(12345).Int64(), fftypes.NewFFBigInt(10).Int64()),
// 			Success:          false,
// 		})
// 		n.Transaction.Confirmed(context.Background(), []confirmations.BlockInfo{})
// 	}).Return(nil)

// 	// Run the policy once to do the send
// 	<-sth.inflightStale // from sending the TX
// 	sth.policyLoopCycle(sth.ctx, true)
// 	assert.Equal(t, mtx.ID, sth.inflight[0].mtx.ID)
// 	assert.Equal(t, apitypes.TxStatusPending, sth.inflight[0].mtx.Status)

// 	// A second time will mark it complete for flush
// 	sth.policyLoopCycle(sth.ctx, false)

// 	<-sth.inflightStale // policy loop should have marked us stale, to clean up the TX
// 	sth.policyLoopCycle(sth.ctx, true)
// 	assert.Empty(t, sth.inflight)

// 	// Check the update is persisted
// 	rtx, err := sth.persistence.GetTransactionByID(sth.ctx, mtx.ID)
// 	assert.NoError(t, err)
// 	assert.Equal(t, apitypes.TxStatusFailed, rtx.Status)

// 	mc.AssertExpectations(t)
// 	mfc.AssertExpectations(t)
// }

// func TestPolicyLoopResubmitNewTXID(t *testing.T) {

// 	_, m, cancel := newTestManager(t)
// 	defer cancel()

// 	mtx := sendSampleTX(t, m, "0xaaaaa", 12345)
// 	txHash1 := "0x" + fftypes.NewRandB32().String()
// 	txHash2 := "0x" + fftypes.NewRandB32().String()

// 	mfc := sth.connector.(*ffcapimocks.API)

// 	mfc.On("TransactionPrepare", mock.Anything, mock.Anything).Return(&ffcapi.TransactionPrepareResponse{
// 		Gas:             fftypes.NewFFBigInt(12345),
// 		TransactionData: "0x12345",
// 	}, ffcapi.ErrorReason(""), nil)

// 	mfc.On("TransactionSend", mock.Anything, mock.Anything).Return(&ffcapi.TransactionSendResponse{
// 		TransactionHash: txHash1,
// 	}, ffcapi.ErrorReason(""), nil).Once()
// 	mfc.On("TransactionSend", mock.Anything, mock.Anything).Return(&ffcapi.TransactionSendResponse{
// 		TransactionHash: txHash2,
// 	}, ffcapi.ErrorReason(""), nil).Once()

// 	mc := sth.confirmations.(*confirmationsmocks.Manager)
// 	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
// 		// First we get notified to add the old TX hash
// 		return n.NotificationType == confirmations.NewTransaction &&
// 			n.Transaction.TransactionHash == txHash1
// 	})).Return(nil)
// 	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
// 		// Then we get notified to remove the old TX hash
// 		return n.NotificationType == confirmations.RemovedTransaction &&
// 			n.Transaction.TransactionHash == txHash1
// 	})).Return(nil)
// 	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
// 		// Then we get the new TX hash, which we confirm
// 		return n.NotificationType == confirmations.NewTransaction &&
// 			n.Transaction.TransactionHash == txHash2
// 	})).Run(func(args mock.Arguments) {
// 		n := args[0].(*confirmations.Notification)
// 		n.Transaction.Receipt(context.Background(), &ffcapi.TransactionReceiptResponse{
// 			BlockNumber:      fftypes.NewFFBigInt(12345),
// 			TransactionIndex: fftypes.NewFFBigInt(10),
// 			BlockHash:        fftypes.NewRandB32().String(),
// 			ProtocolID:       fmt.Sprintf("%.12d/%.6d", fftypes.NewFFBigInt(12345).Int64(), fftypes.NewFFBigInt(10).Int64()),
// 			Success:          true,
// 		})
// 		n.Transaction.Confirmed(context.Background(), []confirmations.BlockInfo{})
// 	}).Return(nil)

// 	// Run the policy once to do the send with the first hash
// 	<-sth.inflightStale // from sending the TX
// 	sth.policyLoopCycle(sth.ctx, true)
// 	assert.Len(t, sth.inflight, 1)
// 	assert.Equal(t, mtx.ID, sth.inflight[0].mtx.ID)
// 	assert.Equal(t, apitypes.TxStatusPending, sth.inflight[0].mtx.Status)

// 	// Run again to confirm it does not change anything, when the state is the same
// 	sth.policyLoopCycle(sth.ctx, true)
// 	assert.Len(t, sth.inflight, 1)
// 	assert.Equal(t, mtx.ID, sth.inflight[0].mtx.ID)
// 	assert.Equal(t, apitypes.TxStatusPending, sth.inflight[0].mtx.Status)

// 	// Reset the transaction so the policy manager resubmits it
// 	sth.inflight[0].mtx.FirstSubmit = nil
// 	sth.policyLoopCycle(sth.ctx, false)
// 	assert.Equal(t, mtx.ID, sth.inflight[0].mtx.ID)
// 	assert.Equal(t, apitypes.TxStatusPending, sth.inflight[0].mtx.Status)

// 	mc.AssertExpectations(t)
// 	mfc.AssertExpectations(t)
// }

// func TestNotifyConfirmationMgrFail(t *testing.T) {

// 	_, m, cancel := newTestManager(t)
// 	defer cancel()

// 	_ = sendSampleTX(t, m, "0xaaaaa", 12345)
// 	txHash := "0x" + fftypes.NewRandB32().String()

// 	mfc := sth.connector.(*ffcapimocks.API)
// 	mfc.On("TransactionSend", mock.Anything, mock.Anything).Return(&ffcapi.TransactionSendResponse{
// 		TransactionHash: txHash,
// 	}, ffcapi.ErrorReason(""), nil).Once()

// 	mc := sth.confirmations.(*confirmationsmocks.Manager)
// 	mc.On("Notify", mock.Anything).Return(fmt.Errorf("pop"))

// 	sth.policyLoopCycle(sth.ctx, true)

// 	mc.AssertExpectations(t)
// 	mfc.AssertExpectations(t)

// }

// func TestInflightSetListFailCancel(t *testing.T) {

// 	_, m, close := newTestManagerMockPersistence(t)
// 	close()

// 	mp := sth.persistence.(*persistencemocks.Persistence)
// 	mp.On("ListTransactionsPending", sth.ctx, (*fftypes.UUID)(nil), sth.maxInFlight, persistence.SortDirectionAscending).
// 		Return(nil, fmt.Errorf("pop"))

// 	sth.policyLoopCycle(sth.ctx, true)

// 	mp.AssertExpectations(t)

// }

// func TestPolicyLoopUpdateFail(t *testing.T) {

// 	_, m, close := newTestManagerMockPersistence(t)
// 	defer close()

// 	sth.inflight = []*pendingState{
// 		{
// 			confirmed: true,
// 			mtx: &apitypes.ManagedTX{
// 				ID:          fmt.Sprintf("ns1/%s", fftypes.NewUUID()),
// 				Created:     fftypes.Now(),
// 				SequenceID:  apitypes.NewULID(),
// 				Nonce:       fftypes.NewFFBigInt(1000),
// 				Status:      apitypes.TxStatusSucceeded,
// 				FirstSubmit: nil,
// 				Receipt:     &ffcapi.TransactionReceiptResponse{},
// 				TransactionHeaders: ffcapi.TransactionHeaders{
// 					From: "0x12345",
// 				},
// 			},
// 		},
// 	}

// 	h := txhistory.NewTxHistoryManager(sth.ctx)
// 	h.SetSubStatus(sth.ctx, sth.inflight[0].mtx, apitypes.TxSubStatusReceived)
// 	mth := &txhandlermocks.TransactionHandler{}
// 	sth.txHandler = mth
// 	mth.On("execute", mock.Anything, mock.Anything, mock.Anything).
// 		Return(txhandler.UpdateYes, ffcapi.ErrorReason(""), nil)

// 	mp := sth.persistence.(*persistencemocks.Persistence)
// 	mp.On("WriteTransaction", sth.ctx, mock.Anything, false).Return(fmt.Errorf("pop"))
// 	mp.On("Close", mock.Anything).Return(nil).Maybe()

// 	sth.policyLoopCycle(sth.ctx, false)

// 	mp.AssertExpectations(t)

// }

// func TestPolicyEngineFailStaleThenUpdated(t *testing.T) {

// 	_, m, cancel := newTestManagerWithMetrics(t)
// 	defer cancel()
// 	sth.policyLoopInterval = 1 * time.Hour

// 	mth := &txhandlermocks.TransactionHandler{}
// 	sth.txHandler = mth
// 	done1 := make(chan struct{})
// 	mth.On("execute", mock.Anything, mock.Anything, mock.Anything).
// 		Return(txhandler.UpdateNo, ffcapi.ErrorReason(""), fmt.Errorf("pop")).
// 		Once().
// 		Run(func(args mock.Arguments) {
// 			close(done1)
// 			sth.markInflightUpdate()
// 		})

// 	done2 := make(chan struct{})
// 	mth.On("execute", mock.Anything, mock.Anything, mock.Anything).
// 		Return(txhandler.UpdateNo, ffcapi.ErrorReason(""), fmt.Errorf("pop")).
// 		Once().
// 		Run(func(args mock.Arguments) {
// 			close(done2)
// 		})

// 	_ = sendSampleTX(t, m, "0xaaaaa", 12345)

// 	sth.Start()

// 	<-done1

// 	<-done2

// 	sth.Close()

// 	mth.AssertExpectations(t)

// }

// func TestMarkInflightStaleDoesNotBlock(t *testing.T) {

// 	_, m, cancel := newTestManager(t)
// 	defer cancel()

// 	sth.markInflightStale()
// 	sth.markInflightStale()

// }

// func TestMarkInflightUpdateDoesNotBlock(t *testing.T) {

// 	_, m, cancel := newTestManager(t)
// 	defer cancel()

// 	sth.markInflightUpdate()
// 	sth.markInflightUpdate()

// }

// func TestExecPolicyDeleteFail(t *testing.T) {

// 	_, m, cancel := newTestManagerMockPersistence(t)
// 	defer cancel()

// 	mth := &txhandlermocks.TransactionHandler{}
// 	sth.txHandler = mth
// 	mth.On("execute", mock.Anything, mock.Anything, mock.Anything).Return(txhandler.UpdateDelete, ffcapi.ErrorReason(""), nil).Maybe()

// 	tx := genTestTxn("0xabcd1234", 12345, apitypes.TxStatusPending)

// 	mp := sth.persistence.(*persistencemocks.Persistence)
// 	mp.On("GetTransactionByID", sth.ctx, tx.ID).Return(tx, nil)
// 	mp.On("DeleteTransaction", sth.ctx, tx.ID).Return(fmt.Errorf("pop"))

// 	req := &policyEngineAPIRequest{
// 		requestType: policyEngineAPIRequestTypeDelete,
// 		txID:        tx.ID,
// 		response:    make(chan policyEngineAPIResponse, 1),
// 	}
// 	sth.policyEngineAPIRequests = append(sth.policyEngineAPIRequests, req)

// 	sth.processPolicyAPIRequests(sth.ctx)

// 	res := <-req.response
// 	assert.Regexp(t, "pop", res.err)

// 	mp.AssertExpectations(t)

// }

// func TestExecPolicyDeleteInflightSync(t *testing.T) {

// 	_, m, cancel := newTestManagerMockPersistence(t)
// 	defer cancel()

// 	mth := &txhandlermocks.TransactionHandler{}
// 	sth.txHandler = mth
// 	mth.On("execute", mock.Anything, mock.Anything, mock.Anything).Return(txhandler.UpdateDelete, ffcapi.ErrorReason(""), nil).Maybe()

// 	tx := genTestTxn("0xabcd1234", 12345, apitypes.TxStatusPending)
// 	sth.inflight = []*pendingState{{mtx: tx}}

// 	mp := sth.persistence.(*persistencemocks.Persistence)
// 	mp.On("DeleteTransaction", sth.ctx, tx.ID).Return(nil)

// 	req := &policyEngineAPIRequest{
// 		requestType: policyEngineAPIRequestTypeDelete,
// 		txID:        tx.ID,
// 		response:    make(chan policyEngineAPIResponse, 1),
// 	}
// 	sth.policyEngineAPIRequests = append(sth.policyEngineAPIRequests, req)

// 	sth.processPolicyAPIRequests(sth.ctx)

// 	res := <-req.response
// 	assert.NoError(t, res.err)
// 	assert.Equal(t, http.StatusOK, res.status)
// 	assert.True(t, sth.inflight[0].remove)

// 	mp.AssertExpectations(t)

// }

// func TestExecPolicyDeleteNotFound(t *testing.T) {

// 	_, m, cancel := newTestManagerMockPersistence(t)
// 	defer cancel()

// 	mp := sth.persistence.(*persistencemocks.Persistence)
// 	mp.On("GetTransactionByID", sth.ctx, "bad-id").Return(nil, nil)

// 	req := &policyEngineAPIRequest{
// 		requestType: policyEngineAPIRequestTypeDelete,
// 		txID:        "bad-id",
// 		response:    make(chan policyEngineAPIResponse, 1),
// 	}
// 	sth.policyEngineAPIRequests = append(sth.policyEngineAPIRequests, req)

// 	sth.processPolicyAPIRequests(sth.ctx)

// 	res := <-req.response
// 	assert.Regexp(t, "FF21067", res.err)

// 	mp.AssertExpectations(t)

// }

// func TestBadPolicyAPIRequest(t *testing.T) {

// 	_, m, cancel := newTestManagerMockPersistence(t)
// 	defer cancel()

// 	tx := genTestTxn("0xabcd1234", 12345, apitypes.TxStatusPending)
// 	mp := sth.persistence.(*persistencemocks.Persistence)
// 	mp.On("GetTransactionByID", sth.ctx, tx.ID).Return(tx, nil)

// 	req := &policyEngineAPIRequest{
// 		requestType: policyEngineAPIRequestType(999),
// 		txID:        tx.ID,
// 		response:    make(chan policyEngineAPIResponse, 1),
// 	}
// 	sth.policyEngineAPIRequests = append(sth.policyEngineAPIRequests, req)

// 	sth.processPolicyAPIRequests(sth.ctx)

// 	res := <-req.response
// 	assert.Regexp(t, "FF21069", res.err)

// 	mp.AssertExpectations(t)

// }

// func TestBadPolicyAPITimeout(t *testing.T) {

// 	_, m, cancel := newTestManagerMockPersistence(t)
// 	defer cancel()

// 	ctx, cancelCtx := context.WithCancel(context.Background())
// 	cancelCtx()

// 	res := sth.policyEngineAPIRequest(ctx, &policyEngineAPIRequest{})
// 	assert.Regexp(t, "FF21068", res.err)

// }

// func TestExecPolicyUpdateNewInfo(t *testing.T) {

// 	_, m, cancel := newTestManagerMockPersistence(t)
// 	defer cancel()

// 	mth := &txhandlermocks.TransactionHandler{}
// 	sth.txHandler = mth
// 	mth.On("execute", mock.Anything, mock.Anything, mock.Anything).Return(
// 		txhandler.UpdateNo, // Even though we return no, our update gets passed through
// 		ffcapi.ErrorReason(""), nil).Maybe()

// 	mp := sth.persistence.(*persistencemocks.Persistence)
// 	mp.On("WriteTransaction", mock.Anything, mock.Anything, false).Return(nil)
// 	mp.On("Close", mock.Anything).Return(nil).Maybe()

// 	ctx, cancelCtx := context.WithCancel(context.Background())
// 	cancelCtx()

// 	err := sth.execPolicy(ctx, &pendingState{
// 		mtx: &apitypes.ManagedTX{
// 			ID: "id1",
// 		},
// 	}, false)
// 	assert.NoError(t, err)

// }
