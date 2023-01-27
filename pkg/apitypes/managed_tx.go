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
	"encoding/json"
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

type TxHistoryRecord struct {
	Time    *fftypes.FFTime  `json:"time"`
	Status  TxSubStatus      `json:"subStatus"`
	Actions []*TxActionEntry `json:"actions"`
	Info    string           `json:"info,omitempty"`
	Error   string           `json:"error,omitempty"`
}

type TxHistorySummaryRecord struct {
	FirstOccurrence *fftypes.FFTime `json:"firstOccurence"`
	Status          TxSubStatus     `json:"subStatus"`
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

// An action taken in order to progress a transaction, e.g. retrieve gas price from an oracle
type TxActionEntry struct {
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
	History            []*TxHistoryRecord                 `json:"history,omitempty"`
	HistorySummary     []*TxHistorySummaryRecord          `json:"historySummary,omitempty"`
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

func (mtx *ManagedTX) CurrentSubStatus(ctx context.Context) *TxHistoryRecord {
	if len(mtx.History) > 0 {
		return mtx.History[len(mtx.History)-1]
	}
	return nil
}

// Transaction sub-status entries can be added for a given transaction so a caller
// can see discrete steps in a transaction moving to confirmation on the blockchain.
// For example a transaction might have a sub-status of "Stale" if a transaction has
// been in pending state for a given period of time. In order to progress the transaction
// while it's in a given sub-status, certain actions might be taken (such as retrieving
// the latest gas price for the chain). See AddSubStatusAction(). Since a transaction
// might go through many sub-status changes before being confirmed on chain the list of
// entries is capped at the configured number and FIFO approach used to keep within that cap.
func (mtx *ManagedTX) AddSubStatus(ctx context.Context, subStatus TxSubStatus) {
	// See if the status being added is the same as the current status. If so we won't create
	// a new record, just increment the total count
	if len(mtx.History) > 0 {
		if mtx.History[len(mtx.History)-1].Status == subStatus {
			return
		}
		log.L(ctx).Debugf("Entered sub-status %s", subStatus)
	}

	// Do we need to remove the oldest entry to make space for this one?
	if len(mtx.History) > 50 { // TODO - get from config
		mtx.History = mtx.History[1:]
	} else {
		// If this is a change in status add a new record
		newStatus := &TxHistoryRecord{
			Time:    fftypes.Now(),
			Status:  subStatus,
			Actions: make([]*TxActionEntry, 0),
		}
		mtx.History = append(mtx.History, newStatus)

		// As was as detailed sub-status records (which might be a long list and early entries
		// get purged at some point) we keep a separate list of all the discrete types of sub-status
		// we've ever seen for this transaction along with a count of them. This means an early sub-status
		// (e.g. "queued") followed by 100s of different sub-status types will still be recorded
		newHistorySummary := true
		for _, statusType := range mtx.HistorySummary {
			if statusType.Status == subStatus {
				// Just increment the counter
				statusType.Count++
				newHistorySummary = false
				break
			}
		}

		if newHistorySummary {
			if len(mtx.HistorySummary) < 50 { // TODO - get from config
				mtx.HistorySummary = append(mtx.HistorySummary, &TxHistorySummaryRecord{Status: subStatus, Count: 1, FirstOccurrence: fftypes.Now()})
			} else {
				log.L(ctx).Warnf("Reached maximum number of history summary records. New summary status will be not be recorded.")
			}
		}
	}
}

// Make sure the provided value can be serialised to JSON
func ensureValidJSON(value *fftypes.JSONAny) *fftypes.JSONAny {
	if json.Valid([]byte(*value)) {
		// Already valid
		return value
	}

	// Convert to hex and wrap in a valid struct
	hex := fmt.Sprintf("%x", []byte(*value))
	return fftypes.JSONAnyPtr(`{"invalidJson":"` + hex + `"}`)
}

// When a transaction is in a given sub-status (e.g. "Stale") the blockchain connector
// may perform certain actions to move it out of the status. For example it might
// retrieve the current gas price for the chain. TxAction's represent an action taken
// while in a given sub-status. In order to limit the number of TxAction entries in
// a TxSubStatusEntry each action has a count of the number of occurrences and a
// latest timestamp to indicate when it was last executed. There is a last error field
// which can be used to indicate the most recent error that occurred, for example an
// HTTP 4xx return code from a gas oracle. There is also an information field to record
// arbitrary data about the action, for example the gas price retrieved from an oracle.
func (mtx *ManagedTX) AddSubStatusAction(ctx context.Context, action TxAction, info *fftypes.JSONAny, error *fftypes.JSONAny) {
	// An action always exists within a sub-status. If a sub-status hasn't been recorded yet we don't record the action
	if len(mtx.History) > 0 {

		// See if this action exists in the list already since we only want to update the single entry, not
		// add a new one
		currentSubStatus := mtx.History[len(mtx.History)-1]
		for _, entry := range currentSubStatus.Actions {
			if entry.Action == action {
				entry.Count++
				entry.LastOccurrence = fftypes.Now()

				if error != nil {
					entry.LastError = ensureValidJSON(error)
					entry.LastErrorTime = fftypes.Now()
				}

				if info != nil {
					entry.LastInfo = ensureValidJSON(info)
				}
				return
			}
		}

		// This action hasn't been recorded yet in this sub-status. Add a new entry for it.
		if len(currentSubStatus.Actions) >= 50 { // TODO - get from config
			log.L(ctx).Warn("Number of unique sub-status actions. New action detail will not be recorded.")
		} else {
			// If this is an entirely new status add it to the list
			newAction := &TxActionEntry{
				Time:           fftypes.Now(),
				Action:         action,
				LastOccurrence: fftypes.Now(),
				Count:          1,
			}

			if error != nil {
				newAction.LastError = ensureValidJSON(error)
				newAction.LastErrorTime = fftypes.Now()
			}

			if info != nil {
				newAction.LastInfo = ensureValidJSON(info)
			}

			currentSubStatus.Actions = append(currentSubStatus.Actions, newAction)
		}
	}
}
