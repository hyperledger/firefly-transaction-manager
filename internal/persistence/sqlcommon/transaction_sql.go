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

package sqlcommon

import (
	"context"
	"database/sql"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

var transactionColumns = []string{
	"seq",
	"id",
	"status",
	"nonce",
	"gas",
	"transaction_headers",
	"transaction_data",
	"transaction_hash",
	"gas_price",
	"policy_info",
	"first_submit",
	"last_submit",
	"receipt",
	"confirmations",
	"error_msg",
	"history",
	"history_summary",
	"created",
	"updated",
	"delete_requested",
}

var transactionColumnsWithoutSeq = []string{
	"id",
	"status",
	"nonce",
	"gas",
	"transaction_headers",
	"transaction_data",
	"transaction_hash",
	"gas_price",
	"policy_info",
	"first_submit",
	"last_submit",
	"receipt",
	"confirmations",
	"error_msg",
	"history",
	"history_summary",
	"created",
	"updated",
	"delete_requested",
}

const transactionsTable = "transactions"

func (s *SQLCommon) InsertTransaction(ctx context.Context, managedTX *apitypes.ManagedTX) (err error) {
	ctx, tx, autoCommit, err := s.Database.BeginOrUseTx(ctx)
	if err != nil {
		return err
	}
	txHeadersJSON, err := json.Marshal(managedTX.TransactionHeaders)
	if err != nil {
		return err
	}

	var gasPriceJSON []byte
	var policyInfoJSON []byte
	var receiptJSON []byte
	var confirmationsJSON []byte
	var historyJSON []byte
	var historySummaryJSON []byte

	if managedTX.GasPrice != nil {
		if gasPriceJSON, err = json.Marshal(managedTX.GasPrice); err != nil {
			return err
		}
	}
	if managedTX.PolicyInfo != nil {
		if policyInfoJSON, err = json.Marshal(managedTX.PolicyInfo); err != nil {
			return err
		}
	}

	if managedTX.Receipt != nil {
		if receiptJSON, err = json.Marshal(managedTX.Receipt); err != nil {
			return err
		}
	}

	if managedTX.Confirmations != nil {
		if confirmationsJSON, err = json.Marshal(managedTX.Confirmations); err != nil {
			return err
		}
	}

	if managedTX.History != nil {
		if historyJSON, err = json.Marshal(managedTX.History); err != nil {
			return err
		}
	}

	if managedTX.HistorySummary != nil {
		if historySummaryJSON, err = json.Marshal(managedTX.HistorySummary); err != nil {
			return err
		}
	}
	defer s.Database.RollbackTx(ctx, tx, autoCommit)
	if _, err = s.Database.InsertTx(ctx, transactionsTable, tx,
		sq.Insert(transactionsTable).
			Columns(transactionColumnsWithoutSeq...).
			Values(
				managedTX.ID,
				managedTX.Status,
				managedTX.Nonce,
				managedTX.Gas,
				txHeadersJSON,
				managedTX.TransactionData,
				managedTX.TransactionHash,
				gasPriceJSON,
				policyInfoJSON,
				managedTX.FirstSubmit,
				managedTX.LastSubmit,
				receiptJSON,
				confirmationsJSON,
				managedTX.ErrorMessage,
				historyJSON,
				historySummaryJSON,
				managedTX.Created,
				managedTX.Updated,
				managedTX.DeleteRequested,
			),
		nil,
	); err != nil {
		return err
	}

	return s.Database.CommitTx(ctx, tx, autoCommit)
}

func (s *SQLCommon) transactionResult(ctx context.Context, row *sql.Rows) (*apitypes.ManagedTX, error) {
	tx := apitypes.ManagedTX{}
	var txHeadersJSON sql.NullString
	var gasPriceJSON sql.NullString
	var policyInfoJSON sql.NullString
	var receiptJSON sql.NullString
	var confirmationsJSON sql.NullString
	var historyJSON sql.NullString
	var historySummaryJSON sql.NullString

	err := row.Scan(
		&tx.SequenceID,
		&tx.ID,
		&tx.Status,
		&tx.Nonce,
		&tx.Gas,
		&txHeadersJSON,
		&tx.TransactionData,
		&tx.TransactionHash,
		&gasPriceJSON,
		&policyInfoJSON,
		&tx.FirstSubmit,
		&tx.LastSubmit,
		&receiptJSON,
		&confirmationsJSON,
		&tx.ErrorMessage,
		&historyJSON,
		&historySummaryJSON,
		&tx.Created,
		&tx.Updated,
		&tx.DeleteRequested,
	)
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, transactionsTable)
	}

	if txHeadersJSON.Valid {
		if err := json.Unmarshal([]byte(txHeadersJSON.String), &tx.TransactionHeaders); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, transactionsTable)
		}
	}

	if gasPriceJSON.Valid {
		if err := json.Unmarshal([]byte(gasPriceJSON.String), tx.GasPrice); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, transactionsTable)
		}
	}

	if policyInfoJSON.Valid {
		if err := json.Unmarshal([]byte(policyInfoJSON.String), tx.PolicyInfo); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, transactionsTable)
		}
	}

	if receiptJSON.Valid {
		if err := json.Unmarshal([]byte(receiptJSON.String), &tx.Receipt); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, transactionsTable)
		}
	}

	if confirmationsJSON.Valid {
		if err := json.Unmarshal([]byte(confirmationsJSON.String), &tx.Confirmations); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, transactionsTable)
		}
	}

	if historyJSON.Valid {
		if err := json.Unmarshal([]byte(historyJSON.String), &tx.History); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, transactionsTable)
		}
	}

	if historySummaryJSON.Valid {
		if err := json.Unmarshal([]byte(historySummaryJSON.String), &tx.HistorySummary); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, transactionsTable)
		}
	}

	return &tx, nil
}

func (s *SQLCommon) getTransactionPred(ctx context.Context, desc string, pred interface{}) (*apitypes.ManagedTX, error) {
	rows, _, err := s.Database.Query(ctx, transactionsTable,
		sq.Select(transactionColumns...).
			From(transactionsTable).
			Where(pred),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		log.L(ctx).Debugf("Transaction '%s' not found", desc)
		return nil, nil
	}

	tx, err := s.transactionResult(ctx, rows)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (s *SQLCommon) GetTransactionByID(ctx context.Context, id *fftypes.UUID) (*apitypes.ManagedTX, error) {
	return s.getTransactionPred(ctx, id.String(), sq.Eq{"id": id})
}

func (s *SQLCommon) GetTransactionByNonce(ctx context.Context, signer string, nonce *fftypes.FFBigInt) (*apitypes.ManagedTX, error) {
	return s.getTransactionPred(ctx, nonce.String(), sq.Eq{"nonce": nonce, "transaction_headers->>'from'": signer})
}
