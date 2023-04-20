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

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/dbsql"

	// Import migrate file source
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Capabilities defines the capabilities a plugin can report as implementing or not
type Capabilities struct {
	Concurrency bool
}

type SQLCommon struct {
	dbsql.Database
	capabilities *Capabilities
}

func (s *SQLCommon) Init(ctx context.Context, provider dbsql.Provider, config config.Section, capabilities *Capabilities) (err error) {
	s.capabilities = capabilities
	return s.Database.Init(ctx, provider, config)
}

func (s *SQLCommon) Capabilities() *Capabilities { return s.capabilities }
