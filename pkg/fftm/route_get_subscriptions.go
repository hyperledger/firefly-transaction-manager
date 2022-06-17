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

package fftm

import (
	"net/http"

	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

var getSubscriptions = func(m *manager) *ffapi.Route {
	return &ffapi.Route{
		Name:       "getSubscriptions",
		Path:       "/subscriptions",
		Deprecated: true, // in favor of "/eventstreams/{id}/listeners"
		Method:     http.MethodGet,
		PathParams: nil,
		QueryParams: []*ffapi.QueryParam{
			{Name: "limit", Description: tmmsgs.APIParamLimit},
			{Name: "after", Description: tmmsgs.APIParamAfter},
		},
		Description:     tmmsgs.APIEndpointGetSubscriptions,
		JSONInputValue:  nil,
		JSONOutputValue: func() interface{} { return []*apitypes.Listener{} },
		JSONOutputCodes: []int{http.StatusOK},
		JSONHandler: func(r *ffapi.APIRequest) (output interface{}, err error) {
			return m.getListeners(r.Req.Context(), r.QP["after"], r.QP["limit"])
		},
	}
}
