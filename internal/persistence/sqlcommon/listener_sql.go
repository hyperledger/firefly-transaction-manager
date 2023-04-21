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

var listenerColumns = []string{
	"id",
	"name",
	"stream_id",
	"eth_compat_address",
	"eth_compat_event",
	"eth_compat_methods",
	"filters",
	"options",
	"signature",
	"from_block",
	"created",
	"updated",
}

var listenerFilterFieldMap = map[string]string{
	"stream.id": "stream_id",
}

const listenersTable = "listeners"

func (s *SQLCommon) InsertListener(ctx context.Context, listener *apitypes.Listener) (err error) {
	ctx, tx, autoCommit, err := s.Database.BeginOrUseTx(ctx)
	if err != nil {
		return err
	}
	defer s.Database.RollbackTx(ctx, tx, autoCommit)

	var ethCompatEventJSON []byte
	var ethCompatMethodsJSON []byte
	var filtersJSON []byte
	var optionsJSON []byte

	if listener.EthCompatEvent != nil {
		if ethCompatEventJSON, err = json.Marshal(listener.EthCompatEvent); err != nil {
			return err
		}
	}
	if listener.EthCompatMethods != nil {
		if ethCompatMethodsJSON, err = json.Marshal(listener.EthCompatMethods); err != nil {
			return err
		}
	}
	if listener.Filters != nil {
		if filtersJSON, err = json.Marshal(listener.Filters); err != nil {
			return err
		}
	}
	if listener.Options != nil {
		if optionsJSON, err = json.Marshal(listener.Options); err != nil {
			return err
		}
	}

	listener.Created = fftypes.Now()
	if _, err = s.Database.InsertTx(ctx, listenersTable, tx,
		sq.Insert(listenersTable).
			Columns(listenerColumns...).
			Values(
				&listener.ID,
				&listener.Name,
				&listener.StreamID,
				&listener.EthCompatAddress,
				&ethCompatEventJSON,
				&ethCompatMethodsJSON,
				&filtersJSON,
				&optionsJSON,
				&listener.Signature,
				&listener.FromBlock,
				&listener.Created,
				&listener.Updated,
			),
		nil,
	); err != nil {
		return err
	}
	return s.Database.CommitTx(ctx, tx, autoCommit)
}

func (s *SQLCommon) listenerResult(ctx context.Context, row *sql.Rows) (*apitypes.Listener, error) {
	listener := apitypes.Listener{}

	var ethCompatEventJSON sql.NullString
	var ethCompatMethodsJSON sql.NullString
	var filtersJSON sql.NullString
	var optionsJSON sql.NullString

	err := row.Scan(
		&listener.ID,
		&listener.Name,
		&listener.StreamID,
		&listener.EthCompatAddress,
		&ethCompatEventJSON,
		&ethCompatMethodsJSON,
		&filtersJSON,
		&optionsJSON,
		&listener.Signature,
		&listener.FromBlock,
		&listener.Created,
		&listener.Updated,
	)
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, listenersTable)
	}

	if ethCompatEventJSON.Valid {
		if err := json.Unmarshal([]byte(ethCompatEventJSON.String), listener.EthCompatEvent); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, listenersTable)
		}
	}

	if ethCompatMethodsJSON.Valid {
		if err := json.Unmarshal([]byte(ethCompatMethodsJSON.String), listener.EthCompatMethods); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, listenersTable)
		}
	}

	if filtersJSON.Valid {
		if err := json.Unmarshal([]byte(filtersJSON.String), &listener.Filters); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, listenersTable)
		}
	}

	if optionsJSON.Valid {
		if err := json.Unmarshal([]byte(optionsJSON.String), listener.Options); err != nil {
			return nil, i18n.WrapError(ctx, err, tmmsgs.MsgPersistenceReadFailed, listenersTable)
		}
	}

	return &listener, nil
}

func (s *SQLCommon) GetListeners(ctx context.Context, filter ffapi.Filter) (message []*apitypes.Listener, res *ffapi.FilterResult, err error) {

	query, fop, fi, err := s.FilterSelect(
		ctx,
		// TODO: need better documentation for the purpose of this table name field
		// vs the .From(tableName) in the query.
		"",
		sq.Select(listenerColumns...).From(listenersTable),
		filter,
		listenerFilterFieldMap,
		// NB: "sequence" is a special keyword
		// it will be translated to the sequence column field name returned by SequenceColumn function of the database.
		// e.g. for postgres, it will be "seq"
		[]interface{}{"sequence"},
	)
	if err != nil {
		return nil, nil, err
	}

	rows, tx, err := s.Query(ctx, listenersTable, query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	listeners := []*apitypes.Listener{}
	for rows.Next() {
		s, err := s.listenerResult(ctx, rows)
		if err != nil {
			return nil, nil, err
		}
		listeners = append(listeners, s)
	}

	return listeners, s.QueryRes(ctx, listenersTable, tx, fop, fi), err

}

func (s *SQLCommon) getListenerPred(ctx context.Context, desc string, pred interface{}) (*apitypes.Listener, error) {
	rows, _, err := s.Database.Query(ctx, listenersTable,
		sq.Select(listenerColumns...).
			From(listenersTable).
			Where(pred),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		log.L(ctx).Debugf("Listener '%s' not found", desc)
		return nil, nil
	}

	listener, err := s.listenerResult(ctx, rows)
	if err != nil {
		return nil, err
	}

	return listener, nil
}

func (s *SQLCommon) GetListenerByID(ctx context.Context, id *fftypes.UUID) (*apitypes.Listener, error) {
	return s.getListenerPred(ctx, id.String(), sq.Eq{"id": id})
}

func (s *SQLCommon) DeleteListenerByID(ctx context.Context, id *fftypes.UUID) error {
	ctx, tx, autoCommit, err := s.Database.BeginOrUseTx(ctx)
	if err != nil {
		return err
	}
	defer s.Database.RollbackTx(ctx, tx, autoCommit)

	listener, err := s.GetListenerByID(ctx, id)
	if err == nil && listener != nil {
		err = s.Database.DeleteTx(ctx, listenersTable, tx, sq.Delete(listenersTable).Where(sq.Eq{
			"id": id,
		}), nil)
		if err != nil {
			return err
		}
	}

	return s.Database.CommitTx(ctx, tx, autoCommit)
}
