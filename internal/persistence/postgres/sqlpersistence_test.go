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
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/dbsql"
	"github.com/stretchr/testify/assert"
)

func newMockSQLPersistence(t *testing.T) (context.Context, *sqlPersistence, sqlmock.Sqlmock, func()) {

	ctx, cancelCtx := context.WithCancel(context.Background())
	db, dbm := dbsql.NewMockProvider().UTInit()

	config.RootConfigReset()
	dbconf := config.RootSection("utdb")
	InitConfig(dbconf)

	p, err := newSQLPersistence(ctx, &db.Database, dbconf)
	assert.NoError(t, err)

	return ctx, p, dbm, cancelCtx

}
