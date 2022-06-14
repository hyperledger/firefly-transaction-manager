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

	"github.com/google/uuid"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
)

// UUIDVersion1 returns a version 1 UUID - where the alphanumeric sequence is assured to be ascending based on the order of generation
func UUIDVersion1() *fftypes.UUID {
	u, _ := uuid.NewUUID()
	return (*fftypes.UUID)(&u)
}

type Persistence interface {
	WriteCheckpoint(ctx context.Context, checkpoint *fftm.EventStreamCheckpoint) error
	GetCheckpoint(ctx context.Context, streamID *fftypes.UUID) (*fftm.EventStreamCheckpoint, error)
	DeleteCheckpoint(ctx context.Context, streamID *fftypes.UUID) error

	ListStreams(ctx context.Context, after *fftypes.UUID, limit int) ([]*fftm.EventStream, error)
	GetStream(ctx context.Context, streamID *fftypes.UUID) (*fftm.EventStream, error)
	WriteStream(ctx context.Context, spec *fftm.EventStream) error
	DeleteStream(ctx context.Context, streamID *fftypes.UUID) error

	ListListeners(ctx context.Context, after *fftypes.UUID, limit int) ([]*fftm.Listener, error)
	ListStreamListeners(ctx context.Context, after *fftypes.UUID, limit int, streamID *fftypes.UUID) ([]*fftm.Listener, error)
	GetListener(ctx context.Context, listenerID *fftypes.UUID) (*fftm.Listener, error)
	WriteListener(ctx context.Context, spec *fftm.Listener) error
	DeleteListener(ctx context.Context, listenerID *fftypes.UUID) error
}
