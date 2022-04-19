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

// SendTransactionRequest is used to send a transaction to the blockchain.
// The connector is responsible for adding it to the transaction pool of the blockchain,
// noting the transaction hash has already been calculated in the prepare step previously.
type SendTransactionRequest struct {
	RequestBase
	From           string           `json:"from"`
	GasPrice       *fftypes.JSONAny `json:"gasPrice,omitempty"` // can be a simple string/number, or a complex object - contract is between policy engine and blockchain connector
	RawTransaction string           `json:"rawTransaction"`
}

type SendTransactionResponse struct {
	ResponseBase
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
