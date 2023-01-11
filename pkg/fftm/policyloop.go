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

package fftm

import (
	"context"
	"fmt"
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
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
)

func (m *manager) policyLoop() {
	defer close(m.policyLoopDone)
	ctx := log.WithLogField(m.ctx, "role", "policyloop")

	for {
		// Wait to be notified, or timeout to run
		timer := time.NewTimer(m.policyLoopInterval)
		select {
		case <-m.inflightUpdate:
		case <-timer.C:
		case <-ctx.Done():
			log.L(ctx).Infof("Receipt poller exiting")
			return
		}
		// Pop whether we were marked stale
		stale := false
		select {
		case <-m.inflightStale:
			stale = true
		default:
		}
		m.policyLoopCycle(ctx, stale)
	}
}

func (m *manager) markInflightStale() {
	// First mark that we're stale
	select {
	case m.inflightStale <- true:
	default:
	}
	// Then ensure we queue a loop that picks up the stale marker
	m.markInflightUpdate()
}

func (m *manager) markInflightUpdate() {
	select {
	case m.inflightUpdate <- true:
	default:
	}
}

func (m *manager) updateInflightSet(ctx context.Context) bool {

	oldInflight := m.inflight
	m.inflight = make([]*pendingState, 0, len(oldInflight))

	// Run through removing those that are removed
	for _, p := range oldInflight {
		if !p.remove {
			m.inflight = append(m.inflight, p)
		}
	}

	// If we are not at maximum, then query if there are more candidates now
	spaces := m.maxInFlight - len(m.inflight)
	if spaces > 0 {
		var after *fftypes.UUID
		if len(m.inflight) > 0 {
			after = m.inflight[len(m.inflight)-1].mtx.SequenceID
		}
		var additional []*apitypes.ManagedTX
		// We retry the get from persistence indefinitely (until the context cancels)
		err := m.retry.Do(ctx, "get pending transactions", func(attempt int) (retry bool, err error) {
			additional, err = m.persistence.ListTransactionsPending(ctx, after, spaces, persistence.SortDirectionAscending)
			return true, err
		})
		if err != nil {
			log.L(ctx).Infof("Policy loop context cancelled while retrying")
			return false
		}
		for _, mtx := range additional {
			m.inflight = append(m.inflight, &pendingState{mtx: mtx})
		}
		newLen := len(m.inflight)
		if newLen > 0 {
			log.L(ctx).Debugf("Inflight set updated len=%d head-seq=%s tail-seq=%s old-tail=%s", len(m.inflight), m.inflight[0].mtx.SequenceID, m.inflight[newLen-1].mtx.SequenceID, after)
		}
	}
	return true

}

func (m *manager) policyLoopCycle(ctx context.Context, inflightStale bool) {

	// Process any synchronous commands first - these might not be in our inflight set
	m.processPolicyAPIRequests(ctx)

	if inflightStale {
		if !m.updateInflightSet(ctx) {
			return
		}
	}

	// Go through executing the policy engine against them
	if m.metricsManager.IsMetricsEnabled() {
		m.metricsManager.TransactionsInFlightSet(float64(len(m.inflight)))
	}

	for _, pending := range m.inflight {
		err := m.execPolicy(ctx, pending, false)
		if err != nil {
			log.L(ctx).Errorf("Failed policy cycle transaction=%s operation=%s: %s", pending.mtx.TransactionHash, pending.mtx.ID, err)
		}
	}

}

// processPolicyAPIRequests executes any API calls requested that require policy engine involvement - such as transaction deletions
func (m *manager) processPolicyAPIRequests(ctx context.Context) {

	m.mux.Lock()
	requests := m.policyEngineAPIRequests
	if len(requests) > 0 {
		m.policyEngineAPIRequests = []*policyEngineAPIRequest{}
	}
	m.mux.Unlock()

	for _, request := range requests {
		var pending *pendingState
		// If this transaction is in-flight, we use that record
		for _, inflight := range m.inflight {
			if inflight.mtx.ID == request.txID {
				pending = inflight
				break
			}
		}
		if pending == nil {
			mtx, err := m.getTransactionByID(ctx, request.txID)
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
			if err := m.execPolicy(ctx, pending, true); err != nil {
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

// updateHistory applies an update to the object, and returns if a new item was added
// to the history.
func (m *manager) updateHistory(mtx *apitypes.ManagedTX, info string, err error, reason ffcapi.ErrorReason) bool {

	if info == "" && err == nil {
		return false
	}

	// Initialize a new entry
	mtx.Updated = fftypes.Now()
	newEntry := &apitypes.ManagedTXUpdate{
		Time:         mtx.Updated,
		Info:         info,
		MappedReason: reason,
		Count:        1,
	}

	// Set or clear the error message - on the entry, and the top-level TX
	if err != nil {
		newEntry.Error = err.Error()
		mtx.ErrorMessage = err.Error()
	} else {
		mtx.ErrorMessage = ""
	}

	// Check if we just need to increment the last occurrence and count on the latest
	if len(mtx.History) > 0 && mtx.History[0].MsgString() == newEntry.MsgString() {
		existingEntry := mtx.History[0]
		existingEntry.Count++
		existingEntry.LastOccurrence = mtx.Updated
		return err != nil // Always store error count bumps
	}

	// Otherwise extend the list - newest first
	newLen := len(mtx.History) + 1
	if newLen > m.errorHistoryCount {
		newLen = m.errorHistoryCount
	}
	oldHistory := mtx.History
	mtx.History = make([]*apitypes.ManagedTXUpdate, newLen)
	mtx.History[0] = newEntry
	for i := 1; i < newLen; i++ {
		mtx.History[i] = oldHistory[i-1]
	}
	return true
}

func (m *manager) execPolicy(ctx context.Context, pending *pendingState, syncDeleteRequest bool) (err error) {

	update := policyengine.UpdateNo
	completed := false
	var receiptProtocolID string

	// Check whether this has been confirmed by the confirmation manager
	m.mux.Lock()
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
	m.mux.Unlock()

	var updateErr error
	var updateReason ffcapi.ErrorReason
	var updateInfo string
	switch {
	case receiptProtocolID != "" && confirmed && !syncDeleteRequest:
		update = policyengine.UpdateYes
		completed = true
		updateInfo = fmt.Sprintf("Success=%t,Receipt=%s,Confirmations=%d,Hash=%s", mtx.Receipt.Success, receiptProtocolID, len(mtx.Confirmations), mtx.TransactionHash)
		if pending.mtx.Receipt.Success {
			mtx.Status = apitypes.TxStatusSucceeded
		} else {
			mtx.Status = apitypes.TxStatusFailed
			updateErr = i18n.NewError(ctx, tmmsgs.MsgTransactionFailed)
		}

	default:
		// We get woken for lots of reasons to go through the policy loop, but we only want
		// to drive the policy engine at regular intervals.
		// So we track the last time we ran the policy engine against each pending item.
		// We always call the policy engine on every loop, when deletion has been requested.
		if syncDeleteRequest || time.Since(pending.lastPolicyCycle) > m.policyLoopInterval {
			// Pass the state to the pluggable policy engine to potentially perform more actions against it,
			// such as submitting for the first time, or raising the gas etc.

			update, updateReason, updateErr = m.policyEngine.Execute(ctx, m.connector, pending.mtx)
			if updateErr != nil {
				log.L(ctx).Errorf("Policy engine returned error for transaction %s reason=%s: %s", mtx.ID, updateReason, err)
				update = policyengine.UpdateYes
				if m.metricsManager.IsMetricsEnabled() {
					m.metricsManager.TransactionSubmissionError()
				}
			} else {
				updateInfo = fmt.Sprintf("Submitted=%t,Receipt=%s,Hash=%s", mtx.FirstSubmit != nil, receiptProtocolID, mtx.TransactionHash)
				log.L(ctx).Debugf("Policy engine executed for tx %s (update=%d,status=%s,hash=%s)", mtx.ID, update, mtx.Status, mtx.TransactionHash)
				if mtx.FirstSubmit != nil && pending.trackingTransactionHash != mtx.TransactionHash {
					// If now submitted, add to confirmations manager for receipt checking
					m.trackSubmittedTransaction(ctx, pending)
				}
				pending.lastPolicyCycle = time.Now()
			}
		}
	}

	infoChanged := m.updateHistory(mtx, updateInfo, updateErr, updateReason)
	if infoChanged && update == policyengine.UpdateNo {
		// TODO: The interface with policy engine could do with enhancing, including
		//       reconciling FireFly core issue 1108. For now, if the policy engine
		//       doesn't mark an update, but the info we have about the TX changed
		//       due to receipt/confirmations popping in or then we publish an update.
		update = policyengine.UpdateYes
	}

	switch update {
	case policyengine.UpdateYes:
		err := m.persistence.WriteTransaction(ctx, mtx, false)
		if err != nil {
			log.L(ctx).Errorf("Failed to update transaction %s (status=%s): %s", mtx.ID, mtx.Status, err)
			return err
		}
		if completed {
			pending.remove = true // for the next time round the loop
			log.L(ctx).Infof("Transaction %s marked complete (status=%s): %s", mtx.ID, mtx.Status, err)
			m.markInflightStale()
		}
		// if and only if the transaction is now resolved send web a socket update
		if mtx.Status == apitypes.TxStatusSucceeded || mtx.Status == apitypes.TxStatusFailed {
			m.sendWSReply(mtx)
		}
	case policyengine.UpdateDelete:
		err := m.persistence.DeleteTransaction(ctx, mtx.ID)
		if err != nil {
			log.L(ctx).Errorf("Failed to delete transaction %s (status=%s): %s", mtx.ID, mtx.Status, err)
			return err
		}
		pending.remove = true // for the next time round the loop
		m.markInflightStale()
		m.sendWSReply(mtx)
	}

	return nil
}

func (m *manager) sendWSReply(mtx *apitypes.ManagedTX) {
	wsr := &apitypes.TransactionUpdateReply{
		Headers: apitypes.ReplyHeaders{
			RequestID: mtx.ID,
		},
		Status:          mtx.Status,
		TransactionHash: mtx.TransactionHash,
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
	m.wsServer.SendReply(wsr)
}

func (m *manager) trackSubmittedTransaction(ctx context.Context, pending *pendingState) {
	var err error

	// Clear any old transaction hash
	if pending.trackingTransactionHash != "" {
		err = m.confirmations.Notify(&confirmations.Notification{
			NotificationType: confirmations.RemovedTransaction,
			Transaction: &confirmations.TransactionInfo{
				TransactionHash: pending.trackingTransactionHash,
			},
		})
	}

	// Notify of the new
	if err == nil {
		err = m.confirmations.Notify(&confirmations.Notification{
			NotificationType: confirmations.NewTransaction,
			Transaction: &confirmations.TransactionInfo{
				TransactionHash: pending.mtx.TransactionHash,
				Receipt: func(ctx context.Context, receipt *ffcapi.TransactionReceiptResponse) {
					// Will be picked up on the next policy loop cycle - guaranteed to occur before Confirmed
					m.mux.Lock()
					pending.mtx.Receipt = receipt
					m.mux.Unlock()
					log.L(m.ctx).Debugf("Receipt received for transaction %s at nonce %s / %d - hash: %s", pending.mtx.ID, pending.mtx.TransactionHeaders.From, pending.mtx.Nonce.Int64(), pending.mtx.TransactionHash)
					pending.mtx.AddSubStatus(ctx, apitypes.TxSubStatusReceivedReceipt)
					m.markInflightUpdate()
				},
				Confirmed: func(ctx context.Context, confirmations []confirmations.BlockInfo) {
					// Will be picked up on the next policy loop cycle
					m.mux.Lock()
					pending.confirmed = true
					pending.mtx.Confirmations = confirmations
					m.mux.Unlock()
					log.L(m.ctx).Debugf("Confirmed transaction %s at nonce %s / %d - hash: %s", pending.mtx.ID, pending.mtx.TransactionHeaders.From, pending.mtx.Nonce.Int64(), pending.mtx.TransactionHash)
					pending.mtx.AddSubStatus(ctx, apitypes.TxSubStatusConfirmed)
					m.markInflightUpdate()
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

func (m *manager) policyEngineAPIRequest(ctx context.Context, req *policyEngineAPIRequest) policyEngineAPIResponse {
	m.mux.Lock()
	m.policyEngineAPIRequests = append(m.policyEngineAPIRequests, req)
	m.mux.Unlock()
	m.markInflightUpdate()
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
