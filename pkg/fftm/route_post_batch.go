// Copyright © 2025 Kaleido, Inc.
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
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

var postBatch = func(m *manager) *ffapi.Route {
	return &ffapi.Route{
		Name:           "postBatch",
		Path:           "/batch",
		Method:         http.MethodPost,
		PathParams:     nil,
		QueryParams:    nil,
		Description:    tmmsgs.APIEndpointPostBatch,
		JSONInputValue: func() interface{} { return &apitypes.BatchRequest{} },
		JSONInputSchema: func(_ context.Context, schemaGen ffapi.SchemaGenerator) (*openapi3.SchemaRef, error) {
			return schemaGen(&apitypes.BatchRequest{})
		},
		JSONOutputSchema: func(_ context.Context, schemaGen ffapi.SchemaGenerator) (*openapi3.SchemaRef, error) {
			return schemaGen(&apitypes.BatchResponse{})
		},
		JSONOutputCodes: []int{http.StatusOK},
		JSONHandler: func(r *ffapi.APIRequest) (output interface{}, err error) {
			batchReq := r.Input.(*apitypes.BatchRequest)
			ctx := r.Req.Context()
			l := log.L(ctx)

			// Initialize responses for all requests
			responses := make([]*apitypes.BatchResponseItem, len(batchReq.Requests))
			for i, baseReq := range batchReq.Requests {
				responses[i] = &apitypes.BatchResponseItem{
					ID:      baseReq.Headers.ID,
					Success: false,
				}
			}

			// Group requests by type for batch processing
			txReqs := make([]*apitypes.TransactionRequest, 0)
			deployReqs := make([]*apitypes.ContractDeployRequest, 0)
			txIndices := make([]int, 0)
			deployIndices := make([]int, 0)

			for i, baseReq := range batchReq.Requests {
				switch baseReq.Headers.Type {
				case apitypes.RequestTypeSendTransaction:
					var tReq apitypes.TransactionRequest
					if err := baseReq.UnmarshalTo(&tReq); err != nil {
						responses[i].Error = &ffcapi.SubmissionError{
							Error:              i18n.NewError(ctx, tmmsgs.MsgInvalidRequestErr, baseReq.Headers.Type, err).Error(),
							SubmissionRejected: true,
						}
						continue
					}
					txReqs = append(txReqs, &tReq)
					txIndices = append(txIndices, i)

				case apitypes.RequestTypeDeploy:
					var tReq apitypes.ContractDeployRequest
					if err := baseReq.UnmarshalTo(&tReq); err != nil {
						responses[i].Error = &ffcapi.SubmissionError{
							Error:              i18n.NewError(ctx, tmmsgs.MsgInvalidRequestErr, baseReq.Headers.Type, err).Error(),
							SubmissionRejected: true,
						}
						continue
					}
					deployReqs = append(deployReqs, &tReq)
					deployIndices = append(deployIndices, i)

				default:
					responses[i].Error = &ffcapi.SubmissionError{
						Error:              i18n.NewError(ctx, tmmsgs.MsgUnsupportedRequestType, baseReq.Headers.Type).Error(),
						SubmissionRejected: true,
					}
					l.Warnf("Unsupported request type in batch: %s (request ID: %s)", baseReq.Headers.Type, baseReq.Headers.ID)
				}
			}

			// Process SendTransaction requests in batch
			if len(txReqs) > 0 {
				mtxs, submissionRejected, errs := m.txHandler.HandleNewTransactions(ctx, txReqs)
				for j, idx := range txIndices {
					if errs[j] != nil {
						responses[idx].Error = &ffcapi.SubmissionError{
							Error:              errs[j].Error(),
							SubmissionRejected: submissionRejected[j],
						}
						l.Errorf("Batch SendTransaction failed for request ID %s (submissionRejected=%t): %s", batchReq.Requests[idx].Headers.ID, submissionRejected[j], errs[j])
					} else if mtxs[j] != nil {
						responses[idx].Success = true
						responses[idx].Output = mtxs[j]
					}
				}
			}

			// Process Deploy requests in batch
			if len(deployReqs) > 0 {
				mtxs, submissionRejected, errs := m.txHandler.HandleNewContractDeployments(ctx, deployReqs)
				for j, idx := range deployIndices {
					if errs[j] != nil {
						responses[idx].Error = &ffcapi.SubmissionError{
							Error:              errs[j].Error(),
							SubmissionRejected: submissionRejected[j],
						}
						l.Errorf("Batch Deploy failed for request ID %s (submissionRejected=%t): %s", batchReq.Requests[idx].Headers.ID, submissionRejected[j], errs[j])
					} else if mtxs[j] != nil {
						responses[idx].Success = true
						responses[idx].Output = mtxs[j]
					}
				}
			}

			return &apitypes.BatchResponse{
				Responses: responses,
			}, nil
		},
	}
}
