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
	"fmt"
	"math/big"
	"strconv"

	"database/sql"

	sq "github.com/Masterminds/squirrel"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/dbsql"
	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence/sqlcommon"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"

	// Import pq driver
	_ "github.com/lib/pq"
)

const sequenceName = "sequence"

func NewPostgresPersistence(ctx context.Context, conf config.Section) (persistence.Persistence, error) {
	var pg = &Postgres{}
	err := pg.Init(ctx, conf)
	if err != nil {
		return nil, err
	}

	return pg, err
}

type Postgres struct {
	sqlcommon.SQLCommon
}

func InitPostgresConfig(conf config.Section) {
	psg := &Postgres{}
	psg.InitConfig(conf)
}

func (psql *Postgres) Init(ctx context.Context, config config.Section) error {
	capabilities := &sqlcommon.Capabilities{}
	return psql.SQLCommon.Init(ctx, psql, config, capabilities)
}

func (psql *Postgres) Name() string {
	return "postgres"
}

func (psql *Postgres) SequenceColumn() string {
	return "seq"
}

func (psql *Postgres) MigrationsDir() string {
	return psql.Name()
}

// Attempt to create a unique 64-bit int from the given name, by selecting 4 bytes from the
// beginning and end of the string.
func lockIndex(lockName string) int64 {
	if len(lockName) >= 4 {
		lockName = lockName[0:4] + lockName[len(lockName)-4:]
	}
	return big.NewInt(0).SetBytes([]byte(lockName)).Int64()
}

func (psql *Postgres) Features() dbsql.SQLFeatures {
	features := dbsql.DefaultSQLProviderFeatures()
	features.PlaceholderFormat = sq.Dollar
	features.UseILIKE = false // slower than lower()
	features.AcquireLock = func(lockName string) string {
		return fmt.Sprintf(`SELECT pg_advisory_xact_lock(%d);`, lockIndex(lockName))
	}
	features.MultiRowInsert = true
	return features
}

func (psql *Postgres) ApplyInsertQueryCustomizations(insert sq.InsertBuilder, requestConflictEmptyResult bool) (sq.InsertBuilder, bool) {
	suffix := " RETURNING seq"
	if requestConflictEmptyResult {
		// Caller wants us to return an empty result set on insert conflict, rather than an error
		suffix = fmt.Sprintf(" ON CONFLICT DO NOTHING%s", suffix)
	}
	return insert.Suffix(suffix), true
}

func (psql *Postgres) Open(url string) (*sql.DB, error) {
	return sql.Open(psql.Name(), url)
}

func (psql *Postgres) GetMigrationDriver(db *sql.DB) (migratedb.Driver, error) {
	return postgres.WithInstance(db, &postgres.Config{})
}

func (psql *Postgres) Close(_ context.Context) {}

// CheckpointFilterBuilderFactory filter fields for checkpoint
var CheckpointFilterBuilderFactory = &ffapi.QueryFields{
	"stream.id": &ffapi.UUIDField{},
	"time":      &ffapi.TimeField{},
}

func (psql *Postgres) WriteCheckpoint(ctx context.Context, checkpoint *apitypes.EventStreamCheckpoint) error {
	return psql.SQLCommon.InsertCheckpoint(ctx, checkpoint)
}
func (psql *Postgres) GetCheckpoint(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStreamCheckpoint, error) {
	return psql.SQLCommon.GetCheckpointByStreamID(ctx, streamID)
}
func (psql *Postgres) DeleteCheckpoint(ctx context.Context, streamID *fftypes.UUID) error {
	return psql.SQLCommon.DeleteCheckpointByStreamID(ctx, streamID)
}

// StreamFilterBuilderFactory filter fields for streams
var StreamFilterBuilderFactory = &ffapi.QueryFields{
	"id":        &ffapi.UUIDField{},
	"suspended": &ffapi.BoolField{},
	"type":      &ffapi.StringField{},
	"created":   &ffapi.TimeField{},
}

func (psql *Postgres) ListStreams(ctx context.Context, _ *fftypes.UUID, limit int, dir persistence.SortDirection) ([]*apitypes.EventStream, error) { // reverse UUIDv1 order
	f := StreamFilterBuilderFactory.NewFilterLimit(ctx, uint64(limit))
	sortFieldName := sequenceName
	if dir == persistence.SortDirectionDescending {
		// TODO: I had to read the code to figure out this is how to do descending, docs needed beside the sort field struct definition
		sortFieldName = "-" + sortFieldName
	}
	es, _, err := psql.SQLCommon.GetStreams(ctx, f.And( /**TODO: it's weird to use And just to be able to sort...**/ ).Sort(sortFieldName))
	if err != nil {
		return nil, err
	}
	return es, fmt.Errorf("haven't figured out how to skip the 'after' UUID yet")
}
func (psql *Postgres) GetStream(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStream, error) {
	return psql.SQLCommon.GetStreamByID(ctx, streamID)
}
func (psql *Postgres) WriteStream(ctx context.Context, stream *apitypes.EventStream) error {
	return psql.SQLCommon.InsertEventStream(ctx, stream)
}
func (psql *Postgres) DeleteStream(ctx context.Context, streamID *fftypes.UUID) error {
	return psql.SQLCommon.DeleteStreamByID(ctx, streamID)
}

// ListenerFilterBuilderFactory filter fields for listener
var ListenerFilterBuilderFactory = &ffapi.QueryFields{
	"id":        &ffapi.UUIDField{},
	"stream.id": &ffapi.UUIDField{},
	"name":      &ffapi.StringField{},
	"created":   &ffapi.TimeField{},
}

func (psql *Postgres) ListListeners(ctx context.Context, _ *fftypes.UUID, limit int, dir persistence.SortDirection) ([]*apitypes.Listener, error) { // reverse UUIDv1 order
	f := StreamFilterBuilderFactory.NewFilterLimit(ctx, uint64(limit))
	sortFieldName := sequenceName
	if dir == persistence.SortDirectionDescending {
		// TODO: I had to read the code to figure out this is how to do descending, docs needed beside the sort field struct definition
		sortFieldName = "-" + sortFieldName
	}
	ls, _, err := psql.SQLCommon.GetListeners(ctx, f.And( /**TODO: it's weird to use And just to be able to sort...**/ ).Sort(sortFieldName).Limit(uint64(limit)))
	if err != nil {
		return nil, err
	}
	return ls, fmt.Errorf("haven't figured out how to skip the 'after' UUID yet")
}
func (psql *Postgres) ListStreamListeners(ctx context.Context, _ *fftypes.UUID, limit int, dir persistence.SortDirection, streamID *fftypes.UUID) ([]*apitypes.Listener, error) {
	f := StreamFilterBuilderFactory.NewFilterLimit(ctx, uint64(limit))
	sortFieldName := sequenceName
	if dir == persistence.SortDirectionDescending {
		// TODO: I had to read the code to figure out this is how to do descending, docs needed beside the sort field struct definition
		sortFieldName = "-" + sortFieldName
	}
	ls, _, err := psql.SQLCommon.GetListeners(ctx, f.And(f.Eq("stream.id", streamID)).Sort(sortFieldName).Limit(uint64(limit)))
	if err != nil {
		return nil, err
	}
	return ls, fmt.Errorf("haven't figured out how to skip the 'after' UUID yet")
}
func (psql *Postgres) GetListener(ctx context.Context, listenerID *fftypes.UUID) (*apitypes.Listener, error) {
	return psql.SQLCommon.GetListenerByID(ctx, listenerID)
}
func (psql *Postgres) WriteListener(ctx context.Context, listener *apitypes.Listener) error {
	return psql.SQLCommon.InsertListener(ctx, listener)
}
func (psql *Postgres) DeleteListener(ctx context.Context, listenerID *fftypes.UUID) error {
	return psql.SQLCommon.DeleteListenerByID(ctx, listenerID)
}

// TransactionFilterBuilderFactory filter fields for txs
var TransactionFilterBuilderFactory = &ffapi.QueryFields{
	"id":      &ffapi.StringField{},
	"status":  &ffapi.StringField{},
	"nonce":   &ffapi.Int64Field{},
	"signer":  &ffapi.StringField{},
	"created": &ffapi.TimeField{},
}

func (psql *Postgres) ListTransactionsByCreateTime(ctx context.Context, _ *apitypes.ManagedTX, limit int, dir persistence.SortDirection) ([]*apitypes.ManagedTX, error) { // reverse create time order
	f := StreamFilterBuilderFactory.NewFilterLimit(ctx, uint64(limit))
	sortFieldName := "created"
	if dir == persistence.SortDirectionDescending {
		// TODO: I had to read the code to figure out this is how to do descending, docs needed beside the sort field struct definition
		sortFieldName = "-" + sortFieldName
	}
	txs, _, err := psql.SQLCommon.GetTransactions(ctx, f.And( /**TODO: it's weird to use And just to be able to sort...**/ ).Sort(sortFieldName).Limit(uint64(limit)))
	if err != nil {
		return nil, err
	}
	return txs, fmt.Errorf("haven't figured out how to skip the 'after' UUID yet")
}
func (psql *Postgres) ListTransactionsByNonce(ctx context.Context, signer string, after *fftypes.FFBigInt, limit int, dir persistence.SortDirection) ([]*apitypes.ManagedTX, error) { // reverse nonce order within signer
	f := StreamFilterBuilderFactory.NewFilterLimit(ctx, uint64(limit))
	sortFieldName := "nonce"
	if dir == persistence.SortDirectionDescending {
		// TODO: I had to read the code to figure out this is how to do descending, docs needed beside the sort field struct definition
		sortFieldName = "-" + sortFieldName
	}
	txs, _, err := psql.SQLCommon.GetTransactions(ctx, f.And(f.Eq("signer", signer), f.Gt("nonce", after)).Sort(sortFieldName).Limit(uint64(limit)))
	if err != nil {
		return nil, err
	}
	return txs, nil
}
func (psql *Postgres) ListTransactionsPending(ctx context.Context, afterSequenceID string, limit int, dir persistence.SortDirection) ([]*apitypes.ManagedTX, error) { // reverse UUIDv1 order, only those in pending state
	f := StreamFilterBuilderFactory.NewFilterLimit(ctx, uint64(limit))
	sortFieldName := sequenceName
	if dir == persistence.SortDirectionDescending {
		// TODO: I had to read the code to figure out this is how to do descending, docs needed beside the sort field struct definition
		sortFieldName = "-" + sortFieldName
	}
	sequenceNumber, err := strconv.Atoi(afterSequenceID)
	if err != nil {
		return nil, i18n.NewError(ctx, tmmsgs.MsgPersistenceSequenceIDInvalid, err.Error())
	}
	txs, _, err := psql.SQLCommon.GetTransactions(ctx, f.And(f.Gt("seq", sequenceNumber), f.Neq("status", apitypes.TxStatusPending)).Sort(sortFieldName).Limit(uint64(limit)))
	if err != nil {
		return nil, err
	}
	return txs, nil
}
func (psql *Postgres) GetTransactionByID(ctx context.Context, txID string) (*apitypes.ManagedTX, error) {
	return psql.SQLCommon.GetTransactionByID(ctx, txID)
}
func (psql *Postgres) GetTransactionByNonce(ctx context.Context, signer string, nonce *fftypes.FFBigInt) (*apitypes.ManagedTX, error) {
	return psql.SQLCommon.GetTransactionByNonce(ctx, signer, nonce)
}
func (psql *Postgres) WriteTransaction(ctx context.Context, tx *apitypes.ManagedTX, new bool) error { // must reject if new is true, and the request ID is no
	if new {
		if tx.ID == "" {
			log.L(ctx).Errorf("Transaction ID must be provided for new transaction")
			return i18n.NewError(ctx, tmmsgs.MsgPersistenceTXIncomplete)
		}
		if tx.SequenceID != "" {
			// for new transactions sequence ID should always be generated by persistence layer
			// as the format of its value is persistence service specific
			log.L(ctx).Errorf("Sequence ID is not allowed for new transaction %s", tx.ID)
			return i18n.NewError(ctx, tmmsgs.MsgPersistenceSequenceIDNotAllowed)
		}

	}
	return psql.SQLCommon.InsertTransaction(ctx, tx)
}
func (psql *Postgres) DeleteTransaction(ctx context.Context, txID string) error {
	return psql.SQLCommon.DeleteTransactionByID(ctx, txID)
}
