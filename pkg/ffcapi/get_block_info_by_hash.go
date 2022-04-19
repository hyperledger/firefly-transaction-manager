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

type GetBlockInfoByHashRequest struct {
	RequestBase
	BlockHash string `json:"blockHash"`
}

type GetBlockInfoByHashResponse struct {
	ResponseBase
	BlockNumber *fftypes.FFBigInt `json:"blockNumber"`
	BlockHash   string            `json:"blockHash"`
	ParentHash  string            `json:"parentHash"`
}

const RequestTypeGetBlockInfoByHash RequestType = "get_block_info_by_hash"

func (r *GetBlockInfoByHashRequest) RequestType() RequestType {
	return RequestTypeGetBlockInfoByHash
}

func (a *api) GetBlockInfoByHash(ctx context.Context, req *GetBlockInfoByHashRequest) (*GetBlockInfoByHashResponse, ErrorReason, error) {
	res := &GetBlockInfoByHashResponse{}
	reason, err := a.invokeAPI(ctx, req, res)
	if err != nil {
		return nil, reason, err
	}
	return res, "", nil
}
