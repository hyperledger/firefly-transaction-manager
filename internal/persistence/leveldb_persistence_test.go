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
	"io/ioutil"
	"os"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
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

	assert.Equal(t, s1.ID, streams[0].ID)
	assert.Equal(t, s2.ID, streams[1].ID)
	assert.Equal(t, s3.ID, streams[2].ID)

	// Test pagination

	streams, err = p.ListStreams(ctx, nil, 2)
	assert.NoError(t, err)
	assert.Len(t, streams, 2)
	assert.Equal(t, s1.ID, streams[0].ID)
	assert.Equal(t, s2.ID, streams[1].ID)

	streams, err = p.ListStreams(ctx, streams[1].ID, 2)
	assert.NoError(t, err)
	assert.Len(t, streams, 1)
	assert.Equal(t, s3.ID, streams[0].ID)

	// Test delete

	err = p.DeleteStream(ctx, s2.ID)
	assert.NoError(t, err)
	streams, err = p.ListStreams(ctx, nil, 2)
	assert.NoError(t, err)
	assert.Len(t, streams, 2)
	assert.Equal(t, s1.ID, streams[0].ID)
	assert.Equal(t, s3.ID, streams[1].ID)

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

	assert.Equal(t, s1l1.ID, listeners[0].ID)
	assert.Equal(t, s2l1.ID, listeners[1].ID)
	assert.Equal(t, s1l2.ID, listeners[2].ID)

	// Test stream filter

	listeners, err = p.ListStreamListeners(ctx, nil, 0, sID1)
	assert.NoError(t, err)
	assert.Len(t, listeners, 2)
	assert.Equal(t, s1l1.ID, listeners[0].ID)
	assert.Equal(t, s1l2.ID, listeners[1].ID)

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

func TestListStreamsBadJSON(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	sID := apitypes.UUIDVersion1()
	err := p.db.Put(p.streamKey(sID), []byte("{! not json"), &opt.WriteOptions{})
	assert.NoError(t, err)

	_, err = p.ListStreams(context.Background(), nil, 0)
	assert.Error(t, err)

}

func TestListListenersBadJSON(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	lID := apitypes.UUIDVersion1()
	err := p.db.Put(p.listenerKey(lID), []byte("{! not json"), &opt.WriteOptions{})
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

func TestWriteCheckpointFail(t *testing.T) {
	p, done := newTestLevelDBPersistence(t)
	defer done()

	p.db.Close()

	id1 := apitypes.UUIDVersion1()
	err := p.WriteCheckpoint(context.Background(), &apitypes.EventStreamCheckpoint{
		Listeners: map[fftypes.UUID]*fftypes.JSONAny{
			*id1: fftypes.JSONAnyPtr(`{!!! bad json`),
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
	err := p.db.Put(p.checkpointKey(sID), []byte("{! not json"), &opt.WriteOptions{})
	assert.NoError(t, err)

	_, err = p.GetCheckpoint(context.Background(), sID)
	assert.Error(t, err)

}
