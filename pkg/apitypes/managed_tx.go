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

// TxSubStatus is an intermediate status a transaction may go through
type TxSubStatus string

const (
	// TxSubStatusReceived indicates the transaction has been received by the connector
	TxSubStatusReceived TxSubStatus = "Received"
	// TxSubStatusStale indicates the transaction is now in stale
	TxSubStatusStale TxSubStatus = "Stale"
	// TxSubStatusTracking indicates we are tracking progress of the transaction
	TxSubStatusTracking TxSubStatus = "Tracking"
	// TxSubStatusConfirmed indicates we have confirmed that the transaction has been fully processed
	TxSubStatusConfirmed TxSubStatus = "Confirmed"
	// TxSubStatusFailed indicates we have failed to process the transaction and it will no longer be tracked
	TxSubStatusFailed TxSubStatus = "Failed"
)

// TxHistoryStateTransitionEntry represents a state that the policy engine that manages transaction submission has entered,
// and a list of the actions attempted within that state in order to attempt to move to the next state.
type TxHistoryStateTransitionEntry struct {
	Status  TxSubStatus             `json:"subStatus"` // the subStatus we entered
	Time    *fftypes.FFTime         `json:"time"`      // the time we transitioned to this subStatus
	Actions []*TxHistoryActionEntry `json:"actions"`   // the unique actions we attempted while in this sub-status
}

// TxHistorySummaryEntry records summarize the transaction history, by recording the number of times each
// subStatus was entered. Because the detailed history might wrap, this means we can retain some basic
// information about the complete history of the transaction beyond the life of the individual history records.
type TxHistorySummaryEntry struct {
	Status          TxSubStatus     `json:"subStatus"`
	FirstOccurrence *fftypes.FFTime `json:"firstOccurrence"`
	Count           int             `json:"count"`
}

// TxAction is an action taken while attempting to progress a transaction between sub-states
type TxAction string

const (
	// TxActionAssignNonce indicates that a nonce has been assigned to the transaction
	TxActionAssignNonce TxAction = "AssignNonce"
	// TxActionRetrieveGasPrice indicates the operation is getting a gas price
	TxActionRetrieveGasPrice TxAction = "RetrieveGasPrice"
	// TxActionTimeout indicates that the transaction has timed out may need intervention to progress it
	TxActionTimeout TxAction = "Timeout"
	// TxActionSubmitTransaction indicates that the transaction has been submitted
	TxActionSubmitTransaction TxAction = "SubmitTransaction"
	// TxActionReceiveReceipt indicates that we have received a receipt for the transaction
	TxActionReceiveReceipt TxAction = "ReceiveReceipt"
	// TxActionConfirmTransaction indicates that the transaction has been confirmed
	TxActionConfirmTransaction TxAction = "Confirm"
)

// An action taken in order to progress a transaction, e.g. retrieve gas price from an oracle.
// Actions are retaining similarly to the TxHistorySummaryEntry records, where we have a finite
// list based on the action name. Actions are only added to the list once, then updated
// when they occur multiple times. So if we are retrying the same set of actions over and over
// again the list of actions does not grow.
type TxHistoryActionEntry struct {
	Time           *fftypes.FFTime  `json:"time"`
	Action         TxAction         `json:"action"`
	LastOccurrence *fftypes.FFTime  `json:"lastOccurrence"`
	Count          int              `json:"count"`
	LastError      *fftypes.JSONAny `json:"lastError,omitempty"`
	LastErrorTime  *fftypes.FFTime  `json:"lastErrorTime,omitempty"`
	LastInfo       *fftypes.JSONAny `json:"lastInfo,omitempty"`
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
	History            []*TxHistoryStateTransitionEntry   `json:"history,omitempty"`
	HistorySummary     []*TxHistorySummaryEntry           `json:"historySummary,omitempty"`
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
