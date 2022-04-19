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

	"github.com/hyperledger/firefly/pkg/fftypes"
)

// PrepareTransactionRequest is used to prepare a set of JSON formatted developer friendly
// inputs, into a raw transaction ready for submission to the blockchain.
//
// The connector is responsible for encoding the transaction ready for sumission,
// and returning the hash for the transaction as well as a string serialization of
// the pre-signed raw transaction in a format of its own choosing (hex etc.).
// The hash is expected to be a function of:
// - the method signature
// - the signing identity
// - the nonce
// - the particular blockchain the transaction is submitted to
// - the input parameters
//
// If "gas" is not supplied, the connector is expected to perform gas estimation
// prior to generating the payload.
//
// See the list of standard error reasons that should be returned for situations that can be
// detected by the back-end connector.
type PrepareTransactionRequest struct {
	RequestBase
	TransactionHeaders
	TransactionPrepareInputs
}

type PrepareTransactionResponse struct {
	ResponseBase
	Gas             *fftypes.FFBigInt `json:"gas"`
	TransactionHash string            `json:"transactionHash"`
	TransactionData string            `json:"transactionData"`
}

const RequestTypePrepareTransaction RequestType = "prepare_transaction"

func (r *PrepareTransactionRequest) RequestType() RequestType {
	return RequestTypePrepareTransaction
}

func (a *api) PrepareTransaction(ctx context.Context, req *PrepareTransactionRequest) (*PrepareTransactionResponse, ErrorReason, error) {
	res := &PrepareTransactionResponse{}
	reason, err := a.invokeAPI(ctx, req, res)
	if err != nil {
		return nil, reason, err
	}
	return res, "", nil
}
