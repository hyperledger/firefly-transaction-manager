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

package postgres

import (
	"context"

	"github.com/hyperledger/firefly-common/pkg/dbsql"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

func (p *sqlPersistence) newTXHistoryCollection() *dbsql.CrudBase[*apitypes.TXHistoryRecord] {
	collection := &dbsql.CrudBase[*apitypes.TXHistoryRecord]{
		DB:    &psql.Database,
		Table: "txhistory",
		Columns: []string{
			dbsql.ColumnID,
			dbsql.ColumnCreated,
			dbsql.ColumnUpdated,
			"tx_id",
			"status",
			"action",
			"count",
			"error",
			"error_time",
			"info",
		},
		FilterFieldMap: map[string]string{
			"sequence":       p.db.SequenceColumn(),
			"transaction":    "tx_id",
			"substatus":      "status",
			"time":           dbsql.ColumnCreated,
			"lastoccurrence": dbsql.ColumnUpdated,
			"lasterror":      "error",
			"lasterrortime":  "error_time",
			"lastinfo":       "info",
		},
		PatchDisabled: true,
		NilValue:      func() *apitypes.TXHistoryRecord { return nil },
		NewInstance:   func() *apitypes.TXHistoryRecord { return &apitypes.TXHistoryRecord{} },
		GetFieldPtr: func(inst *apitypes.TXHistoryRecord, col string) interface{} {
			switch col {
			case dbsql.ColumnID:
				return &inst.ID
			case dbsql.ColumnCreated:
				return &inst.Time
			case dbsql.ColumnUpdated:
				return &inst.LastOccurrence
			case "tx_id":
				return &inst.TransactionID
			case "status":
				return &inst.SubStatus
			case "action":
				return &inst.Action
			case "count":
				return &inst.Count
			case "error":
				return &inst.LastError
			case "error_time":
				return &inst.LastErrorTime
			case "info":
				return &inst.LastInfo
			}
			return nil
		},
	}
	collection.Validate()
	return collection
}

func (p *sqlPersistence) SetSubStatus(ctx context.Context, txID string, subStatus apitypes.TxSubStatus) error {
	// TODO: Consider
	return p.AddSubStatusAction(ctx, txID, subStatus, apitypes.TxActionStateTransition, nil, nil)
}

func (p *sqlPersistence) AddSubStatusAction(ctx context.Context, txID string, subStatus apitypes.TxSubStatus, action apitypes.TxAction, info *fftypes.JSONAny, err *fftypes.JSONAny) error {
	// Dispatch to TX writer
	op := newTransactionOperation(txID)
	op.historyRecord = &apitypes.TXHistoryRecord{
		ID:            fftypes.NewUUID(),
		TransactionID: txID,
		SubStatus:     subStatus,
		TxHistoryActionEntry: apitypes.TxHistoryActionEntry{
			Count:     1,
			Action:    action,
			LastInfo:  info,
			LastError: err,
		},
	}
	if err != nil {
		op.historyRecord.LastErrorTime = fftypes.Now()
	}
	p.writer.queue(ctx, op)
	return nil // completely async
}

// buildHistorySummary builds a compressed summary of actions, grouped within subStatus changes, and with redundant actions removed.
func (p *sqlPersistence) buildHistorySummary(ctx context.Context, txID string, resultLimit int, mergeCallback func(from, to *apitypes.TXHistoryRecord) error) ([]*apitypes.TxHistoryStateTransitionEntry, error) {
	result := []*apitypes.TxHistoryStateTransitionEntry{}
	skip := 0
	pageSize := 50
	for {
		filter := persistence.TXHistoryFilters.
			NewFilterLimit(ctx, uint64(pageSize)).Eq("transaction", txID).
			Skip(uint64(skip))
		page, _, err := p.txHistory.GetMany(ctx, filter)
		if err != nil {
			return nil, err
		}
		var lastEntrySameSubStatus *apitypes.TXHistoryRecord
		for _, h := range page {
			if len(result) == 0 || result[len(result)-1].Status != h.SubStatus {
				lastEntrySameSubStatus = nil // we've changed subStatus
				result = append(result, &apitypes.TxHistoryStateTransitionEntry{
					Time:    h.Time,
					Status:  h.SubStatus,
					Actions: []*apitypes.TxHistoryActionEntry{},
				})
			}
			statusEntry := result[len(result)-1]
			var actionEntry *apitypes.TxHistoryActionEntry
			if lastEntrySameSubStatus == nil || lastEntrySameSubStatus.Action != h.Action {
				actionEntry = &apitypes.TxHistoryActionEntry{
					Time:           h.Time,
					LastOccurrence: h.LastOccurrence,
					Action:         h.Action,
					Count:          h.Count,
				}
				statusEntry.Actions = append(statusEntry.Actions, actionEntry)
			} else {
				// We can compress these records together. We might have a callback to persist this, if we're
				// running this function as part of a history compression.
				if mergeCallback != nil {
					if err := mergeCallback(h, lastEntrySameSubStatus); err != nil {
						log.L(ctx).Errorf("Merging status record %s into %s failed: %s", h.ID, lastEntrySameSubStatus.ID, err)
					}
				}
				actionEntry = statusEntry.Actions[len(statusEntry.Actions)-1]
				actionEntry.Count++
				actionEntry.LastOccurrence = h.LastOccurrence
			}
			actionEntry.LastInfo = h.LastInfo
			if h.LastErrorTime != nil {
				actionEntry.LastErrorTime = h.LastErrorTime
				actionEntry.LastError = h.LastError
			}
			lastEntrySameSubStatus = h
		}
		// We're done when we run out of input, or we hit the target maximum output records
		if len(page) != pageSize || (resultLimit > 0 && len(result) >= resultLimit) {
			return result, nil
		}
		skip += pageSize
	}

}
