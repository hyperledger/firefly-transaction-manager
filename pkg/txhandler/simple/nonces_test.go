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

package simple

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/persistencemocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNonceStaleStateContention(t *testing.T) {

	f, tk, _, conf, cleanup := newTestTransactionHandlerFactoryWithFilePersistence(t)
	defer cleanup()
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	th, err := f.NewTransactionHandler(context.Background(), conf)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()

	// Write a stale record to persistence
	oldTime := fftypes.FFTime(time.Now().Add(-100 * time.Hour))
	err = tk.Persistence.WriteTransaction(sth.ctx, &apitypes.ManagedTX{
		ID:         "stale1",
		Created:    &oldTime,
		Status:     apitypes.TxStatusSucceeded,
		SequenceID: apitypes.NewULID(),
		Nonce:      fftypes.NewFFBigInt(1000), // old nonce
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x12345",
		},
	}, true)
	assert.NoError(t, err)

	mFFC := tk.Connector.(*ffcapimocks.API)

	mFFC.On("NextNonceForSigner", mock.Anything, mock.MatchedBy(func(nonceReq *ffcapi.NextNonceForSignerRequest) bool {
		return "0x12345" == nonceReq.Signer
	})).Return(&ffcapi.NextNonceForSignerResponse{
		Nonce: fftypes.NewFFBigInt(1111),
	}, ffcapi.ErrorReason(""), nil)
	sth.Init(sth.ctx, tk)
	locked1 := make(chan struct{})
	done1 := make(chan struct{})
	done2 := make(chan struct{})

	go func() {
		defer close(done1)

		ln, err := sth.assignAndLockNonce(context.Background(), "ns1:"+fftypes.NewUUID().String(), "0x12345")
		assert.NoError(t, err)
		assert.Equal(t, uint64(1111), ln.nonce)
		close(locked1)

		time.Sleep(1 * time.Millisecond)
		ln.spent = &apitypes.ManagedTX{
			ID:         "ns1:" + fftypes.NewUUID().String(),
			Created:    &oldTime,
			Nonce:      fftypes.NewFFBigInt(int64(ln.nonce)),
			Status:     apitypes.TxStatusPending,
			SequenceID: apitypes.NewULID(),
			TransactionHeaders: ffcapi.TransactionHeaders{
				From: "0x12345",
			},
		}
		err = sth.toolkit.Persistence.WriteTransaction(sth.ctx, ln.spent, true)
		assert.NoError(t, err)
		ln.complete(context.Background())
	}()

	go func() {
		defer close(done2)

		<-locked1
		ln, err := sth.assignAndLockNonce(context.Background(), "ns2:"+fftypes.NewUUID().String(), "0x12345")
		assert.NoError(t, err)

		assert.Equal(t, uint64(1112), ln.nonce)

		ln.complete(context.Background())

	}()

	<-done1
	<-done2

}

func TestNonceListError(t *testing.T) {

	f, tk, _, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	th, err := f.NewTransactionHandler(context.Background(), conf)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()

	mFFC := tk.Connector.(*ffcapimocks.API)
	mFFC.On("TransactionPrepare", mock.Anything, mock.Anything).Return(&ffcapi.TransactionPrepareResponse{
		TransactionData: "RAW_UNSIGNED_BYTES",
		Gas:             fftypes.NewFFBigInt(2000000), // gas estimate simulation
	}, ffcapi.ErrorReason(""), nil)

	mp := tk.Persistence.(*persistencemocks.Persistence)
	mp.On("ListTransactionsByNonce", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("pop"))
	sth.Init(sth.ctx, tk)
	_, err = sth.HandleNewTransaction(context.Background(), &apitypes.TransactionRequest{
		TransactionInput: ffcapi.TransactionInput{
			TransactionHeaders: ffcapi.TransactionHeaders{
				From: "0x12345",
			},
		},
	})
	assert.Regexp(t, "pop", err)

	mp.AssertExpectations(t)
	mFFC.AssertExpectations(t)

}

func TestNonceListStaleThenQueryFail(t *testing.T) {

	f, tk, _, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	th, err := f.NewTransactionHandler(context.Background(), conf)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()

	mp := tk.Persistence.(*persistencemocks.Persistence)
	old := fftypes.FFTime(time.Now().Add(-10000 * time.Hour))
	mp.On("ListTransactionsByNonce", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*apitypes.ManagedTX{
			{ID: "id12345", Created: &old, Status: apitypes.TxStatusSucceeded, Nonce: fftypes.NewFFBigInt(1000)},
		}, nil)

	mFFC := tk.Connector.(*ffcapimocks.API)
	mFFC.On("TransactionPrepare", mock.Anything, mock.Anything).Return(&ffcapi.TransactionPrepareResponse{
		TransactionData: "RAW_UNSIGNED_BYTES",
		Gas:             fftypes.NewFFBigInt(2000000), // gas estimate simulation
	}, ffcapi.ErrorReason(""), nil)
	mFFC.On("NextNonceForSigner", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop"))
	sth.Init(sth.ctx, tk)
	_, err = sth.HandleNewTransaction(context.Background(), &apitypes.TransactionRequest{
		TransactionInput: ffcapi.TransactionInput{
			TransactionHeaders: ffcapi.TransactionHeaders{
				From: "0x12345",
			},
		},
	})
	assert.Regexp(t, "pop", err)

	mp.AssertExpectations(t)
	mFFC.AssertExpectations(t)

}

func TestNonceListNotStale(t *testing.T) {

	f, tk, _, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	th, err := f.NewTransactionHandler(context.Background(), conf)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()

	sth.nonceStateTimeout = 1 * time.Hour

	mp := tk.Persistence.(*persistencemocks.Persistence)

	mp.On("ListTransactionsByNonce", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*apitypes.ManagedTX{
			{ID: "id12345", Created: fftypes.Now(), Status: apitypes.TxStatusSucceeded, Nonce: fftypes.NewFFBigInt(1000)},
		}, nil)
	sth.Init(sth.ctx, tk)
	n, err := sth.calcNextNonce(context.Background(), "0x12345")
	assert.NoError(t, err)
	assert.Equal(t, uint64(1001), n)

}
