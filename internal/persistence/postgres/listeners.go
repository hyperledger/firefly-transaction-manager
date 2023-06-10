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

	"github.com/hyperledger/firefly-common/pkg/dbsql"
	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

func (p *sqlPersistence) newListenersCollection() *dbsql.CrudBase[*apitypes.Listener] {
	collection := &dbsql.CrudBase[*apitypes.Listener]{
		DB:    p.db,
		Table: "listeners",
		Columns: []string{
			dbsql.ColumnID,
			dbsql.ColumnCreated,
			dbsql.ColumnUpdated,
			"name",
			"stream_id",
			"filters",
			"options",
			"signature",
			"from_block",
		},
		FilterFieldMap: map[string]string{
			"sequence":   p.db.SequenceColumn(),
			"streamid":   "stream_id",
			"from_block": "fromblock",
		},
		NilValue:    func() *apitypes.Listener { return nil },
		NewInstance: func() *apitypes.Listener { return &apitypes.Listener{} },
		GetFieldPtr: func(inst *apitypes.Listener, col string) interface{} {
			switch col {
			case dbsql.ColumnID:
				return &inst.ID
			case dbsql.ColumnCreated:
				return &inst.Created
			case dbsql.ColumnUpdated:
				return &inst.Updated
			case "name":
				return &inst.Name
			case "stream_id":
				return &inst.StreamID
			case "filters":
				return &inst.Filters
			case "options":
				return &inst.Options
			case "signature":
				return &inst.Signature
			case "from_block":
				return &inst.FromBlock
			}
			return nil
		},
	}
	collection.Validate()
	return collection
}

func (p *sqlPersistence) ListListeners(ctx context.Context, filter ffapi.Filter) ([]*apitypes.Listener, *ffapi.FilterResult, error) {
	return p.listeners.GetMany(ctx, filter)
}

func (p *sqlPersistence) ListStreamListeners(ctx context.Context, streamID *fftypes.UUID, filter ffapi.Filter) ([]*apitypes.Listener, *ffapi.FilterResult, error) {
	fb := persistence.ListenerFilters.NewFilter(ctx)
	return p.listeners.GetMany(ctx, fb.And(
		fb.Eq("streamid", streamID),
		filter,
	))
}

func (p *sqlPersistence) ListListenersByCreateTime(ctx context.Context, after *fftypes.UUID, limit int, dir persistence.SortDirection) ([]*apitypes.Listener, error) {
	var afterSeq *int64
	if after != nil {
		seq, err := p.listeners.GetSequenceForID(ctx, after.String())
		if err != nil {
			return nil, err
		}
		afterSeq = &seq
	}
	filter := p.seqAfterFilter(ctx, persistence.ListenerFilters, afterSeq, limit, dir)
	listeners, _, err := p.listeners.GetMany(ctx, filter)
	return listeners, err
}

func (p *sqlPersistence) ListStreamListenersByCreateTime(ctx context.Context, after *fftypes.UUID, limit int, dir persistence.SortDirection, streamID *fftypes.UUID) ([]*apitypes.Listener, error) {
	var afterSeq *int64
	if after != nil {
		seq, err := p.listeners.GetSequenceForID(ctx, after.String())
		if err != nil {
			return nil, err
		}
		afterSeq = &seq
	}
	filter := p.seqAfterFilter(ctx, persistence.ListenerFilters, afterSeq, limit, dir,
		persistence.ListenerFilters.NewFilter(ctx).Eq("streamid", streamID))
	listeners, _, err := p.listeners.GetMany(ctx, filter)
	return listeners, err
}

func (p *sqlPersistence) GetListener(ctx context.Context, listenerID *fftypes.UUID) (*apitypes.Listener, error) {
	return p.listeners.GetByID(ctx, listenerID.String())
}

func (p *sqlPersistence) WriteListener(ctx context.Context, spec *apitypes.Listener) error {
	_, err := p.listeners.Upsert(ctx, spec, dbsql.UpsertOptimizationNew)
	return err
}

func (p *sqlPersistence) DeleteListener(ctx context.Context, listenerID *fftypes.UUID) error {
	return p.listeners.Delete(ctx, listenerID.String())
}
