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

package apitypes

import (
	"context"
	"fmt"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/log"
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

// TxSubStatus is an intermediate status a transaction may go through
type TxSubStatus string

const (
	// TxSubStatusRetrievingGasPrice indicates the operation is getting a gas price
	TxSubStatusRetrievingGasPrice TxSubStatus = "RetrievingGasPrice"
	// TxSubStatusRetrievedGasPrice indicates the operation has had gas price calculated for it
	TxSubStatusRetrievedGasPrice TxSubStatus = "RetrievedGasPrice"
	// TxSubStatusSubmitting indicates that the transaction is about to be submitted
	TxSubStatusSubmitting TxSubStatus = "Submitting"
	// TxSubStatusSubmitted indicates that the transaction has been submitted to the JSON/RPC endpoint
	TxSubStatusSubmitted TxSubStatus = "Submitted"
	// TxSubStatusReceivedReceipt indicates that we have received a receipt for the transaction
	TxSubStatusReceivedReceipt TxSubStatus = "ReceivedReceipt"
	// TxSubStatusConfirmed indicates that we have met the required number of confirmations for the transaction
	TxSubStatusConfirmed TxSubStatus = "Confirmed"
)

type TxSubStatusEntry struct {
	Time           *fftypes.FFTime `json:"time"`
	LastOccurrence *fftypes.FFTime `json:"lastOccurrence"`
	Status         TxSubStatus     `json:"subStatus"`
	Count          int             `json:"count"`
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
	SubStatusHistory   []*TxSubStatusEntry                `json:"subStatusHistory"`
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

// Transaction sub-status entries can be added for a given transaction so a caller
// can see discrete steps in a transaction moving to confirmation on the blockchain.
// There only exists a single entry in the list for each unique sub-status type. If
// a transaction goes through the same sub-status more than once (for example if it
// has gas recalculated for it more than once) then the "Count" and "LastOccurence" fields
// should be updated accordingly. This design allows a caller to see when the most recent
// sub-status changes took place and if necessary, to order them by time, but prevents
// the potential for the sub-status list to grow indefinitely.
func (mtx *ManagedTX) AddSubStatus(ctx context.Context, subStatus TxSubStatus) {
	// See if this status exists in the list already
	for _, entry := range mtx.SubStatusHistory {
		if entry.Status == subStatus {
			entry.Count++
			entry.LastOccurrence = fftypes.Now()
			return
		}
	}

	// Prevent crazy run-away situations by limiting the number of unique sub-status
	// types we will keep
	if len(mtx.SubStatusHistory) >= 50 {
		log.L(ctx).Warn("Number of unique sub-status types reached. Some status detail may be lost.")
	} else {
		// If this is an entirely new status add it to the list
		newStatus := &TxSubStatusEntry{
			Time:           fftypes.Now(),
			LastOccurrence: fftypes.Now(),
			Count:          1,
			Status:         subStatus,
		}
		mtx.SubStatusHistory = append(mtx.SubStatusHistory, newStatus)
	}
}
