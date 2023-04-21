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

package sqlcommon

import (
	"context"
	"database/sql"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

var streamColumns = []string{
	"id",
	"name",
	"suspended",
	"type",
	"error_handling",
	"batch_size",
	"batch_timeout",
	"retry_timeout",
	"blocked_retry_delay",
	"eth_batch_timeout",
	"eth_retry_timeout",
	"eth_blocked_retry_delay",
	"webhook",
	"websocket",
	"updated",
	"created",
}

const streamsTable = "streams"

func (s *SQLCommon) InsertEventStream(ctx context.Context, stream *apitypes.EventStream) (err error) {
	ctx, tx, autoCommit, err := s.Database.BeginOrUseTx(ctx)
	if err != nil {
		return err
	}
	defer s.Database.RollbackTx(ctx, tx, autoCommit)

	var webhookJSON []byte
	var webSocketJSON []byte

	if stream.Webhook != nil {
		if webhookJSON, err = json.Marshal(stream.Webhook); err != nil {
			return err
		}
	}
	if stream.WebSocket != nil {
		if webSocketJSON, err = json.Marshal(stream.WebSocket); err != nil {
			return err
		}
	}

	stream.Created = fftypes.Now()
	if _, err = s.Database.InsertTx(ctx, streamsTable, tx,
		sq.Insert(streamsTable).
			Columns(streamColumns...).
			Values(
				&stream.ID,
				&stream.Name,
				&stream.Suspended,
				&stream.Type,
				&stream.ErrorHandling,
				&stream.BatchSize,
				&stream.BatchTimeout,
				&stream.RetryTimeout,
				&stream.BlockedRetryDelay,
				&stream.EthCompatBatchTimeoutMS,
				&stream.EthCompatRetryTimeoutSec,
				&stream.EthCompatBlockedRetryDelaySec,
				&webhookJSON,
				&webSocketJSON,
				&stream.Updated,
				&stream.Created,
			),
		nil,
	); err != nil {
		return err
	}
	return s.Database.CommitTx(ctx, tx, autoCommit)
}

func (s *SQLCommon) streamResult(ctx context.Context, row *sql.Rows) (*apitypes.EventStream, error) {
	stream := apitypes.EventStream{}
	var webhookJSON sql.NullString
	var webSocketJSON sql.NullString
	err := row.Scan(
		&stream.ID,
		&stream.Name,
		&stream.Suspended,
		&stream.Type,
		&stream.ErrorHandling,
		&stream.BatchSize,
		&stream.BatchTimeout,
		&stream.RetryTimeout,
		&stream.BlockedRetryDelay,
		&stream.EthCompatBatchTimeoutMS,
		&stream.EthCompatRetryTimeoutSec,
		&stream.EthCompatBlockedRetryDelaySec,
		&webhookJSON,
		&webSocketJSON,
		&stream.Updated,
		&stream.Created,
	)
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, streamsTable)
	}

	if webhookJSON.Valid {
		if err := json.Unmarshal([]byte(webhookJSON.String), stream.Webhook); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, streamsTable)
		}
	}

	if webSocketJSON.Valid {
		if err := json.Unmarshal([]byte(webSocketJSON.String), stream.WebSocket); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, streamsTable)
		}
	}

	return &stream, nil
}

func (s *SQLCommon) GetStreams(ctx context.Context, filter ffapi.Filter) (message []*apitypes.EventStream, res *ffapi.FilterResult, err error) {

	query, fop, fi, err := s.FilterSelect(
		ctx,
		// TODO: need better documentation for the purpose of this table name field
		// vs the .From(tableName) in the query.
		"",
		sq.Select(streamColumns...).From(streamsTable),
		filter,
		nil,
		// NB: "sequence" is a special keyword
		// it will be translated to the sequence column field name returned by SequenceColumn function of the database.
		// e.g. for postgres, it will be "seq"
		[]interface{}{"sequence"},
	)
	if err != nil {
		return nil, nil, err
	}

	rows, tx, err := s.Query(ctx, streamsTable, query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	streams := []*apitypes.EventStream{}
	for rows.Next() {
		s, err := s.streamResult(ctx, rows)
		if err != nil {
			return nil, nil, err
		}
		streams = append(streams, s)
	}

	return streams, s.QueryRes(ctx, streamsTable, tx, fop, fi), err

}

func (s *SQLCommon) getStreamPred(ctx context.Context, desc string, pred interface{}) (*apitypes.EventStream, error) {
	rows, _, err := s.Database.Query(ctx, streamsTable,
		sq.Select(streamColumns...).
			From(streamsTable).
			Where(pred),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		log.L(ctx).Debugf("Stream '%s' not found", desc)
		return nil, nil
	}

	stream, err := s.streamResult(ctx, rows)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

func (s *SQLCommon) GetStreamByID(ctx context.Context, id *fftypes.UUID) (*apitypes.EventStream, error) {
	return s.getStreamPred(ctx, id.String(), sq.Eq{"id": id})
}

func (s *SQLCommon) DeleteStreamByID(ctx context.Context, id *fftypes.UUID) error {
	ctx, tx, autoCommit, err := s.Database.BeginOrUseTx(ctx)
	if err != nil {
		return err
	}
	defer s.Database.RollbackTx(ctx, tx, autoCommit)

	stream, err := s.GetStreamByID(ctx, id)
	if err == nil && stream != nil {
		err = s.Database.DeleteTx(ctx, streamsTable, tx, sq.Delete(streamsTable).Where(sq.Eq{
			"id": id,
		}), nil)
		if err != nil {
			return err
		}
	}

	return s.Database.CommitTx(ctx, tx, autoCommit)
}
