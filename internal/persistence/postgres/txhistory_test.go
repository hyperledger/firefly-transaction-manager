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
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestTXHistoryCompressionPSQL(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)

	// Do a set of transaction operations through the writers, and confirm the results are correct
	ctx, p, _, done := initTestPSQL(t)
	defer done()

	// Write an initial transaction
	txID := fmt.Sprintf("ns1:%s", fftypes.NewUUID())
	tx := &apitypes.ManagedTX{
		ID:     txID,
		Status: apitypes.TxStatusPending,
		TransactionHeaders: ffcapi.TransactionHeaders{
			From:  "0x111111",
			Nonce: fftypes.NewFFBigInt(333333),
		},
	}
	err := p.InsertTransaction(ctx, tx)
	assert.NoError(t, err)
	assert.NotEmpty(t, tx.SequenceID)

	// Add a bunch of simulated status entries, which are good for compression
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusReceived, apitypes.TxActionAssignNonce,
		fftypes.JSONAnyPtr(`{"nonce":"11111"}`), nil)
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusReceived, apitypes.TxActionSubmitTransaction,
		nil, fftypes.JSONAnyPtr(`"failed to submit 111"`))
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusReceived, apitypes.TxActionSubmitTransaction,
		nil, fftypes.JSONAnyPtr(`"failed to submit 222"`))
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusReceived, apitypes.TxActionSubmitTransaction,
		nil, fftypes.JSONAnyPtr(`"failed to submit 333"`))
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusTracking, apitypes.TxActionSubmitTransaction,
		fftypes.JSONAnyPtr(`{"transactionHash":"0x12345"}`), nil)
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusTracking, apitypes.TxActionReceiveReceipt,
		fftypes.JSONAnyPtr(`{"transactionHash":"0x111111","blockNumber":"11111"}`), nil)
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusTracking, apitypes.TxActionReceiveReceipt,
		fftypes.JSONAnyPtr(`{"transactionHash":"0x222222","blockNumber":"22222"}`), nil)
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusTracking, apitypes.TxActionReceiveReceipt,
		fftypes.JSONAnyPtr(`{"transactionHash":"0x333333","blockNumber":"33333"}`), nil)
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusTracking, apitypes.TxActionTimeout,
		nil, fftypes.JSONAnyPtr(`"some error 444"`))
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusTracking, apitypes.TxActionTimeout,
		nil, fftypes.JSONAnyPtr(`"some error 555"`))
	assert.NoError(t, err)
	err = p.AddSubStatusAction(ctx, txID, apitypes.TxSubStatusConfirmed, apitypes.TxActionConfirmTransaction,
		fftypes.JSONAnyPtr(`{"confirmed":true}`), nil)
	assert.NoError(t, err)

	// Flush with a TX update
	newStatus := apitypes.TxStatusSucceeded
	err = p.UpdateTransaction(ctx, txID, &apitypes.TXUpdates{
		Status: &newStatus,
	})
	assert.NoError(t, err)

	// Compress the state
	err = p.db.RunAsGroup(ctx, func(ctx context.Context) error {
		return p.compressHistory(ctx, txID)
	})
	assert.NoError(t, err)

	// Get the history
	history, _, err := p.ListTransactionHistory(ctx, txID, persistence.TXHistoryFilters.NewFilter(ctx).And())
	assert.NoError(t, err)
	// Time strip the history for compare
	for _, h := range history {
		assert.NotNil(t, h.Time)
		if h.LastError != nil {
			assert.NotNil(t, h.LastErrorTime)
		}
		assert.NotZero(t, h.Count)
		if h.Count == 1 {
			assert.Equal(t, h.LastOccurrence.String(), h.Time.String())
		} else {
			assert.GreaterOrEqual(t, *h.LastOccurrence.Time(), *h.Time.Time())
		}
		h.ID = nil
		h.Time = nil
		h.LastOccurrence = nil
		h.LastErrorTime = nil
	}
	assert.Equal(t, []*apitypes.TXHistoryRecord{
		{
			TransactionID: txID,
			SubStatus:     apitypes.TxSubStatusConfirmed,
			TxHistoryActionEntry: apitypes.TxHistoryActionEntry{
				Action:   apitypes.TxActionConfirmTransaction,
				Count:    1,
				LastInfo: fftypes.JSONAnyPtr(`{"confirmed":true}`),
			},
		},
		{
			TransactionID: txID,
			SubStatus:     apitypes.TxSubStatusTracking,
			TxHistoryActionEntry: apitypes.TxHistoryActionEntry{
				Action:    apitypes.TxActionTimeout,
				Count:     2,
				LastError: fftypes.JSONAnyPtr(`"some error 555"`),
			},
		},
		{
			TransactionID: txID,
			SubStatus:     apitypes.TxSubStatusTracking,
			TxHistoryActionEntry: apitypes.TxHistoryActionEntry{
				Action:   apitypes.TxActionReceiveReceipt,
				Count:    3,
				LastInfo: fftypes.JSONAnyPtr(`{"transactionHash":"0x333333","blockNumber":"33333"}`),
			},
		},
		{
			TransactionID: txID,
			SubStatus:     apitypes.TxSubStatusTracking,
			TxHistoryActionEntry: apitypes.TxHistoryActionEntry{
				Action:   apitypes.TxActionSubmitTransaction,
				Count:    1,
				LastInfo: fftypes.JSONAnyPtr(`{"transactionHash":"0x12345"}`),
			},
		},
		{
			TransactionID: txID,
			SubStatus:     apitypes.TxSubStatusReceived,
			TxHistoryActionEntry: apitypes.TxHistoryActionEntry{
				Action:    apitypes.TxActionSubmitTransaction,
				Count:     3,
				LastError: fftypes.JSONAnyPtr(`"failed to submit 333"`),
			},
		},
		{
			TransactionID: txID,
			SubStatus:     apitypes.TxSubStatusReceived,
			TxHistoryActionEntry: apitypes.TxHistoryActionEntry{
				Action:   apitypes.TxActionAssignNonce,
				Count:    1,
				LastInfo: fftypes.JSONAnyPtr(`{"nonce":"11111"}`),
			},
		},
	}, history)

}
