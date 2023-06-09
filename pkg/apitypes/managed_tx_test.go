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
	"fmt"
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

func TestTxHistoryRecord(t *testing.T) {
	r := &TXHistoryRecord{
		ID: fftypes.NewUUID(),
	}
	assert.Equal(t, r.ID.String(), r.GetID())
	t1 := fftypes.Now()
	r.SetCreated(t1)
	assert.Equal(t, t1, r.Time)
	t2 := fftypes.Now()
	r.SetUpdated(t2)
	assert.Equal(t, t2, r.LastOccurrence)
}

func TestManagedTX(t *testing.T) {
	u1 := fftypes.NewUUID()
	mtx := &ManagedTX{
		ID: fmt.Sprintf("ns1:%s", u1),
	}
	ns, id, err := fftypes.ParseNamespacedUUID(context.Background(), mtx.ID)
	assert.NoError(t, err)
	assert.Equal(t, "ns1", ns)
	assert.Equal(t, u1, id)
	assert.Equal(t, mtx.ID, mtx.GetID())
	t1 := fftypes.Now()
	mtx.SetCreated(t1)
	assert.Equal(t, t1, mtx.Created)
	t2 := fftypes.Now()
	mtx.SetUpdated(fftypes.Now())
	assert.Equal(t, t2, mtx.Updated)
}

func TestReceiptRecord(t *testing.T) {
	u1 := fftypes.NewUUID()
	r := &ReceiptRecord{
		TransactionID: fmt.Sprintf("ns1:%s", u1),
	}
	assert.Equal(t, r.TransactionID, r.GetID())
	t1 := fftypes.Now()
	r.SetCreated(t1)
	assert.Equal(t, t1, r.Created)
	t2 := fftypes.Now()
	r.SetUpdated(fftypes.Now())
	assert.Equal(t, t2, r.Updated)
}
