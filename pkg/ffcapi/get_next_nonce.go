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

// GetNextNonceRequest used to do a query for the next nonce to use for a
// given signing identity. This is only used when there are no pending
// operations outstanding for this signer known to the transaction manager.
type GetNextNonceRequest struct {
	RequestBase
	Signer string `json:"signer"`
}

type GetNextNonceResponse struct {
	ResponseBase
	Nonce *fftypes.FFBigInt `json:"nonce"`
}

const RequestTypeGetNextNonce RequestType = "get_next_nonce"

func (r *GetNextNonceRequest) RequestType() RequestType {
	return RequestTypeGetNextNonce
}

func (a *api) GetNextNonce(ctx context.Context, req *GetNextNonceRequest) (*GetNextNonceResponse, ErrorReason, error) {
	res := &GetNextNonceResponse{}
	reason, err := a.invokeAPI(ctx, req, res)
	if err != nil {
		return nil, reason, err
	}
	return res, "", nil
}
