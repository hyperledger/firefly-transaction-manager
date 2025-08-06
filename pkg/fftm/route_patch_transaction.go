// Copyright © 2023 Kaleido, Inc.
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

package fftm

import (
	"net/http"

	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

var patchTransaction = func(m *manager) *ffapi.Route {
	return &ffapi.Route{
		Name:   "patchTransaction",
		Path:   "/transactions/{transactionId}",
		Method: http.MethodPatch,
		PathParams: []*ffapi.PathParam{
			{Name: "transactionId", Description: tmmsgs.APIParamTransactionID},
		},
		QueryParams:     nil,
		Description:     tmmsgs.APIEndpointPatchTransaction,
		JSONInputValue:  func() interface{} { return &apitypes.TXUpdatesExternal{} },
		JSONOutputValue: func() interface{} { return &apitypes.ManagedTX{} },
		JSONOutputCodes: []int{http.StatusOK, http.StatusAccepted},
		JSONHandler: func(r *ffapi.APIRequest) (output interface{}, err error) {
			output, err = m.updateTransaction(r.Req.Context(), r.PP["transactionId"], r.Input.(*apitypes.TXUpdatesExternal))
			return output, err
		},
	}
}
