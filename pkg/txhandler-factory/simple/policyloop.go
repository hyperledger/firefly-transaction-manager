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
	"net/http"
	"time"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

type policyEngineAPIRequestType int

const (
	policyEngineAPIRequestTypeDelete policyEngineAPIRequestType = iota
)

type policyEngineAPIRequest struct {
	requestType policyEngineAPIRequestType
	txID        string
	startTime   time.Time
	response    chan policyEngineAPIResponse
}

// policyEngineAPIRequest requests are queued to the policy engine thread for processing against a given Transaction
type policyEngineAPIResponse struct {
	tx     *apitypes.ManagedTX
	err    error
	status int // http status code (200 Ok vs. 202 Accepted) - only set for success cases
}

func (t *simpleTransactionHandler) policyLoop() {
	defer close(t.policyLoopDone)
	ctx := log.WithLogField(t.ctx, "role", "policyloop")

	for {
		// Wait to be notified, or timeout to run
		timer := time.NewTimer(t.policyLoopInterval)
		select {
		case <-t.inflightUpdate:
		case <-timer.C:
		case <-ctx.Done():
			log.L(ctx).Infof("Receipt poller exiting")
			return
		}
		// Pop whether we were marked stale
		stale := false
		select {
		case <-t.inflightStale:
			stale = true
		default:
		}
		t.policyLoopCycle(ctx, stale)
	}
}

func (t *simpleTransactionHandler) markInflightStale() {
	// First mark that we're stale
	select {
	case t.inflightStale <- true:
	default:
	}
	// Then ensure we queue a loop that picks up the stale marker
	t.markInflightUpdate()
}

func (t *simpleTransactionHandler) markInflightUpdate() {
	select {
	case t.inflightUpdate <- true:
	default:
	}
}

func (t *simpleTransactionHandler) updateInflightSet(ctx context.Context) bool {

	oldInflight := t.inflight
	t.inflight = make([]*pendingState, 0, len(oldInflight))

	// Run through removing those that are removed
	for _, p := range oldInflight {
		if !p.remove {
			t.inflight = append(t.inflight, p)
		}
	}

	// If we are not at maximum, then query if there are more candidates now
	spaces := t.maxInFlight - len(t.inflight)
	if spaces > 0 {
		var after *fftypes.UUID
		if len(t.inflight) > 0 {
			after = t.inflight[len(t.inflight)-1].mtx.SequenceID
		}
		var additional []*apitypes.ManagedTX
		// We retry the get from persistence indefinitely (until the context cancels)
		err := t.retry.Do(ctx, "get pending transactions", func(attempt int) (retry bool, err error) {
			additional, err = t.tkAPI.Persistence.ListTransactionsPending(ctx, after, spaces, persistence.SortDirectionAscending)
			return true, err
		})
		if err != nil {
			log.L(ctx).Infof("Policy loop context cancelled while retrying")
			return false
		}
		for _, mtx := range additional {
			t.inflight = append(t.inflight, &pendingState{mtx: mtx})
		}
		newLen := len(t.inflight)
		if newLen > 0 {
			log.L(ctx).Debugf("Inflight set updated len=%d head-seq=%s tail-seq=%s old-tail=%s", len(t.inflight), t.inflight[0].mtx.SequenceID, t.inflight[newLen-1].mtx.SequenceID, after)
		}
	}
	return true

}

func (t *simpleTransactionHandler) policyLoopCycle(ctx context.Context, inflightStale bool) {

	// Process any synchronous commands first - these might not be in our inflight set
	t.processPolicyAPIRequests(ctx)

	if inflightStale {
		if !t.updateInflightSet(ctx) {
			return
		}
	}

	// Go through executing the policy engine against them
	if t.tkAPI.MetricsManager.IsMetricsEnabled() {
		t.tkAPI.MetricsManager.TransactionsInFlightSet(float64(len(t.inflight)))
	}

	for _, pending := range t.inflight {
		err := t.execPolicy(ctx, pending, false)
		if err != nil {
			log.L(ctx).Errorf("Failed policy cycle transaction=%s operation=%s: %s", pending.mtx.TransactionHash, pending.mtx.ID, err)
		}
	}

}

func (t *simpleTransactionHandler) getTransactionByID(ctx context.Context, txID string) (transaction *apitypes.ManagedTX, err error) {
	tx, err := t.tkAPI.Persistence.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, i18n.NewError(ctx, tmmsgs.MsgTransactionNotFound, txID)
	}
	return tx, nil
}

// processPolicyAPIRequests executes any API calls requested that require policy engine involvement - such as transaction deletions
func (t *simpleTransactionHandler) processPolicyAPIRequests(ctx context.Context) {

	t.mux.Lock()
	requests := t.policyEngineAPIRequests
	if len(requests) > 0 {
		t.policyEngineAPIRequests = []*policyEngineAPIRequest{}
	}
	t.mux.Unlock()

	for _, request := range requests {
		var pending *pendingState
		// If this transaction is in-flight, we use that record
		for _, inflight := range t.inflight {
			if inflight.mtx.ID == request.txID {
				pending = inflight
				break
			}
		}
		if pending == nil {
			mtx, err := t.getTransactionByID(ctx, request.txID)
			if err != nil {
				request.response <- policyEngineAPIResponse{err: err}
				continue
			}
			// This transaction was valid, but outside of our in-flight set - we still evaluate the policy engine in-line for it.
			// This does NOT cause it to be added to the in-flight set
			pending = &pendingState{mtx: mtx}
		}

		switch request.requestType {
		case policyEngineAPIRequestTypeDelete:
			if err := t.execPolicy(ctx, pending, true); err != nil {
				request.response <- policyEngineAPIResponse{err: err}
			} else {
				res := policyEngineAPIResponse{tx: pending.mtx, status: http.StatusAccepted}
				if pending.remove {
					res.status = http.StatusOK // synchronously completed
				}
				request.response <- res
			}
		default:
			request.response <- policyEngineAPIResponse{
				err: i18n.NewError(ctx, tmmsgs.MsgPolicyEngineRequestInvalid, request.requestType),
			}
		}
	}

}

func (t *simpleTransactionHandler) execPolicy(ctx context.Context, pending *pendingState, syncDeleteRequest bool) (err error) {

	update := UpdateNo
	completed := false
	var receiptProtocolID string
	var lastStatusChange *fftypes.FFTime
	currentSubStatus := t.tkAPI.TXHistory.CurrentSubStatus(ctx, pending.mtx)

	if currentSubStatus != nil {
		lastStatusChange = currentSubStatus.Time
	}

	// Check whether this has been confirmed by the confirmation manager
	t.mux.Lock()
	mtx := pending.mtx
	if mtx.Receipt != nil {
		receiptProtocolID = mtx.Receipt.ProtocolID
	} else {
		receiptProtocolID = ""
	}
	confirmed := pending.confirmed
	if syncDeleteRequest && mtx.DeleteRequested == nil {
		mtx.DeleteRequested = fftypes.Now()
	}
	t.mux.Unlock()

	var updateErr error
	var updateReason ffcapi.ErrorReason
	switch {
	case receiptProtocolID != "" && confirmed && !syncDeleteRequest:
		update = UpdateYes
		completed = true
		if pending.mtx.Receipt.Success {
			mtx.Status = apitypes.TxStatusSucceeded
		} else {
			mtx.Status = apitypes.TxStatusFailed
		}

	default:
		// We get woken for lots of reasons to go through the policy loop, but we only want
		// to drive the policy engine at regular intervals.
		// So we track the last time we ran the policy engine against each pending item.
		// We always call the policy engine on every loop, when deletion has been requested.
		if syncDeleteRequest || time.Since(pending.lastPolicyCycle) > t.policyLoopInterval {
			// Pass the state to the pluggable policy engine to potentially perform more actions against it,
			// such as submitting for the first time, or raising the gas etc.

			update, updateReason, updateErr = t.processTransaction(ctx, t.tkAPI, pending.mtx)
			if updateErr != nil {
				log.L(ctx).Errorf("Policy engine returned error for transaction %s reason=%s: %s", mtx.ID, updateReason, err)
				update = UpdateYes
				if t.tkAPI.MetricsManager.IsMetricsEnabled() {
					t.tkAPI.MetricsManager.TransactionSubmissionError()
				}
			} else {
				log.L(ctx).Debugf("Policy engine executed for tx %s (update=%d,status=%s,hash=%s)", mtx.ID, update, mtx.Status, mtx.TransactionHash)
				if mtx.FirstSubmit != nil && pending.trackingTransactionHash != mtx.TransactionHash {
					// If now submitted, add to confirmations manager for receipt checking
					t.trackSubmittedTransaction(ctx, pending)
				}
				pending.lastPolicyCycle = time.Now()
			}
		}
	}

	if t.tkAPI.TXHistory.CurrentSubStatus(ctx, mtx) != nil {
		if !t.tkAPI.TXHistory.CurrentSubStatus(ctx, mtx).Time.Equal(lastStatusChange) {
			update = UpdateYes
		}
	}

	switch update {
	case UpdateYes:
		err := t.tkAPI.Persistence.WriteTransaction(ctx, mtx, false)
		if err != nil {
			log.L(ctx).Errorf("Failed to update transaction %s (status=%s): %s", mtx.ID, mtx.Status, err)
			return err
		}
		if completed {
			pending.remove = true // for the next time round the loop
			log.L(ctx).Infof("Transaction %s marked complete (status=%s): %s", mtx.ID, mtx.Status, err)
			t.markInflightStale()
		}
		// if and only if the transaction is now resolved send web a socket update
		if mtx.Status == apitypes.TxStatusSucceeded || mtx.Status == apitypes.TxStatusFailed {
			t.sendWSReply(mtx)
		}
	case UpdateDelete:
		err := t.tkAPI.Persistence.DeleteTransaction(ctx, mtx.ID)
		if err != nil {
			log.L(ctx).Errorf("Failed to delete transaction %s (status=%s): %s", mtx.ID, mtx.Status, err)
			return err
		}
		pending.remove = true // for the next time round the loop
		t.markInflightStale()
		t.sendWSReply(mtx)
	}

	return nil
}

func (t *simpleTransactionHandler) sendWSReply(mtx *apitypes.ManagedTX) {
	wsr := &apitypes.TransactionUpdateReply{
		Headers: apitypes.ReplyHeaders{
			RequestID: mtx.ID,
		},
		Status:          mtx.Status,
		TransactionHash: mtx.TransactionHash,
	}

	if mtx.Receipt != nil && mtx.Receipt.ContractLocation != nil {
		wsr.ContractLocation = mtx.Receipt.ContractLocation
	}

	if mtx.Receipt != nil {
		wsr.ProtocolID = mtx.Receipt.ProtocolID
	} else {
		wsr.ProtocolID = ""
	}

	switch mtx.Status {
	case apitypes.TxStatusSucceeded:
		wsr.Headers.Type = apitypes.TransactionUpdateSuccess
	case apitypes.TxStatusFailed:
		wsr.Headers.Type = apitypes.TransactionUpdateFailure
	}
	// Notify on the websocket - this is best-effort (there is no subscription/acknowledgement)
	t.tkAPI.WsServer.SendReply(wsr)
}

func (t *simpleTransactionHandler) trackSubmittedTransaction(ctx context.Context, pending *pendingState) {
	var err error

	// Clear any old transaction hash
	if pending.trackingTransactionHash != "" {
		err = t.tkAPI.ConfirmationManager.Notify(&confirmations.Notification{
			NotificationType: confirmations.RemovedTransaction,
			Transaction: &confirmations.TransactionInfo{
				TransactionHash: pending.trackingTransactionHash,
			},
		})
	}

	// Notify of the new
	if err == nil {
		err = t.tkAPI.ConfirmationManager.Notify(&confirmations.Notification{
			NotificationType: confirmations.NewTransaction,
			Transaction: &confirmations.TransactionInfo{
				TransactionHash: pending.mtx.TransactionHash,
				Receipt: func(ctx context.Context, receipt *ffcapi.TransactionReceiptResponse) {
					// Will be picked up on the next policy loop cycle - guaranteed to occur before Confirmed
					t.mux.Lock()
					pending.mtx.Receipt = receipt
					t.mux.Unlock()
					log.L(t.ctx).Debugf("Receipt received for transaction %s at nonce %s / %d - hash: %s", pending.mtx.ID, pending.mtx.TransactionHeaders.From, pending.mtx.Nonce.Int64(), pending.mtx.TransactionHash)
					t.tkAPI.TXHistory.AddSubStatusAction(ctx, pending.mtx, apitypes.TxActionReceiveReceipt, fftypes.JSONAnyPtr(`{"protocolId":"`+receipt.ProtocolID+`"}`), nil)
					t.markInflightUpdate()
				},
				Confirmed: func(ctx context.Context, confirmations []confirmations.BlockInfo) {
					// Will be picked up on the next policy loop cycle
					t.mux.Lock()
					pending.confirmed = true
					pending.mtx.Confirmations = confirmations
					t.mux.Unlock()
					log.L(t.ctx).Debugf("Confirmed transaction %s at nonce %s / %d - hash: %s", pending.mtx.ID, pending.mtx.TransactionHeaders.From, pending.mtx.Nonce.Int64(), pending.mtx.TransactionHash)
					t.tkAPI.TXHistory.AddSubStatusAction(ctx, pending.mtx, apitypes.TxActionConfirmTransaction, nil, nil)
					t.tkAPI.TXHistory.SetSubStatus(ctx, pending.mtx, apitypes.TxSubStatusConfirmed)
					t.markInflightUpdate()
				},
			},
		})
	}

	// Only reason for error here should be a cancelled context
	if err != nil {
		log.L(ctx).Infof("Error detected notifying confirmation manager: %s", err)
	} else {
		pending.trackingTransactionHash = pending.mtx.TransactionHash
	}
}

func (t *simpleTransactionHandler) policyEngineAPIRequest(ctx context.Context, req *policyEngineAPIRequest) policyEngineAPIResponse {
	t.mux.Lock()
	t.policyEngineAPIRequests = append(t.policyEngineAPIRequests, req)
	t.mux.Unlock()
	t.markInflightUpdate()
	req.response = make(chan policyEngineAPIResponse, 1)
	req.startTime = time.Now()
	select {
	case res := <-req.response:
		return res
	case <-ctx.Done():
		return policyEngineAPIResponse{
			err: i18n.NewError(ctx, tmmsgs.MsgPolicyEngineRequestTimeout, time.Since(req.startTime).Seconds()),
		}
	}
}
