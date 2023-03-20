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

package apitypes

import (
	"context"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/stretchr/testify/assert"
)

func TestManagedTxNamespace(t *testing.T) {
	ctx := context.Background()
	mtx := &ManagedTX{
		ID: "not a valid ID",
	}

	// returns empty string when the ID is invalid
	ns := mtx.Namespace(ctx)
	assert.Equal(t, "", ns)

	// returns the namespace correctly
	mtx.ID = fftypes.NewNamespacedUUIDString(ctx, "ns1", fftypes.NewUUID())
	ns = mtx.Namespace(ctx)
	assert.Equal(t, "ns1", ns)
}
