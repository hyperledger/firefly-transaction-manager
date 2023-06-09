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
