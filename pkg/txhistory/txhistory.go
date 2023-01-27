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

package txhistory

import (
	"context"
	"encoding/json"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

type Manager interface {
	CurrentSubStatus(ctx context.Context, mtx *apitypes.ManagedTX) *apitypes.TxHistoryStateTransitionEntry
	SetSubStatus(ctx context.Context, mtx *apitypes.ManagedTX, subStatus apitypes.TxSubStatus)
	AddSubStatusAction(ctx context.Context, mtx *apitypes.ManagedTX, action apitypes.TxAction, info *fftypes.JSONAny, err *fftypes.JSONAny)
}

type manager struct {
	maxHistoryCount int
}

func NewTxHistoryManager(ctx context.Context) Manager {
	return &manager{
		maxHistoryCount: config.GetInt(tmconfig.TransactionsMaxHistoryCount),
	}
}

func (h *manager) CurrentSubStatus(ctx context.Context, mtx *apitypes.ManagedTX) *apitypes.TxHistoryStateTransitionEntry {
	if len(mtx.History) > 0 {
		return mtx.History[len(mtx.History)-1]
	}
	return nil
}

// Transaction sub-status entries can be added for a given transaction so a caller
// can see discrete steps in a transaction moving to confirmation on the blockchain.
// For example a transaction might have a sub-status of "Stale" if a transaction has
// been in pending state for a given period of time. In order to progress the transaction
// while it's in a given sub-status, certain actions might be taken (such as retrieving
// the latest gas price for the chain). See AddSubStatusAction(). Since a transaction
// might go through many sub-status changes before being confirmed on chain the list of
// entries is capped at the configured number and FIFO approach used to keep within that cap.
func (h *manager) SetSubStatus(ctx context.Context, mtx *apitypes.ManagedTX, subStatus apitypes.TxSubStatus) {
	// See if the status being transitioned to is the same as the current status.
	// If so, there's nothing to do.
	if len(mtx.History) > 0 {
		if mtx.History[len(mtx.History)-1].Status == subStatus {
			return
		}
		log.L(ctx).Debugf("State transition to sub-status %s", subStatus)
	}

	// If this is a change in status add a new record
	newStatus := &apitypes.TxHistoryStateTransitionEntry{
		Time:    fftypes.Now(),
		Status:  subStatus,
		Actions: make([]*apitypes.TxHistoryActionEntry, 0),
	}
	mtx.History = append(mtx.History, newStatus)

	if len(mtx.History) > h.maxHistoryCount {
		// Need to trim the oldest record
		mtx.History = mtx.History[1:]
	}

	// As was as detailed sub-status records (which might be a long list and early entries
	// get purged at some point) we keep a separate list of all the discrete types of sub-status
	// we've ever seen for this transaction along with a count of them. This means an early sub-status
	// (e.g. "queued") followed by 100s of different sub-status types will still be recorded
	for _, statusType := range mtx.HistorySummary {
		if statusType.Status == subStatus {
			// Just increment the counter
			statusType.Count++
			return
		}
	}

	mtx.HistorySummary = append(mtx.HistorySummary, &apitypes.TxHistorySummaryEntry{Status: subStatus, Count: 1, FirstOccurrence: fftypes.Now()})
}

// Takes a string that might be valid JSON, and returns valid JSON that is either:
// a) The original JSON if it is valid
// b) An escaped string
func jsonOrString(value *fftypes.JSONAny) *fftypes.JSONAny {
	if value == nil {
		return nil
	}

	if json.Valid([]byte(*value)) {
		// Already valid
		return value
	}

	// Quote it as a string
	b, _ := json.Marshal((string)(*value))
	return fftypes.JSONAnyPtrBytes(b)
}

// When a transaction is in a given sub-status (e.g. "Stale") the blockchain connector
// may perform certain actions to move it out of the status. For example it might
// retrieve the current gas price for the chain. TxAction's represent an action taken
// while in a given sub-status. In order to limit the number of TxAction entries in
// a TxSubStatusEntry each action has a count of the number of occurrences and a
// latest timestamp to indicate when it was last executed. There is a last error field
// which can be used to indicate the most recent error that occurred, for example an
// HTTP 4xx return code from a gas oracle. There is also an information field to record
// arbitrary data about the action, for example the gas price retrieved from an oracle.
func (h *manager) AddSubStatusAction(ctx context.Context, mtx *apitypes.ManagedTX, action apitypes.TxAction, info *fftypes.JSONAny, err *fftypes.JSONAny) {

	// See if this action exists in the list already since we only want to update the single entry, not
	// add a new one
	currentSubStatus := mtx.History[len(mtx.History)-1]
	for _, entry := range currentSubStatus.Actions {
		if entry.Action == action {
			entry.Count++
			entry.LastOccurrence = fftypes.Now()

			if err != nil {
				entry.LastError = jsonOrString(err)
				entry.LastErrorTime = fftypes.Now()
			}

			if info != nil {
				entry.LastInfo = jsonOrString(info)
			}
			return
		}
	}

	// If this is an entirely new status add it to the list
	newAction := &apitypes.TxHistoryActionEntry{
		Time:           fftypes.Now(),
		Action:         action,
		LastOccurrence: fftypes.Now(),
		Count:          1,
	}

	if err != nil {
		newAction.LastError = jsonOrString(err)
		newAction.LastErrorTime = fftypes.Now()
	}

	if info != nil {
		newAction.LastInfo = jsonOrString(info)
	}

	currentSubStatus.Actions = append(currentSubStatus.Actions, newAction)
}
