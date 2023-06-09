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
	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

func (p *sqlPersistence) newTransactionCollection() *dbsql.CrudBase[*apitypes.ManagedTX] {
	collection := &dbsql.CrudBase[*apitypes.ManagedTX]{
		DB:    &psql.Database,
		Table: "transactions",
		Columns: []string{
			dbsql.ColumnID,
			dbsql.ColumnCreated,
			dbsql.ColumnUpdated,
			"status",
			"delete",
			"tx_from",
			"tx_to",
			"tx_nonce",
			"tx_gas",
			"tx_value",
			"tx_gasprice",
			"tx_data",
			"tx_hash",
			"policy_info",
			"first_submit",
			"last_submit",
			"error_message",
		},
		FilterFieldMap: map[string]string{
			"sequence":        p.db.SequenceColumn(),
			"transactiondata": "tx_data",
			"transactionhash": "tx_hash",
			"deleterequested": "delete",
			"from":            "tx_from",
			"to":              "tx_to",
			"nonce":           "tx_nonce",
			"gas":             "tx_gas",
			"value":           "tx_value",
			"gasprice":        "tx_gasprice",
			"policyinfo":      "policy_info",
			"firstsubmit":     "first_submit",
			"lastsubmit":      "last_submit",
			"errormessage":    "error_message",
		},
		PatchDisabled: true,
		NilValue:      func() *apitypes.ManagedTX { return nil },
		NewInstance:   func() *apitypes.ManagedTX { return &apitypes.ManagedTX{} },
		GetFieldPtr: func(inst *apitypes.ManagedTX, col string) interface{} {
			switch col {
			case dbsql.ColumnID:
				return &inst.ID
			case dbsql.ColumnCreated:
				return &inst.Created
			case dbsql.ColumnUpdated:
				return &inst.Updated
			case "status":
				return &inst.Status
			case "delete":
				return &inst.DeleteRequested
			case "tx_from":
				return &inst.From
			case "tx_to":
				return &inst.To
			case "tx_nonce":
				return &inst.Nonce
			case "tx_gas":
				return &inst.Gas
			case "tx_value":
				return &inst.Value
			case "tx_gasprice":
				return &inst.GasPrice
			case "tx_data":
				return &inst.TransactionData
			case "tx_hash":
				return &inst.TransactionHash
			case "policy_info":
				return &inst.PolicyInfo
			case "first_submit":
				return &inst.FirstSubmit
			case "last_submit":
				return &inst.LastSubmit
			case "error_message":
				return &inst.ErrorMessage
			}
			return nil
		},
	}
	collection.Validate()
	return collection
}

func (p *sqlPersistence) ListTransactions(ctx context.Context, filter ffapi.Filter) ([]*apitypes.ManagedTX, *ffapi.FilterResult, error) {
	return nil, nil, nil
}

func (p *sqlPersistence) ListTransactionsByCreateTime(ctx context.Context, after *apitypes.ManagedTX, limit int, dir persistence.SortDirection) ([]*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) ListTransactionsByNonce(ctx context.Context, signer string, after *fftypes.FFBigInt, limit int, dir persistence.SortDirection) ([]*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) ListTransactionsPending(ctx context.Context, afterSequenceID string, limit int, dir persistence.SortDirection) ([]*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) GetTransactionByID(ctx context.Context, txID string) (*apitypes.ManagedTX, error) {
	return p.transactions.GetByID(ctx, txID)
}

func (p *sqlPersistence) GetTransactionByIDWithHistory(ctx context.Context, txID string) (*apitypes.TXWithStatus, error) {
	tx, err := p.transactions.GetByID(ctx, txID)
	if tx == nil || err != nil {
		return nil, err
	}
	receiptRecord, err := p.receipts.GetByID(ctx, txID)
	if err != nil {
		return nil, err
	}
	var receipt *ffcapi.TransactionReceiptResponse
	if receiptRecord != nil {
		receipt = receiptRecord.TransactionReceiptResponse
	}
	confirmations, err := p.GetTransactionConfirmations(ctx, txID)
	if err != nil {
		return nil, err
	}
	history, err := p.buildHistorySummary(ctx, txID, p.historySummaryLimit, nil)
	if err != nil {
		return nil, err
	}
	return &apitypes.TXWithStatus{
		ManagedTX:     tx,
		Receipt:       receipt,
		Confirmations: confirmations,
		History:       history,
	}, nil
}

func (p *sqlPersistence) GetTransactionByNonce(ctx context.Context, signer string, nonce *fftypes.FFBigInt) (*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) InsertTransaction(ctx context.Context, tx *apitypes.ManagedTX) error {
	// Dispatch to TX writer
	op := newTransactionOperation(tx.ID)
	op.txInsert = tx
	p.writer.queue(ctx, op)
	return op.flush(ctx) // wait for completion
}

func (p *sqlPersistence) UpdateTransaction(ctx context.Context, txID string, updates *apitypes.TXUpdates) error {
	// Dispatch to TX writer
	op := newTransactionOperation(txID)
	op.txUpdate = updates
	p.writer.queue(ctx, op)
	return op.flush(ctx) // wait for completion
}

func (p *sqlPersistence) DeleteTransaction(ctx context.Context, txID string) error {
	// Dispatch to TX writer, as we need to sequence it against updates and delete all associated records
	op := newTransactionOperation(txID)
	op.txDelete = &txID
	p.writer.queue(ctx, op)
	return op.flush(ctx) // wait for completion
}

func (p *sqlPersistence) updateTransaction(ctx context.Context, txID string, updates *apitypes.TXUpdates) error {
	sqlUpdate := persistence.TransactionFilters.NewUpdate(ctx).S()
	if updates.Status != nil {
		sqlUpdate = sqlUpdate.Set("status", *updates.Status)
	}
	if updates.DeleteRequested != nil {
		sqlUpdate = sqlUpdate.Set("deleterequested", updates.DeleteRequested)
	}
	if updates.From != nil {
		sqlUpdate = sqlUpdate.Set("from", *updates.From)
	}
	if updates.To != nil {
		sqlUpdate = sqlUpdate.Set("to", *updates.To)
	}
	if updates.Nonce != nil {
		sqlUpdate = sqlUpdate.Set("nonce", updates.Nonce)
	}
	if updates.Gas != nil {
		sqlUpdate = sqlUpdate.Set("gas", updates.Gas)
	}
	if updates.Value != nil {
		sqlUpdate = sqlUpdate.Set("value", updates.Value)
	}
	if updates.GasPrice != nil {
		sqlUpdate = sqlUpdate.Set("gasprice", updates.GasPrice)
	}
	if updates.TransactionData != nil {
		sqlUpdate = sqlUpdate.Set("transactiondata", *updates.TransactionData)
	}
	if updates.TransactionHash != nil {
		sqlUpdate = sqlUpdate.Set("transactionhash", *updates.TransactionHash)
	}
	if updates.PolicyInfo != nil {
		sqlUpdate = sqlUpdate.Set("policyinfo", updates.PolicyInfo)
	}
	if updates.FirstSubmit != nil {
		sqlUpdate = sqlUpdate.Set("firstsubmit", updates.FirstSubmit)
	}
	if updates.LastSubmit != nil {
		sqlUpdate = sqlUpdate.Set("lastsubmit", updates.LastSubmit)
	}
	if updates.ErrorMessage != nil {
		sqlUpdate = sqlUpdate.Set("errormessage", *updates.ErrorMessage)
	}
	return p.transactions.Update(ctx, txID, sqlUpdate)
}
