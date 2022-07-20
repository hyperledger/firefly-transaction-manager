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
	"context"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

var postRootCommand = func(m *manager) *ffapi.Route {
	return &ffapi.Route{
		Name:            "postRootCommand",
		Path:            "/",
		Method:          http.MethodPost,
		PathParams:      nil,
		QueryParams:     nil,
		Description:     tmmsgs.APIEndpointPostSubscriptions,
		JSONInputValue:  func() interface{} { return &apitypes.BaseRequest{} },
		JSONOutputValue: func() interface{} { return map[string]interface{}{} },
		JSONInputSchema: func(ctx context.Context, schemaGen ffapi.SchemaGenerator) (*openapi3.SchemaRef, error) {
			schemas := []*openapi3.SchemaRef{}
			txRequest, err := schemaGen(&apitypes.TransactionRequest{})
			if err == nil {
				schemas = append(schemas, txRequest)
			}
			queryRequest, err := schemaGen(&apitypes.QueryRequest{})
			if err == nil {
				schemas = append(schemas, queryRequest)
			}
			return &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					AnyOf: schemas,
				},
			}, err
		},
		JSONOutputCodes: []int{http.StatusOK},
		JSONHandler: func(r *ffapi.APIRequest) (output interface{}, err error) {
			baseReq := r.Input.(*apitypes.BaseRequest)
			switch baseReq.Headers.Type {
			case apitypes.RequestTypeSendTransaction:
				var tReq apitypes.TransactionRequest
				if err = baseReq.UnmarshalTo(&tReq); err != nil {
					return nil, i18n.NewError(r.Req.Context(), tmmsgs.MsgInvalidRequestErr, baseReq.Headers.Type, err)
				}
				return m.sendManagedTransaction(r.Req.Context(), &tReq)
			case apitypes.RequestTypeQuery:
				var tReq apitypes.QueryRequest
				if err = baseReq.UnmarshalTo(&tReq); err != nil {
					return nil, i18n.NewError(r.Req.Context(), tmmsgs.MsgInvalidRequestErr, baseReq.Headers.Type, err)
				}
				res, _, err := m.connector.QueryInvoke(r.Req.Context(), &ffcapi.QueryInvokeRequest{
					TransactionInput: tReq.TransactionInput,
				})
				return res.Outputs, err
			default:
				return nil, i18n.NewError(r.Req.Context(), tmmsgs.MsgUnsupportedRequestType, baseReq.Headers.Type)
			}
		},
	}
}
