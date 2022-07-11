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

package events

import (
	"context"
	"sync"
	"testing"

	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
)

func TestBlockListenerDoesNotBlock(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)

	ss := &startedStreamState{
		blocks:            make(chan *ffcapi.BlockHashEvent, 1),
		blockListenerDone: make(chan struct{}),
	}
	ss.ctx, ss.cancelCtx = context.WithCancel(context.Background())

	blockIt := make(chan *ffcapi.BlockHashEvent, 1)
	mcm := &confirmationsmocks.Manager{}
	mcm.On("NewBlockHashes").Return((chan<- *ffcapi.BlockHashEvent)(blockIt))
	es.confirmations = mcm

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		es.blockListener(ss)
		wg.Done()
	}()

	for i := 0; i < 100; i++ {
		ss.blocks <- &ffcapi.BlockHashEvent{}
	}

	// Get the one that was stuck in the pipe
	bhe := <-blockIt
	assert.False(t, bhe.GapPotential)

	// We should get the unblocking one too, with GapPotential set
	bhe = <-blockIt
	assert.True(t, bhe.GapPotential)

	// Block it again
	for i := 0; i < 100; i++ {
		ss.blocks <- &ffcapi.BlockHashEvent{}
	}

	// And check we can exit while blocked
	ss.cancelCtx()
	wg.Wait()

}
