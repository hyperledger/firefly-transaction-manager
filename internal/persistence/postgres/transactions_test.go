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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestTransactionBasicValidationE2E(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)

	// Do a set of transaction operations through the writers, and confirm the results are correct
	ctx, p, _, done := initTestPSQL(t)
	defer done()

	// Write an initial transaction
	txID := fmt.Sprintf("ns1:%s", fftypes.NewUUID())
	tx := &apitypes.ManagedTX{
		ID:              txID,
		Status:          apitypes.TxStatusPending,
		DeleteRequested: nil,
		TransactionHeaders: ffcapi.TransactionHeaders{
			From:  "0x111111",
			To:    "0x222222",
			Nonce: fftypes.NewFFBigInt(333333),
			Gas:   fftypes.NewFFBigInt(555555),
			Value: fftypes.NewFFBigInt(666666),
		},
		GasPrice:        fftypes.JSONAnyPtr(`{"gas":"777777"}`),
		TransactionData: "0x999999",
		TransactionHash: "0xaaaaaa",
		PolicyInfo:      fftypes.JSONAnyPtr(`{"policy":"888888"}`),
		FirstSubmit:     fftypes.Now(),
		LastSubmit:      fftypes.Now(),
		ErrorMessage:    "error bbbbbb",
	}
	err := p.InsertTransaction(ctx, tx)
	assert.NoError(t, err)
	origTXJson, err := json.Marshal(tx)
	assert.NoError(t, err)

	// Read it back immediately (testing the flush)
	tx1, err := p.GetTransactionByID(ctx, txID)
	assert.NoError(t, err)
	insertedTXJson, err := json.Marshal(tx1)
	assert.NoError(t, err)
	assert.JSONEq(t, string(origTXJson), string(insertedTXJson))

	// Queue up a bunch of items, which will flush with the transaction update at the end ...

	// A receipt
	receipt := &ffcapi.TransactionReceiptResponse{
		BlockNumber:      fftypes.NewFFBigInt(111111),
		TransactionIndex: fftypes.NewFFBigInt(222222),
		BlockHash:        "0x333333",
		Success:          true,
		ProtocolID:       "000/111/222",
		ExtraInfo:        fftypes.JSONAnyPtr(`{"extra":"444444"}`),
		ContractLocation: fftypes.JSONAnyPtr(`{"address":"0x555555"}`),
	}
	err = p.SetTransactionReceipt(ctx, txID, receipt)
	assert.NoError(t, err)

	// A few confirmations
	confirmations := make([]*apitypes.Confirmation, 5)
	for i := 0; i < len(confirmations); i++ {
		confirmations[i] = &apitypes.Confirmation{
			BlockNumber: fftypes.FFuint64(i),
			BlockHash:   fmt.Sprintf("0x1%.3d", i),
			ParentHash:  fmt.Sprintf("0x2%.3d", i),
		}
	}
	err = p.AddTransactionConfirmations(ctx, txID, true, confirmations...)
	assert.NoError(t, err)

	// A couple of transaction history entries
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusReceived, apitypes.TxActionAssignNonce, fftypes.JSONAnyPtr(`{"nonce":"11111"}`), nil)
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusReceived, apitypes.TxActionSubmitTransaction, nil, fftypes.JSONAnyPtr(`"failed to submit"`))
	assert.NoError(t, err)

	// Finally the update - do a comprehensive one
	newStatus := apitypes.TxStatusFailed
	txUpdates := &apitypes.TXUpdates{
		Status:          &newStatus,
		DeleteRequested: fftypes.Now(),
		From:            strPtr("0xfff111111"),
		To:              strPtr("0xfff222222"),
		Nonce:           fftypes.NewFFBigInt(999333333),
		Gas:             fftypes.NewFFBigInt(999555555),
		Value:           fftypes.NewFFBigInt(999666666),
		GasPrice:        fftypes.JSONAnyPtr(`{"gas":"777777"}`),
		TransactionData: strPtr("0x999999"),
		TransactionHash: strPtr("0xaaaaaa"),
		PolicyInfo:      fftypes.JSONAnyPtr(`{"policy":"888888"}`),
		FirstSubmit:     fftypes.Now(),
		LastSubmit:      fftypes.Now(),
		ErrorMessage:    strPtr("error bbbbbb"),
	}
	err = p.UpdateTransaction(ctx, txID, txUpdates)
	assert.NoError(t, err)

	// Get back the merged object to check everything
	mtx, err := p.GetTransactionByIDWithHistory(ctx, txID)
	assert.NoError(t, err)
	assert.NotEqual(t, tx.Updated.String(), mtx.Updated.String())

	mtxExpected := &apitypes.TXWithStatus{
		ManagedTX: &apitypes.ManagedTX{
			ID:              txID,
			Created:         tx.Created,  // will not have changed
			Updated:         mtx.Updated, // will have changed
			Status:          *txUpdates.Status,
			DeleteRequested: txUpdates.DeleteRequested,
			TransactionHeaders: ffcapi.TransactionHeaders{
				From:  *txUpdates.From,
				To:    *txUpdates.To,
				Nonce: txUpdates.Nonce,
				Gas:   txUpdates.Gas,
				Value: txUpdates.Value,
			},
			GasPrice:        txUpdates.GasPrice,
			TransactionData: *txUpdates.TransactionData,
			TransactionHash: *txUpdates.TransactionHash,
			PolicyInfo:      txUpdates.PolicyInfo,
			FirstSubmit:     txUpdates.FirstSubmit,
			LastSubmit:      txUpdates.LastSubmit,
			ErrorMessage:    *txUpdates.ErrorMessage,
		},
		Receipt:       receipt,
		Confirmations: confirmations,
		History: []*apitypes.TxHistoryStateTransitionEntry{
			{
				Status: apitypes.TxSubStatusReceived,
				Time:   mtx.History[0].Time,
				Actions: []*apitypes.TxHistoryActionEntry{ // newest first
					{
						Action:         apitypes.TxActionSubmitTransaction,
						Time:           mtx.History[0].Actions[0].Time,
						LastOccurrence: mtx.History[0].Actions[0].LastOccurrence,
						Count:          1,
						LastInfo:       nil,
						LastError:      fftypes.JSONAnyPtr(`"failed to submit"`),
						LastErrorTime:  mtx.History[0].Actions[0].LastErrorTime,
					},
					{
						Action:         apitypes.TxActionAssignNonce,
						Time:           mtx.History[0].Actions[1].Time,
						LastOccurrence: mtx.History[0].Actions[1].LastOccurrence,
						Count:          1,
						LastInfo:       fftypes.JSONAnyPtr(`{"nonce":"11111"}`),
						LastError:      nil,
						LastErrorTime:  nil,
					},
				},
			},
		},
	}
	expectedMTXJson, err := json.Marshal(mtxExpected)
	assert.NoError(t, err)
	actualMTXJson, err := json.Marshal(mtx)
	assert.NoError(t, err)
	assert.JSONEq(t, string(expectedMTXJson), string(actualMTXJson))

}

func strPtr(s string) *string {
	return &s
}
