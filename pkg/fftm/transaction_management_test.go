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
	"fmt"
	"net/http"
	"testing"

	"github.com/hyperledger/firefly-transaction-manager/mocks/persistencemocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/txhandlermocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetTransactionErrors(t *testing.T) {

	_, m, close := newTestManagerMockPersistence(t)
	defer close()

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("GetTransactionByID", m.ctx, mock.Anything).Return(nil, fmt.Errorf("pop")).Once()
	mp.On("GetTransactionByID", m.ctx, mock.Anything).Return(nil, nil).Once()
	mp.On("Close", mock.Anything).Return(nil).Maybe()

	_, err := m.getTransactionByID(m.ctx, "id")
	assert.Regexp(t, "pop", err)

	_, err = m.getTransactionByID(m.ctx, "id")
	assert.Regexp(t, "FF21067", err)

	mp.AssertExpectations(t)

}

func TestGetTransactionsErrors(t *testing.T) {

	_, m, close := newTestManagerMockPersistence(t)
	defer close()

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("GetTransactionByID", m.ctx, mock.Anything).Return(nil, fmt.Errorf("pop")).Once()
	mp.On("GetTransactionByID", m.ctx, mock.Anything).Return(nil, nil).Once()
	mp.On("Close", mock.Anything).Return(nil).Maybe()

	_, err := m.getTransactions(m.ctx, "", "bad limit", "", false, "")
	assert.Regexp(t, "FF21044", err)

	_, err = m.getTransactions(m.ctx, "", "", "", false, "wrong")
	assert.Regexp(t, "FF21064", err)

	_, err = m.getTransactions(m.ctx, "", "", "cannot be specified with pending", true, "")
	assert.Regexp(t, "FF21063", err)

	_, err = m.getTransactions(m.ctx, "after-causes-failure", "", "", false, "")
	assert.Regexp(t, "pop", err)

	_, err = m.getTransactions(m.ctx, "after-not-found", "", "", false, "")
	assert.Regexp(t, "FF21062", err)

	mp.AssertExpectations(t)

}

func TestDeleteTransactionError(t *testing.T) {
	_, m, done := newTestManager(t)
	defer done()
	mth := &txhandlermocks.TransactionHandler{}
	transientChannel := make(chan struct{})
	defer close(transientChannel)
	mth.On("HandleCancelTransaction", m.ctx, mock.Anything).Return(nil, fmt.Errorf("pop")).Once()
	m.txHandler = mth

	status, _, err := m.requestTransactionDeletion(m.ctx, "")
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Regexp(t, "pop", err)

	mth.AssertExpectations(t)

}
