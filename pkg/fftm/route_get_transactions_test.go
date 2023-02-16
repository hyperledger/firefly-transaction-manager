// Copyright © 2022 Kaleido, Inc.
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

package fftm

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
)

func genTestTxn(signer string, nonce int64, status apitypes.TxStatus) *apitypes.ManagedTX {
	return &apitypes.ManagedTX{
		ID:         fmt.Sprintf("ns1:%s", fftypes.NewUUID()),
		Created:    fftypes.Now(),
		SequenceID: apitypes.NewULID(),
		Nonce:      fftypes.NewFFBigInt(nonce),
		Status:     status,
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: signer,
		},
	}
}

func newTestTxn(t *testing.T, m *manager, signer string, nonce int64, status apitypes.TxStatus) *apitypes.ManagedTX {
	tx := genTestTxn(signer, nonce, status)
	err := m.persistence.WriteTransaction(context.Background(), tx, true)
	assert.NoError(t, err)
	return tx
}

func TestGetTransactions(t *testing.T) {

	url, m, done := newTestManager(t)
	defer done()
	err := m.Start()
	assert.NoError(t, err)

	// Create a few persisted transaction directly in the persistence
	s1t1 := newTestTxn(t, m, "0xaaaaa", 10001, apitypes.TxStatusSucceeded)
	s2t1 := newTestTxn(t, m, "0xbbbbb", 10001, apitypes.TxStatusPending)
	s1t2 := newTestTxn(t, m, "0xaaaaa", 10002, apitypes.TxStatusFailed)
	s1t3 := newTestTxn(t, m, "0xaaaaa", 10003, apitypes.TxStatusPending)

	// Get with no filtering (not reverse order)
	var transactions []*apitypes.ManagedTX
	res, err := resty.New().R().
		SetResult(&transactions).
		Get(url + "/transactions?direction=asc")
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Len(t, transactions, 4)
	assert.Equal(t, s1t1.ID, transactions[0].ID)
	assert.Equal(t, s2t1.ID, transactions[1].ID)
	assert.Equal(t, s1t2.ID, transactions[2].ID)
	assert.Equal(t, s1t3.ID, transactions[3].ID)

	// Test pagination on default sort/filter
	res, err = resty.New().R().
		SetResult(&transactions).
		Get(url + "/transactions?limit=1&after=" + s1t2.ID)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Len(t, transactions, 1)
	assert.Equal(t, s2t1.ID, transactions[0].ID)

	// Test pagination on nonce filter
	res, err = resty.New().R().
		SetResult(&transactions).
		Get(url + "/transactions?signer=0xaaaaa&after=" + s1t2.ID)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Len(t, transactions, 1)
	assert.Equal(t, s1t1.ID, transactions[0].ID)

	// Test pagination on pending filter
	res, err = resty.New().R().
		SetResult(&transactions).
		Get(url + "/transactions?pending&after=" + s1t3.ID)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Len(t, transactions, 1)
	assert.Equal(t, s2t1.ID, transactions[0].ID)

}
