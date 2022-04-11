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
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestBlockConfirmationManager(t *testing.T, enabled bool) (*blockConfirmationManager, *ffcapimocks.API) {
	tmconfig.Reset()
	config.Set(tmconfig.ConfirmationsRequired, 3)
	config.Set(tmconfig.ConfirmationsBlockPollingInterval, "1ms")
	config.Set(tmconfig.ConfirmationsNotificationQueueLength, 1)
	return newTestBlockConfirmationManagerCustomConfig(t)
}

func newTestBlockConfirmationManagerCustomConfig(t *testing.T) (*blockConfirmationManager, *ffcapimocks.API) {
	logrus.SetLevel(logrus.DebugLevel)
	mca := &ffcapimocks.API{}
	bcm, err := NewBlockConfirmationManager(context.Background(), mca)
	assert.NoError(t, err)
	return bcm.(*blockConfirmationManager), mca
}

func TestBCMInitError(t *testing.T) {
	tmconfig.Reset()
	config.Set(tmconfig.ConfirmationsBlockCacheSize, -1)
	mca := &ffcapimocks.API{}
	_, err := NewBlockConfirmationManager(context.Background(), mca)
	assert.Regexp(t, "FF201034", err)
}

func TestBlockConfirmationManagerE2ENewEvent(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager(t, true)

	confirmed := make(chan []BlockInfo, 1)
	eventToConfirm := &EventInfo{
		StreamID:         "stream1",
		TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
		BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
		BlockNumber:      1001,
		TransactionIndex: 5,
		LogIndex:         10,
		Confirmed: func(confirmations []BlockInfo) {
			confirmed <- confirmations
		},
	}
	lastBlockDetected := false

	// Establish the block filter
	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once()

	// First poll for changes gives nothing, but we load up the event at this point for the next round
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Run(func(args mock.Arguments) {
		bcm.Notify(&Notification{
			NotificationType: NewEventLog,
			Event:            eventToConfirm,
		})
	}).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil).Once()

	// Next time round gives a block that is in the confirmation chain, but one block ahead
	block1003 := &BlockInfo{
		BlockNumber: 1003,
		BlockHash:   "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
		ParentHash:  "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
	}
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{block1003.BlockHash},
	}, ffcapi.ErrorReason(""), nil).Once()

	// The next filter gives us 1003 - which is two blocks ahead of our notified log
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == block1003.BlockHash
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1003.BlockNumber)),
		BlockHash:   block1003.BlockHash,
		ParentHash:  block1003.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()

	// Then we should walk the chain by number to fill in 1002/1003, because our HWM is 1003
	block1002 := &BlockInfo{
		BlockNumber: 1002,
		BlockHash:   "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
		ParentHash:  "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
	}
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.GetBlockInfoByNumberResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1002.BlockNumber)),
		BlockHash:   block1002.BlockHash,
		ParentHash:  block1002.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()

	// Then we should walk the chain by number to fill in 1002, because our HWM is 1003.
	// Note this doesn't result in any RPC calls, as we just cached the block and it matches

	// Then we get notified of 1004 to complete the last confirmation
	block1004 := &BlockInfo{
		BlockNumber: 1004,
		BlockHash:   "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
		ParentHash:  "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
	}
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{block1004.BlockHash},
	}, ffcapi.ErrorReason(""), nil).Once()

	// Which then gets downloaded, and should complete the confirmation
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == block1004.BlockHash
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1004.BlockNumber)),
		BlockHash:   block1004.BlockHash,
		ParentHash:  block1004.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()

	// Subsequent calls get nothing, and blocks until close anyway
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Run(func(args mock.Arguments) {
		if lastBlockDetected {
			<-bcm.ctx.Done()
		}
	}).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil).Maybe()

	bcm.Start()

	dispatched := <-confirmed
	assert.Equal(t, []BlockInfo{
		*block1002,
		*block1003,
		*block1004,
	}, dispatched)

	bcm.Stop()
	<-bcm.done

	mca.AssertExpectations(t)
}

func TestBlockConfirmationManagerE2EFork(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager(t, true)

	confirmed := make(chan []BlockInfo, 1)
	eventToConfirm := &EventInfo{
		StreamID:         "stream1",
		TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
		BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
		BlockNumber:      1001,
		TransactionIndex: 5,
		LogIndex:         10,
		Confirmed: func(confirmations []BlockInfo) {
			confirmed <- confirmations
		},
	}
	lastBlockDetected := false

	// Establish the block filter
	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once()

	// The next filter gives us 1002, and a first 1003 block - which will later be removed
	block1002 := &BlockInfo{
		BlockNumber: 1002,
		BlockHash:   "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
		ParentHash:  "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
	}
	block1003a := &BlockInfo{
		BlockNumber: 1003,
		BlockHash:   "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
		ParentHash:  "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
	}
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{
			block1002.BlockHash,
			block1003a.BlockHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == block1002.BlockHash
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1002.BlockNumber)),
		BlockHash:   block1002.BlockHash,
		ParentHash:  block1002.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == block1003a.BlockHash
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1003a.BlockNumber)),
		BlockHash:   block1003a.BlockHash,
		ParentHash:  block1003a.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()

	// Then we get the final fork up to our confirmation
	block1003b := &BlockInfo{
		BlockNumber: 1003,
		BlockHash:   "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
		ParentHash:  "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
	}
	block1004 := &BlockInfo{
		BlockNumber: 1004,
		BlockHash:   "0x110282339db2dfe4bfd13d78375f7883048cac6bc12f8393bd080a4e263d5d21",
		ParentHash:  "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
	}
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{
			block1003b.BlockHash,
			block1004.BlockHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == block1003b.BlockHash
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1003b.BlockNumber)),
		BlockHash:   block1003b.BlockHash,
		ParentHash:  block1003b.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == block1004.BlockHash
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1004.BlockNumber)),
		BlockHash:   block1004.BlockHash,
		ParentHash:  block1004.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()

	// Subsequent calls get nothing, and blocks until close anyway
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Run(func(args mock.Arguments) {
		if lastBlockDetected {
			<-bcm.ctx.Done()
		}
	}).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil).Maybe()

	bcm.Start()

	bcm.Notify(&Notification{
		NotificationType: NewEventLog,
		Event:            eventToConfirm,
	})

	dispatched := <-confirmed
	assert.Equal(t, []BlockInfo{
		*block1002,
		*block1003b,
		*block1004,
	}, dispatched)

	bcm.Stop()
	<-bcm.done

	mca.AssertExpectations(t)

}

func TestBlockConfirmationManagerE2ETransactionMovedFork(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager(t, true)

	confirmed := make(chan []BlockInfo, 1)
	txToConfirmForkA := &TransactionInfo{
		TransactionHash: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		BlockHash:       "0x33eb56730878a08e126f2d52b19242d3b3127dc7611447255928be91b2dda455",
		BlockNumber:     1001,
		Confirmed: func(confirmations []BlockInfo) {
			assert.Fail(t, "this is not the fork we are looking for")
		},
	}
	block1002a := &BlockInfo{
		BlockNumber: 1002,
		BlockHash:   "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
		ParentHash:  "0x33eb56730878a08e126f2d52b19242d3b3127dc7611447255928be91b2dda455",
	}
	// We start with a notification for this one
	bcm.Notify(&Notification{
		NotificationType: NewTransaction,
		Transaction:      txToConfirmForkA,
	})

	txToConfirmForkB := &TransactionInfo{
		TransactionHash: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		BlockHash:       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		BlockNumber:     1001,
		Confirmed: func(confirmations []BlockInfo) {
			confirmed <- confirmations
		},
	}
	block1002b := &BlockInfo{
		BlockNumber: 1002,
		BlockHash:   "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
		ParentHash:  "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
	}
	lastBlockDetected := false

	// Establish the block filter
	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once()

	// The next filter gives us 1002a, which will later be removed
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{
			block1002a.BlockHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == block1002a.BlockHash
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1002a.BlockNumber)),
		BlockHash:   block1002a.BlockHash,
		ParentHash:  block1002a.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once().
		Run(func(args mock.Arguments) {
			// At this point submit the notification for the TX moving to the other fork
			bcm.Notify(&Notification{
				NotificationType: NewTransaction,
				Transaction:      txToConfirmForkB,
			})
		})

	// Then we get the final fork up to our confirmation
	block1003 := &BlockInfo{
		BlockNumber: 1003,
		BlockHash:   "0xaf47ddbd9ba81736f808045b7fccc2179bba360573b362c82544f7360db0802e",
		ParentHash:  "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
	}
	block1004 := &BlockInfo{
		BlockNumber: 1004,
		BlockHash:   "0x110282339db2dfe4bfd13d78375f7883048cac6bc12f8393bd080a4e263d5d21",
		ParentHash:  "0xaf47ddbd9ba81736f808045b7fccc2179bba360573b362c82544f7360db0802e",
	}
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{
			block1003.BlockHash,
			block1004.BlockHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == block1003.BlockHash
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1003.BlockNumber)),
		BlockHash:   block1003.BlockHash,
		ParentHash:  block1003.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == block1004.BlockHash
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1004.BlockNumber)),
		BlockHash:   block1004.BlockHash,
		ParentHash:  block1004.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()

	// We will go back and ask for block 1002 again, as the hash mismatches our updated notification
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.GetBlockInfoByNumberResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1002b.BlockNumber)),
		BlockHash:   block1002b.BlockHash,
		ParentHash:  block1002b.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()

	// Subsequent calls get nothing, and blocks until close anyway
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Run(func(args mock.Arguments) {
		if lastBlockDetected {
			<-bcm.ctx.Done()
		}
	}).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil).Maybe()

	bcm.Start()

	dispatched := <-confirmed
	assert.Equal(t, []BlockInfo{
		*block1002b,
		*block1003,
		*block1004,
	}, dispatched)

	bcm.Stop()
	<-bcm.done

	mca.AssertExpectations(t)

}

func TestBlockConfirmationManagerE2EHistoricalEvent(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager(t, true)

	confirmed := make(chan []BlockInfo, 1)
	eventToConfirm := &EventInfo{
		StreamID:         "stream1",
		TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
		BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
		BlockNumber:      1001,
		TransactionIndex: 5,
		LogIndex:         10,
		Confirmed: func(confirmations []BlockInfo) {
			confirmed <- confirmations
		},
	}

	// Establish the block filter
	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once()

	// We don't notify of any new blocks
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil)

	// Then we should walk the chain by number to fill in 1002/1003, because our HWM is 1003
	block1002 := &BlockInfo{
		BlockNumber: 1002,
		BlockHash:   "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
		ParentHash:  "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
	}
	block1003 := &BlockInfo{
		BlockNumber: 1003,
		BlockHash:   "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
		ParentHash:  "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
	}
	block1004 := &BlockInfo{
		BlockNumber: 1004,
		BlockHash:   "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
		ParentHash:  "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
	}
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.GetBlockInfoByNumberResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1002.BlockNumber)),
		BlockHash:   block1002.BlockHash,
		ParentHash:  block1002.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1003
	})).Return(&ffcapi.GetBlockInfoByNumberResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1003.BlockNumber)),
		BlockHash:   block1003.BlockHash,
		ParentHash:  block1003.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1004
	})).Return(&ffcapi.GetBlockInfoByNumberResponse{
		BlockNumber: fftypes.NewFFBigInt(int64(block1004.BlockNumber)),
		BlockHash:   block1004.BlockHash,
		ParentHash:  block1004.ParentHash,
	}, ffcapi.ErrorReason(""), nil).Once()

	bcm.Notify(&Notification{
		NotificationType: NewEventLog,
		Event:            eventToConfirm,
	})

	bcm.Start()

	dispatched := <-confirmed
	assert.Equal(t, []BlockInfo{
		*block1002,
		*block1003,
		*block1004,
	}, dispatched)

	bcm.Stop()
	<-bcm.done

	mca.AssertExpectations(t)
}

func TestSortPendingEvents(t *testing.T) {
	events := pendingItems{
		{blockNumber: 1000, transactionIndex: 10, logIndex: 2},
		{blockNumber: 1003, transactionIndex: 0, logIndex: 10},
		{blockNumber: 1000, transactionIndex: 5, logIndex: 5},
		{blockNumber: 1000, transactionIndex: 10, logIndex: 0},
		{blockNumber: 1002, transactionIndex: 0, logIndex: 0},
	}
	sort.Sort(events)
	assert.Equal(t, pendingItems{
		{blockNumber: 1000, transactionIndex: 5, logIndex: 5},
		{blockNumber: 1000, transactionIndex: 10, logIndex: 0},
		{blockNumber: 1000, transactionIndex: 10, logIndex: 2},
		{blockNumber: 1002, transactionIndex: 0, logIndex: 0},
		{blockNumber: 1003, transactionIndex: 0, logIndex: 10},
	}, events)
}

func TestCreateBlockFilterFail(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})
	bcm.blockListenerID = "listener1"

	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(
		&ffcapi.CreateBlockListenerResponse{ListenerID: "listener1"},
		ffcapi.ErrorReason(""),
		fmt.Errorf("pop"),
	).Once().Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})

	bcm.confirmationsListener()

	mca.AssertExpectations(t)
}

func TestConfirmationsListenerFailWalkingChain(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	n := &Notification{
		NotificationType: NewTransaction,
		Transaction: &TransactionInfo{
			TransactionHash: "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
			BlockNumber:     1001,
			BlockHash:       "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
		},
	}
	bcm.addOrReplaceItem(n.transactionPendingItem())

	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once().Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once()

	bcm.confirmationsListener()

	mca.AssertExpectations(t)
}

func TestConfirmationsListenerFailPollingBlocks(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once().Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	bcm.confirmationsListener()

	mca.AssertExpectations(t)
}

func TestConfirmationsListenerLostFilterReestablish(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once().Twice()
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReasonNotFound, fmt.Errorf("pop")).Once()
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil).Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})

	bcm.confirmationsListener()

	mca.AssertExpectations(t)
}

func TestConfirmationsListenerFailWalkingChainForNewEvent(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	confirmed := make(chan []BlockInfo, 1)
	eventToConfirm := &EventInfo{
		StreamID:         "stream1",
		TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
		BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
		BlockNumber:      1001,
		TransactionIndex: 5,
		LogIndex:         10,
		Confirmed: func(confirmations []BlockInfo) {
			confirmed <- confirmations
		},
	}
	bcm.Notify(&Notification{
		NotificationType: NewEventLog,
		Event:            eventToConfirm,
	})

	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once().Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})

	bcm.confirmationsListener()

	mca.AssertExpectations(t)
}

func TestConfirmationsListenerStopStream(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	n := &Notification{
		Event: &EventInfo{
			StreamID:         "stream1",
			TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
			BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
			BlockNumber:      1001,
			TransactionIndex: 5,
			LogIndex:         10,
		},
	}
	bcm.addOrReplaceItem(n.eventPendingItem())
	completed := make(chan struct{})
	bcm.Notify(&Notification{
		NotificationType: StopStream,
		StoppedStream: &StoppedStreamInfo{
			StreamID:  "stream1",
			Completed: completed,
		},
	})

	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Maybe()
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil).Maybe()

	bcm.Start()

	<-completed
	assert.Empty(t, bcm.pending)

	bcm.Stop()
	mca.AssertExpectations(t)
}

func TestConfirmationsRemoveEvent(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	eventInfo := &EventInfo{
		StreamID:         "stream1",
		TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
		BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
		BlockNumber:      1001,
		TransactionIndex: 5,
		LogIndex:         10,
	}
	bcm.addOrReplaceItem((&Notification{
		Event: eventInfo,
	}).eventPendingItem())
	bcm.Notify(&Notification{
		NotificationType: RemovedEventLog,
		Event:            eventInfo,
	})

	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})

	bcm.confirmationsListener()
	<-bcm.done

	assert.Empty(t, bcm.pending)
	mca.AssertExpectations(t)
}

func TestConfirmationsRemoveTransaction(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	txInfo := &TransactionInfo{
		TransactionHash: "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
		BlockHash:       "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
		BlockNumber:     1001,
	}
	bcm.addOrReplaceItem((&Notification{
		Transaction: txInfo,
	}).transactionPendingItem())
	bcm.Notify(&Notification{
		NotificationType: RemovedTransaction,
		Transaction:      txInfo,
	})

	mca.On("CreateBlockListener", mock.Anything, mock.Anything).Return(&ffcapi.CreateBlockListenerResponse{
		ListenerID: "listener1",
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetNewBlockHashes", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetNewBlockHashesRequest) bool {
		return r.ListenerID == "listener1"
	})).Return(&ffcapi.GetNewBlockHashesResponse{
		BlockHashes: []string{},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})

	bcm.confirmationsListener()
	<-bcm.done

	assert.Empty(t, bcm.pending)
	mca.AssertExpectations(t)
}

func TestWalkChainForEventBlockNotInConfirmationChain(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	pending := (&Notification{
		Event: &EventInfo{
			StreamID:         "stream1",
			TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
			BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
			BlockNumber:      1001,
			TransactionIndex: 5,
			LogIndex:         10,
		},
	}).eventPendingItem()

	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.GetBlockInfoByNumberResponse{
		BlockNumber: fftypes.NewFFBigInt(1002),
		BlockHash:   "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
		ParentHash:  "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
	}, ffcapi.ErrorReason(""), nil).Once()

	err := bcm.walkChainForItem(pending)
	assert.NoError(t, err)

	mca.AssertExpectations(t)
}

func TestWalkChainForEventBlockLookupFail(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	pending := (&Notification{
		Event: &EventInfo{
			StreamID:         "stream1",
			TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
			BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
			BlockNumber:      1001,
			TransactionIndex: 5,
			LogIndex:         10,
		},
	}).eventPendingItem()

	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once()

	err := bcm.walkChainForItem(pending)
	assert.Regexp(t, "pop", err)

	mca.AssertExpectations(t)
}

func TestProcessBlockHashesLookupFail(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	blockHash := "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8"
	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == blockHash
	})).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once()

	bcm.processBlockHashes([]string{
		blockHash,
	})

	mca.AssertExpectations(t)
}

func TestProcessNotificationsSwallowsUnknownType(t *testing.T) {

	bcm, _ := newTestBlockConfirmationManager(t, false)
	bcm.processNotifications([]*Notification{
		{NotificationType: NotificationType(999)},
	})
}

func TestGetBlockByNumberForceLookupMismatchedBlockType(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.GetBlockInfoByNumberResponse{
		BlockNumber: fftypes.NewFFBigInt(1002),
		BlockHash:   "0x110282339db2dfe4bfd13d78375f7883048cac6bc12f8393bd080a4e263d5d21",
		ParentHash:  "0xaf47ddbd9ba81736f808045b7fccc2179bba360573b362c82544f7360db0802e",
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.GetBlockInfoByNumberResponse{
		BlockNumber: fftypes.NewFFBigInt(1002),
		BlockHash:   "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
		ParentHash:  "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
	}, ffcapi.ErrorReason(""), nil).Once()

	// Make the first call that caches
	blockInfo, err := bcm.getBlockByNumber(1002, "0xaf47ddbd9ba81736f808045b7fccc2179bba360573b362c82544f7360db0802e")
	assert.NoError(t, err)
	assert.Equal(t, "0xaf47ddbd9ba81736f808045b7fccc2179bba360573b362c82544f7360db0802e", blockInfo.ParentHash)

	// Make second call that is cached as parent matches
	blockInfo, err = bcm.getBlockByNumber(1002, "0xaf47ddbd9ba81736f808045b7fccc2179bba360573b362c82544f7360db0802e")
	assert.NoError(t, err)
	assert.Equal(t, "0xaf47ddbd9ba81736f808045b7fccc2179bba360573b362c82544f7360db0802e", blockInfo.ParentHash)

	// Make third call that does not as parent mismatched
	blockInfo, err = bcm.getBlockByNumber(1002, "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542")
	assert.NoError(t, err)
	assert.Equal(t, "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542", blockInfo.ParentHash)

}

func TestGetBlockByHashCached(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df"
	})).Return(&ffcapi.GetBlockInfoByHashResponse{
		BlockNumber: fftypes.NewFFBigInt(1003),
		BlockHash:   "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
		ParentHash:  "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
	}, ffcapi.ErrorReason(""), nil).Once()

	blockInfo, err := bcm.getBlockByHash("0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df")
	assert.NoError(t, err)
	assert.Equal(t, "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df", blockInfo.BlockHash)

	// Get again cached
	blockInfo, err = bcm.getBlockByHash("0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df")
	assert.NoError(t, err)
	assert.Equal(t, "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df", blockInfo.BlockHash)

}

func TestGetBlockNotFound(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	mca.On("GetBlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.GetBlockInfoByHashRequest) bool {
		return r.BlockHash == "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df"
	})).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Once()

	blockInfo, err := bcm.getBlockByHash("0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df")
	assert.NoError(t, err)
	assert.Nil(t, blockInfo)

}

func TestPanicBadKey(t *testing.T) {

	pi := &pendingItem{
		pType: pendingType(999),
	}
	assert.Panics(t, func() {
		pi.getKey()
	})

}

func TestNotificationValidation(t *testing.T) {

	bcm, _ := newTestBlockConfirmationManager(t, false)
	bcm.bcmNotifications = make(chan *Notification)

	err := bcm.Notify(&Notification{
		NotificationType: NewTransaction,
	})
	assert.Regexp(t, "FF201035", err)

	err = bcm.Notify(&Notification{
		NotificationType: NewEventLog,
	})
	assert.Regexp(t, "FF201035", err)

	err = bcm.Notify(&Notification{
		NotificationType: StopStream,
	})
	assert.Regexp(t, "FF201035", err)

	bcm.cancelFunc()
	err = bcm.Notify(&Notification{
		NotificationType: NewTransaction,
		Transaction: &TransactionInfo{
			TransactionHash: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
			BlockHash:       "0x33eb56730878a08e126f2d52b19242d3b3127dc7611447255928be91b2dda455",
			BlockNumber:     1001,
			Confirmed:       func(confirmations []BlockInfo) {},
		},
	})
	assert.NoError(t, err)

}
