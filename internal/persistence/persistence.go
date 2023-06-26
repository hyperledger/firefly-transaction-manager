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

package persistence

import (
	"context"
	"encoding/json"

	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

type SortDirection int

const (
	SortDirectionAscending SortDirection = iota
	SortDirectionDescending
)

// Persistence interface contains all the functions a persistence instance needs to implement.
// Sub set of functions are grouped into sub interfaces to provide a clear view of what
// persistent functions will be made available for each sub components to use after the persistent
// instance is initialized by the manager.
type Persistence interface {
	EventStreamPersistence
	CheckpointPersistence
	ListenerPersistence
	TransactionPersistence
	TransactionHistoryPersistence

	RichQuery() RichQuery      // panics if not supported
	Close(ctx context.Context) // close function is controlled by the manager
}

var EventStreamFilters = &ffapi.QueryFields{
	"sequence":            &ffapi.Int64Field{},
	"id":                  &ffapi.UUIDField{},
	"name":                &ffapi.StringField{},
	"created":             &ffapi.TimeField{},
	"updated":             &ffapi.TimeField{},
	"suspended":           &ffapi.BoolField{},
	"type":                &ffapi.StringField{},
	"errorhandling":       &ffapi.StringField{},
	"batchsize":           &ffapi.Int64Field{},
	"batchtimeout":        &ffapi.Int64Field{},
	"retrytimeout":        &ffapi.Int64Field{},
	"blockedretrytimeout": &ffapi.Int64Field{},
	"webhook":             &ffapi.JSONField{},
	"websocket":           &ffapi.JSONField{},
}

var ListenerFilters = &ffapi.QueryFields{
	"sequence":  &ffapi.Int64Field{},
	"id":        &ffapi.UUIDField{},
	"name":      &ffapi.StringField{},
	"created":   &ffapi.TimeField{},
	"updated":   &ffapi.TimeField{},
	"streamid":  &ffapi.UUIDField{},
	"filters":   &ffapi.JSONField{},
	"options":   &ffapi.JSONField{},
	"signature": &ffapi.StringField{},
	"fromblock": &ffapi.StringField{},
}

var TransactionFilters = &ffapi.QueryFields{
	"sequence":        &ffapi.Int64Field{},
	"id":              &ffapi.StringField{},
	"created":         &ffapi.TimeField{},
	"updated":         &ffapi.TimeField{},
	"status":          &ffapi.StringField{},
	"deleterequested": &ffapi.TimeField{},
	"from":            &ffapi.StringField{},
	"to":              &ffapi.StringField{},
	"nonce":           &ffapi.BigIntField{},
	"gas":             &ffapi.BigIntField{},
	"value":           &ffapi.BigIntField{},
	"gasprice":        &ffapi.JSONField{},
	"transactiondata": &ffapi.StringField{},
	"transactionhash": &ffapi.StringField{},
	"policyinfo":      &ffapi.JSONField{},
	"firstsubmit":     &ffapi.TimeField{},
	"lastsubmit":      &ffapi.TimeField{},
	"errormessage":    &ffapi.StringField{},
}

var ConfirmationFilters = &ffapi.QueryFields{
	"sequence":    &ffapi.Int64Field{},
	"id":          &ffapi.UUIDField{},
	"transaction": &ffapi.StringField{},
	"blocknumber": &ffapi.Int64Field{},
	"blockhash":   &ffapi.StringField{},
	"parenthash":  &ffapi.StringField{},
}

var ReceiptFilters = &ffapi.QueryFields{
	"sequence":         &ffapi.Int64Field{},
	"transaction":      &ffapi.StringField{},
	"created":          &ffapi.TimeField{},
	"updated":          &ffapi.TimeField{},
	"blocknumber":      &ffapi.Int64Field{},
	"transactionindex": &ffapi.BigIntField{},
	"blockhash":        &ffapi.StringField{},
	"success":          &ffapi.BoolField{},
	"protocolid":       &ffapi.StringField{},
	"extrainfo":        &ffapi.JSONField{},
	"contractlocation": &ffapi.JSONField{},
}

var TXHistoryFilters = &ffapi.QueryFields{
	"sequence":       &ffapi.Int64Field{},
	"id":             &ffapi.UUIDField{},
	"transaction":    &ffapi.StringField{},
	"time":           &ffapi.TimeField{},
	"lastoccurrence": &ffapi.TimeField{},
	"substatus":      &ffapi.StringField{},
	"action":         &ffapi.StringField{},
	"count":          &ffapi.Int64Field{},
	"lasterror":      &ffapi.JSONField{},
	"lasterrortime":  &ffapi.TimeField{},
	"lastinfo":       &ffapi.JSONField{},
}

type NextNonceCallback func(ctx context.Context, signer string) (uint64, error)

type RichQuery interface {
	ListStreams(ctx context.Context, filter ffapi.AndFilter) ([]*apitypes.EventStream, *ffapi.FilterResult, error)
	ListListeners(ctx context.Context, filter ffapi.AndFilter) ([]*apitypes.Listener, *ffapi.FilterResult, error)
	ListTransactions(ctx context.Context, filter ffapi.AndFilter) ([]*apitypes.ManagedTX, *ffapi.FilterResult, error)
	ListTransactionConfirmations(ctx context.Context, txID string, filter ffapi.AndFilter) ([]*apitypes.ConfirmationRecord, *ffapi.FilterResult, error)
	ListTransactionHistory(ctx context.Context, txID string, filter ffapi.AndFilter) ([]*apitypes.TXHistoryRecord, *ffapi.FilterResult, error)
	ListStreamListeners(ctx context.Context, streamID *fftypes.UUID, filter ffapi.AndFilter) ([]*apitypes.Listener, *ffapi.FilterResult, error)

	GetStreamFilter(ctx context.Context) *ffapi.QueryFields
	GetListenerFilter(ctx context.Context) *ffapi.QueryFields
	GetTransactionFilter(ctx context.Context) *ffapi.QueryFields
	GetConfirmationFilter(ctx context.Context) *ffapi.QueryFields
	GetTxHistoryFilter(ctx context.Context) *ffapi.QueryFields
}

type CheckpointPersistence interface {
	WriteCheckpoint(ctx context.Context, checkpoint *apitypes.EventStreamCheckpoint) error
	GetCheckpoint(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStreamCheckpoint, error)
	DeleteCheckpoint(ctx context.Context, streamID *fftypes.UUID) error
}

type EventStreamPersistence interface {
	ListStreamsByCreateTime(ctx context.Context, after *fftypes.UUID, limit int, dir SortDirection) ([]*apitypes.EventStream, error) // reverse insertion order
	GetStream(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStream, error)
	WriteStream(ctx context.Context, spec *apitypes.EventStream) error
	DeleteStream(ctx context.Context, streamID *fftypes.UUID) error
}

type ListenerPersistence interface {
	ListListenersByCreateTime(ctx context.Context, after *fftypes.UUID, limit int, dir SortDirection) ([]*apitypes.Listener, error) // reverse insertion
	ListStreamListenersByCreateTime(ctx context.Context, after *fftypes.UUID, limit int, dir SortDirection, streamID *fftypes.UUID) ([]*apitypes.Listener, error)
	GetListener(ctx context.Context, listenerID *fftypes.UUID) (*apitypes.Listener, error)
	WriteListener(ctx context.Context, spec *apitypes.Listener) error
	DeleteListener(ctx context.Context, listenerID *fftypes.UUID) error
}

type TransactionPersistence interface {
	ListTransactionsByCreateTime(ctx context.Context, after *apitypes.ManagedTX, limit int, dir SortDirection) ([]*apitypes.ManagedTX, error)         // reverse create time order
	ListTransactionsByNonce(ctx context.Context, signer string, after *fftypes.FFBigInt, limit int, dir SortDirection) ([]*apitypes.ManagedTX, error) // reverse nonce order within signer
	ListTransactionsPending(ctx context.Context, afterSequenceID string, limit int, dir SortDirection) ([]*apitypes.ManagedTX, error)                 // reverse insertion order, only those in pending state
	GetTransactionByID(ctx context.Context, txID string) (*apitypes.ManagedTX, error)
	GetTransactionByIDWithStatus(ctx context.Context, txID string) (*apitypes.TXWithStatus, error)
	GetTransactionByNonce(ctx context.Context, signer string, nonce *fftypes.FFBigInt) (*apitypes.ManagedTX, error)
	InsertTransactionWithNextNonce(ctx context.Context, tx *apitypes.ManagedTX, lookupNextNonce NextNonceCallback) error
	UpdateTransaction(ctx context.Context, txID string, updates *apitypes.TXUpdates) error
	DeleteTransaction(ctx context.Context, txID string) error

	GetTransactionReceipt(ctx context.Context, txID string) (receipt *ffcapi.TransactionReceiptResponse, err error)
	SetTransactionReceipt(ctx context.Context, txID string, receipt *ffcapi.TransactionReceiptResponse) error

	GetTransactionConfirmations(ctx context.Context, txID string) ([]*apitypes.Confirmation, error)
	AddTransactionConfirmations(ctx context.Context, txID string, clearExisting bool, confirmations ...*apitypes.Confirmation) error
}

type TransactionHistoryPersistence interface {
	AddSubStatusAction(ctx context.Context, txID string, subStatus apitypes.TxSubStatus, action apitypes.TxAction, info *fftypes.JSONAny, err *fftypes.JSONAny) error
}

// Takes a string that might be valid JSON, and returns valid JSON that is either:
// a) The original JSON if it is valid
// b) An escaped string
func JSONOrString(value *fftypes.JSONAny) *fftypes.JSONAny {
	if value == nil {
		return nil
	}

	if json.Valid([]byte(*value)) {
		// Already valid
		return value
	}

	// Quote it as a string
	b, _ := json.Marshal((string)(*value))
	return fftypes.JSONAnyPtrBytes(b)
}
