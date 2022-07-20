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

package ffcapi

import (
	"context"
	"fmt"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
)

// API is the interface to the blockchain specific connector, from the FFTM server and policy engine.
//
// The functions follow a consistent pattern of request/response objects, to allow extensibility of the
// inputs/outputs with minimal code change to existing connector implementations.
type API interface {

	// BlockInfoByHash gets block information using the hash of the block
	BlockInfoByHash(ctx context.Context, req *BlockInfoByHashRequest) (*BlockInfoByHashResponse, ErrorReason, error)

	// BlockInfoByNumber gets block information from the specified position (block number/index) in the canonical chain currently known to the local node
	BlockInfoByNumber(ctx context.Context, req *BlockInfoByNumberRequest) (*BlockInfoByNumberResponse, ErrorReason, error)

	// NextNonceForSigner is used when there are no outstanding transactions for a given signing identity, to determine the next nonce to use for submission of a transaction
	NextNonceForSigner(ctx context.Context, req *NextNonceForSignerRequest) (*NextNonceForSignerResponse, ErrorReason, error)

	// GasPriceEstimate provides a blockchain specific gas price estimate
	GasPriceEstimate(ctx context.Context, req *GasPriceEstimateRequest) (*GasPriceEstimateResponse, ErrorReason, error)

	// QueryInvoke executes a method on a blockchain smart contract, which might execute Smart Contract code, but does not affect the blockchain state.
	QueryInvoke(ctx context.Context, req *QueryInvokeRequest) (*QueryInvokeResponse, ErrorReason, error)

	// TransactionReceipt queries to see if a receipt is available for a given transaction hash
	TransactionReceipt(ctx context.Context, req *TransactionReceiptRequest) (*TransactionReceiptResponse, ErrorReason, error)

	// TransactionPrepare validates transaction inputs against the supplied schema/ABI and performs any binary serialization required (prior to signing) to encode a transaction from JSON into the native blockchain format
	TransactionPrepare(ctx context.Context, req *TransactionPrepareRequest) (*TransactionPrepareResponse, ErrorReason, error)

	// TransactionSend combines a previously prepared encoded transaction, with a current gas price, and submits it to the transaction pool of the blockchain for mining
	TransactionSend(ctx context.Context, req *TransactionSendRequest) (*TransactionSendResponse, ErrorReason, error)

	// EventStreamStart starts an event stream with an initial set of listeners (which might be empty), a channel to deliver events, and a context that will close to stop the stream
	EventStreamStart(ctx context.Context, req *EventStreamStartRequest) (*EventStreamStartResponse, ErrorReason, error)

	// EventStreamStopped informs a connector that an event stream has been requested to stop, and the context has been cancelled. So the state associated with it can be removed (and a future start of the same ID can be performed)
	EventStreamStopped(ctx context.Context, req *EventStreamStoppedRequest) (*EventStreamStoppedResponse, ErrorReason, error)

	// EventListenerVerifyOptions validates the configuration options for a listener, applying any defaults needed by the connector, and returning the update options for FFTM to persist
	EventListenerVerifyOptions(ctx context.Context, req *EventListenerVerifyOptionsRequest) (*EventListenerVerifyOptionsResponse, ErrorReason, error)

	// EventListenerAdd begins/resumes listening on set of events that must be consistently ordered. Blockchain specific signatures of the events are included, along with initial conditions (initial block number etc.), and the last stored checkpoint (if any)
	EventListenerAdd(ctx context.Context, req *EventListenerAddRequest) (*EventListenerAddResponse, ErrorReason, error)

	// EventListenerRemove ends listening on a set of events previous started
	EventListenerRemove(ctx context.Context, req *EventListenerRemoveRequest) (*EventListenerRemoveResponse, ErrorReason, error)

	// EventListenerHWM queries the current high water mark checkpoint for a listener. Called at regular intervals when there are no events in flight for a listener, to ensure checkpoint are written regularly even when there is no activity
	EventListenerHWM(ctx context.Context, req *EventListenerHWMRequest) (*EventListenerHWMResponse, ErrorReason, error)

	// EventStreamNewCheckpointStruct used during checkpoint restore, to get the specific into which to restore the JSON bytes
	EventStreamNewCheckpointStruct() EventListenerCheckpoint
}

type BlockHashEvent struct {
	BlockHashes  []string `json:"blockHash"`              // zero or more hashes (can be nil)
	GapPotential bool     `json:"gapPotential,omitempty"` // when true, the caller cannot be sure if blocks have been missed (use on reconnect of a websocket for example)
}

// EventID are the set of required fields an FFCAPI compatible connector needs to map to the underlying blockchain constructs, to uniquely identify an event
type EventID struct {
	ListenerID       *fftypes.UUID // The listener for the event
	BlockHash        string        // String representation of the block, which will change if any transaction info in the block changes
	BlockNumber      uint64        // A numeric identifier for the block
	TransactionHash  string        // The transaction
	TransactionIndex uint64        // Index within the block of the transaction that emitted the event
	LogIndex         uint64        // Index within the transaction of this emitted event log
}

// Event is a blockchain event that matches one of the started listeners.
// The implementation is responsible for ensuring all events on a listener are
// ordered on to this channel in the exact sequence from the blockchain.
type Event struct {
	EventID
	Data *fftypes.JSONAny `json:"data"` // the JSON data to deliver for this event (can be array or object structure)
	Info *fftypes.JSONAny `json:"info"` // additional blockchain specific information
}

// EventListenerCheckpoint is the interface that a checkpoint must implement, basically to make it sortable.
// The checkpoint must also be JSON serializable
type EventListenerCheckpoint interface {
	LessThan(b EventListenerCheckpoint) bool
}

// String is unique in all cases for an event, by combining the protocol ID with the listener ID and block hash
func (eid *EventID) String() string {
	return fmt.Sprintf("%s/B=%s/L=%s", eid.ProtocolID(), eid.BlockHash, eid.ListenerID)
}

// ProtocolID represents the unique (once finality is reached) sortable position within the blockchain
func (eid *EventID) ProtocolID() string {
	return fmt.Sprintf("%.12d/%.6d/%.6d", eid.BlockNumber, eid.TransactionIndex, eid.LogIndex)
}

// Events array has a natural sort order of the block/txIndex/logIndex
type Events []*Event

func (es Events) Len() int           { return len(es) }
func (es Events) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es Events) Less(i, j int) bool { return evLess(es[i], es[j]) }

// ListenerEvents array has a natural sort order of the event
type ListenerEvents []*ListenerEvent

func (lu ListenerEvents) Len() int           { return len(lu) }
func (lu ListenerEvents) Swap(i, j int)      { lu[i], lu[j] = lu[j], lu[i] }
func (lu ListenerEvents) Less(i, j int) bool { return evLess(lu[i].Event, lu[j].Event) }

func evLess(eI *Event, eJ *Event) bool {
	return eI.BlockNumber < eJ.BlockNumber ||
		((eI.BlockNumber == eJ.BlockNumber) &&
			((eI.TransactionIndex < eJ.TransactionIndex) ||
				((eI.TransactionIndex == eJ.TransactionIndex) && (eI.LogIndex < eJ.LogIndex))))
}

type EventWithContext struct {
	StreamID *fftypes.UUID `json:"streamId"` // the ID of the event stream for this event
	Event
}

// ListenerEvent is an event+checkpoint for a particular listener, and is the object delivered over the event stream channel when
// a new event is detected for delivery to the confirmation manager.
type ListenerEvent struct {
	Checkpoint EventListenerCheckpoint `json:"checkpoint"`        // the checkpoint information associated with the event, must be non-nil if the event is not removed
	Event      *Event                  `json:"event"`             // the event - for removed events, can only have the EventID fields set (to generate the protocol ID)
	Removed    bool                    `json:"removed,omitempty"` // when true, this is an explicit cancellation of a previous event
}

// ErrorReason are a set of standard error conditions that a blockchain connector can return
// from execution, that affect the action of the transaction manager to the response.
// It is important that error mapping is performed for each of these classification
type ErrorReason string

const (
	// ErrorReasonInvalidInputs transaction inputs could not be parsed by the connector according to the interface (nothing was sent to the blockchain)
	ErrorReasonInvalidInputs ErrorReason = "invalid_inputs"
	// ErrorReasonTransactionReverted on-chain execution (only expected to be returned when the connector is doing gas estimation, or executing a query)
	ErrorReasonTransactionReverted ErrorReason = "transaction_reverted"
	// ErrorReasonNonceTooLow on transaction submission, if the nonce has already been used for a transaction that has made it into a block on the canonical chain known to the local node
	ErrorReasonNonceTooLow ErrorReason = "nonce_too_low"
	// ErrorReasonTransactionUnderpriced if the transaction is rejected due to too low gas price. Either because it was too low according to the minimum configured on the node, or because it's a rescue transaction without a price bump.
	ErrorReasonTransactionUnderpriced ErrorReason = "transaction_underpriced"
	// ErrorReasonInsufficientFunds if the transaction is rejected due to not having enough of the underlying network coin (ether etc.) in your wallet
	ErrorReasonInsufficientFunds ErrorReason = "insufficient_funds"
	// ErrorReasonNotFound if the requested object (block/receipt etc.) was not found
	ErrorReasonNotFound ErrorReason = "not_found"
	// ErrorKnownTransaction if the exact transaction is already known
	ErrorKnownTransaction ErrorReason = "known_transaction"
)

// TransactionInput is a standardized set of parameters that describe a transaction submission to a blockchain.
// For convenience, ths structure is compatible with the EthConnect `TransactionSend` structure, for the subset of usage made by FireFly core / Tokens connectors.
// - Numeric values such as nonce/gas/gasPrice, are all passed as string encoded Base 10 integers
// - From/To are passed as strings, and are pass-through for FFTM from the values it receives from FireFly core after signing key resolution
// - The interface is a structure describing the method to invoke. The `variant` in the header tells you how to decode it. For variant=evm it will be an ABI method definition
// - The supplied value is passed through for each input parameter. It could be any JSON type (simple number/boolean/string, or complex object/array). The blockchain connection is responsible for serializing these according to the rules in the interface.
type TransactionInput struct {
	TransactionHeaders
	Method *fftypes.JSONAny   `json:"method"`
	Params []*fftypes.JSONAny `json:"params"`
}

type TransactionHeaders struct {
	From  string            `json:"from,omitempty"`
	To    string            `json:"to,omitempty"`
	Nonce *fftypes.FFBigInt `json:"nonce,omitempty"`
	Gas   *fftypes.FFBigInt `json:"gas,omitempty"`
	Value *fftypes.FFBigInt `json:"value,omitempty"`
}

type BlockInfo struct {
	BlockNumber       *fftypes.FFBigInt `json:"blockNumber"`
	BlockHash         string            `json:"blockHash"`
	ParentHash        string            `json:"parentHash"`
	TransactionHashes []string          `json:"transactionHashes"`
}
