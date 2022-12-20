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

package apitypes

import (
	"fmt"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

// TxStatus is the current status of a transaction
type TxStatus string

const (
	// TxStatusPending indicates the operation has been submitted, but is not yet confirmed as successful or failed
	TxStatusPending TxStatus = "Pending"
	// TxStatusSucceeded the infrastructure runtime has returned success for the operation
	TxStatusSucceeded TxStatus = "Succeeded"
	// TxStatusFailed happens when an error is reported by the infrastructure runtime
	TxStatusFailed TxStatus = "Failed"
)

type ManagedTXUpdate struct {
	Time           *fftypes.FFTime    `json:"time"`
	LastOccurrence *fftypes.FFTime    `json:"lastOccurrence"`
	Count          int                `json:"count"`
	Info           string             `json:"info,omitempty"`
	Error          string             `json:"error,omitempty"`
	MappedReason   ffcapi.ErrorReason `json:"reason,omitempty"`
}

// MsgString is assured to be the same, as long as the type/message is the same.
// Does not change if the count/times are different - so allows comparison.
func (mtu *ManagedTXUpdate) MsgString() string {
	if mtu == nil {
		return ""
	}
	msg := ""
	if mtu.Error != "" {
		msg = fmt.Sprintf("error: %s, ", mtu.Error)
	}
	if mtu.MappedReason != "" {
		msg += fmt.Sprintf("reason: %s, ", mtu.MappedReason)
	}
	msg += fmt.Sprintf("info: %s", mtu.Info)
	return msg
}

// ManagedTX is the structure stored for each new transaction request, using the external ID of the operation
//
// Indexing:
//
//	Multiple index collection are stored for the managed transactions, to allow them to be managed including:
//
//	- Nonce allocation: this is a critical index, and why cleanup is so important (mentioned below).
//	  We use this index to determine the next nonce to assign to a given signing key.
//	- Created time: a timestamp ordered index for the transactions for convenient ordering.
//	  the key includes the ID of the TX for uniqueness.
//	- Pending sequence: An entry in this index only exists while the transaction is pending, and is
//	  ordered by a UUIDv1 sequence allocated to each entry.
//
// Index cleanup after partial write:
//   - All indexes are stored before the TX itself.
//   - When listing back entries, the persistence layer will automatically clean up indexes if the underlying
//     TX they refer to is not available. For this reason the index records are written first.
type ManagedTX struct {
	ID                 string                             `json:"id"`
	Created            *fftypes.FFTime                    `json:"created"`
	Updated            *fftypes.FFTime                    `json:"updated"`
	Status             TxStatus                           `json:"status"`
	DeleteRequested    *fftypes.FFTime                    `json:"deleteRequested,omitempty"`
	SequenceID         *fftypes.UUID                      `json:"sequenceId"`
	Nonce              *fftypes.FFBigInt                  `json:"nonce"`
	Gas                *fftypes.FFBigInt                  `json:"gas"`
	TransactionHeaders ffcapi.TransactionHeaders          `json:"transactionHeaders"`
	TransactionData    string                             `json:"transactionData"`
	TransactionHash    string                             `json:"transactionHash,omitempty"`
	GasPrice           *fftypes.JSONAny                   `json:"gasPrice"`
	PolicyInfo         *fftypes.JSONAny                   `json:"policyInfo"`
	FirstSubmit        *fftypes.FFTime                    `json:"firstSubmit,omitempty"`
	LastSubmit         *fftypes.FFTime                    `json:"lastSubmit,omitempty"`
	Receipt            *ffcapi.TransactionReceiptResponse `json:"receipt,omitempty"`
	ErrorMessage       string                             `json:"errorMessage,omitempty"`
	History            []*ManagedTXUpdate                 `json:"history"`
	Confirmations      []confirmations.BlockInfo          `json:"confirmations,omitempty"`
}

type ReplyType string

const (
	TransactionUpdate        ReplyType = "TransactionUpdate"
	TransactionUpdateSuccess ReplyType = "TransactionSuccess"
	TransactionUpdateFailure ReplyType = "TransactionFailure"
)

type ReplyHeaders struct {
	RequestID string    `json:"requestId"`
	Type      ReplyType `json:"type"`
}

// TransactionUpdateReply add a "headers" structure that allows a processor of websocket
// replies/updates to filter on a standard structure to know how to process the message.
// Extensible to update types in the future. The reply is a small summary of the
// latest status change. Full status for a transaction must be retrieved with
// /transactions/{txid}
type TransactionUpdateReply struct {
	Headers         ReplyHeaders `json:"headers"`
	Status          TxStatus     `json:"status"`
	ProtocolID      string       `json:"protocolId"`
	TransactionHash string       `json:"transactionHash,omitempty"`
}
