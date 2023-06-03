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
	// TODO: complete
}

var ListenerFilters = &ffapi.QueryFields{
	// TODO: complete
}

var TransactionFilters = &ffapi.QueryFields{
	// TODO: complete
}

type RichQuery interface {
	ListStreams(ctx context.Context, filter ffapi.Filter) ([]*apitypes.EventStream, *ffapi.FilterResult, error)
	ListListeners(ctx context.Context, filter ffapi.Filter) ([]*apitypes.Listener, *ffapi.FilterResult, error)
	ListTransactions(ctx context.Context, filter ffapi.Filter) ([]*apitypes.ManagedTX, *ffapi.FilterResult, error)
	ListStreamListeners(ctx context.Context, streamID *fftypes.UUID, filter ffapi.Filter) ([]*apitypes.Listener, *ffapi.FilterResult, error)
	ListTransactionHistory(ctx context.Context, streamID *fftypes.UUID, filter ffapi.Filter) ([]*apitypes.Listener, *ffapi.FilterResult, error)
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
	GetTransactionByIDWithHistory(ctx context.Context, txID string) (*apitypes.TXWithStatus, error)
	GetTransactionByNonce(ctx context.Context, signer string, nonce *fftypes.FFBigInt) (*apitypes.ManagedTX, error)
	InsertTransaction(ctx context.Context, tx *apitypes.ManagedTX) error
	UpdateTransaction(ctx context.Context, txID string, updates *apitypes.TXUpdates) error
	DeleteTransaction(ctx context.Context, txID string) error

	GetTransactionReceipt(ctx context.Context, txID string) (receipt *ffcapi.TransactionReceiptResponse, err error)
	SetTransactionReceipt(ctx context.Context, txID string, receipt *ffcapi.TransactionReceiptResponse) error

	GetTransactionConfirmations(ctx context.Context, txID string) ([]apitypes.BlockInfo, error)
	AddTransactionConfirmations(ctx context.Context, txID string, clearExisting bool, confirmations ...apitypes.BlockInfo) error
}

type TransactionHistoryPersistence interface {
	SetSubStatus(ctx context.Context, txID string, subStatus apitypes.TxSubStatus) error
	AddSubStatusAction(ctx context.Context, txID string, action apitypes.TxAction, info *fftypes.JSONAny, err *fftypes.JSONAny) error
}
