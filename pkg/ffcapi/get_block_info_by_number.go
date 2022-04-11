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

type GetBlockInfoByNumberRequest struct {
	RequestBase
	BlockNumber *fftypes.FFBigInt `json:"blockNumber"`
}

type GetBlockInfoByNumberResponse struct {
	ResponseBase
	BlockNumber *fftypes.FFBigInt `json:"blockNumber"`
	BlockHash   string            `json:"blockHash"`
	ParentHash  string            `json:"parentHash"`
}

const RequestTypeGetBlockInfoByNumber RequestType = "get_block_info_by_number"

func (r *GetBlockInfoByNumberRequest) RequestType() RequestType {
	return RequestTypeGetBlockInfoByNumber
}

func (a *api) GetBlockInfoByNumber(ctx context.Context, req *GetBlockInfoByNumberRequest) (*GetBlockInfoByNumberResponse, ErrorReason, error) {
	res := &GetBlockInfoByNumberResponse{}
	reason, err := a.invokeAPI(ctx, req, res)
	if err != nil {
		return nil, reason, err
	}
	return res, "", nil
}
