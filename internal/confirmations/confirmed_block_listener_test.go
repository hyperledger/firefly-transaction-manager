// Copyright 2019 Kaleido

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package confirmations

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCBLCatchUpToHeadFromZeroNoConfirmations(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager()

	esDispatch := make(chan *apitypes.BlockInfo)

	id := fftypes.NewUUID()

	blocks := testBlockArray(10)

	mbiNum := mca.On("BlockInfoByNumber", mock.Anything, mock.Anything)
	mbiNum.Run(func(args mock.Arguments) { mockBlockNumberReturn(mbiNum, args, blocks) })

	bcm.requiredConfirmations = 0
	cbl := bcm.StartConfirmedBlockListener(id, nil, esDispatch)

	for i := 0; i < len(blocks)-bcm.requiredConfirmations; i++ {
		b := <-esDispatch
		assert.Equal(t, b, transformBlockInfo(&blocks[i].BlockInfo))
	}

	time.Sleep(1 * time.Millisecond)
	assert.Empty(t, cbl.blocksSinceCheckpoint)

	cbl.Stop()
	bcm.Stop()
	mca.AssertExpectations(t)
}

func TestCBLCatchUpToHeadFromZeroWithConfirmations(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager()

	esDispatch := make(chan *apitypes.BlockInfo)

	id := fftypes.NewUUID()

	blocks := testBlockArray(15)

	mbiNum := mca.On("BlockInfoByNumber", mock.Anything, mock.Anything)
	mbiNum.Run(func(args mock.Arguments) { mockBlockNumberReturn(mbiNum, args, blocks) })

	bcm.requiredConfirmations = 5
	cbl := bcm.StartConfirmedBlockListener(id, nil, esDispatch)

	for i := 0; i < len(blocks)-bcm.requiredConfirmations; i++ {
		b := <-esDispatch
		assert.Equal(t, b, transformBlockInfo(&blocks[i].BlockInfo))
	}

	time.Sleep(1 * time.Millisecond)
	assert.Len(t, cbl.blocksSinceCheckpoint, bcm.requiredConfirmations)
	select {
	case <-esDispatch:
		assert.Fail(t, "should not have received block in confirmation window")
	default: // good - we should have the confirmations sat there, but no dispatch
	}

	cbl.Stop()
	bcm.Stop()
	mca.AssertExpectations(t)
}

func TestCBLHandleReorgInConfirmationWindow1(t *testing.T) {
	// test where the reorg happens at the edge of the confirmation window
	testCBLHandleReorgInConfirmationWindow(t,
		10, // blocks in chain before re-org
		5,  // blocks that remain from original chain after re-org
		5,  // required confirmations
	)
}

func TestCBLHandleReorgInConfirmationWindow2(t *testing.T) {
	// test where the reorg happens replacing some blocks
	// WE ALREADY CONFIRMED - meaning we dispatched them incorrectly
	// because the confirmations were not tuned correctly
	testCBLHandleReorgInConfirmationWindow(t,
		10, // blocks in chain before re-org
		0,  // blocks that remain from original chain after re-org
		5,  // required confirmations
	)
}

func TestCBLHandleReorgInConfirmationWindow3(t *testing.T) {
	// test without confirmations, so everything is a problem
	testCBLHandleReorgInConfirmationWindow(t,
		10, // blocks in chain before re-org
		0,  // blocks that remain from original chain after re-org
		0,  // required confirmations
	)
}

func TestCBLHandleReorgInConfirmationWindow4(t *testing.T) {
	// test of a re-org of one
	testCBLHandleReorgInConfirmationWindow(t,
		5, // blocks in chain before re-org
		4, // blocks that remain from original chain after re-org
		4, // required confirmations
	)
}

func testCBLHandleReorgInConfirmationWindow(t *testing.T, blockLenBeforeReorg, overlap, reqConf int) {
	bcm, mca := newTestBlockConfirmationManager()

	esDispatch := make(chan *apitypes.BlockInfo)

	id := fftypes.NewUUID()
	blocksBeforeReorg := testBlockArray(blockLenBeforeReorg)
	blocksAfterReorg := testBlockArray(blockLenBeforeReorg + overlap)
	if overlap > 0 {
		copy(blocksAfterReorg, blocksBeforeReorg[0:overlap])
		blocksAfterReorg[overlap] = &ffcapi.BlockInfoByNumberResponse{
			BlockInfo: blocksAfterReorg[overlap].BlockInfo,
		}
		blocksAfterReorg[overlap].ParentHash = blocksAfterReorg[overlap-1].BlockHash
	}
	checkBlocksSequential(t, "before", blocksBeforeReorg)
	checkBlocksSequential(t, "after ", blocksAfterReorg)

	var cbl *confirmedBlockListener
	var isAfterReorg atomic.Bool
	notificationsDone := make(chan struct{})
	mbiNum := mca.On("BlockInfoByNumber", mock.Anything, mock.Anything)
	mbiNum.Run(func(args mock.Arguments) {
		if isAfterReorg.Load() {
			mockBlockNumberReturn(mbiNum, args, blocksAfterReorg)
		} else {
			mockBlockNumberReturn(mbiNum, args, blocksBeforeReorg)
			// we instigate the re-org when we've returned all the blocks
			if int(args[1].(*ffcapi.BlockInfoByNumberRequest).BlockNumber.Int64()) >= len(blocksBeforeReorg) {
				isAfterReorg.Store(true)
				go func() {
					defer close(notificationsDone)
					// Simulate the modified blocks only coming in with delays
					for i := overlap; i < len(blocksAfterReorg); i++ {
						time.Sleep(100 * time.Microsecond)
						cbl.newBlockHashes <- &ffcapi.BlockHashEvent{
							BlockHashes: []string{blocksAfterReorg[i].BlockHash},
						}
					}
				}()
			}
		}
	})
	// We query by hash only for the notifications, which are only on the after-reorg blocks
	mbiHash := mca.On("BlockInfoByHash", mock.Anything, mock.Anything)
	mbiHash.Run(func(args mock.Arguments) { mockBlockHashReturn(mbiHash, args, blocksAfterReorg) })

	bcm.requiredConfirmations = reqConf
	cbl = bcm.StartConfirmedBlockListener(id, nil, esDispatch)

	for i := 0; i < len(blocksAfterReorg)-bcm.requiredConfirmations; i++ {
		b := <-esDispatch
		dangerArea := len(blocksAfterReorg) - overlap
		if i >= overlap && i < (dangerArea-reqConf) {
			// This would be a bad situation in reality, where a reorg crossed the confirmations
			// boundary. An indication someone incorrectly configured their confirmations
			assert.Equal(t, b, transformBlockInfo(&blocksBeforeReorg[i].BlockInfo))
		} else {
			assert.Equal(t, b, transformBlockInfo(&blocksAfterReorg[i].BlockInfo))
		}
	}

	time.Sleep(1 * time.Millisecond)
	assert.LessOrEqual(t, len(cbl.blocksSinceCheckpoint), bcm.requiredConfirmations)
	select {
	case <-esDispatch:
		assert.Fail(t, "should not have received block in confirmation window")
	default: // good - we should have the confirmations sat there, but no dispatch
	}

	// Wait for the notifications to go through
	<-notificationsDone

	cbl.Stop()
	bcm.Stop()
	mca.AssertExpectations(t)
}

func TestCBLHandleRandomConflictingBlockNotification(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager()

	esDispatch := make(chan *apitypes.BlockInfo)

	id := fftypes.NewUUID()
	blocks := testBlockArray(50)

	randBlock := &ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(3),
			BlockHash:   fftypes.NewRandB32().String(),
			ParentHash:  fftypes.NewRandB32().String(),
		},
	}

	var cbl *confirmedBlockListener
	sentRandom := false
	mbiNum := mca.On("BlockInfoByNumber", mock.Anything, mock.Anything)
	mbiNum.Run(func(args mock.Arguments) {
		mockBlockNumberReturn(mbiNum, args, blocks)
		if !sentRandom && args[1].(*ffcapi.BlockInfoByNumberRequest).BlockNumber.Int64() == 4 {
			sentRandom = true
			cbl.newBlockHashes <- &ffcapi.BlockHashEvent{
				BlockHashes: []string{randBlock.BlockHash},
			}
			// Give notification handler likelihood to run before we continue the by-number getting
			time.Sleep(1 * time.Millisecond)
		}
	})
	// We query by hash only for the notifications, which are only on the after-reorg blocks
	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(req *ffcapi.BlockInfoByHashRequest) bool {
		return req.BlockHash == randBlock.BlockHash
	})).Return(randBlock, ffcapi.ErrorReason(""), nil)

	cbl = bcm.StartConfirmedBlockListener(id, nil, esDispatch)
	cbl.requiredConfirmations = 5

	for i := 0; i < len(blocks)-cbl.requiredConfirmations; i++ {
		b := <-esDispatch
		assert.Equal(t, b, transformBlockInfo(&blocks[i].BlockInfo))
	}

	cbl.Stop()
	bcm.Stop()
	mca.AssertExpectations(t)
}

func TestProcessBlockHashesSwallowsFailure(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager()
	cbs := &confirmedBlockListener{
		ctx: bcm.ctx,
		bcm: bcm,
	}
	blocks := testBlockArray(1)
	mbiHash := mca.On("BlockInfoByHash", mock.Anything, mock.Anything)
	mbiHash.Return(nil, ffcapi.ErrorReasonDownstreamDown, fmt.Errorf("nope"))
	cbs.processBlockHashes([]string{blocks[0].BlockHash})
	bcm.Stop()
	mca.AssertExpectations(t)
}

func TestDispatchAllConfirmedNonBlocking(t *testing.T) {
	bcm, _ := newTestBlockConfirmationManager()
	cbs := &confirmedBlockListener{
		ctx:         bcm.ctx,
		bcm:         bcm,
		eventStream: make(chan<- *apitypes.BlockInfo), // blocks indefinitely
	}

	cbs.requiredConfirmations = 0
	cbs.blocksSinceCheckpoint = []*apitypes.BlockInfo{
		{BlockNumber: fftypes.FFuint64(12345), BlockHash: fftypes.NewRandB32().String()},
	}
	waitForDispatchReturn := make(chan struct{})
	go func() {
		defer close(waitForDispatchReturn)
		cbs.dispatchAllConfirmed()
	}()

	bcm.cancelFunc()
	bcm.Stop()
	<-waitForDispatchReturn
}

func testBlockArray(l int) []*ffcapi.BlockInfoByNumberResponse {
	blocks := make([]*ffcapi.BlockInfoByNumberResponse, l)
	for i := 0; i < l; i++ {
		blocks[i] = &ffcapi.BlockInfoByNumberResponse{
			BlockInfo: ffcapi.BlockInfo{
				BlockNumber: fftypes.NewFFBigInt(int64(i)),
				BlockHash:   fftypes.NewRandB32().String(),
			},
		}
		if i == 0 {
			blocks[i].ParentHash = fftypes.NewRandB32().String()
		} else {
			blocks[i].ParentHash = blocks[i-1].BlockHash
		}
	}
	return blocks
}

func mockBlockNumberReturn(mbiBlockInfoByNumber *mock.Call, args mock.Arguments, blocks []*ffcapi.BlockInfoByNumberResponse) {
	req := args[1].(*ffcapi.BlockInfoByNumberRequest)
	blockNo := int(req.BlockNumber.Uint64())
	if blockNo >= len(blocks) {
		mbiBlockInfoByNumber.Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found"))
	} else {
		mbiBlockInfoByNumber.Return(blocks[blockNo], ffcapi.ErrorReason(""), nil)
	}
}

func mockBlockHashReturn(mbiBlockInfoByHash *mock.Call, args mock.Arguments, blocks []*ffcapi.BlockInfoByNumberResponse) {
	req := args[1].(*ffcapi.BlockInfoByHashRequest)
	for _, b := range blocks {
		if b.BlockHash == req.BlockHash {
			mbiBlockInfoByHash.Return(&ffcapi.BlockInfoByHashResponse{
				BlockInfo: b.BlockInfo,
			}, ffcapi.ErrorReason(""), nil)
			return
		}
	}
	mbiBlockInfoByHash.Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found"))
}

func checkBlocksSequential(t *testing.T, desc string, blocks []*ffcapi.BlockInfoByNumberResponse) {
	blockHashes := make([]string, len(blocks))
	var lastBlock *ffcapi.BlockInfoByNumberResponse
	invalid := false
	for i, b := range blocks {
		assert.NotEmpty(t, b.BlockHash)
		blockHashes[i] = b.BlockHash
		if i == 0 {
			assert.NotEmpty(t, b.ParentHash)
		} else if lastBlock.BlockHash != b.ParentHash {
			invalid = true
		}
		lastBlock = b
	}
	fmt.Printf("%s: %s\n", desc, strings.Join(blockHashes, ","))
	if invalid {
		panic("wrong sequence") // aid to writing tests that build sequences
	}
}
