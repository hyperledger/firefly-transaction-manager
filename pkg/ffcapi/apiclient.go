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
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/ffresty"
	"github.com/hyperledger/firefly/pkg/i18n"
)

type API interface {
	CreateBlockListener(ctx context.Context, req *CreateBlockListenerRequest) (*CreateBlockListenerResponse, ErrorReason, error)
	ExecQuery(ctx context.Context, req *ExecQueryRequest) (*ExecQueryResponse, ErrorReason, error)
	GetBlockInfoByHash(ctx context.Context, req *GetBlockInfoByHashRequest) (*GetBlockInfoByHashResponse, ErrorReason, error)
	GetBlockInfoByNumber(ctx context.Context, req *GetBlockInfoByNumberRequest) (*GetBlockInfoByNumberResponse, ErrorReason, error)
	GetNewBlockHashes(ctx context.Context, req *GetNewBlockHashesRequest) (*GetNewBlockHashesResponse, ErrorReason, error)
	GetNextNonce(ctx context.Context, req *GetNextNonceRequest) (*GetNextNonceResponse, ErrorReason, error)
	GetReceipt(ctx context.Context, req *GetReceiptRequest) (*GetReceiptResponse, ErrorReason, error)
	PrepareTransaction(ctx context.Context, req *PrepareTransactionRequest) (*PrepareTransactionResponse, ErrorReason, error)
	SendTransaction(ctx context.Context, req *SendTransactionRequest) (*SendTransactionResponse, ErrorReason, error)
}

type api struct {
	client  *resty.Client
	variant Variant
}

func NewFFCAPI(ctx context.Context) API {
	return newAPI(ctx, tmconfig.ConnectorPrefix)
}

func newAPI(ctx context.Context, prefix config.Prefix) *api {
	return &api{
		client:  ffresty.New(ctx, prefix),
		variant: Variant(config.GetString(tmconfig.ConnectorVariant)),
	}
}

func (a *api) invokeAPI(ctx context.Context, input ffcapiRequest, output ffcapiResponse) (ErrorReason, error) {

	initHeader(input.FFCAPIHeader(), a.variant, input.RequestType())
	res, err := a.client.R().
		SetBody(input).
		SetResult(output).
		SetError(output).
		Post("/")
	if err != nil {
		return "", i18n.WrapError(ctx, err, tmmsgs.MsgConnectorFailInvoke, input.FFCAPIHeader().RequestID)
	}
	if !strings.Contains(res.Header().Get("Content-Type"), "application/json") {
		return "", i18n.NewError(ctx, tmmsgs.MsgConnectorInvalidConentType, input.FFCAPIHeader().RequestID, res.Header().Get("Content-Type"))
	}
	if res.IsError() {
		return output.ErrorReason(), i18n.NewError(ctx, tmmsgs.MsgConnectorError, input.FFCAPIHeader().RequestID, output.ErrorReason(), output.ErrorMessage())
	}

	return "", nil
}
