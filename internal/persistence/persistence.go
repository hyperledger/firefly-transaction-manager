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

package persistence

import (
	"context"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/toolkit"
)

// Persistence interface contains all the functions a persistence instance needs to implement.
// Sub set of functions are grouped into sub interfaces to provide a clear view of what
// persistent functions will be made available for each sub components to use after the persistent
// instance is initialized by the manager.
type Persistence interface {
	EventStreamPersistence
	ListenerPersistence
	TransactionPersistence

	// close function is controlled by the manager
	Close(ctx context.Context)
}

type EventStreamPersistence interface {
	WriteCheckpoint(ctx context.Context, checkpoint *apitypes.EventStreamCheckpoint) error
	GetCheckpoint(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStreamCheckpoint, error)
	DeleteCheckpoint(ctx context.Context, streamID *fftypes.UUID) error

	ListStreams(ctx context.Context, after *fftypes.UUID, limit int, dir toolkit.SortDirection) ([]*apitypes.EventStream, error) // reverse UUIDv1 order
	GetStream(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStream, error)
	WriteStream(ctx context.Context, spec *apitypes.EventStream) error
	DeleteStream(ctx context.Context, streamID *fftypes.UUID) error
}
type ListenerPersistence interface {
	ListListeners(ctx context.Context, after *fftypes.UUID, limit int, dir toolkit.SortDirection) ([]*apitypes.Listener, error) // reverse UUIDv1 order
	ListStreamListeners(ctx context.Context, after *fftypes.UUID, limit int, dir toolkit.SortDirection, streamID *fftypes.UUID) ([]*apitypes.Listener, error)
	GetListener(ctx context.Context, listenerID *fftypes.UUID) (*apitypes.Listener, error)
	WriteListener(ctx context.Context, spec *apitypes.Listener) error
	DeleteListener(ctx context.Context, listenerID *fftypes.UUID) error
}

type TransactionPersistence toolkit.Persistence
