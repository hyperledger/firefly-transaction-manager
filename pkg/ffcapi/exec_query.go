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

// ExecQueryRequest requests execution of a smart contract method in order to either:
// 1) Query state
// 2) Attempt to extract the revert reason from an on-chain failure to execute a transaction
//
// See the list of standard error reasons that should be returned for situations that can be
// detected by the back-end connector.
type ExecQueryRequest struct {
	RequestBase
	TransactionInput
	BlockNumber **fftypes.FFBigInt `json:"blockNumber,omitempty"`
}

type ExecQueryResponse struct {
	ResponseBase
	Valid           bool              `json:"valid"` // false if the inputs could not be parsed
	ValidationError string            `json:"validationError,omitempty"`
	Success         bool              `json:"success"`
	OnchainError    string            `json:"onchainError,omitempty"`
	Outputs         []fftypes.JSONAny `json:"outputs"`
}

const RequestTypeExecQuery RequestType = "exec_query"

func (r *ExecQueryRequest) RequestType() RequestType {
	return RequestTypeExecQuery
}

func (a *api) ExecQuery(ctx context.Context, req *ExecQueryRequest) (*ExecQueryResponse, ErrorReason, error) {
	res := &ExecQueryResponse{}
	reason, err := a.invokeAPI(ctx, req, res)
	if err != nil {
		return nil, reason, err
	}
	return res, "", nil
}
