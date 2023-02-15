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

package txhandler

import (
	"context"

	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/metrics"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/ws"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhistory"
)

type ToolkitAPI struct {
	Connector           ffcapi.API
	TXHistory           txhistory.Manager
	Persistence         persistence.Persistence
	MetricsManager      metrics.Manager
	ConfirmationManager confirmations.Manager
	WsServer            ws.WebSocketServer
}

// Transaction handler owns the lifecycle of ManagedTransaction records
// Transaction manager delegates all transaction specific operations to transaction apart from the triggers (REST API call for actions, Event stream for events) of those operations
// This design allows the Transaction handler to apply customized logic at different stage of transaction life cycles listed below.
type TransactionHandler interface {
	Init(ctx context.Context, tkAPI *ToolkitAPI) error

	Start(ctx context.Context) (done chan struct{}, err error)

	RegisterNewTransaction(ctx context.Context, txReq *apitypes.TransactionRequest) (mtx *apitypes.ManagedTX, err error)
	RegisterNewContractDeployment(ctx context.Context, txReq *apitypes.ContractDeployRequest) (mtx *apitypes.ManagedTX, err error)
	CancelTransaction(ctx context.Context, txID string) (mtx *apitypes.ManagedTX, err error)
	// TransactionConfirmed(ctx context.Context, cAPI *ToolkitAPI, mtx *apitypes.ManagedTX) (err error) // ?? Delete transaction once it's confirmed could be the intention
	// TransactionReceipt(ctx context.Context, cAPI *ToolkitAPI, mtx *apitypes.ManagedTX) (err error)   // ?? Delete transaction once it's confirmed could be the intention
}
