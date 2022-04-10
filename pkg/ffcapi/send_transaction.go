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

import "context"

// SendTransactionRequest is used to send a transaction to the blockchain.
//
// The connector is responsible for adding it to the transaction pool of the blockchain,
// and returning the hash for the transaction. The hash is expected to be a function of:
// - the method signature
// - the signing identity
// - the nonce
// - the particular blockchain the transaction is submitted to
// - the input parameters
//
// If "gas" is not supplied, the connector is expected to perform as gas estimation
// prior to submission, and return the calculated gas amount.
//
// See the list of standard error reasons that should be returned for situations that can be
// detected by the back-end connector.
type SendTransactionRequest struct {
	RequestBase
	TransactionInput
}

type SendTransactionResponse struct {
	ResponseBase
	TransactionHash string `json:"transactionHash"`
}

const RequestTypeSendTransaction RequestType = "send_transaction"

func (r *SendTransactionRequest) RequestType() RequestType {
	return RequestTypeSendTransaction
}

func (a *api) SendTransaction(ctx context.Context, req *SendTransactionRequest) (*SendTransactionResponse, ErrorReason, error) {
	res := &SendTransactionResponse{}
	reason, err := a.invokeAPI(ctx, req, res)
	if err != nil {
		return nil, reason, err
	}
	return res, "", nil
}
