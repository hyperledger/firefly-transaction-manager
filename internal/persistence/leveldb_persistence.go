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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type leveldbPersistence struct {
	db         *leveldb.DB
	syncWrites bool
}

func NewLevelDBPersistence(ctx context.Context) (Persistence, error) {
	dbPath := config.GetString(tmconfig.PersistenceLevelDBPath)
	db, err := leveldb.OpenFile(dbPath, &opt.Options{
		OpenFilesCacheCapacity: config.GetInt(tmconfig.PersistenceLevelDBMaxHandles),
	})
	if err != nil {
		return nil, err
	}
	return &leveldbPersistence{
		db:         db,
		syncWrites: config.GetBool(tmconfig.PersistenceLevelDBSyncWrites),
	}, nil
}

func (p *leveldbPersistence) checkpointKey(streamID *fftypes.UUID) []byte {
	return []byte(fmt.Sprintf("checkpoints/%s", streamID))
}

func (p *leveldbPersistence) streamKey(streamID *fftypes.UUID) []byte {
	return []byte(fmt.Sprintf("eventstreams/%s", streamID))
}

func (p *leveldbPersistence) listenerKey(listenerID *fftypes.UUID) []byte {
	return []byte(fmt.Sprintf("listeners/%s", listenerID))
}

func (p *leveldbPersistence) writeJSON(ctx context.Context, key []byte, target interface{}) error {
	b, err := json.Marshal(target)
	if err != nil {
		return err
	}
	log.L(ctx).Debugf("Wrote %s", key)
	return p.db.Put(key, b, &opt.WriteOptions{Sync: p.syncWrites})
}

func (p *leveldbPersistence) readJSON(ctx context.Context, key []byte, target interface{}) error {
	b, err := p.db.Get(key, &opt.ReadOptions{})
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil
		}
		return err
	}
	err = json.Unmarshal(b, target)
	if err != nil {
		return err
	}
	log.L(ctx).Debugf("Read %s", key)
	return nil
}

func (p *leveldbPersistence) listJSON(ctx context.Context, prefix, after string, limit int,
	val func() interface{}, // return a pointer to a pointer variable, of the type to unmarshal
	add func(interface{}), // passes back the val() for adding to the list, if the filters match
	filters ...func(interface{}) bool, // filters to apply to the val() after unmarshalling
) error {
	rangeStart := &util.Range{Start: []byte(prefix)}
	if after != "" {
		rangeStart.Start = []byte(prefix + after)
	}
	it := p.db.NewIterator(rangeStart, &opt.ReadOptions{DontFillCache: true})
	defer it.Release()
	count := 0
	skippedAfter := false
itLoop:
	for it.Next() {
		if after != "" && !skippedAfter && bytes.Equal(it.Key(), rangeStart.Start) {
			skippedAfter = true // need to skip the first one, as the range is inclusive
			continue
		}
		if !strings.HasPrefix(string(it.Key()), prefix) {
			break itLoop
		}
		v := val()
		err := json.Unmarshal(it.Value(), v)
		if err != nil {
			return err
		}
		for _, f := range filters {
			if !f(v) {
				continue itLoop
			}
		}
		add(v)
		count++
		if limit > 0 && count >= limit {
			return nil
		}
	}
	log.L(ctx).Debugf("Listed %d items", count)
	return nil
}

func (p *leveldbPersistence) deleteKeys(ctx context.Context, keys ...[]byte) error {
	for _, key := range keys {
		err := p.db.Delete(key, &opt.WriteOptions{Sync: p.syncWrites})
		if err != nil {
			return err
		}
		log.L(ctx).Debugf("Deleted %s", key)
	}
	return nil
}

func (p *leveldbPersistence) WriteCheckpoint(ctx context.Context, checkpoint *apitypes.EventStreamCheckpoint) error {
	return p.writeJSON(ctx, p.checkpointKey(checkpoint.StreamID), checkpoint)
}

func (p *leveldbPersistence) GetCheckpoint(ctx context.Context, streamID *fftypes.UUID) (cp *apitypes.EventStreamCheckpoint, err error) {
	err = p.readJSON(ctx, p.checkpointKey(streamID), &cp)
	return cp, err
}

func (p *leveldbPersistence) DeleteCheckpoint(ctx context.Context, streamID *fftypes.UUID) error {
	return p.deleteKeys(ctx, p.checkpointKey(streamID))
}

func (p *leveldbPersistence) ListStreams(ctx context.Context, after *fftypes.UUID, limit int) ([]*apitypes.EventStream, error) {
	streams := make([]*apitypes.EventStream, 0)
	if err := p.listJSON(ctx, "eventstreams/", after.String(), limit,
		func() interface{} { var v *apitypes.EventStream; return &v },
		func(v interface{}) { streams = append(streams, *(v.(**apitypes.EventStream))) },
	); err != nil {
		return nil, err
	}
	return streams, nil
}

func (p *leveldbPersistence) GetStream(ctx context.Context, streamID *fftypes.UUID) (es *apitypes.EventStream, err error) {
	err = p.readJSON(ctx, p.streamKey(streamID), &es)
	return es, err
}

func (p *leveldbPersistence) WriteStream(ctx context.Context, spec *apitypes.EventStream) error {
	return p.writeJSON(ctx, p.streamKey(spec.ID), spec)
}

func (p *leveldbPersistence) DeleteStream(ctx context.Context, streamID *fftypes.UUID) error {
	return p.deleteKeys(ctx, p.streamKey(streamID))
}

func (p *leveldbPersistence) ListListeners(ctx context.Context, after *fftypes.UUID, limit int) ([]*apitypes.Listener, error) {
	listeners := make([]*apitypes.Listener, 0)
	if err := p.listJSON(ctx, "listeners/", after.String(), limit,
		func() interface{} { var v *apitypes.Listener; return &v },
		func(v interface{}) { listeners = append(listeners, *(v.(**apitypes.Listener))) },
	); err != nil {
		return nil, err
	}
	return listeners, nil
}

func (p *leveldbPersistence) ListStreamListeners(ctx context.Context, after *fftypes.UUID, limit int, streamID *fftypes.UUID) ([]*apitypes.Listener, error) {
	listeners := make([]*apitypes.Listener, 0)
	if err := p.listJSON(ctx, "listeners/", after.String(), limit,
		func() interface{} { var v *apitypes.Listener; return &v },
		func(v interface{}) { listeners = append(listeners, *(v.(**apitypes.Listener))) },
		func(v interface{}) bool { return (*(v.(**apitypes.Listener))).StreamID.Equals(streamID) },
	); err != nil {
		return nil, err
	}
	return listeners, nil
}

func (p *leveldbPersistence) GetListener(ctx context.Context, listenerID *fftypes.UUID) (l *apitypes.Listener, err error) {
	err = p.readJSON(ctx, p.listenerKey(listenerID), &l)
	return l, err
}

func (p *leveldbPersistence) WriteListener(ctx context.Context, spec *apitypes.Listener) error {
	return p.writeJSON(ctx, p.listenerKey(spec.ID), spec)
}

func (p *leveldbPersistence) DeleteListener(ctx context.Context, listenerID *fftypes.UUID) error {
	return p.deleteKeys(ctx, p.listenerKey(listenerID))
}
