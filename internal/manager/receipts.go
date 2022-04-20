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
	defer close(m.receiptPollerDone)

	for {
		timer := time.NewTimer(m.receiptsPollingInterval)
		select {
		case <-timer.C:
			m.checkReceipts()
		case <-m.ctx.Done():
			log.L(m.ctx).Infof("Receipt poller exiting")
			return
		}
	}
}

func (m *manager) checkReceipts() {

	// Grab the lock to build a list of things to check
	m.mux.Lock()
	allPending := make([]*pendingState, 0, len(m.pendingOpsByID))
	for _, pending := range m.pendingOpsByID {
		allPending = append(allPending, pending)
	}
	m.mux.Unlock()

	// Go through trying to query all of them
	for _, pending := range allPending {
		err := m.checkReceiptCycle(pending)
		if err != nil {
			log.L(m.ctx).Errorf("Failed to receipt cycle transaction=%s operation=%s", pending.mtx.TransactionHash, pending.mtx.ID)
		}
	}

}

// checkReceiptCycle runs against each pending item, on each cycle, and is the one place responsible
// for state updates - to avoid those happening in parallel.
func (m *manager) checkReceiptCycle(pending *pendingState) (err error) {

	updated := true
	newStatus := fftypes.OpStatusPending
	mtx := pending.mtx
	switch {
	case pending.confirmed:
		updated = true
		if mtx.Receipt.Success {
			newStatus = fftypes.OpStatusSucceeded
		} else {
			newStatus = fftypes.OpStatusFailed
		}
	case pending.removed:
		// Remove from our state
		m.removeIfTracked(mtx.ID)
	default:
		if mtx.FirstSubmit != nil {
			if err = m.checkReceipt(pending); err != nil {
				return err
			}
		}

		// Pass the state to the pluggable policy engine to potentially perform more actions against it,
		// such as submitting for the first time, or raising the gas etc.
		updated, err = m.policyEngine.Execute(m.ctx, m.connectorAPI, pending.mtx)
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
		if pending.confirmed {
			// We can remove it now
			m.removeIfTracked(mtx.ID)
		}
	}

	return nil
}

func (m *manager) checkReceipt(pending *pendingState) error {
	mtx := pending.mtx
	res, reason, err := m.connectorAPI.GetReceipt(m.ctx, &ffcapi.GetReceiptRequest{
		TransactionHash: mtx.TransactionHash,
	})
	if err != nil {
		if reason == ffcapi.ErrorReasonNotFound {
			// If we previously thought we had a receipt, then tell the confirmation manager
			// to remove this transaction, and update our state.
			if mtx.Receipt != nil {
				m.clearConfirmationTracking(mtx)
				pending.lastReceiptBlockHash = ""

			}
		} else {
			// Others errors are logged
			return err
		}
	} else {
		// If the receipt has been changed (it's new, or the block hash changed) then let
		// the confirmation manager know to track it
		pending.mtx.Receipt = res
		if pending.lastReceiptBlockHash == "" || pending.lastReceiptBlockHash != res.BlockHash {
			m.requestConfirmationsNewReceipt(pending)
		}
	}
	return nil
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

func (m *manager) requestConfirmationsNewReceipt(pending *pendingState) {
	pending.lastReceiptBlockHash = pending.mtx.Receipt.BlockHash
	// The only error condition on confirmations manager is if we are exiting, which it logs
	_ = m.confirmations.Notify(&confirmations.Notification{
		NotificationType: confirmations.NewTransaction,
		Transaction: &confirmations.TransactionInfo{
			TransactionHash: pending.mtx.TransactionHash,
			BlockHash:       pending.mtx.Receipt.BlockHash,
			BlockNumber:     pending.mtx.Receipt.BlockNumber.Uint64(),
			Confirmed: func(confirmations []confirmations.BlockInfo) {
				// Will be picked up on the next receipt loop cycle
				m.mux.Lock()
				pending.confirmed = true
				pending.mtx.Confirmations = confirmations
				m.mux.Unlock()
			},
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
