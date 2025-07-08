// Copyright © 2024 Kaleido, Inc.
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
	// TxStatusSuspended indicates we are not actively doing any work with this transaction right now, until it's resumed to pending again
	TxStatusSuspended TxStatus = "Suspended"
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
	Status          TxSubStatus     `json:"subStatus,omitempty"`
	Action          TxAction        `json:"action,omitempty"`
	FirstOccurrence *fftypes.FFTime `json:"firstOccurrence"`
	LastOccurrence  *fftypes.FFTime `json:"lastOccurrence"`
	Count           int             `json:"count"`
}

// TxAction is an action taken while attempting to progress a transaction between sub-states
type TxAction string

const (
	// TxActionStateTransition is a special value used for state transition entries, which are created using SetSubStatus
	TxActionStateTransition TxAction = "StateTransition"
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
	Time            *fftypes.FFTime  `json:"time"`
	Action          TxAction         `json:"action"`
	LastOccurrence  *fftypes.FFTime  `json:"lastOccurrence,omitempty"`
	OccurrenceCount int              `json:"count"` // serialized as count for historical reasons
	LastError       *fftypes.JSONAny `json:"lastError,omitempty"`
	LastErrorTime   *fftypes.FFTime  `json:"lastErrorTime,omitempty"`
	LastInfo        *fftypes.JSONAny `json:"lastInfo,omitempty"`
}

// TXHistoryRecord are the sequential persisted records, which might be state transitions, or actions within the current state.
// Note LevelDB does not use this, as the []*TxHistoryStateTransitionEntry array is maintained directly on the large single JSON document
type TXHistoryRecord struct {
	ID            *fftypes.UUID `json:"id"`          // unique identifier for this entry
	TransactionID string        `json:"transaction"` // owning transaction
	SubStatus     TxSubStatus   `json:"subStatus"`
	TxHistoryActionEntry
}

func (r *TXHistoryRecord) GetID() string {
	return r.ID.String()
}

func (r *TXHistoryRecord) SetCreated(t *fftypes.FFTime) {
	r.LastOccurrence = t
}

func (r *TXHistoryRecord) SetUpdated(_ *fftypes.FFTime) {}

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
	ID              string          `json:"id"`
	Created         *fftypes.FFTime `json:"created"`
	Updated         *fftypes.FFTime `json:"updated"`
	Status          TxStatus        `json:"status"`
	DeleteRequested *fftypes.FFTime `json:"deleteRequested,omitempty"`
	SequenceID      string          `json:"sequenceId,omitempty"`
	ffcapi.TransactionHeaders
	GasPrice                     *fftypes.JSONAny           `json:"gasPrice"`
	TransactionData              string                     `json:"transactionData"`
	TransactionHash              string                     `json:"transactionHash,omitempty"`
	PolicyInfo                   *fftypes.JSONAny           `json:"policyInfo"`
	FirstSubmit                  *fftypes.FFTime            `json:"firstSubmit,omitempty"`
	LastSubmit                   *fftypes.FFTime            `json:"lastSubmit,omitempty"`
	ErrorMessage                 string                     `json:"errorMessage,omitempty"`
	DeprecatedTransactionHeaders *ffcapi.TransactionHeaders `json:"transactionHeaders,omitempty"` // LevelDB only: for lost-in-time historical reasons we duplicate these fields at the base too on this query structure
}

func (mtx *ManagedTX) GetID() string {
	return mtx.ID
}

func (mtx *ManagedTX) SetCreated(t *fftypes.FFTime) {
	mtx.Created = t
}

func (mtx *ManagedTX) SetUpdated(t *fftypes.FFTime) {
	mtx.Updated = t
}

func (mtx *ManagedTX) SetSequence(i int64) {
	// For SQL we set the sequence to be a number generated by the DB (handled by the CRUD utilities in firefly-common)
	mtx.SequenceID = fmt.Sprintf("%.12d", i)
}

// TXUpdates specifies a set of updates that are possible on the base structure.
//
// Any non-nil fields will be set.
// Sub-objects are set as a whole, apart from TransactionHeaders where each field
// is considered and stored individually.
// JSONAny fields can be set explicitly to null using fftypes.NullString
//
// This is the update interface for the policy engine to update base status on the
// transaction object.
//
// There are separate setter functions for fields that depending on the persistence
// mechanism might be in separate tables - including History, Receipt, and Confirmations
type TXUpdates struct {
	Status          *TxStatus         `json:"status"`
	DeleteRequested *fftypes.FFTime   `json:"deleteRequested,omitempty"`
	From            *string           `json:"from,omitempty"`
	To              *string           `json:"to,omitempty"`
	Nonce           *fftypes.FFBigInt `json:"nonce,omitempty"`
	Gas             *fftypes.FFBigInt `json:"gas,omitempty"`
	Value           *fftypes.FFBigInt `json:"value,omitempty"`
	GasPrice        *fftypes.JSONAny  `json:"gasPrice"`
	TransactionData *string           `json:"transactionData"`
	TransactionHash *string           `json:"transactionHash,omitempty"`
	PolicyInfo      *fftypes.JSONAny  `json:"policyInfo"`
	FirstSubmit     *fftypes.FFTime   `json:"firstSubmit,omitempty"`
	LastSubmit      *fftypes.FFTime   `json:"lastSubmit,omitempty"`
	ErrorMessage    *string           `json:"errorMessage,omitempty"`
}

type TXCompletion struct {
	Sequence *int64          `json:"sequence,omitempty"`
	ID       string          `json:"id"`
	Time     *fftypes.FFTime `json:"created"`
	Status   TxStatus        `json:"status"`
}

func (txu *TXUpdates) Merge(txu2 *TXUpdates) {
	if txu2.Status != nil {
		txu.Status = txu2.Status
	}
	if txu2.DeleteRequested != nil {
		txu.DeleteRequested = txu2.DeleteRequested
	}
	if txu2.From != nil {
		txu.From = txu2.From
	}
	if txu2.To != nil {
		txu.To = txu2.To
	}
	if txu2.Nonce != nil {
		txu.Nonce = txu2.Nonce
	}
	if txu2.Gas != nil {
		txu.Gas = txu2.Gas
	}
	if txu2.Value != nil {
		txu.Value = txu2.Value
	}
	if txu2.GasPrice != nil {
		txu.GasPrice = txu2.GasPrice
	}
	if txu2.TransactionData != nil {
		txu.TransactionData = txu2.TransactionData
	}
	if txu2.TransactionHash != nil {
		txu.TransactionHash = txu2.TransactionHash
	}
	if txu2.PolicyInfo != nil {
		txu.PolicyInfo = txu2.PolicyInfo
	}
	if txu2.FirstSubmit != nil {
		txu.FirstSubmit = txu2.FirstSubmit
	}
	if txu2.LastSubmit != nil {
		txu.LastSubmit = txu2.LastSubmit
	}
	if txu2.ErrorMessage != nil {
		txu.ErrorMessage = txu2.ErrorMessage
	}
}

// TXWithStatus is a convenience object that fetches all data about a transaction into one
// large JSON payload (with limits on certain parts, such as the history entries).
// Note that in LevelDB persistence this is the stored form of the single document object.
type TXWithStatus struct {
	*ManagedTX
	Receipt                  *ffcapi.TransactionReceiptResponse `json:"receipt,omitempty"`
	Confirmations            []*ffcapi.Confirmation             `json:"confirmations,omitempty"`
	DeprecatedHistorySummary []*TxHistorySummaryEntry           `json:"historySummary,omitempty"` // LevelDB only: maintains a summary to retain data while limiting single JSON payload size
	History                  []*TxHistoryStateTransitionEntry   `json:"history,omitempty"`
}

func (mtx *ManagedTX) Namespace(ctx context.Context) string {
	namespace, _, _ := fftypes.ParseNamespacedUUID(ctx, mtx.ID)
	return namespace
}

type BlockInfo struct {
	BlockNumber       fftypes.FFuint64 `json:"blockNumber"`
	BlockHash         string           `json:"blockHash"`
	ParentHash        string           `json:"parentHash"`
	TransactionHashes []string         `json:"transactionHashes,omitempty"`
}

func (b *BlockInfo) ToConfirmation() *ffcapi.Confirmation {
	return &ffcapi.Confirmation{
		BlockNumber: b.BlockNumber,
		BlockHash:   b.BlockHash,
		ParentHash:  b.ParentHash,
	}
}

func BlockInfosToConfirmations(blocks []*BlockInfo) []*ffcapi.Confirmation {
	confirmations := make([]*ffcapi.Confirmation, len(blocks))
	for i, b := range blocks {
		confirmations[i] = b.ToConfirmation()
	}
	return confirmations
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
	Headers          ReplyHeaders     `json:"headers"`
	Status           TxStatus         `json:"status"`
	ProtocolID       string           `json:"protocolId"`
	TransactionHash  string           `json:"transactionHash,omitempty"`
	ContractLocation *fftypes.JSONAny `json:"contractLocation,omitempty"`
}

// ManagedTransactionEventType is a enum type that contains all types of transaction process events
// that a transaction handler emits.
type ManagedTransactionEventType int

const (
	ManagedTXProcessSucceeded ManagedTransactionEventType = iota
	ManagedTXProcessFailed
	ManagedTXDeleted
	ManagedTXTransactionHashAdded
	ManagedTXTransactionHashRemoved
)

type ManagedTransactionEvent struct {
	Type    ManagedTransactionEventType
	Tx      *ManagedTX
	Receipt *ffcapi.TransactionReceiptResponse
	// ReceiptHandler can be passed on the event as a closure with extra variables
	ReceiptHandler func(ctx context.Context, txID string, receipt *ffcapi.TransactionReceiptResponse) error
	// ConfirmationHandler can be passed on the event as a closure with extra variables
	ConfirmationHandler func(ctx context.Context, txID string, notification *ffcapi.ConfirmationsNotification) error
}

type ReceiptRecord struct {
	TransactionID string          `json:"transaction"` // owning transaction
	Created       *fftypes.FFTime `json:"created"`
	Updated       *fftypes.FFTime `json:"updated"`
	*ffcapi.TransactionReceiptResponse
}

func (r *ReceiptRecord) GetID() string {
	return r.TransactionID
}

func (r *ReceiptRecord) SetCreated(t *fftypes.FFTime) {
	r.Created = t
}

func (r *ReceiptRecord) SetUpdated(t *fftypes.FFTime) {
	r.Updated = t
}
