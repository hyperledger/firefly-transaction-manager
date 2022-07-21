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
	"io/ioutil"
	"os"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func newTestLevelDBPersistence(t *testing.T) (*leveldbPersistence, func()) {

	dir, err := ioutil.TempDir("", "ldb_*")
	assert.NoError(t, err)

	tmconfig.Reset()
	config.Set(tmconfig.PersistenceLevelDBPath, dir)

	pp, err := NewLevelDBPersistence(context.Background())
	assert.NoError(t, err)

	// Write some random stuff to the DB
	p := pp.(*leveldbPersistence)
	for i := 0; i < 26; i++ {
		letter := (byte)('a' + i)
		key := make([]byte, 10)
		for i := range key {
			key[i] = letter
		}
		err := p.db.Put(key, key, &opt.WriteOptions{})
		assert.NoError(t, err)
	}

	return p, func() {
		p.Close(context.Background())
		os.RemoveAll(dir)
	}

}

func strPtr(s string) *string { return &s }

type badJSONCheckpointType map[bool]bool

func (cp *badJSONCheckpointType) LessThan(b ffcapi.EventListenerCheckpoint) bool {
	return false
}

func TestLevelDBInitMissingPath(t *testing.T) {

	tmconfig.Reset()

	_, err := NewLevelDBPersistence(context.Background())
	assert.Regexp(t, "FF21050", err)

}

func TestLevelDBInitFail(t *testing.T) {
	file, err := ioutil.TempFile("", "ldb_*")
	assert.NoError(t, err)
	ioutil.WriteFile(file.Name(), []byte("not a leveldb"), 0777)
	defer os.Remove(file.Name())

	tmconfig.Reset()
	config.Set(tmconfig.PersistenceLevelDBPath, file.Name())

	_, err = NewLevelDBPersistence(context.Background())
	assert.Error(t, err)

}

func TestReadWriteStreams(t *testing.T) {

	p, done := newTestLevelDBPersistence(t)
	defer done()

	ctx := context.Background()
	s1 := &apitypes.EventStream{
		ID:   apitypes.UUIDVersion1(), // ensure we get sequentially ascending IDs
		Name: strPtr("stream1"),
	}
	p.WriteStream(ctx, s1)
	s2 := &apitypes.EventStream{
		ID:   apitypes.UUIDVersion1(),
		Name: strPtr("stream2"),
	}
	p.WriteStream(ctx, s2)
	s3 := &apitypes.EventStream{
		ID:   apitypes.UUIDVersion1(),
		Name: strPtr("stream3"),
	}
	p.WriteStream(ctx, s3)

	streams, err := p.ListStreams(ctx, nil, 0)
	assert.NoError(t, err)
	assert.Len(t, streams, 3)

	assert.Equal(t, s3.ID, streams[0].ID)
	assert.Equal(t, s2.ID, streams[1].ID)
	assert.Equal(t, s1.ID, streams[2].ID)

	// Test pagination

	streams, err = p.ListStreams(ctx, nil, 2)
	assert.NoError(t, err)
	assert.Len(t, streams, 2)
	assert.Equal(t, s3.ID, streams[0].ID)
	assert.Equal(t, s2.ID, streams[1].ID)

	streams, err = p.ListStreams(ctx, streams[1].ID, 2)
	assert.NoError(t, err)
	assert.Len(t, streams, 1)
	assert.Equal(t, s1.ID, streams[0].ID)

	// Test delete

	err = p.DeleteStream(ctx, s2.ID)
	assert.NoError(t, err)
	streams, err = p.ListStreams(ctx, nil, 2)
	assert.NoError(t, err)
	assert.Len(t, streams, 2)
	assert.Equal(t, s3.ID, streams[0].ID)
	assert.Equal(t, s1.ID, streams[1].ID)

	// Test get direct

	s, err := p.GetStream(ctx, s3.ID)
	assert.NoError(t, err)
	assert.Equal(t, s3.ID, s.ID)
	assert.Equal(t, s3.Name, s.Name)

	s, err = p.GetStream(ctx, s2.ID)
	assert.NoError(t, err)
	assert.Nil(t, s)
}

func TestReadWriteListeners(t *testing.T) {

	p, done := newTestLevelDBPersistence(t)
	defer done()

	ctx := context.Background()

	sID1 := apitypes.UUIDVersion1()
	sID2 := apitypes.UUIDVersion1()

	s1l1 := &apitypes.Listener{
		ID:       apitypes.UUIDVersion1(),
		StreamID: sID1,
	}
	err := p.WriteListener(ctx, s1l1)
	assert.NoError(t, err)

	s2l1 := &apitypes.Listener{
		ID:       apitypes.UUIDVersion1(),
		StreamID: sID2,
	}
	err = p.WriteListener(ctx, s2l1)
	assert.NoError(t, err)

	s1l2 := &apitypes.Listener{
		ID:       apitypes.UUIDVersion1(),
		StreamID: sID1,
	}
	err = p.WriteListener(ctx, s1l2)
	assert.NoError(t, err)

	listeners, err := p.ListListeners(ctx, nil, 0)
	assert.NoError(t, err)
	assert.Len(t, listeners, 3)

	assert.Equal(t, s1l2.ID, listeners[0].ID)
	assert.Equal(t, s2l1.ID, listeners[1].ID)
	assert.Equal(t, s1l1.ID, listeners[2].ID)

	// Test stream filter

	listeners, err = p.ListStreamListeners(ctx, nil, 0, sID1)
	assert.NoError(t, err)
	assert.Len(t, listeners, 2)
	assert.Equal(t, s1l2.ID, listeners[0].ID)
	assert.Equal(t, s1l1.ID, listeners[1].ID)

	// Test delete

	err = p.DeleteListener(ctx, s2l1.ID)
	assert.NoError(t, err)
	listeners, err = p.ListStreamListeners(ctx, nil, 0, sID2)
	assert.NoError(t, err)
	assert.Len(t, listeners, 0)

	// Test get direct

	l, err := p.GetListener(ctx, s1l2.ID)
	assert.NoError(t, err)
	assert.Equal(t, s1l2.ID, l.ID)

	l, err = p.GetListener(ctx, s2l1.ID)
	assert.NoError(t, err)
	assert.Nil(t, l)
}

func TestReadWriteCheckpoints(t *testing.T) {

	p, done := newTestLevelDBPersistence(t)
	defer done()

	ctx := context.Background()
	cp1 := &apitypes.EventStreamCheckpoint{
		StreamID: apitypes.UUIDVersion1(),
	}
	cp2 := &apitypes.EventStreamCheckpoint{
		StreamID: apitypes.UUIDVersion1(),
	}

	err := p.WriteCheckpoint(ctx, cp1)
	assert.NoError(t, err)

	err = p.WriteCheckpoint(ctx, cp2)
	assert.NoError(t, err)

	err = p.DeleteCheckpoint(ctx, cp1.StreamID)
	assert.NoError(t, err)

	err = p.DeleteCheckpoint(ctx, cp1.StreamID)
	assert.NoError(t, err) // No-op

	cp, err := p.GetCheckpoint(ctx, cp1.StreamID)
	assert.NoError(t, err)
	assert.Nil(t, cp)

	cp, err = p.GetCheckpoint(ctx, cp2.StreamID)
	assert.NoError(t, err)
	assert.Equal(t, cp2.StreamID, cp.StreamID)
}

func TestReadWriteManagedTransactions(t *testing.T) {

	p, done := newTestLevelDBPersistence(t)
	defer done()

	ctx := context.Background()
	textTX := func(signer string, nonce int64, status apitypes.TxStatus) *apitypes.ManagedTX {
		tx := &apitypes.ManagedTX{
			ID:         fmt.Sprintf("ns1/%s", fftypes.NewUUID()),
			SequenceID: apitypes.UUIDVersion1(),
			Nonce:      fftypes.NewFFBigInt(nonce),
			Created:    fftypes.Now(),
			Request: &apitypes.TransactionRequest{
				TransactionInput: ffcapi.TransactionInput{
					TransactionHeaders: ffcapi.TransactionHeaders{
						From: signer,
					},
				},
			},
			Status: status,
		}
		err := p.WriteTransaction(ctx, tx, true)
		assert.NoError(t, err)
		return tx
	}

	s1t1 := textTX("0xaaaaa", 10001, apitypes.TxStatusSucceeded)
	s2t1 := textTX("0xbbbbb", 10001, apitypes.TxStatusFailed)
	s1t2 := textTX("0xaaaaa", 10002, apitypes.TxStatusPending)
	s1t3 := textTX("0xaaaaa", 10003, apitypes.TxStatusPending)

	txns, err := p.ListTransactionsByCreateTime(ctx, nil, 0)
	assert.NoError(t, err)
	assert.Len(t, txns, 4)

	assert.Equal(t, s1t3.ID, txns[0].ID)
	assert.Equal(t, s1t2.ID, txns[1].ID)
	assert.Equal(t, s2t1.ID, txns[2].ID)
	assert.Equal(t, s1t1.ID, txns[3].ID)

	// Only list pending

	txns, err = p.ListTransactionsPending(ctx, nil, 0)
	assert.NoError(t, err)
	assert.Len(t, txns, 2)

	assert.Equal(t, s1t3.ID, txns[0].ID)
	assert.Equal(t, s1t2.ID, txns[1].ID)

	// List with time range

	txns, err = p.ListTransactionsByCreateTime(ctx, s1t2, 0)
	assert.NoError(t, err)
	assert.Len(t, txns, 2)
	assert.Equal(t, s2t1.ID, txns[0].ID)
	assert.Equal(t, s1t1.ID, txns[1].ID)

	// Test delete, and querying by nonce to limit TX returned

	err = p.DeleteTransaction(ctx, s1t2.ID)
	assert.NoError(t, err)
	txns, err = p.ListTransactionsByNonce(ctx, "0xaaaaa", s1t3.Nonce, 0)
	assert.NoError(t, err)
	assert.Len(t, txns, 1)
	assert.Equal(t, s1t1.ID, txns[0].ID)

	// Test get direct

	v, err := p.GetTransactionByID(ctx, s1t3.ID)
	assert.NoError(t, err)
	assert.Equal(t, s1t3.ID, v.ID)
	assert.Equal(t, s1t3.Nonce, v.Nonce)

	v, err = p.GetTransactionByNonce(ctx, "0xbbbbb", s2t1.Nonce)
	assert.NoError(t, err)
	assert.Equal(t, s2t1.ID, v.ID)
	assert.Equal(t, s2t1.Nonce, v.Nonce)

	v, err = p.GetTransactionByID(ctx, s1t2.ID)
	assert.NoError(t, err)
	assert.Nil(t, v)
}

func TestListStreamsBadJSON(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	sID := apitypes.UUIDVersion1()
	err := p.db.Put(prefixedKey(eventstreamsPrefix, sID), []byte("{! not json"), &opt.WriteOptions{})
	assert.NoError(t, err)

	_, err = p.ListStreams(context.Background(), nil, 0)
	assert.Error(t, err)

}

func TestListListenersBadJSON(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	lID := apitypes.UUIDVersion1()
	err := p.db.Put(prefixedKey(listenersPrefix, lID), []byte("{! not json"), &opt.WriteOptions{})
	assert.NoError(t, err)

	_, err = p.ListListeners(context.Background(), nil, 0)
	assert.Error(t, err)

	_, err = p.ListStreamListeners(context.Background(), nil, 0, apitypes.UUIDVersion1())
	assert.Error(t, err)

}

func TestDeleteStreamFail(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	p.db.Close()

	err := p.DeleteStream(context.Background(), apitypes.UUIDVersion1())
	assert.Error(t, err)

}

func TestWriteCheckpointFailMarshal(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	p.db.Close()

	id1 := apitypes.UUIDVersion1()
	err := p.WriteCheckpoint(context.Background(), &apitypes.EventStreamCheckpoint{
		Listeners: map[fftypes.UUID]json.RawMessage{
			*id1: json.RawMessage([]byte(`{"bad": "json"!`)),
		},
	})
	assert.Error(t, err)

}

func TestWriteCheckpointFail(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	p.db.Close()

	id1 := apitypes.UUIDVersion1()
	err := p.WriteCheckpoint(context.Background(), &apitypes.EventStreamCheckpoint{
		Listeners: map[fftypes.UUID]json.RawMessage{
			*id1: json.RawMessage([]byte(`{}`)),
		},
	})
	assert.Error(t, err)

}

func TestReadListenerFail(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	p.db.Close()

	_, err := p.GetListener(context.Background(), apitypes.UUIDVersion1())
	assert.Error(t, err)

}

func TestReadCheckpointFail(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	sID := apitypes.UUIDVersion1()
	err := p.db.Put(prefixedKey(checkpointsPrefix, sID), []byte("{! not json"), &opt.WriteOptions{})
	assert.NoError(t, err)

	_, err = p.GetCheckpoint(context.Background(), sID)
	assert.Error(t, err)

}

func TestListManagedTransactionFail(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	tx := &apitypes.ManagedTX{
		ID:         fmt.Sprintf("ns1:%s", apitypes.UUIDVersion1()),
		Created:    fftypes.Now(),
		SequenceID: apitypes.UUIDVersion1(),
	}
	err := p.writeKeyValue(context.Background(), txCreatedIndexKey(tx), txDataKey(tx.ID))
	assert.NoError(t, err)
	err = p.db.Put(txDataKey(tx.ID), []byte("{! not json"), &opt.WriteOptions{})
	assert.NoError(t, err)

	_, err = p.ListTransactionsByCreateTime(context.Background(), nil, 0)
	assert.Error(t, err)

}

func TestListManagedTransactionCleanupOrphans(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	tx := &apitypes.ManagedTX{
		ID:         fmt.Sprintf("ns1:%s", apitypes.UUIDVersion1()),
		Created:    fftypes.Now(),
		SequenceID: apitypes.UUIDVersion1(),
	}
	err := p.writeKeyValue(context.Background(), txCreatedIndexKey(tx), txDataKey(tx.ID))
	assert.NoError(t, err)

	txns, err := p.ListTransactionsByCreateTime(context.Background(), nil, 0)
	assert.NoError(t, err)
	assert.Empty(t, txns)

	cleanedUpIndex, err := p.getKeyValue(context.Background(), txCreatedIndexKey(tx))
	assert.NoError(t, err)
	assert.Nil(t, cleanedUpIndex)

}

func TestListNonceAllocationsFail(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	txID := fmt.Sprintf("ns1:%s", apitypes.UUIDVersion1())
	err := p.writeKeyValue(context.Background(), txNonceAllocationKey("0xaaa", fftypes.NewFFBigInt(12345)), txDataKey(txID))
	assert.NoError(t, err)
	err = p.db.Put(txDataKey(txID), []byte("{! not json"), &opt.WriteOptions{})
	assert.NoError(t, err)

	_, err = p.ListTransactionsByNonce(context.Background(), "0xaaa", nil, 0)
	assert.Error(t, err)

}

func TestListInflightTransactionFail(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	txID := fmt.Sprintf("ns1:%s", apitypes.UUIDVersion1())
	err := p.writeKeyValue(context.Background(), txPendingIndexKey(apitypes.UUIDVersion1()), txDataKey(txID))
	assert.NoError(t, err)
	err = p.db.Put(txDataKey(txID), []byte("{! not json"), &opt.WriteOptions{})
	assert.NoError(t, err)

	_, err = p.ListTransactionsPending(context.Background(), nil, 0)
	assert.Error(t, err)

}

func TestIndexLookupCallbackErr(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()
	p.Close(context.Background())

	_, err := p.indexLookupCallback(context.Background(), ([]byte("any key")))
	assert.NotNil(t, err)

}

func TestIndexLookupCallbackNotFound(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	b, err := p.indexLookupCallback(context.Background(), ([]byte("any key")))
	assert.Nil(t, err)
	assert.Nil(t, b)

}

func TestGetTransactionByNonceFail(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()
	p.Close(context.Background())

	_, err := p.GetTransactionByNonce(context.Background(), "0xaaa", fftypes.NewFFBigInt(12345))
	assert.Regexp(t, "FF21055", err)

}

func TestIterateReverseJSONFailIdxResolve(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	err := p.writeKeyValue(context.Background(), []byte(`test_0/key`), []byte(`test/value`))
	assert.NoError(t, err)
	_, err = p.listJSON(context.Background(),
		"test_0/",
		"test_1",
		"",
		0,
		func() interface{} { return make(map[string]interface{}) },
		func(i interface{}) {},
		func(ctx context.Context, k []byte) ([]byte, error) {
			return nil, fmt.Errorf("pop")
		},
	)
	assert.Regexp(t, "pop", err)

}

func TestIterateReverseJSONSkipIdxResolve(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	err := p.writeKeyValue(context.Background(), []byte(`test_0/key`), []byte(`test/value`))
	assert.NoError(t, err)
	orphans, err := p.listJSON(context.Background(),
		"test_0/",
		"test_1",
		"",
		0,
		func() interface{} { return make(map[string]interface{}) },
		func(_ interface{}) {
			assert.Fail(t, "Should not be called")
		},
		func(ctx context.Context, k []byte) ([]byte, error) {
			return nil, nil
		},
	)
	assert.NoError(t, err)
	assert.Len(t, orphans, 1)

}

func TestCleanupOrphanedTXIdxKeysSwallowError(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()
	p.Close(context.Background())

	p.cleanupOrphanedTXIdxKeys(context.Background(), [][]byte{[]byte("test")})

}

func TestWriteTransactionIncomplete(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	err := p.WriteTransaction(context.Background(), &apitypes.ManagedTX{}, true)
	assert.Regexp(t, "FF21059", err)

}

func TestDeleteTransactionMissing(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	err := p.DeleteTransaction(context.Background(), "missing")
	assert.NoError(t, err)

}
