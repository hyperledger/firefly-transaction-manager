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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/stretchr/testify/assert"
)

func TestManagedTXSubStatus(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	mtx := ManagedTX{}

	// No sub-status entries initially
	assert.Nil(t, mtx.CurrentSubStatus(ctx))

	// Adding the same sub-status lots of times in succession should only result
	// in a single entry for that instance
	for i := 0; i < 100; i++ {
		mtx.AddSubStatus(ctx, TxSubStatusReceived)
	}

	assert.Equal(t, 1, len(mtx.History))
	assert.Equal(t, "Received", string(mtx.CurrentSubStatus(ctx).Status))

	// Adding a different type of sub-status should result in
	// a new entry in the list
	mtx.AddSubStatus(ctx, TxSubStatusTracking)

	assert.Equal(t, 2, len(mtx.History))

	// Even if many new types are added we shouldn't go over the
	// configured upper limit
	for i := 0; i < 100; i++ {
		mtx.AddSubStatus(ctx, TxSubStatusStale)
		mtx.AddSubStatus(ctx, TxSubStatusTracking)
	}
	assert.Equal(t, 50, len(mtx.History))

	cancelCtx()
}

func TestManagedTXSubStatusRepeat(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	mtx := ManagedTX{}

	// Add a sub-status
	mtx.AddSubStatus(ctx, TxSubStatusReceived)
	assert.Equal(t, 1, len(mtx.History))
	assert.Equal(t, 1, len(mtx.HistorySummary))

	// Add another sub-status
	mtx.AddSubStatus(ctx, TxSubStatusTracking)
	assert.Equal(t, 2, len(mtx.History))
	assert.Equal(t, 2, len(mtx.HistorySummary))

	// Add another that we've seen before
	mtx.AddSubStatus(ctx, TxSubStatusReceived)
	assert.Equal(t, 3, len(mtx.History))        // This goes up
	assert.Equal(t, 2, len(mtx.HistorySummary)) // This doesn't

	cancelCtx()
}

func TestManagedTXSubStatusAction(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	mtx := ManagedTX{}

	// Add at least 1 sub-status
	mtx.AddSubStatus(ctx, TxSubStatusReceived)

	// Add an action
	mtx.AddSubStatusAction(ctx, TxActionAssignNonce, nil, nil)
	assert.Equal(t, 1, len(mtx.History[0].Actions))
	assert.Nil(t, mtx.History[0].Actions[0].LastErrorTime)

	// Add an action
	mtx.AddSubStatusAction(ctx, TxActionRetrieveGasPrice, nil, fftypes.JSONAnyPtr(`{"gasError":"Acme Gas Oracle RC=12345"}`))
	assert.Equal(t, 2, len(mtx.History[0].Actions))
	assert.Equal(t, (*mtx.History[0].Actions[1].LastError).String(), `{"gasError":"Acme Gas Oracle RC=12345"}`)

	// Add the same action which should cause the previous one to inc its counter
	mtx.AddSubStatusAction(ctx, TxActionRetrieveGasPrice, fftypes.JSONAnyPtr(`{"info":"helloworld"}`), fftypes.JSONAnyPtr(`{"error":"nogood"}`))
	assert.Equal(t, 2, len(mtx.History[0].Actions))
	assert.Equal(t, mtx.History[0].Actions[1].Action, TxActionRetrieveGasPrice)
	assert.Equal(t, 2, mtx.History[0].Actions[1].Count)

	// Add the same action but with new error information should update the last error field
	mtx.AddSubStatusAction(ctx, TxActionRetrieveGasPrice, nil, fftypes.JSONAnyPtr(`{"gasError":"Acme Gas Oracle RC=67890"}`))
	assert.Equal(t, 2, len(mtx.History[0].Actions))
	assert.NotNil(t, mtx.History[0].Actions[1].LastErrorTime)
	assert.Equal(t, (*mtx.History[0].Actions[1].LastError).String(), `{"gasError":"Acme Gas Oracle RC=67890"}`)

	// Add a new type of action
	reason := "known_transaction"
	mtx.AddSubStatusAction(ctx, TxActionSubmitTransaction, fftypes.JSONAnyPtr(`{"reason":"`+reason+`"}`), nil)
	assert.Equal(t, 3, len(mtx.History[0].Actions))
	assert.Equal(t, mtx.History[0].Actions[2].Action, TxActionSubmitTransaction)
	assert.Equal(t, 1, mtx.History[0].Actions[2].Count)
	assert.Nil(t, mtx.History[0].Actions[2].LastErrorTime)

	// Add one more type of action

	receiptId := "123456"
	mtx.AddSubStatusAction(ctx, TxActionReceiveReceipt, fftypes.JSONAnyPtr(`{"receiptId":"`+receiptId+`"}`), nil)
	assert.Equal(t, 4, len(mtx.History[0].Actions))
	assert.Equal(t, mtx.History[0].Actions[3].Action, TxActionReceiveReceipt)
	assert.Equal(t, 1, mtx.History[0].Actions[3].Count)
	assert.Nil(t, mtx.History[0].Actions[3].LastErrorTime)

	cancelCtx()
}

func TestManagedTXSubStatusInvalidJSON(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	mtx := ManagedTX{}

	reason := "\"cannot-marshall\""

	// Add a new type of action
	mtx.AddSubStatus(ctx, TxSubStatusReceived)
	mtx.AddSubStatusAction(ctx, TxActionSubmitTransaction, fftypes.JSONAnyPtr(`{"reason":"`+reason+`"}`), nil)
	val, err := json.Marshal(mtx.History)

	// It should never be possible to cause the sub-status history to become un-marshallable
	assert.NoError(t, err)
	assert.Contains(t, string(val), "invalidJson")

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

	assert.Equal(t, 50, len(mtx.History))

	cancelCtx()
}

func TestManagedTXSubStatusMaxActions(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	mtx := ManagedTX{}

	// Add a sub-status
	mtx.AddSubStatus(ctx, TxSubStatusReceived)

	// Add lots of unique sub-status actions which we cap at the configured amount
	for i := 0; i < 100; i++ {
		newAction := fmt.Sprintf("action-%d", i)
		mtx.AddSubStatusAction(ctx, TxAction(newAction), nil, nil)
	}
	assert.Equal(t, 50, len(mtx.History[0].Actions))

	cancelCtx()
}
