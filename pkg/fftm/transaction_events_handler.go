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

package fftm

import (
	"context"

	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/ws"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhandler"
)

type ManagedTransactionEventHandler struct {
	Ctx                 context.Context
	ConfirmationManager confirmations.Manager
	WsServer            ws.WebSocketServer
	TxHandler           txhandler.TransactionHandler
}

func NewManagedTransactionEventHandler(ctx context.Context, cm confirmations.Manager, ws ws.WebSocketServer, th txhandler.TransactionHandler) txhandler.ManagedTxEventHandler {
	eh := &ManagedTransactionEventHandler{
		Ctx:                 ctx,
		ConfirmationManager: cm,
		WsServer:            ws,
		TxHandler:           th,
	}
	return eh
}

func (eh *ManagedTransactionEventHandler) HandleEvent(ctx context.Context, e apitypes.ManagedTransactionEvent) (err error) {
	switch e.Type {
	case apitypes.ManagedTXProcessSucceeded:
		eh.sendWSReply(e.Tx)
	case apitypes.ManagedTXProcessFailed:
		eh.sendWSReply(e.Tx)
	case apitypes.ManagedTXDeleted:
		eh.sendWSReply(e.Tx)
	case apitypes.ManagedTXTransactionHashUpdated:
		err = eh.trackSubmittedTransaction(e.Tx)
	}
	return
}

func (eh *ManagedTransactionEventHandler) sendWSReply(mtx *apitypes.ManagedTX) {
	wsr := &apitypes.TransactionUpdateReply{
		Headers: apitypes.ReplyHeaders{
			RequestID: mtx.ID,
		},
		Status:          mtx.Status,
		TransactionHash: mtx.TransactionHash,
	}

	if mtx.Receipt != nil && mtx.Receipt.ContractLocation != nil {
		wsr.ContractLocation = mtx.Receipt.ContractLocation
	}

	if mtx.Receipt != nil {
		wsr.ProtocolID = mtx.Receipt.ProtocolID
	} else {
		wsr.ProtocolID = ""
	}

	switch mtx.Status {
	case apitypes.TxStatusSucceeded:
		wsr.Headers.Type = apitypes.TransactionUpdateSuccess
	case apitypes.TxStatusFailed:
		wsr.Headers.Type = apitypes.TransactionUpdateFailure
	}
	// Notify on the websocket - this is best-effort (there is no subscription/acknowledgement)
	eh.WsServer.SendReply(wsr)
}

func (eh *ManagedTransactionEventHandler) trackSubmittedTransaction(mtx *apitypes.ManagedTX) error {
	var err error

	// Clear any old transaction hash
	if mtx.PreviousTransactionHash != "" {
		err = eh.ConfirmationManager.Notify(&confirmations.Notification{
			NotificationType: confirmations.RemovedTransaction,
			Transaction: &confirmations.TransactionInfo{
				TransactionHash: mtx.PreviousTransactionHash,
			},
		})
	}

	// Notify of the new
	if err == nil {
		err = eh.ConfirmationManager.Notify(&confirmations.Notification{
			NotificationType: confirmations.NewTransaction,
			Transaction: &confirmations.TransactionInfo{
				TransactionHash: mtx.TransactionHash,
				Receipt: func(ctx context.Context, receipt *ffcapi.TransactionReceiptResponse) {
					if err := eh.TxHandler.HandleTransactionReceipt(ctx, mtx.ID, receipt); err != nil {
						log.L(ctx).Errorf("Receipt for transaction %s at nonce %s / %d - hash: %s was not handled due to %s", mtx.ID, mtx.TransactionHeaders.From, mtx.Nonce.Int64(), mtx.TransactionHash, err.Error())
					}
				},
				Confirmed: func(ctx context.Context, confirmations []confirmations.BlockInfo) {
					if err := eh.TxHandler.HandleTransactionConfirmed(ctx, mtx.ID, confirmations); err != nil {
						log.L(ctx).Errorf("Confirmation for transaction %s at nonce %s / %d - hash: %s was not handled due to %s", mtx.ID, mtx.TransactionHeaders.From, mtx.Nonce.Int64(), mtx.TransactionHash, err.Error())
					}
				},
			},
		})
	}
	return err
}
