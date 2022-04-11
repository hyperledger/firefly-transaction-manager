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

type GetNewBlockHashesRequest struct {
	RequestBase
	ListenerID string `json:"listenerId"`
}

type GetNewBlockHashesResponse struct {
	ResponseBase
	BlockHashes []string `json:"blockHashes"`
}

const RequestTypeGetNewBlockHashes RequestType = "get_new_block_hashes"

func (r *GetNewBlockHashesRequest) RequestType() RequestType {
	return RequestTypeGetNewBlockHashes
}

func (a *api) GetNewBlockHashes(ctx context.Context, req *GetNewBlockHashesRequest) (*GetNewBlockHashesResponse, ErrorReason, error) {
	res := &GetNewBlockHashesResponse{}
	reason, err := a.invokeAPI(ctx, req, res)
	if err != nil {
		return nil, reason, err
	}
	return res, "", nil
}
