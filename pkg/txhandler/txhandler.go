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

package txhandler

import (
	"context"

	"github.com/hyperledger/firefly-transaction-manager/internal/metrics"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

type TransactionPersistence interface {
	persistence.TransactionPersistence
}

type RichQuery interface {
	persistence.RichQuery
}

type TransactionHistoryPersistence interface {
	persistence.TransactionHistoryPersistence
}

type TransactionMetrics interface {
	metrics.TransactionHandlerMetrics
}

type Toolkit struct {
	// Connector toolkit contains methods to interact with the plugged-in JSON-RPC endpoint of a Blockchain network
	Connector ffcapi.API

	// Transaction History toolkit contains methods to easily manage and set historical status of a Managed Transaction
	TXHistory TransactionHistoryPersistence

	// TransactionPersistence toolkit contains methods to persist Managed Transaction objects into the plugged-in persistence service
	TXPersistence TransactionPersistence

	// When a rich-query enabled Database is available (PSQL) the full rich query interface for all objects is passed to the policy engine.
	// If not available, this will be nil.
	RichQuery RichQuery

	// Metric toolkit contains methods to emit Managed Transaction specific metrics using the plugged-in metrics service
	MetricsManager TransactionMetrics

	// Event Handler toolkit contains methods to handle a defined set of events when processing managed transactions
	EventHandler ManagedTxEventHandler
}

// Handler checks received transaction process events and dispatch them to an event
// manager accordingly.
type ManagedTxEventHandler interface {
	HandleEvent(ctx context.Context, e apitypes.ManagedTransactionEvent) error
}

// Transaction Handler owns the lifecycle of ManagedTransaction records
// Transaction Manager delegates all transaction specific operations to transaction apart from the triggers (REST API call for actions,
// Event stream for events) of those operations. This design allows the Transaction Handler to apply customized logic at different stages
// of transaction life cycles.
// The Transaction Handler interface consists of x important responsibilities:
// 1. Functions for handling its own lifecycle. (e.g. Init, Start)
//
//  2. Functions for handling events:
//     When implementing functions in this category, developers need to keep concurrency in mind. Because multiple invocations of a single
//     function can be process in parallel in any given time, it's recommended to use channels to ensure they are not blocking each other.
//     - inbound events: these events are received by transaction handler
//     a. instructional events: transaction handler need to implement the logic to make those event happen on blockchain. (e.g. creating
//     a new transaction)
//     b. informational events: transaction handler need to update its own record when an event has already happened to a related
//     blockchain transaction of its managed transaction
//     - outbound events: these events are emitted by transaction handler. apitypes.ManagedTransactionEventType contains a list of
//     managed transaction events that can be emitted by transaction handler.
type TransactionHandler interface {
	// Lifecycle functions

	// Init - setting a set of initialized toolkit plugins in the constructed transaction handler object. Safe checks & initialization
	//        can take place inside this function as well. It also enables toolkit plugins to be able to embed a reference to its parent
	//        transaction handler instance.
	Init(ctx context.Context, toolkit *Toolkit)

	// Start - starting the transaction handler to handle inbound events.
	// It takes in a context, of which upon cancellation will stop the transaction handler.
	// It returns a read-only channel. When this channel gets closed, it indicates transaction handler has been stopped gracefully.
	// It returns an error when failed to start.
	Start(ctx context.Context) (done <-chan struct{}, err error)

	// Event handling functions
	// Instructional events:
	// HandleNewTransaction - handles event of adding new transactions onto blockchain
	HandleNewTransaction(ctx context.Context, txReq *apitypes.TransactionRequest) (mtx *apitypes.ManagedTX, err error)
	// HandleNewContractDeployment - handles event of adding new smart contract deployment onto blockchain
	HandleNewContractDeployment(ctx context.Context, txReq *apitypes.ContractDeployRequest) (mtx *apitypes.ManagedTX, err error)
	// HandleCancelTransaction - handles event of cancelling a managed transaction
	HandleCancelTransaction(ctx context.Context, txID string) (mtx *apitypes.ManagedTX, err error)
	// HandleSuspendTransaction - handles event of suspending a managed transaction
	HandleSuspendTransaction(ctx context.Context, txID string) (mtx *apitypes.ManagedTX, err error)
	// HandleResumeTransaction - handles event of resuming a suspended managed transaction
	HandleResumeTransaction(ctx context.Context, txID string) (mtx *apitypes.ManagedTX, err error)

	// Informational events:
	// HandleTransactionConfirmations - handles confirmations of blockchain transactions for a managed transaction
	HandleTransactionConfirmations(ctx context.Context, txID string, notification *apitypes.ConfirmationsNotification) (err error)
	// HandleTransactionReceiptReceived - handles receipt of blockchain transactions for a managed transaction
	HandleTransactionReceiptReceived(ctx context.Context, txID string, receipt *ffcapi.TransactionReceiptResponse) (err error)
}
