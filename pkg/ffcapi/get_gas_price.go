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

// GetGasPriceRequest used to do a query for the next nonce to use for a
// given signing identity. This is only used when there are no pending
// operations outstanding for this signer known to the transaction manager.
type GetGasPriceRequest struct {
	RequestBase
}

type GetGasPriceResponse struct {
	ResponseBase
	GasPrice *fftypes.JSONAny
}

const RequestTypeGetGasPrice RequestType = "get_gas_price"

func (r *GetGasPriceRequest) RequestType() RequestType {
	return RequestTypeGetGasPrice
}

func (a *api) GetGasPrice(ctx context.Context, req *GetGasPriceRequest) (*GetGasPriceResponse, ErrorReason, error) {
	res := &GetGasPriceResponse{}
	reason, err := a.invokeAPI(ctx, req, res)
	if err != nil {
		return nil, reason, err
	}
	return res, "", nil
}
