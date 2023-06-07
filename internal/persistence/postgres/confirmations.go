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
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

func (p *sqlPersistence) newConfirmationsCollection() *dbsql.CrudBase[*apitypes.ConfirmationRecord] {
	return &dbsql.CrudBase[*apitypes.ConfirmationRecord]{
		DB:    &psql.Database,
		Table: "confirmations",
		Columns: []string{
			dbsql.ColumnID,
			dbsql.ColumnCreated,
			dbsql.ColumnUpdated,
			"listeners",
		},
		FilterFieldMap: map[string]string{
			"streamid": "id",
		},
		NilValue:     func() *apitypes.ConfirmationRecord { return nil },
		NewInstance:  func() *apitypes.ConfirmationRecord { return &apitypes.ConfirmationRecord{} },
		ScopedFilter: func() sq.Eq { return sq.Eq{} },
		EventHandler: nil, // set below
		GetFieldPtr: func(inst *apitypes.ConfirmationRecord, col string) interface{} {
			switch col {
			case dbsql.ColumnID:
				return &inst.ID
			case dbsql.ColumnCreated:
				return &inst.Created
			case dbsql.ColumnUpdated:
				return &inst.Updated
			}
			return nil
		},
	}
}

func (p *sqlPersistence) GetTransactionConfirmations(ctx context.Context, txID string) ([]*apitypes.Confirmation, error) {
	return nil, nil
}

func (p *sqlPersistence) AddTransactionConfirmations(ctx context.Context, txID string, clearExisting bool, confirmations ...*apitypes.Confirmation) error {
	return nil
}
