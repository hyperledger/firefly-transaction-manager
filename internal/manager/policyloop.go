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
	"time"

	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/log"
)

func (m *manager) receiptPollingLoop() {
	defer close(m.policyLoopDone)

	for {
		timer := time.NewTimer(m.policyLoopInterval)
		select {
		case <-timer.C:
			m.policyLoopCycle()
		case <-m.ctx.Done():
			log.L(m.ctx).Infof("Receipt poller exiting")
			return
		}
	}
}

func (m *manager) policyLoopCycle() {

	// Grab the lock to build a list of things to check
	m.mux.Lock()
	allPending := make([]*pendingState, 0, len(m.pendingOpsByID))
	for _, pending := range m.pendingOpsByID {
		allPending = append(allPending, pending)
	}
	m.mux.Unlock()

	// Go through trying to query all of them
	for _, pending := range allPending {
		err := m.execPolicy(pending)
		if err != nil {
			log.L(m.ctx).Errorf("Failed policy cycle transaction=%s operation=%s: %s", pending.mtx.TransactionHash, pending.mtx.ID, err)
		}
	}

}

func (m *manager) addError(mtx *fftm.ManagedTXOutput, reason ffcapi.ErrorReason, err error) {
	newLen := len(mtx.ErrorHistory) + 1
	if newLen > m.errorHistoryCount {
		newLen = m.errorHistoryCount
	}
	oldHistory := mtx.ErrorHistory
	mtx.ErrorHistory = make([]*fftm.ManagedTXError, newLen)
	mtx.ErrorHistory[0] = &fftm.ManagedTXError{
		Time:   fftypes.Now(),
		Mapped: reason,
		Error:  err.Error(),
	}
	for i := 1; i < newLen; i++ {
		mtx.ErrorHistory[i] = oldHistory[i-1]
	}
}

// checkReceiptCycle runs against each pending item, on each cycle, and is the one place responsible
// for state updates - to avoid those happening in parallel.
func (m *manager) execPolicy(pending *pendingState) (err error) {

	updated := true
	completed := false
	newStatus := fftypes.OpStatusPending
	mtx := pending.mtx
	switch {
	case pending.confirmed:
		updated = true
		completed = true
		if mtx.Receipt.Success {
			newStatus = fftypes.OpStatusSucceeded
		} else {
			newStatus = fftypes.OpStatusFailed
		}
	case pending.removed:
		// Remove from our state
		m.removeIfTracked(mtx.ID)
	default:
		// Pass the state to the pluggable policy engine to potentially perform more actions against it,
		// such as submitting for the first time, or raising the gas etc.
		var reason ffcapi.ErrorReason
		updated, reason, err = m.policyEngine.Execute(m.ctx, m.connectorAPI, pending.mtx)
		if err != nil {
			log.L(m.ctx).Errorf("Policy engine returned error for operation %s reason=%s: %s", mtx.ID, reason, err)
			m.addError(mtx, reason, err)
		} else if mtx.FirstSubmit != nil && pending.trackingTransactionHash != mtx.TransactionHash {
			// If now submitted, add to confirmations manager for receipt checking
			m.trackSubmittedTransaction(pending)
		}
	}

	if updated || err != nil {
		errorString := ""
		if err != nil {
			// In the case of errors, we keep the record updated with the latest error - but leave it in Pending
			errorString = err.Error()
		}
		err := m.writeManagedTX(m.ctx, &opUpdate{
			ID:     mtx.ID,
			Status: newStatus,
			Output: mtx,
			Error:  errorString,
		})
		if err != nil {
			log.L(m.ctx).Errorf("Failed to update operation %s (status=%s): %s", mtx.ID, newStatus, err)
			return err
		}
		if completed {
			// We can remove it now
			m.removeIfTracked(mtx.ID)
		}
	}

	return nil
}

func (m *manager) trackSubmittedTransaction(pending *pendingState) {
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
				Receipt: func(receipt *ffcapi.GetReceiptResponse) {
					// Will be picked up on the next policy loop cycle - guaranteed to occur before Confirmed
					m.mux.Lock()
					pending.mtx.Receipt = receipt
					m.mux.Unlock()
				},
				Confirmed: func(confirmations []confirmations.BlockInfo) {
					// Will be picked up on the next policy loop cycle
					m.mux.Lock()
					pending.confirmed = true
					pending.mtx.Confirmations = confirmations
					m.mux.Unlock()
				},
			},
		})
	}

	// Only reason for error here should be a cancelled context
	if err != nil {
		log.L(m.ctx).Infof("Error detected notifying confirmation manager: %s", err)
	} else {
		pending.trackingTransactionHash = pending.mtx.TransactionHash
	}
}

func (m *manager) clearConfirmationTracking(mtx *fftm.ManagedTXOutput) {
	// The only error condition on confirmations manager is if we are exiting, which it logs
	_ = m.confirmations.Notify(&confirmations.Notification{
		NotificationType: confirmations.RemovedTransaction,
		Transaction: &confirmations.TransactionInfo{
			TransactionHash: mtx.TransactionHash,
		},
	})
}

func (m *manager) removeIfTracked(opID *fftypes.UUID) {
	m.mux.Lock()
	pending, existing := m.pendingOpsByID[*opID]
	if existing {
		delete(m.pendingOpsByID, *opID)
	}
	m.mux.Unlock()
	// Outside the lock tap the confirmation manager on the shoulder so it can clean up too
	if existing {
		m.clearConfirmationTracking(pending.mtx)
	}
}
