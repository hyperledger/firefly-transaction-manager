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
	"database/sql/driver"
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
)

func TestExecuteBatchOpsInsertTXFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("INSERT.*").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.writer.executeBatchOps(ctx, &transactionWriterBatch{
		txInserts: []*apitypes.ManagedTX{{}},
	})
	assert.Regexp(t, "FF00177", err)

	mdb.ExpectationsWereMet()
}

func TestExecuteBatchOpsUpdateTXFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("UPDATE.*").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.writer.executeBatchOps(ctx, &transactionWriterBatch{
		txUpdates: []*transactionOperation{{
			txUpdate: &apitypes.TXUpdates{},
		}},
	})
	assert.Regexp(t, "FF00178", err)

	mdb.ExpectationsWereMet()
}

func TestExecuteBatchOpsUpsertReceiptFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("INSERT.*").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.db.RunAsGroup(ctx, func(ctx context.Context) error {
		return p.writer.executeBatchOps(ctx, &transactionWriterBatch{
			receiptInserts: []*apitypes.ReceiptRecord{{TransactionReceiptResponse: &ffcapi.TransactionReceiptResponse{}}},
		})
	})
	assert.Regexp(t, "FF00176", err)

	mdb.ExpectationsWereMet()
}

func TestExecuteBatchOpsInsertTXHistoryFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("INSERT.*").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.writer.executeBatchOps(ctx, &transactionWriterBatch{
		historyInserts: []*apitypes.TXHistoryRecord{{}},
	})
	assert.Regexp(t, "FF00177", err)

	mdb.ExpectationsWereMet()
}

func TestExecuteBatchOpsDeleteConfirmationsFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("DELETE.*").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.writer.executeBatchOps(ctx, &transactionWriterBatch{
		confirmationResets: map[string]bool{"1": true},
	})
	assert.Regexp(t, "FF00179", err)

	mdb.ExpectationsWereMet()
}

func TestExecuteBatchOpsInsertConfirmationFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("INSERT.*").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.writer.executeBatchOps(ctx, &transactionWriterBatch{
		confirmationInserts: []*apitypes.ConfirmationRecord{{
			Confirmation: &apitypes.Confirmation{},
		}},
	})
	assert.Regexp(t, "FF00177", err)

	mdb.ExpectationsWereMet()
}

func TestExecuteBatchOpsDeleteTXFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("DELETE.*receipts").WillReturnResult(driver.RowsAffected(0))
	mdb.ExpectExec("DELETE.*confirmations").WillReturnResult(driver.RowsAffected(0))
	mdb.ExpectExec("DELETE.*txhistory").WillReturnResult(driver.RowsAffected(0))
	mdb.ExpectExec("DELETE.*transactions").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.db.RunAsGroup(ctx, func(ctx context.Context) error {
		return p.writer.executeBatchOps(ctx, &transactionWriterBatch{
			txDeletes: []string{"1"},
		})
	})
	assert.Regexp(t, "FF00179", err)

	mdb.ExpectationsWereMet()
}

func TestExecuteBatchOpsDeleteTXHistoryFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("DELETE.*receipts").WillReturnResult(driver.RowsAffected(0))
	mdb.ExpectExec("DELETE.*confirmations").WillReturnResult(driver.RowsAffected(0))
	mdb.ExpectExec("DELETE.*txhistory").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.db.RunAsGroup(ctx, func(ctx context.Context) error {
		return p.writer.executeBatchOps(ctx, &transactionWriterBatch{
			txDeletes: []string{"1"},
		})
	})
	assert.Regexp(t, "FF00179", err)

	mdb.ExpectationsWereMet()
}

func TestExecuteBatchOpsDeleteTXConfirmationsFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("DELETE.*receipts").WillReturnResult(driver.RowsAffected(0))
	mdb.ExpectExec("DELETE.*confirmations").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.db.RunAsGroup(ctx, func(ctx context.Context) error {
		return p.writer.executeBatchOps(ctx, &transactionWriterBatch{
			txDeletes: []string{"1"},
		})
	})
	assert.Regexp(t, "FF00179", err)

	mdb.ExpectationsWereMet()
}

func TestExecuteBatchOpsDeleteTXReceiptFail(t *testing.T) {
	ctx, p, mdb, done := newMockSQLPersistence(t)
	defer done()

	mdb.ExpectBegin()
	mdb.ExpectExec("DELETE.*receipts").WillReturnError(fmt.Errorf("pop"))
	mdb.ExpectRollback()

	err := p.db.RunAsGroup(ctx, func(ctx context.Context) error {
		return p.writer.executeBatchOps(ctx, &transactionWriterBatch{
			txDeletes: []string{"1"},
		})
	})
	assert.Regexp(t, "FF00179", err)

	mdb.ExpectationsWereMet()
}
