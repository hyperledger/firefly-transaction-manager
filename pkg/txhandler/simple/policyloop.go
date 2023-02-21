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
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/toolkit"
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

func (sth *simpleTransactionHandler) policyLoop() {
	defer close(sth.policyLoopDone)
	ctx := log.WithLogField(sth.ctx, "role", "policyloop")

	for {
		// Wait to be notified, or timeout to run
		timer := time.NewTimer(sth.policyLoopInterval)
		select {
		case <-sth.inflightUpdate:
		case <-timer.C:
		case <-ctx.Done():
			log.L(ctx).Infof("Receipt poller exiting")
			return
		}
		// Pop whether we were marked stale
		stale := false
		select {
		case <-sth.inflightStale:
			stale = true
		default:
		}
		sth.policyLoopCycle(ctx, stale)
	}
}

func (sth *simpleTransactionHandler) markInflightStale() {
	// First mark that we're stale
	select {
	case sth.inflightStale <- true:
	default:
	}
	// Then ensure we queue a loop that picks up the stale marker
	sth.markInflightUpdate()
}

func (sth *simpleTransactionHandler) markInflightUpdate() {
	select {
	case sth.inflightUpdate <- true:
	default:
	}
}

func (sth *simpleTransactionHandler) updateInflightSet(ctx context.Context) bool {

	oldInflight := sth.inflight
	sth.inflight = make([]*pendingState, 0, len(oldInflight))

	// Run through removing those that are removed
	for _, p := range oldInflight {
		if !p.remove {
			sth.inflight = append(sth.inflight, p)
		}
	}

	// If we are not at maximum, then query if there are more candidates now
	spaces := sth.maxInFlight - len(sth.inflight)
	if spaces > 0 {
		var after *fftypes.UUID
		if len(sth.inflight) > 0 {
			after = sth.inflight[len(sth.inflight)-1].mtx.SequenceID
		}
		var additional []*apitypes.ManagedTX
		// We retry the get from persistence indefinitely (until the context cancels)
		err := sth.retry.Do(ctx, "get pending transactions", func(attempt int) (retry bool, err error) {
			additional, err = sth.toolkit.Persistence.ListTransactionsPending(ctx, after, spaces, toolkit.SortDirectionAscending)
			return true, err
		})
		if err != nil {
			log.L(ctx).Infof("Policy loop context cancelled while retrying")
			return false
		}
		for _, mtx := range additional {
			sth.inflight = append(sth.inflight, &pendingState{mtx: mtx})
		}
		newLen := len(sth.inflight)
		if newLen > 0 {
			log.L(ctx).Debugf("Inflight set updated len=%d head-seq=%s tail-seq=%s old-tail=%s", len(sth.inflight), sth.inflight[0].mtx.SequenceID, sth.inflight[newLen-1].mtx.SequenceID, after)
		}
	}
	return true

}

func (sth *simpleTransactionHandler) policyLoopCycle(ctx context.Context, inflightStale bool) {

	// Process any synchronous commands first - these might not be in our inflight set
	sth.processPolicyAPIRequests(ctx)

	if inflightStale {
		if !sth.updateInflightSet(ctx) {
			return
		}
	}

	// Go through executing the policy engine against them
	if sth.toolkit.MetricsManager.IsMetricsEnabled() {
		sth.toolkit.MetricsManager.TransactionsInFlightSet(float64(len(sth.inflight)))
	}

	for _, pending := range sth.inflight {
		err := sth.execPolicy(ctx, pending, false)
		if err != nil {
			log.L(ctx).Errorf("Failed policy cycle transaction=%s operation=%s: %s", pending.mtx.TransactionHash, pending.mtx.ID, err)
		}
	}

}

func (sth *simpleTransactionHandler) getTransactionByID(ctx context.Context, txID string) (transaction *apitypes.ManagedTX, err error) {
	tx, err := sth.toolkit.Persistence.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, i18n.NewError(ctx, tmmsgs.MsgTransactionNotFound, txID)
	}
	return tx, nil
}

// processPolicyAPIRequests executes any API calls requested that require policy engine involvement - such as transaction deletions
func (sth *simpleTransactionHandler) processPolicyAPIRequests(ctx context.Context) {

	sth.mux.Lock()
	requests := sth.policyEngineAPIRequests
	if len(requests) > 0 {
		sth.policyEngineAPIRequests = []*policyEngineAPIRequest{}
	}
	sth.mux.Unlock()

	for _, request := range requests {
		var pending *pendingState
		// If this transaction is in-flight, we use that record
		for _, inflight := range sth.inflight {
			if inflight.mtx.ID == request.txID {
				pending = inflight
				break
			}
		}
		if pending == nil {
			mtx, err := sth.getTransactionByID(ctx, request.txID)
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
			if err := sth.execPolicy(ctx, pending, true); err != nil {
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
				err: i18n.NewError(ctx, tmmsgs.MsgTransactionHandlerRequestInvalid, request.requestType),
			}
		}
	}

}

func (sth *simpleTransactionHandler) execPolicy(ctx context.Context, pending *pendingState, syncDeleteRequest bool) (err error) {

	update := UpdateNo
	completed := false
	var receiptProtocolID string
	var lastStatusChange *fftypes.FFTime
	currentSubStatus := sth.toolkit.TXHistory.CurrentSubStatus(ctx, pending.mtx)

	if currentSubStatus != nil {
		lastStatusChange = currentSubStatus.Time
	}

	// Check whether this has been confirmed by the confirmation manager
	sth.mux.Lock()
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
	sth.mux.Unlock()

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
		if syncDeleteRequest || time.Since(pending.lastPolicyCycle) > sth.policyLoopInterval {
			// Pass the state to the pluggable policy engine to potentially perform more actions against it,
			// such as submitting for the first time, or raising the gas etc.

			update, updateReason, updateErr = sth.processTransaction(ctx, pending.mtx)
			if updateErr != nil {
				log.L(ctx).Errorf("Policy engine returned error for transaction %s reason=%s: %s", mtx.ID, updateReason, err)
				update = UpdateYes
				if sth.toolkit.MetricsManager.IsMetricsEnabled() {
					sth.toolkit.MetricsManager.TransactionSubmissionError()
				}
			} else {
				log.L(ctx).Debugf("Policy engine executed for tx %s (update=%d,status=%s,hash=%s)", mtx.ID, update, mtx.Status, mtx.TransactionHash)
				if mtx.FirstSubmit != nil &&
					pending.trackingTransactionHash != mtx.TransactionHash {

					if pending.trackingTransactionHash != "" {
						// if had a previous transaction hash, emit an event to for transaction hash removal
						if err = sth.toolkit.EventHandler.HandleEvent(ctx, apitypes.ManagedTransactionEvent{
							Type: apitypes.ManagedTXTransactionHashRemoved,
							Tx: &apitypes.ManagedTX{
								ID:                 mtx.ID,
								Created:            mtx.Created,
								Updated:            mtx.Updated,
								Status:             mtx.Status,
								DeleteRequested:    mtx.DeleteRequested,
								SequenceID:         mtx.SequenceID,
								Nonce:              mtx.Nonce,
								Gas:                mtx.Gas,
								TransactionHeaders: mtx.TransactionHeaders,
								TransactionData:    mtx.TransactionData,
								TransactionHash:    pending.trackingTransactionHash, // using the previous hash
								GasPrice:           mtx.GasPrice,
								PolicyInfo:         mtx.PolicyInfo,
								FirstSubmit:        mtx.FirstSubmit,
								LastSubmit:         mtx.LastSubmit,
								Receipt:            mtx.Receipt,
								ErrorMessage:       mtx.ErrorMessage,
								Confirmations:      mtx.Confirmations,
								History:            mtx.History,
								HistorySummary:     mtx.HistorySummary,
							},
						}); err != nil {
							log.L(ctx).Infof("Error detected notifying confirmation manager to remove old transaction hash: %s", err.Error())
						}
					}

					// If now submitted, add to confirmations manager for receipt checking
					err = sth.toolkit.EventHandler.HandleEvent(ctx, apitypes.ManagedTransactionEvent{
						Type: apitypes.ManagedTXTransactionHashAdded,
						Tx:   mtx,
					})
					if err != nil {
						log.L(ctx).Infof("Error detected notifying confirmation manager to add new transaction hash: %s", err.Error())
					} else {
						pending.trackingTransactionHash = mtx.TransactionHash
					}
				}
				pending.lastPolicyCycle = time.Now()
			}
		}
	}

	if sth.toolkit.TXHistory.CurrentSubStatus(ctx, mtx) != nil {
		if !sth.toolkit.TXHistory.CurrentSubStatus(ctx, mtx).Time.Equal(lastStatusChange) {
			update = UpdateYes
		}
	}

	switch update {
	case UpdateYes:
		err := sth.toolkit.Persistence.WriteTransaction(ctx, mtx, false)
		if err != nil {
			log.L(ctx).Errorf("Failed to update transaction %s (status=%s): %s", mtx.ID, mtx.Status, err)
			return err
		}
		if completed {
			pending.remove = true // for the next time round the loop
			log.L(ctx).Infof("Transaction %s marked complete (status=%s): %s", mtx.ID, mtx.Status, err)
			sth.markInflightStale()
		}
		// if and only if the transaction is now resolved dispatch an event to event handler
		// and discard any handling errors
		if mtx.Status == apitypes.TxStatusSucceeded {
			_ = sth.toolkit.EventHandler.HandleEvent(ctx, apitypes.ManagedTransactionEvent{
				Type: apitypes.ManagedTXProcessSucceeded,
				Tx:   mtx,
			})
		} else if mtx.Status == apitypes.TxStatusFailed {
			_ = sth.toolkit.EventHandler.HandleEvent(ctx, apitypes.ManagedTransactionEvent{
				Type: apitypes.ManagedTXProcessFailed,
				Tx:   mtx,
			})
		}
	case UpdateDelete:
		err := sth.toolkit.Persistence.DeleteTransaction(ctx, mtx.ID)
		if err != nil {
			log.L(ctx).Errorf("Failed to delete transaction %s (status=%s): %s", mtx.ID, mtx.Status, err)
			return err
		}
		pending.remove = true // for the next time round the loop
		sth.markInflightStale()
		// dispatch an event to event handler
		// and discard any handling errors
		_ = sth.toolkit.EventHandler.HandleEvent(ctx, apitypes.ManagedTransactionEvent{
			Type: apitypes.ManagedTXDeleted,
			Tx:   mtx,
		})
	}

	return nil
}

func (sth *simpleTransactionHandler) policyEngineAPIRequest(ctx context.Context, req *policyEngineAPIRequest) policyEngineAPIResponse {
	sth.mux.Lock()
	sth.policyEngineAPIRequests = append(sth.policyEngineAPIRequests, req)
	sth.mux.Unlock()
	sth.markInflightUpdate()
	req.response = make(chan policyEngineAPIResponse, 1)
	req.startTime = time.Now()
	select {
	case res := <-req.response:
		return res
	case <-ctx.Done():
		return policyEngineAPIResponse{
			err: i18n.NewError(ctx, tmmsgs.MsgTransactionHandlerRequestTimeout, time.Since(req.startTime).Seconds()),
		}
	}
}

func (sth *simpleTransactionHandler) HandleTransactionConfirmed(ctx context.Context, txID string, confirmations []apitypes.BlockInfo) (err error) {
	// Will be picked up on the next policy loop cycle
	var pending *pendingState
	for _, p := range sth.inflight {
		if p.mtx.ID == txID {
			pending = p
			break
		}
	}
	if pending == nil {
		err = i18n.NewError(ctx, tmmsgs.MsgTransactionNotFound, txID)
		return
	}
	sth.mux.Lock()
	pending.confirmed = true
	pending.mtx.Confirmations = confirmations
	sth.mux.Unlock()
	log.L(ctx).Debugf("Confirmed transaction %s at nonce %s / %d - hash: %s", pending.mtx.ID, pending.mtx.TransactionHeaders.From, pending.mtx.Nonce.Int64(), pending.mtx.TransactionHash)
	sth.toolkit.TXHistory.AddSubStatusAction(ctx, pending.mtx, apitypes.TxActionConfirmTransaction, nil, nil)
	sth.toolkit.TXHistory.SetSubStatus(ctx, pending.mtx, apitypes.TxSubStatusConfirmed)
	sth.markInflightUpdate()
	return
}
func (sth *simpleTransactionHandler) HandleTransactionReceiptReceived(ctx context.Context, txID string, receipt *ffcapi.TransactionReceiptResponse) (err error) {
	var pending *pendingState
	for _, p := range sth.inflight {
		if p.mtx.ID == txID {
			pending = p
			break
		}
	}
	if pending == nil {
		err = i18n.NewError(ctx, tmmsgs.MsgTransactionNotFound, txID)
		return
	}
	// Will be picked up on the next policy loop cycle - guaranteed to occur before Confirmed
	sth.mux.Lock()
	pending.mtx.Receipt = receipt
	sth.mux.Unlock()
	log.L(ctx).Debugf("Receipt received for transaction %s at nonce %s / %d - hash: %s", pending.mtx.ID, pending.mtx.TransactionHeaders.From, pending.mtx.Nonce.Int64(), pending.mtx.TransactionHash)
	sth.toolkit.TXHistory.AddSubStatusAction(ctx, pending.mtx, apitypes.TxActionReceiveReceipt, fftypes.JSONAnyPtr(`{"protocolId":"`+receipt.ProtocolID+`"}`), nil)
	sth.markInflightUpdate()
	return
}
