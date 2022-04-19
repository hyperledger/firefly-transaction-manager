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
	"github.com/hyperledger/firefly/pkg/fftypes"
)

// RequestType for each request is defined in the individual file
type RequestType string

// Semver API versioning
type Version string

const (
	Version1_0_0 Version = "v1.0.0"
)

const VersionCurrent = Version1_0_0

type Variant string

const (
	VariantEVM Variant = "evm"
)

// ErrorReason are a set of standard error conditions that a blockchain connector can return
// from execution, that affect the action of the transaction manager to the response.
// It is important that error mapping is performed for each of these classification
type ErrorReason string

const (
	// ErrorReasonInvalidInputs transaction inputs could not be parsed by the connector according to the interface (nothing was sent to the blockchain)
	ErrorReasonInvalidInputs ErrorReason = "invalid_inputs"
	// ErrorReasonTransactionReverted on-chain execution (only expected to be returned when the connector is doing gas estimation, or executing a query)
	ErrorReasonTransactionReverted ErrorReason = "transaction_reverted"
	// ErrorReasonNonceTooLow on transaction submission, if the nonce has already been used for a transaction that has made it into a block on canonical chain known to the local node
	ErrorReasonNonceTooLow ErrorReason = "nonce_too_low"
	// ErrorReasonTransactionUnderpriced if the transaction is rejected due to too low gas price. Either because it was too low according to the minimum configured on the node, or because it's a rescue transaction without a price bump.
	ErrorReasonTransactionUnderpriced ErrorReason = "transaction_underpriced"
	// ErrorReasonNotFound if the requested object (block/receipt etc.) was not found
	ErrorReasonNotFound ErrorReason = "not_found"
)

// Header is included consistently as a "ffcapi" structure on each request
type Header struct {
	RequestID   *fftypes.UUID `json:"id"`      // Unique for each request
	Version     Version       `json:"version"` // The API version
	Variant     Variant       `json:"variant"` // Defines the format of the input/output bodies, which FFTM operates pass-through on from FireFly core to the Blockchain connector
	RequestType RequestType   `json:"type"`    // The type of the request, which defines how it should be processed, and the structure of the rest of the payload
}

// TransactionInput is a standardized set of parameters that describe a transaction submission to a blockchain.
// For convenience, ths structure is compatible with the EthConnect `SendTransaction` structure, for the subset of usage made by FireFly core / Tokens connectors.
// - Numberic values such as nonce/gas/gasPrice, are all passed as string encoded Base 10 integers
// - From/To are passed as strings, and are pass-through for FFTM from the values it receives from FireFly core after signing key resolution
// - The interface is a structure describing the method to invoke. The `variant` in the header tells you how to decode it. For variant=evm it will be an ABI method definition
// - The supplied value is passed through for each input parameter. It could be any JSON type (simple number/boolean/string, or complex object/array). The blockchain connection is responsible for serializing these according to the rules in the interface.
type TransactionInput struct {
	GasPrice *fftypes.JSONAny `json:"gasPrice,omitempty"` // can be a simple string/number, or a complex object - contract is between policy engine and blockchain connector
	TransactionHeaders
	TransactionPrepareInputs
}

type TransactionHeaders struct {
	From  string            `json:"from"`
	To    string            `json:"to"`
	Nonce *fftypes.FFBigInt `json:"nonce"`
	Gas   *fftypes.FFBigInt `json:"gas,omitempty"`
	Value *fftypes.FFBigInt `json:"value"`
}

type TransactionPrepareInputs struct {
	Method fftypes.JSONAny   `json:"method"`
	Params []fftypes.JSONAny `json:"params"`
}

// ErrorResponse allows blockchain connectors to encode useful information about an error in a JSON response body.
// This should be accompanied with a suitable non-success HTTP response code. However, the "reason" (if supplied)
// is the only information that will be used to change the transaction manager's handling of the error.
type ErrorResponse struct {
	Reason ErrorReason `json:"reason,omitempty"`
	Error  string      `json:"error"`
}

type RequestBase struct {
	FFCAPI Header `json:"ffcapi"`
}

func (r *RequestBase) FFCAPIHeader() *Header {
	return &r.FFCAPI
}

type ResponseBase struct {
	ErrorResponse
}

func (r *ResponseBase) ErrorMessage() string {
	return r.Error
}

func (r *ResponseBase) ErrorReason() ErrorReason {
	return r.Reason
}

type ffcapiRequest interface {
	FFCAPIHeader() *Header
	RequestType() RequestType
}

type ffcapiResponse interface {
	ErrorMessage() string
	ErrorReason() ErrorReason
}

type RequestID string

func initHeader(header *Header, variant Variant, requestType RequestType) {
	header.RequestID = fftypes.NewUUID()
	header.Version = VersionCurrent
	header.Variant = variant
	header.RequestType = requestType
}
