// Copyright © 2024 Kaleido, Inc.
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

package confirmations

import (
	"context"
	"fmt"
	"sync"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-common/pkg/retry"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

// confirmedBlockListener works differently to the main confirmation listener function,
// as an individual checkpoint-restart ordered stream of all blocks from the chain
// that have met a configured threshold of confirmations.
//
// Note that this builds upon the connector-specific block listener, which likely itself
// have some detailed handling of re-orgs at the front of the chian (the EVM one does).
//
// This implementation is thus deliberately simple assuming that when instability is found
// in the notifications it can simply wipe out its view and start again.
type confirmedBlockListener struct {
	bcm                   *blockConfirmationManager
	ctx                   context.Context
	cancelFunc            func()
	id                    *fftypes.UUID
	stateLock             sync.Mutex
	rollingCheckpoint     *uint64
	blocksSinceCheckpoint []*apitypes.BlockInfo
	newBlockHashes        chan *ffcapi.BlockHashEvent
	dispatcherTap         chan struct{}
	eventStream           chan<- *apitypes.BlockInfo
	connector             ffcapi.API
	requiredConfirmations int
	retry                 *retry.Retry
	processorDone         chan struct{}
	dispatcherDone        chan struct{}
}

func (bcm *blockConfirmationManager) StartConfirmedBlockListener(ctx context.Context, id *fftypes.UUID, checkpoint *uint64, eventStream chan<- *apitypes.BlockInfo) error {
	_, err := bcm.startConfirmedBlockListener(ctx, id, checkpoint, eventStream)
	return err
}

func (bcm *blockConfirmationManager) startConfirmedBlockListener(fgCtx context.Context, id *fftypes.UUID, checkpoint *uint64, eventStream chan<- *apitypes.BlockInfo) (*confirmedBlockListener, error) {
	cbl := &confirmedBlockListener{
		bcm: bcm,
		// We need our own listener for each confirmed block stream, and the bcm has to fan out
		newBlockHashes:        make(chan *ffcapi.BlockHashEvent, config.GetInt(tmconfig.ConfirmationsBlockQueueLength)),
		dispatcherTap:         make(chan struct{}, 1),
		id:                    id,
		eventStream:           eventStream,
		requiredConfirmations: bcm.requiredConfirmations,
		connector:             bcm.connector,
		retry:                 bcm.retry,
		rollingCheckpoint:     checkpoint,
		processorDone:         make(chan struct{}),
		dispatcherDone:        make(chan struct{}),
	}
	cbl.ctx, cbl.cancelFunc = context.WithCancel(bcm.ctx)
	// add a log context for this specific confirmation manager (as there are many within the )
	cbl.ctx = log.WithLogField(cbl.ctx, "role", fmt.Sprintf("confirmed_block_stream_%s", id))

	bcm.cblLock.Lock()
	defer bcm.cblLock.Unlock()
	if _, alreadyStarted := bcm.cbls[*id]; alreadyStarted {
		return nil, i18n.NewError(fgCtx, tmmsgs.MsgBlockListenerAlreadyStarted, id)
	}
	bcm.cbls[*id] = cbl

	go cbl.notificationProcessor()
	go cbl.dispatcher()
	return cbl, nil
}

func (bcm *blockConfirmationManager) StopConfirmedBlockListener(fgCtx context.Context, id *fftypes.UUID) error {
	bcm.cblLock.Lock()
	defer bcm.cblLock.Unlock()

	cbs := bcm.cbls[*id]
	if cbs == nil {
		return i18n.NewError(fgCtx, tmmsgs.MsgBlockListenerNotStarted, id)
	}

	// Don't hold lock while waiting, but do re-lock before deleting from the map
	// (means multiple callers could enter this block in the middle, but that's re-entrant)
	bcm.cblLock.Unlock()
	cbs.cancelFunc()
	<-cbs.processorDone
	<-cbs.dispatcherDone
	bcm.cblLock.Lock()

	delete(bcm.cbls, *id)
	return nil
}

// The notificationProcessor processes all notification immediately from the head of the chain
// regardless of how far back in the chain the dispatcher is.
func (cbs *confirmedBlockListener) notificationProcessor() {
	defer close(cbs.processorDone)
	for {
		select {
		case bhe := <-cbs.newBlockHashes:
			cbs.processBlockHashes(bhe.BlockHashes)
		case <-cbs.ctx.Done():
			log.L(cbs.ctx).Debugf("Confirmed block listener stopping")
			return
		}
	}
}

func (cbs *confirmedBlockListener) processBlockHashes(blockHashes []string) {
	for _, blockHash := range blockHashes {
		block, err := cbs.bcm.getBlockByHash(blockHash)
		if err != nil || block == nil {
			// regardless of the failure, as long as we get notified of subsequent
			// blocks that work this will work itself out.
			log.L(cbs.ctx).Errorf("Failed to retrieve block %s: %v", blockHash, err)
			continue
		}
		cbs.processBlockNotification(block)
	}
}

// Whenever we get a new block we try and reconcile it into our current view of the
// canonical chain ahead of the last checkpoint.
// Then we update the state the dispatcher uses to walk forwards from and see what
// is confirmed and ready to dispatch
func (cbs *confirmedBlockListener) processBlockNotification(block *apitypes.BlockInfo) {

	cbs.stateLock.Lock()
	defer cbs.stateLock.Unlock()

	// If the block is before our checkpoint, we ignore it completely
	if cbs.rollingCheckpoint != nil && block.BlockNumber.Uint64() <= *cbs.rollingCheckpoint {
		log.L(cbs.ctx).Debugf("Notification of block %d/%s <= checkpoint %d", block.BlockNumber, block.BlockHash, *cbs.rollingCheckpoint)
		return
	}

	// Otherwise see if it's a conflicting fork to any of our existing blocks
	for idx, existingBlock := range cbs.blocksSinceCheckpoint {
		if existingBlock.BlockNumber == block.BlockNumber {
			// Must discard up to this point
			cbs.blocksSinceCheckpoint = cbs.blocksSinceCheckpoint[0:idx]
			// This block fits, and add this on the end.
			if idx == 0 || block.ParentHash == cbs.blocksSinceCheckpoint[idx-1].BlockHash {
				log.L(cbs.ctx).Debugf("Notification of block %d/%s after block %d/%s", block.BlockNumber, block.BlockHash, existingBlock.BlockNumber, existingBlock.BlockHash)
				cbs.blocksSinceCheckpoint = append(cbs.blocksSinceCheckpoint[0:idx], block)
			} else {
				log.L(cbs.ctx).Debugf("Notification of block %d/%s conflicting with previous block %d/%s", block.BlockNumber, block.BlockHash, existingBlock.BlockNumber, existingBlock.BlockHash)
			}
			break
		}
	}

	// We didn't fit the block into our existing tree, so this means it's ahead of where we are up to.
	// So just ensure the dispatcher is racing up to it
	cbs.tapDispatcher()

}

func (cbs *confirmedBlockListener) tapDispatcher() {
	select {
	case cbs.dispatcherTap <- struct{}{}:
	default:
	}
}

func (cbs *confirmedBlockListener) dispatcher() {
	defer close(cbs.dispatcherDone)
	for {
		// tight spin getting blocks until we it looks like we need to wait for a notification
		for cbs.getNextBlock() {
			// In all cases we ensure that we move our confirmation window forwards.
			// The checkpoint block is always final, and we never move backwards
			cbs.dispatchAllConfirmed()
		}

		select {
		case <-cbs.dispatcherTap:
		case <-cbs.ctx.Done():
			log.L(cbs.ctx).Debugf("Confirmed block dispatcher stopping")
			return
		}

	}
}

func (cbs *confirmedBlockListener) getNextBlock() (more bool) {

	var nextBlock *apitypes.BlockInfo
	err := cbs.retry.Do(cbs.ctx, "next block", func(_ int) (retry bool, err error) {
		// Find the highest block in the lock
		cbs.stateLock.Lock()
		blockNumberToFetch := uint64(0)
		if cbs.rollingCheckpoint != nil {
			blockNumberToFetch = *cbs.rollingCheckpoint + 1
		}
		if len(cbs.blocksSinceCheckpoint) > 0 {
			blockNumberToFetch = cbs.blocksSinceCheckpoint[len(cbs.blocksSinceCheckpoint)-1].BlockNumber.Uint64() + 1
		}
		cbs.stateLock.Unlock()

		// Get the next block
		nextBlock, err = cbs.bcm.getBlockByNumber(blockNumberToFetch, false, "")
		return true, err
	})
	if nextBlock == nil || err != nil {
		// We didn't get the next block, and maybe our context completed
		return false
	}

	// In the lock append it to our list, checking it's valid to append to what we have
	cbs.stateLock.Lock()
	defer cbs.stateLock.Unlock()

	if len(cbs.blocksSinceCheckpoint) > 0 {
		if cbs.blocksSinceCheckpoint[len(cbs.blocksSinceCheckpoint)-1].BlockHash != nextBlock.ParentHash {
			// This doesn't attach to the end of our list. Trim it off and try again.
			cbs.blocksSinceCheckpoint = cbs.blocksSinceCheckpoint[0 : len(cbs.blocksSinceCheckpoint)-1]
			return true
		}
	}

	// We successfully attached it
	cbs.blocksSinceCheckpoint = append(cbs.blocksSinceCheckpoint, nextBlock)
	return true

}

func (cbs *confirmedBlockListener) dispatchAllConfirmed() {
	for {
		var toDispatch *apitypes.BlockInfo
		cbs.stateLock.Lock()
		if len(cbs.blocksSinceCheckpoint) > cbs.requiredConfirmations {
			toDispatch = cbs.blocksSinceCheckpoint[0]
			// don't want memory to grow indefinitely by shifting right, so we create a new slice here
			cbs.blocksSinceCheckpoint = append([]*apitypes.BlockInfo{}, cbs.blocksSinceCheckpoint[1:]...)
			cbs.rollingCheckpoint = ptrTo(toDispatch.BlockNumber.Uint64())
		}
		cbs.stateLock.Unlock()
		if toDispatch == nil {
			return
		}
		select {
		case cbs.eventStream <- toDispatch:
		case <-cbs.ctx.Done():
		}
	}
}
