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
	"context"
	"net/http"
	"strings"

	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/toolkit"
)

func (m *manager) getTransactionByID(ctx context.Context, txID string) (transaction *apitypes.ManagedTX, err error) {
	return m.txHandler.GetTransactionByID(ctx, txID)
}

func (m *manager) getTransactions(ctx context.Context, afterStr, limitStr, signer string, pending bool, dirString string) (transactions []*apitypes.ManagedTX, err error) {
	limit, err := m.parseLimit(ctx, limitStr)
	if err != nil {
		return nil, err
	}
	var dir toolkit.SortDirection
	switch strings.ToLower(dirString) {
	case "", "desc", "descending":
		dir = toolkit.SortDirectionDescending // descending is default
	case "asc", "ascending":
		dir = toolkit.SortDirectionAscending
	default:
		return nil, i18n.NewError(ctx, tmmsgs.MsgInvalidSortDirection, dirString)
	}
	return m.txHandler.GetTransactions(ctx, afterStr, signer, pending, limit, dir)
}

func (m *manager) requestTransactionDeletion(ctx context.Context, txID string) (status int, transaction *apitypes.ManagedTX, err error) {

	canceledTx, err := m.txHandler.HandleCancelTransaction(ctx, txID)

	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusAccepted, canceledTx, nil

}
