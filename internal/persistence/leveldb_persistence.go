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
	"encoding/json"
	"fmt"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
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
	if dbPath == "" {
		return nil, i18n.NewError(ctx, tmmsgs.MsgLevelDBPathMissing)
	}
	db, err := leveldb.OpenFile(dbPath, &opt.Options{
		OpenFilesCacheCapacity: config.GetInt(tmconfig.PersistenceLevelDBMaxHandles),
	})
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceInitFailed, dbPath)
	}
	return &leveldbPersistence{
		db:         db,
		syncWrites: config.GetBool(tmconfig.PersistenceLevelDBSyncWrites),
	}, nil
}

const checkpointsPrefix = "checkpoints_0/"
const eventstreamsPrefix = "eventstreams_0/"
const eventstreamsEnd = "eventstreams_1"
const listenersPrefix = "listeners_0/"
const listenersEnd = "listeners_1"
const transactionsPrefix = "transactions_0/"
const transactionsEnd = "transactions_1"
const nonceAllocPrefix = "nonces_0/"
const inflightPrefix = "inflight_0/"
const inflightEnd = "inflight_1"

func signerNonceAllocPrefix(signer string) string {
	return fmt.Sprintf("%s%s_0/", nonceAllocPrefix, signer)
}

func signerNonceKey(signer string, n int64) []byte {
	return []byte(fmt.Sprintf("%s%s_0/%.12d", nonceAllocPrefix, signer, n))
}

func signerNonceAllocEnd(signer string) string {
	return fmt.Sprintf("%s%s_1", nonceAllocPrefix, signer)
}

func prefixedKey(prefix string, id fmt.Stringer) []byte {
	return []byte(fmt.Sprintf("%s%s", prefix, id))
}

func prefixedStrKey(prefix string, k string) []byte {
	return []byte(fmt.Sprintf("%s%s", prefix, k))
}

func (p *leveldbPersistence) writeJSON(ctx context.Context, key []byte, value interface{}) error {
	b, err := json.Marshal(value)
	if err != nil {
		return i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceMarshalFailed)
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
		return i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed)
	}
	err = json.Unmarshal(b, target)
	if err != nil {
		return i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceUnmarshalFailed)
	}
	log.L(ctx).Debugf("Read %s", key)
	return nil
}

func (p *leveldbPersistence) listJSON(ctx context.Context, collectionPrefix, collectionEnd, after string, limit int,
	val func() interface{}, // return a pointer to a pointer variable, of the type to unmarshal
	add func(interface{}), // passes back the val() for adding to the list, if the filters match
	filters ...func(interface{}) bool, // filters to apply to the val() after unmarshalling
) error {
	collectionRange := &util.Range{
		Start: []byte(collectionPrefix),
		Limit: []byte(collectionEnd),
	}
	if after != "" {
		collectionRange.Limit = []byte(collectionPrefix + after)
	}
	it := p.db.NewIterator(collectionRange, &opt.ReadOptions{DontFillCache: true})
	defer it.Release()
	count := 0
	next := it.Last // First iteration of the loop goes to the end
itLoop:
	for next() {
		next = it.Prev // Future iterations call prev (note reverse sort order moving backwards through the selection)
		v := val()
		err := json.Unmarshal(it.Value(), v)
		if err != nil {
			return i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceUnmarshalFailed)
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
			return i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceDeleteFailed)
		}
		log.L(ctx).Debugf("Deleted %s", key)
	}
	return nil
}

func (p *leveldbPersistence) WriteCheckpoint(ctx context.Context, checkpoint *apitypes.EventStreamCheckpoint) error {
	return p.writeJSON(ctx, prefixedKey(checkpointsPrefix, checkpoint.StreamID), checkpoint)
}

func (p *leveldbPersistence) GetCheckpoint(ctx context.Context, streamID *fftypes.UUID) (cp *apitypes.EventStreamCheckpoint, err error) {
	err = p.readJSON(ctx, prefixedKey(checkpointsPrefix, streamID), &cp)
	return cp, err
}

func (p *leveldbPersistence) DeleteCheckpoint(ctx context.Context, streamID *fftypes.UUID) error {
	return p.deleteKeys(ctx, prefixedKey(checkpointsPrefix, streamID))
}

func (p *leveldbPersistence) ListStreams(ctx context.Context, after *fftypes.UUID, limit int) ([]*apitypes.EventStream, error) {
	streams := make([]*apitypes.EventStream, 0)
	if err := p.listJSON(ctx, eventstreamsPrefix, eventstreamsEnd, after.String(), limit,
		func() interface{} { var v *apitypes.EventStream; return &v },
		func(v interface{}) { streams = append(streams, *(v.(**apitypes.EventStream))) },
	); err != nil {
		return nil, err
	}
	return streams, nil
}

func (p *leveldbPersistence) GetStream(ctx context.Context, streamID *fftypes.UUID) (es *apitypes.EventStream, err error) {
	err = p.readJSON(ctx, prefixedKey(eventstreamsPrefix, streamID), &es)
	return es, err
}

func (p *leveldbPersistence) WriteStream(ctx context.Context, spec *apitypes.EventStream) error {
	return p.writeJSON(ctx, prefixedKey(eventstreamsPrefix, spec.ID), spec)
}

func (p *leveldbPersistence) DeleteStream(ctx context.Context, streamID *fftypes.UUID) error {
	return p.deleteKeys(ctx, prefixedKey(eventstreamsPrefix, streamID))
}

func (p *leveldbPersistence) ListListeners(ctx context.Context, after *fftypes.UUID, limit int) ([]*apitypes.Listener, error) {
	listeners := make([]*apitypes.Listener, 0)
	if err := p.listJSON(ctx, listenersPrefix, listenersEnd, after.String(), limit,
		func() interface{} { var v *apitypes.Listener; return &v },
		func(v interface{}) { listeners = append(listeners, *(v.(**apitypes.Listener))) },
	); err != nil {
		return nil, err
	}
	return listeners, nil
}

func (p *leveldbPersistence) ListStreamListeners(ctx context.Context, after *fftypes.UUID, limit int, streamID *fftypes.UUID) ([]*apitypes.Listener, error) {
	listeners := make([]*apitypes.Listener, 0)
	if err := p.listJSON(ctx, listenersPrefix, listenersEnd, after.String(), limit,
		func() interface{} { var v *apitypes.Listener; return &v },
		func(v interface{}) { listeners = append(listeners, *(v.(**apitypes.Listener))) },
		func(v interface{}) bool { return (*(v.(**apitypes.Listener))).StreamID.Equals(streamID) },
	); err != nil {
		return nil, err
	}
	return listeners, nil
}

func (p *leveldbPersistence) GetListener(ctx context.Context, listenerID *fftypes.UUID) (l *apitypes.Listener, err error) {
	err = p.readJSON(ctx, prefixedKey(listenersPrefix, listenerID), &l)
	return l, err
}

func (p *leveldbPersistence) WriteListener(ctx context.Context, spec *apitypes.Listener) error {
	return p.writeJSON(ctx, prefixedKey(listenersPrefix, spec.ID), spec)
}

func (p *leveldbPersistence) DeleteListener(ctx context.Context, listenerID *fftypes.UUID) error {
	return p.deleteKeys(ctx, prefixedKey(listenersPrefix, listenerID))
}

func (p *leveldbPersistence) ListManagedTransactions(ctx context.Context, after string, limit int) ([]*apitypes.ManagedTX, error) {
	transactions := make([]*apitypes.ManagedTX, 0)
	if err := p.listJSON(ctx, transactionsPrefix, transactionsEnd, after, limit,
		func() interface{} { var v *apitypes.ManagedTX; return &v },
		func(v interface{}) { transactions = append(transactions, *(v.(**apitypes.ManagedTX))) },
	); err != nil {
		return nil, err
	}
	return transactions, nil
}

func (p *leveldbPersistence) GetManagedTransaction(ctx context.Context, txID string) (tx *apitypes.ManagedTX, err error) {
	err = p.readJSON(ctx, prefixedStrKey(transactionsPrefix, txID), &tx)
	return tx, err
}

func (p *leveldbPersistence) WriteManagedTransaction(ctx context.Context, tx *apitypes.ManagedTX) error {
	return p.writeJSON(ctx, prefixedStrKey(transactionsPrefix, tx.ID), tx)
}

func (p *leveldbPersistence) DeleteManagedTransaction(ctx context.Context, txID string) error {
	return p.deleteKeys(ctx, prefixedStrKey(transactionsPrefix, txID))
}

func (p *leveldbPersistence) ListNonceAllocations(ctx context.Context, signer string, after *int64, limit int) ([]*apitypes.NonceAllocation, error) {
	nonceAllocations := make([]*apitypes.NonceAllocation, 0)
	afterStr := ""
	if after != nil {
		afterStr = fmt.Sprintf("%.12d", *after)
	}
	if err := p.listJSON(ctx, signerNonceAllocPrefix(signer), signerNonceAllocEnd(signer), afterStr, limit,
		func() interface{} { var v *apitypes.NonceAllocation; return &v },
		func(v interface{}) { nonceAllocations = append(nonceAllocations, *(v.(**apitypes.NonceAllocation))) },
	); err != nil {
		return nil, err
	}
	return nonceAllocations, nil
}

func (p *leveldbPersistence) GetNonceAllocation(ctx context.Context, signer string, nonce int64) (alloc *apitypes.NonceAllocation, err error) {
	err = p.readJSON(ctx, signerNonceKey(signer, nonce), &alloc)
	return alloc, err
}

func (p *leveldbPersistence) WriteNonceAllocation(ctx context.Context, alloc *apitypes.NonceAllocation) error {
	return p.writeJSON(ctx, signerNonceKey(alloc.Signer, alloc.Nonce), alloc)
}

func (p *leveldbPersistence) DeleteNonceAllocation(ctx context.Context, signer string, nonce int64) error {
	return p.deleteKeys(ctx, signerNonceKey(signer, nonce))
}

func (p *leveldbPersistence) ListInflightTransactions(ctx context.Context, after *fftypes.UUID, limit int) ([]*apitypes.InflightTX, error) {
	inflight := make([]*apitypes.InflightTX, 0)
	if err := p.listJSON(ctx, inflightPrefix, inflightEnd, after.String(), limit,
		func() interface{} { var v *apitypes.InflightTX; return &v },
		func(v interface{}) { inflight = append(inflight, *(v.(**apitypes.InflightTX))) },
	); err != nil {
		return nil, err
	}
	return inflight, nil
}

func (p *leveldbPersistence) GetInflightTransaction(ctx context.Context, inflightID *fftypes.UUID) (inflight *apitypes.InflightTX, err error) {
	err = p.readJSON(ctx, prefixedKey(inflightPrefix, inflightID), &inflight)
	return inflight, err
}

func (p *leveldbPersistence) WriteInflightTransaction(ctx context.Context, inflight *apitypes.InflightTX) error {
	return p.writeJSON(ctx, prefixedKey(inflightPrefix, inflight.ID), inflight)
}

func (p *leveldbPersistence) DeleteInflightTransaction(ctx context.Context, inflightID *fftypes.UUID) error {
	return p.deleteKeys(ctx, prefixedKey(inflightPrefix, inflightID))
}

func (p *leveldbPersistence) Close(ctx context.Context) {
	err := p.db.Close()
	if err != nil {
		log.L(ctx).Warnf("Error closing leveldb: %s", err)
	}
}
