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

type ManagedTXError struct {
	Time   *fftypes.FFTime    `json:"time"`
	Error  string             `json:"error,omitempty"`
	Mapped ffcapi.ErrorReason `json:"mapped,omitempty"`
}

type ManagedTXHeaders struct {
	RequestID    string          `json:"requestId"`
	TimeReceived *fftypes.FFTime `json:"timeReceived"`
	LastUpdate   *fftypes.FFTime `json:"lastUpdate"`
}

// ManagedTX is the structure stored for each new transaction request, using the external ID of the operation
//
// Indexing:
//   Multiple index collection are stored for the managed transactions, to allow them to be managed including:
//
//   - Nonce allocation: this is a critical index, and why cleanup is so important (mentioned below).
//     We use this index to determine the next nonce to assign to a given signing key.
//   - Created time: a timestamp ordered index for the transactions for convenient ordering.
//     the key includes the ID of the TX for uniqueness.
//   - Pending sequence: An entry in this index only exists while the transaction is pending, and is
//     ordered by a UUIDv1 sequence allocated to each entry.
//
// Index cleanup after partial write:
//   - All indexes are stored before the TX itself.
//   - When listing back entries, the persistence layer will automatically clean up indexes if the underlying
//     TX they refer to is not available. For this reason the index records are written first.
type ManagedTX struct {
	Headers            ManagedTXHeaders                   `json:"headers"`
	Status             TxStatus                           `json:"status"`
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
	ErrorHistory       []*ManagedTXError                  `json:"errorHistory"`
	Confirmations      []confirmations.BlockInfo          `json:"confirmations,omitempty"`
}
