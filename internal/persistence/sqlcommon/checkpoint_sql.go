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

	sq "github.com/Masterminds/squirrel"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

var checkpointColumns = []string{
	"streamId",
	"listeners",
	"time",
}

const checkpointsTable = "checkpoints"

func (s *SQLCommon) InsertCheckpoint(ctx context.Context, checkpoint *apitypes.EventStreamCheckpoint) (err error) {
	ctx, tx, autoCommit, err := s.Database.BeginOrUseTx(ctx)
	if err != nil {
		return err
	}
	defer s.Database.RollbackTx(ctx, tx, autoCommit)
	if _, err = s.Database.InsertTx(ctx, checkpointsTable, tx,
		sq.Insert(checkpointsTable).
			Columns(checkpointColumns...).
			Values(
				checkpoint.StreamID,
				checkpoint.Listeners,
				checkpoint.Time,
			),
		nil,
	); err != nil {
		return err
	}
	return s.Database.CommitTx(ctx, tx, autoCommit)
}

func (s *SQLCommon) checkpointResult(ctx context.Context, row *sql.Rows) (*apitypes.EventStreamCheckpoint, error) {
	checkpoint := apitypes.EventStreamCheckpoint{}
	err := row.Scan(
		&checkpoint.StreamID,
		&checkpoint.Time,
		&checkpoint.Listeners,
	)
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, checkpointsTable)
	}
	return &checkpoint, nil
}

func (s *SQLCommon) getCheckpointPred(ctx context.Context, desc string, pred interface{}) (*apitypes.EventStreamCheckpoint, error) {
	rows, _, err := s.Database.Query(ctx, checkpointsTable,
		sq.Select(checkpointColumns...).
			From(checkpointsTable).
			Where(pred),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		log.L(ctx).Debugf("Checkpoint '%s' not found", desc)
		return nil, nil
	}

	checkpoint, err := s.checkpointResult(ctx, rows)
	if err != nil {
		return nil, err
	}

	return checkpoint, nil
}

func (s *SQLCommon) GetCheckpointByStreamID(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStreamCheckpoint, error) {
	return s.getCheckpointPred(ctx, streamID.String(), sq.Eq{"streamId": streamID})
}

func (s *SQLCommon) DeleteCheckpointByStreamID(ctx context.Context, streamID *fftypes.UUID) error {
	ctx, tx, autoCommit, err := s.Database.BeginOrUseTx(ctx)
	if err != nil {
		return err
	}
	defer s.Database.RollbackTx(ctx, tx, autoCommit)

	checkpoint, err := s.GetCheckpointByStreamID(ctx, streamID)
	if err == nil && checkpoint != nil {
		err = s.Database.DeleteTx(ctx, checkpointsTable, tx, sq.Delete(checkpointsTable).Where(sq.Eq{
			"streamId": streamID,
		}), nil)
		if err != nil {
			return err
		}
	}

	return s.Database.CommitTx(ctx, tx, autoCommit)
}
