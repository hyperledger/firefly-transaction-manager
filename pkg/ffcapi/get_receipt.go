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

type GetReceiptRequest struct {
	RequestBase
	TransactionHash string `json:"transactionHash"`
}

type GetReceiptResponse struct {
	ResponseBase
	BlockNumber      *fftypes.FFBigInt `json:"blockNumber"`
	TransactionIndex *fftypes.FFBigInt `json:"transactinIndex"`
	BlockHash        string            `json:"blockHash"`
	Success          bool              `json:"success"`
	ExtraInfo        fftypes.JSONAny   `json:"extraInfo"`
}

const RequestTypeGetReceipt RequestType = "get_receipt"

func (r *GetReceiptRequest) RequestType() RequestType {
	return RequestTypeGetReceipt
}

func (a *api) GetReceipt(ctx context.Context, req *GetReceiptRequest) (*GetReceiptResponse, ErrorReason, error) {
	res := &GetReceiptResponse{}
	reason, err := a.invokeAPI(ctx, req, res)
	if err != nil {
		return nil, reason, err
	}
	return res, "", nil
}
