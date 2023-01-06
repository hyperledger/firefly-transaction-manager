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
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
)

func TestManagedTXUpdateMsgStringEqual(t *testing.T) {

	assert.Empty(t, ((*ManagedTXUpdate)(nil)).MsgString())
	mtu1 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          10,
		Info:           "things happened",
		Error:          "it was bad",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	mtu2 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          1,
		Info:           "things happened",
		Error:          "it was bad",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	assert.Equal(t, mtu1.MsgString(), mtu2.MsgString())
}

func TestManagedTXUpdateMsgStringNotMatchErr(t *testing.T) {

	mtu1 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          10,
		Info:           "things happened",
		Error:          "it was bad",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	mtu2 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          1,
		Info:           "things happened",
		Error:          "it was more bad",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	assert.NotEqual(t, mtu1.MsgString(), mtu2.MsgString())
}

func TestManagedTXUpdateMsgStringNotMatchReason(t *testing.T) {

	mtu1 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          10,
		Info:           "things happened",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	mtu2 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          1,
		Info:           "things happened",
		MappedReason:   "",
	}
	assert.NotEqual(t, mtu1.MsgString(), mtu2.MsgString())
}

func TestManagedTXUpdateMsgStringNotMatchInfo(t *testing.T) {

	mtu1 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          10,
		Info:           "things happened",
	}
	mtu2 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          1,
		Info:           "then we were done",
	}
	assert.NotEqual(t, mtu1.MsgString(), mtu2.MsgString())
}

func TestManagedTXSubStatus(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	mtx := ManagedTX{}

	// Add lots of sub-status entries, make sure we only
	// store a finite number
	for i := 0; i < 100; i++ {
		mtx.AddSubStatus(ctx, TxSubStatusRetrievingGasPrice)
		mtx.AddSubStatus(ctx, TxSubStatusRetrievedGasPrice)
		mtx.AddSubStatus(ctx, TxSubStatusSubmitting)
		mtx.AddSubStatus(ctx, TxSubStatusSubmitted)
		mtx.AddSubStatus(ctx, TxSubStatusReceivedReceipt)
		mtx.AddSubStatus(ctx, TxSubStatusConfirmed)
	}

	// Ensure we don't store more than one sub-status history
	// per sub-status type
	assert.Equal(t, 6, len(mtx.SubStatusHistory))

	// All of the status entries should have a count of 100
	assert.Equal(t, 100, mtx.SubStatusHistory[0].Count)
	assert.Equal(t, 100, mtx.SubStatusHistory[1].Count)
	assert.Equal(t, 100, mtx.SubStatusHistory[2].Count)
	assert.Equal(t, 100, mtx.SubStatusHistory[3].Count)
	assert.Equal(t, 100, mtx.SubStatusHistory[4].Count)
	assert.Equal(t, 100, mtx.SubStatusHistory[5].Count)

	cancelCtx()
}

func TestManagedTXSubStatusWrapping(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	mtx := ManagedTX{}
	firstEntryFound := false

	// If we wrap we should keep at least one entry of every
	// unique sub-status type. This sub-sstatus should not be
	// lost after the for loop
	mtx.AddSubStatus(ctx, TxSubStatusRetrievingGasPrice)

	for i := 0; i < 100; i++ {
		mtx.AddSubStatus(ctx, TxSubStatusRetrievedGasPrice)
		mtx.AddSubStatus(ctx, TxSubStatusSubmitting)
		mtx.AddSubStatus(ctx, TxSubStatusSubmitted)
		mtx.AddSubStatus(ctx, TxSubStatusReceivedReceipt)
		mtx.AddSubStatus(ctx, TxSubStatusConfirmed)
	}

	assert.Equal(t, 6, len(mtx.SubStatusHistory))

	// Check that all sub-status entries are retained in the list
	// by making sure that the first one we added is still in the list
	for _, entry := range mtx.SubStatusHistory {
		if entry.Status == TxSubStatusRetrievingGasPrice {
			firstEntryFound = true
		}
	}

	assert.True(t, firstEntryFound)

	cancelCtx()
}

func TestManagedTXSubStatusMaxEntries(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	mtx := ManagedTX{}
	var nextSubStatus TxSubStatus

	// Create 100 unique sub-status strings. We should only keep the
	// first 50
	for i := 0; i < 100; i++ {
		nextSubStatus = TxSubStatus(fmt.Sprint(i))
		mtx.AddSubStatus(ctx, nextSubStatus)
	}

	assert.Equal(t, 50, len(mtx.SubStatusHistory))

	cancelCtx()
}
