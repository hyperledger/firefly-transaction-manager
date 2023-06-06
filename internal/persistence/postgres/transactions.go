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

package postgres

import (
	"context"

	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

func (p *sqlPersistence) ListTransactions(ctx context.Context, filter ffapi.Filter) ([]*apitypes.ManagedTX, *ffapi.FilterResult, error) {
	return nil, nil, nil
}

func (p *sqlPersistence) ListTransactionsByCreateTime(ctx context.Context, after *apitypes.ManagedTX, limit int, dir persistence.SortDirection) ([]*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) ListTransactionsByNonce(ctx context.Context, signer string, after *fftypes.FFBigInt, limit int, dir persistence.SortDirection) ([]*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) ListTransactionsPending(ctx context.Context, afterSequenceID string, limit int, dir persistence.SortDirection) ([]*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) GetTransactionByID(ctx context.Context, txID string) (*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) GetTransactionByIDWithHistory(ctx context.Context, txID string) (*apitypes.TXWithStatus, error) {
	return nil, nil
}

func (p *sqlPersistence) GetTransactionByNonce(ctx context.Context, signer string, nonce *fftypes.FFBigInt) (*apitypes.ManagedTX, error) {
	return nil, nil
}

func (p *sqlPersistence) InsertTransaction(ctx context.Context, tx *apitypes.ManagedTX) error {
	return nil
}

func (p *sqlPersistence) UpdateTransaction(ctx context.Context, txID string, updates *apitypes.TXUpdates) error {
	return nil
}

func (p *sqlPersistence) DeleteTransaction(ctx context.Context, txID string) error {
	return nil
}
