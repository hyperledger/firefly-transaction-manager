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

// NonceAllocation is a mapping from an address + nonce, to a managed transaction.
// These are stored such that we can easily find the next nonce to assign to a managed transaction
//
// ** Stored first **
//   Because there's a non-zero chance we crash after writing this, and before writing the ManagedTX
//   record. The nonce-allocation code must read the most recently written nonce allocation for the
//   signer and check the TX object has been written. If not, it will re-allocate (cleaning up the
//   in-flight record if it exists)
//
type NonceAllocation struct {
	Signer string `json:"signer"`
	Nonce  int64  `json:"nonce"`
	TX     string `json:"tx"`
}

// InflightTX is a UUIDv1 (so ordered) entry, that refers to an in-flight transaction that needs to be tracked.
// These are deleted when the transaction is complete.
//
// ** Stored second **
//   This means we might have a nonce+inflight record, but not have written the managed TX.
//   The code that reads the in-flight TX list looks for this scenario, and clean up the orphaned in-flight record
//   (if it gets to it before)
type InflightTX struct {
	ID      *fftypes.UUID   `json:"id"`
	TX      string          `json:"tx"`
	Created *fftypes.FFTime `json:"created"`
}

// ManagedTX is the structure stored for each new transaction request, using the external ID of the operation
//
// ** Stored last **
//   This is persisted (along with the two objects above) before we reply to the API call to initiate a transaction,
//   and is updated as the transaction progresses onto the chain.
type ManagedTX struct {
	ID              string                             `json:"id"`
	Status          TxStatus                           `json:"status"`
	Nonce           *fftypes.FFBigInt                  `json:"nonce"` // persisted separately
	Gas             *fftypes.FFBigInt                  `json:"gas"`
	TransactionHash string                             `json:"transactionHash,omitempty"`
	TransactionData string                             `json:"transactionData,omitempty"`
	GasPrice        *fftypes.JSONAny                   `json:"gasPrice"`
	PolicyInfo      *fftypes.JSONAny                   `json:"policyInfo"`
	FirstSubmit     *fftypes.FFTime                    `json:"firstSubmit,omitempty"`
	LastSubmit      *fftypes.FFTime                    `json:"lastSubmit,omitempty"`
	Request         *TransactionRequest                `json:"request,omitempty"`
	Receipt         *ffcapi.TransactionReceiptResponse `json:"receipt,omitempty"`
	ErrorHistory    []*ManagedTXError                  `json:"errorHistory"`
	Confirmations   []confirmations.BlockInfo          `json:"confirmations,omitempty"`
}
