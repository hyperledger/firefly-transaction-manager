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

package persistence

import (
	"context"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

type Persistence interface {
	WriteCheckpoint(ctx context.Context, checkpoint *apitypes.EventStreamCheckpoint) error
	GetCheckpoint(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStreamCheckpoint, error)
	DeleteCheckpoint(ctx context.Context, streamID *fftypes.UUID) error

	ListStreams(ctx context.Context, after *fftypes.UUID, limit int) ([]*apitypes.EventStream, error) // reverse UUIDv1 order
	GetStream(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStream, error)
	WriteStream(ctx context.Context, spec *apitypes.EventStream) error
	DeleteStream(ctx context.Context, streamID *fftypes.UUID) error

	ListListeners(ctx context.Context, after *fftypes.UUID, limit int) ([]*apitypes.Listener, error) // reverse UUIDv1 order
	ListStreamListeners(ctx context.Context, after *fftypes.UUID, limit int, streamID *fftypes.UUID) ([]*apitypes.Listener, error)
	GetListener(ctx context.Context, listenerID *fftypes.UUID) (*apitypes.Listener, error)
	WriteListener(ctx context.Context, spec *apitypes.Listener) error
	DeleteListener(ctx context.Context, listenerID *fftypes.UUID) error

	ListManagedTransactions(ctx context.Context, after string, limit int) ([]*apitypes.ManagedTX, error) // note there is no order to these, so list isn't very useful
	GetManagedTransaction(ctx context.Context, txID string) (*apitypes.ManagedTX, error)
	WriteManagedTransaction(ctx context.Context, tx *apitypes.ManagedTX) error
	DeleteManagedTransaction(ctx context.Context, txID string) error

	ListNonceAllocations(ctx context.Context, signer string, after *int64, limit int) ([]*apitypes.NonceAllocation, error) // reverse nonce order
	GetNonceAllocation(ctx context.Context, signer string, nonce int64) (*apitypes.NonceAllocation, error)
	WriteNonceAllocation(ctx context.Context, alloc *apitypes.NonceAllocation) error
	DeleteNonceAllocation(ctx context.Context, signer string, nonce int64) error

	ListInflightTransactions(ctx context.Context, after *fftypes.UUID, limit int) ([]*apitypes.InflightTX, error) // reverse UUIDv1 order
	GetInflightTransaction(ctx context.Context, inflightID *fftypes.UUID) (*apitypes.InflightTX, error)
	WriteInflightTransaction(ctx context.Context, inflight *apitypes.InflightTX) error
	DeleteInflightTransaction(ctx context.Context, inflightID *fftypes.UUID) error

	Close(ctx context.Context)
}
