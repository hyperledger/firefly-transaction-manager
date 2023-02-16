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

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/wsmocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/mock"
)

func newTestManagedTransactionEventHandler() *ManagedTransactionEventHandler {
	eh := &ManagedTransactionEventHandler{
		Ctx: context.Background(),
	}
	return eh
}

func TestHandleTransactionProcessSuccessEvent(t *testing.T) {
	eh := newTestManagedTransactionEventHandler()
	mws := &wsmocks.WebSocketServer{}
	mws.On("SendReply", mock.Anything).Return(nil).Once()
	eh.WsServer = mws
	testTx := &apitypes.ManagedTX{
		ID:         fmt.Sprintf("ns1:%s", fftypes.NewUUID()),
		Created:    fftypes.Now(),
		SequenceID: apitypes.NewULID(),
		Nonce:      fftypes.NewFFBigInt(1),
		Status:     apitypes.TxStatusSucceeded,
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x0000",
		},
		TransactionHash: "0x1111",
	}

	eh.HandleEvent(context.Background(), apitypes.ManagedTransactionEvent{
		Type: apitypes.ManagedTXProcessSucceeded,
		Tx:   testTx,
	})

	mws.AssertExpectations(t)
}

func TestHandleTransactionHashUpdateEventNewHash(t *testing.T) {
	eh := newTestManagedTransactionEventHandler()
	mcm := &confirmationsmocks.Manager{}
	mcm.On("Notify", mock.Anything).Return(nil).Once()
	eh.ConfirmationManager = mcm
	testTx := &apitypes.ManagedTX{
		ID:         fmt.Sprintf("ns1:%s", fftypes.NewUUID()),
		Created:    fftypes.Now(),
		SequenceID: apitypes.NewULID(),
		Nonce:      fftypes.NewFFBigInt(1),
		Status:     apitypes.TxStatusPending,
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x0000",
		},
		PreviousTransactionHash: "",
		TransactionHash:         "0x1111",
	}

	eh.HandleEvent(context.Background(), apitypes.ManagedTransactionEvent{
		Type: apitypes.ManagedTXTransactionHashUpdated,
		Tx:   testTx,
	})

	mcm.AssertExpectations(t)
}

func TestHandleTransactionHashUpdateEventOldHash(t *testing.T) {
	eh := newTestManagedTransactionEventHandler()
	mcm := &confirmationsmocks.Manager{}
	mcm.On("Notify", mock.Anything).Return(nil).Twice()
	eh.ConfirmationManager = mcm
	testTx := &apitypes.ManagedTX{
		ID:         fmt.Sprintf("ns1:%s", fftypes.NewUUID()),
		Created:    fftypes.Now(),
		SequenceID: apitypes.NewULID(),
		Nonce:      fftypes.NewFFBigInt(1),
		Status:     apitypes.TxStatusPending,
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x0000",
		},
		PreviousTransactionHash: "0x1111",
		TransactionHash:         "0x2222",
	}

	eh.HandleEvent(context.Background(), apitypes.ManagedTransactionEvent{
		Type: apitypes.ManagedTXTransactionHashUpdated,
		Tx:   testTx,
	})

	mcm.AssertExpectations(t)
}
