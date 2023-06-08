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

	sq "github.com/Masterminds/squirrel"
	"github.com/hyperledger/firefly-common/pkg/dbsql"
	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
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
			"from",
			"to",
			"nonce",
			"gas",
			"value",
			"gasprice",
			"tx_data",
			"tx_hash",
			"policy_info",
			"first_submit",
			"last_submit",
			"error_message",
		},
		FilterFieldMap: map[string]string{
			"transactiondata": "tx_data",
			"transactionhash": "tx_hash",
			"deleterequested": "delete",
			"policyinfo":      "policy_info",
			"firstsubmit":     "first_submit",
			"lastsubmit":      "last_submit",
			"errormessage":    "error_message",
		},
		NilValue:     func() *apitypes.ManagedTX { return nil },
		NewInstance:  func() *apitypes.ManagedTX { return &apitypes.ManagedTX{} },
		ScopedFilter: func() sq.Eq { return sq.Eq{} },
		EventHandler: nil, // set below
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
			case "from":
				return &inst.From
			case "to":
				return &inst.To
			case "nonce":
				return &inst.Nonce
			case "gas":
				return &inst.Gas
			case "value":
				return &inst.Value
			case "gasprice":
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
	return nil, nil
}

func (p *sqlPersistence) GetTransactionByIDWithHistory(ctx context.Context, txID string) (*apitypes.TXWithStatus, error) {
	return nil, nil
}

func (p *sqlPersistence) GetTransactionByNonce(ctx context.Context, signer string, nonce *fftypes.FFBigInt) (*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) InsertTransaction(ctx context.Context, tx *apitypes.ManagedTX) error {
	return nil
}

func (p *sqlPersistence) UpdateTransaction(ctx context.Context, txID string, updates *apitypes.TXUpdates) error {
	return nil
}

func (p *sqlPersistence) DeleteTransaction(ctx context.Context, txID string) error {
	return nil
}
