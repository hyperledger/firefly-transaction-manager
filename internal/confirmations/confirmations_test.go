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
	"time"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestBlockConfirmationManager(t *testing.T, enabled bool) (*blockConfirmationManager, *ffcapimocks.API) {
	tmconfig.Reset()
	config.Set(tmconfig.ConfirmationsRequired, 3)
	config.Set(tmconfig.ConfirmationsNotificationQueueLength, 1)
	return newTestBlockConfirmationManagerCustomConfig(t)
}

func newTestBlockConfirmationManagerCustomConfig(t *testing.T) (*blockConfirmationManager, *ffcapimocks.API) {
	logrus.SetLevel(logrus.DebugLevel)
	mca := &ffcapimocks.API{}
	bcm := NewBlockConfirmationManager(context.Background(), mca, "ut")
	return bcm.(*blockConfirmationManager), mca
}

func TestBlockConfirmationManagerE2ENewEvent(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager(t, true)

	confirmed := make(chan []BlockInfo, 1)
	eventToConfirm := &EventInfo{
		ID: &ffcapi.EventID{
			ListenerID:       fftypes.NewUUID(),
			TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
			BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
			BlockNumber:      1001,
			TransactionIndex: 5,
			LogIndex:         10,
		},
		Confirmed: func(ctx context.Context, confirmations []BlockInfo) {
			confirmed <- confirmations
		},
	}

	// First poll for changes gives nothing, but we load up the event at this point for the next round
	blockHashes := bcm.NewBlockHashes()

	// Next time round gives a block that is in the confirmation chain, but one block ahead
	block1003 := &BlockInfo{
		BlockNumber: 1003,
		BlockHash:   "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
		ParentHash:  "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
	}
	blockHashes <- &ffcapi.BlockHashEvent{
		BlockHashes: []string{block1003.BlockHash},
	}

	// The next filter gives us 1003 - which is two blocks ahead of our notified log
	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1003.BlockHash
	})).Run(func(args mock.Arguments) {
		err := bcm.Notify(&Notification{
			NotificationType: NewEventLog,
			Event:            eventToConfirm,
		})
		assert.NoError(t, err)
	}).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1003.BlockNumber)),
			BlockHash:   block1003.BlockHash,
			ParentHash:  block1003.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()

	// Then we should walk the chain by number to fill in 1002/1003, because our HWM is 1003
	block1002 := &BlockInfo{
		BlockNumber: 1002,
		BlockHash:   "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
		ParentHash:  "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
	}
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.BlockInfoByNumberResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1002.BlockNumber)),
			BlockHash:   block1002.BlockHash,
			ParentHash:  block1002.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil)

	// Notify of 1004 after we download 1003
	block1004 := &BlockInfo{
		BlockNumber: 1004,
		BlockHash:   "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
		ParentHash:  "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
	}

	// Then we should walk the chain by number to fill in 1003, because our HWM is 1003.
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1003
	})).Run(func(args mock.Arguments) {
		blockHashes <- &ffcapi.BlockHashEvent{
			BlockHashes: []string{block1004.BlockHash},
		}
	}).Return(&ffcapi.BlockInfoByNumberResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1003.BlockNumber)),
			BlockHash:   block1003.BlockHash,
			ParentHash:  block1003.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil)

	// Which then gets downloaded, and should complete the confirmation
	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1004.BlockHash
	})).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1004.BlockNumber)),
			BlockHash:   block1004.BlockHash,
			ParentHash:  block1004.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()

	bcm.Start()

	dispatched := <-confirmed
	assert.Equal(t, []BlockInfo{
		*block1002,
		*block1003,
		*block1004,
	}, dispatched)

	bcm.Stop()

	mca.AssertExpectations(t)
}

func TestBlockConfirmationManagerE2EFork(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager(t, true)

	confirmed := make(chan []BlockInfo, 1)
	eventToConfirm := &EventInfo{
		ID: &ffcapi.EventID{
			ListenerID:       fftypes.NewUUID(),
			TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
			BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
			BlockNumber:      1001,
			TransactionIndex: 5,
			LogIndex:         10,
		},
		Confirmed: func(ctx context.Context, confirmations []BlockInfo) {
			confirmed <- confirmations
		},
	}

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

	blockHashes := bcm.NewBlockHashes()
	blockHashes <- &ffcapi.BlockHashEvent{
		BlockHashes: []string{
			block1002.BlockHash,
			block1003a.BlockHash,
		},
	}

	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1002.BlockHash
	})).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1002.BlockNumber)),
			BlockHash:   block1002.BlockHash,
			ParentHash:  block1002.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1003a.BlockHash
	})).Run(func(args mock.Arguments) {
		// Notify of event after we've downloaded the 1002/1003a
		err := bcm.Notify(&Notification{
			NotificationType: NewEventLog,
			Event:            eventToConfirm,
		})
		assert.NoError(t, err)
	}).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1003a.BlockNumber)),
			BlockHash:   block1003a.BlockHash,
			ParentHash:  block1003a.ParentHash,
		},
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
	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1003b.BlockHash
	})).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1003b.BlockNumber)),
			BlockHash:   block1003b.BlockHash,
			ParentHash:  block1003b.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1004.BlockHash
	})).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1004.BlockNumber)),
			BlockHash:   block1004.BlockHash,
			ParentHash:  block1004.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()

	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.BlockInfoByNumberResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1002.BlockNumber)),
			BlockHash:   block1002.BlockHash,
			ParentHash:  block1002.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil)
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		// Simulate 1003 disappearing from the chain
		return r.BlockNumber.Uint64() == 1003
	})).Run(func(args mock.Arguments) {
		// Then notify about a new 1003 which matches the event, and a 1004
		blockHashes <- &ffcapi.BlockHashEvent{
			BlockHashes: []string{
				block1003b.BlockHash,
				block1004.BlockHash,
			},
		}
	}).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found"))

	bcm.Start()

	dispatched := <-confirmed
	assert.Equal(t, []BlockInfo{
		*block1002,
		*block1003b,
		*block1004,
	}, dispatched)

	bcm.Stop()

	mca.AssertExpectations(t)

}

func TestBlockConfirmationManagerE2ETransactionMovedFork(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager(t, true)

	confirmed := make(chan []BlockInfo, 1)
	receiptReceived := make(chan *ffcapi.TransactionReceiptResponse, 1)
	txToConfirmForkA := &TransactionInfo{
		TransactionHash: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		Confirmed: func(ctx context.Context, confirmations []BlockInfo) {
			confirmed <- confirmations
		},
		Receipt: func(ctx context.Context, receipt *ffcapi.TransactionReceiptResponse) {
			receiptReceived <- receipt
		},
	}
	block1002a := &BlockInfo{
		BlockNumber: 1002,
		BlockHash:   "0x46210d224888265c269359529618bf2f6adb2697ff52c63c10f16a2391bdd295",
		ParentHash:  "0xea681fadcf56ee6254a0d30b255c56636ee9199c73c45f0dd5823759b2ad1ef8",
	}
	// We start with a notification for this one
	err := bcm.Notify(&Notification{
		NotificationType: NewTransaction,
		Transaction:      txToConfirmForkA,
	})
	assert.NoError(t, err)

	block1001b := &BlockInfo{
		BlockNumber:       1001,
		BlockHash:         "0x33eb56730878a08e126f2d52b19242d3b3127dc7611447255928be91b2dda455",
		ParentHash:        "0xe9afc4ff48efed19fc9256d2964c4194320d4d20dca25bb2ebcf7d047e1b83c6",
		TransactionHashes: []string{txToConfirmForkA.TransactionHash},
	}
	block1002b := &BlockInfo{
		BlockNumber: 1002,
		BlockHash:   "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
		ParentHash:  "0x33eb56730878a08e126f2d52b19242d3b3127dc7611447255928be91b2dda455",
	}

	// The next filter gives us 1002a, which will later be removed
	blockHashes := bcm.NewBlockHashes()

	// First check while walking the chain does not yield a block
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Once()

	// Transaction receipt is immediately available on fork A
	mca.On("TransactionReceipt", mock.Anything, mock.MatchedBy(func(r *ffcapi.TransactionReceiptRequest) bool {
		return r.TransactionHash == txToConfirmForkA.TransactionHash
	})).Run(func(args mock.Arguments) {
		// Notify of the first confirmation for the first receipt - 1002a
		blockHashes <- &ffcapi.BlockHashEvent{
			BlockHashes: []string{
				block1002a.BlockHash,
			},
		}
	}).Return(&ffcapi.TransactionReceiptResponse{
		BlockHash:        block1002a.ParentHash,
		BlockNumber:      fftypes.NewFFBigInt(1001),
		TransactionIndex: fftypes.NewFFBigInt(0),
		ProtocolID:       fmt.Sprintf("%.12d/%.6d", fftypes.NewFFBigInt(1001).Int64(), fftypes.NewFFBigInt(0).Int64()),
		Success:          true,
	}, ffcapi.ErrorReason(""), nil).Once()

	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1002a.BlockHash
	})).Run(func(args mock.Arguments) {
		// Next we notify of the new block 1001b
		blockHashes <- &ffcapi.BlockHashEvent{
			BlockHashes: []string{
				block1001b.BlockHash,
			},
		}
	}).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1002a.BlockNumber)),
			BlockHash:   block1002a.BlockHash,
			ParentHash:  block1002a.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()

	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1001b.BlockHash
	})).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber:       fftypes.NewFFBigInt(int64(block1001b.BlockNumber)),
			BlockHash:         block1001b.BlockHash,
			ParentHash:        block1001b.ParentHash,
			TransactionHashes: block1001b.TransactionHashes,
		},
	}, ffcapi.ErrorReason(""), nil).Once()

	// Transaction receipt is then found on fork B via new block header notification
	mca.On("TransactionReceipt", mock.Anything, mock.MatchedBy(func(r *ffcapi.TransactionReceiptRequest) bool {
		return r.TransactionHash == txToConfirmForkA.TransactionHash
	})).Return(&ffcapi.TransactionReceiptResponse{
		BlockHash:        block1001b.BlockHash,
		BlockNumber:      fftypes.NewFFBigInt(1001),
		TransactionIndex: fftypes.NewFFBigInt(0),
		ProtocolID:       fmt.Sprintf("%.12d/%.6d", fftypes.NewFFBigInt(1001).Int64(), fftypes.NewFFBigInt(0).Int64()),
		Success:          true,
	}, ffcapi.ErrorReason(""), nil).Once()

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

	// We will go and ask for block 1002 again, as the hash mismatches our updated notification
	// Give the right answer now
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Run(func(args mock.Arguments) {
		// Notify of the new block 1003/1004
		blockHashes <- &ffcapi.BlockHashEvent{
			BlockHashes: []string{
				block1003.BlockHash,
				block1004.BlockHash,
			},
		}
	}).Return(&ffcapi.BlockInfoByNumberResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1002b.BlockNumber)),
			BlockHash:   block1002b.BlockHash,
			ParentHash:  block1002b.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()

	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1003.BlockHash
	})).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1003.BlockNumber)),
			BlockHash:   block1003.BlockHash,
			ParentHash:  block1003.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == block1004.BlockHash
	})).Return(&ffcapi.BlockInfoByHashResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1004.BlockNumber)),
			BlockHash:   block1004.BlockHash,
			ParentHash:  block1004.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()

	bcm.Start()

	receipt := <-receiptReceived
	assert.True(t, receipt.Success)

	dispatched := <-confirmed
	assert.Equal(t, []BlockInfo{
		*block1002b,
		*block1003,
		*block1004,
	}, dispatched)

	bcm.Stop()

	mca.AssertExpectations(t)

}

func TestBlockConfirmationManagerE2EHistoricalEvent(t *testing.T) {
	bcm, mca := newTestBlockConfirmationManager(t, true)

	confirmed := make(chan []BlockInfo, 1)
	eventToConfirm := &EventInfo{
		ID: &ffcapi.EventID{
			ListenerID:       fftypes.NewUUID(),
			TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
			BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
			BlockNumber:      1001,
			TransactionIndex: 5,
			LogIndex:         10,
		},
		Confirmed: func(ctx context.Context, confirmations []BlockInfo) {
			confirmed <- confirmations
		},
	}

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
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.BlockInfoByNumberResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1002.BlockNumber)),
			BlockHash:   block1002.BlockHash,
			ParentHash:  block1002.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1003
	})).Return(&ffcapi.BlockInfoByNumberResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1003.BlockNumber)),
			BlockHash:   block1003.BlockHash,
			ParentHash:  block1003.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1004
	})).Return(&ffcapi.BlockInfoByNumberResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(int64(block1004.BlockNumber)),
			BlockHash:   block1004.BlockHash,
			ParentHash:  block1004.ParentHash,
		},
	}, ffcapi.ErrorReason(""), nil).Once()

	err := bcm.Notify(&Notification{
		NotificationType: NewEventLog,
		Event:            eventToConfirm,
	})
	assert.NoError(t, err)

	bcm.Start()

	dispatched := <-confirmed
	assert.Equal(t, []BlockInfo{
		*block1002,
		*block1003,
		*block1004,
	}, dispatched)

	bcm.Stop()

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

func TestConfirmationsListenerFailWalkingChain(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	err := bcm.Notify(&Notification{
		NotificationType: NewEventLog,
		Event: &EventInfo{
			ID: &ffcapi.EventID{
				ListenerID:      fftypes.NewUUID(),
				TransactionHash: "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
				BlockHash:       "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
				BlockNumber:     1001,
			},
		},
	})
	assert.NoError(t, err)

	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	}).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once()

	bcm.confirmationsListener()

	mca.AssertExpectations(t)
}

func TestConfirmationsListenerFailWalkingChainForNewEvent(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	confirmed := make(chan []BlockInfo, 1)
	eventToConfirm := &EventInfo{
		ID: &ffcapi.EventID{
			ListenerID:       fftypes.NewUUID(),
			TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
			BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
			BlockNumber:      1001,
			TransactionIndex: 5,
			LogIndex:         10,
		},
		Confirmed: func(ctx context.Context, confirmations []BlockInfo) {
			confirmed <- confirmations
		},
	}
	err := bcm.Notify(&Notification{
		NotificationType: NewEventLog,
		Event:            eventToConfirm,
	})
	assert.NoError(t, err)

	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once().Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})

	bcm.confirmationsListener()

	mca.AssertExpectations(t)
}

func TestConfirmationsListenerRemoved(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	lid := fftypes.NewUUID()
	n := &Notification{
		Event: &EventInfo{
			ID: &ffcapi.EventID{
				ListenerID:       lid,
				TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
				BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
				BlockNumber:      1001,
				TransactionIndex: 5,
				LogIndex:         10,
			},
		},
	}
	bcm.addOrReplaceItem(n.eventPendingItem())
	completed := make(chan struct{})
	err := bcm.Notify(&Notification{
		NotificationType: ListenerRemoved,
		RemovedListener: &RemovedListenerInfo{
			ListenerID: lid,
			Completed:  completed,
		},
	})
	assert.NoError(t, err)

	mca.On("BlockInfoByNumber", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Maybe()
	mca.On("GetBlockInfoByNumber", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Maybe()

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
		ID: &ffcapi.EventID{
			ListenerID:       fftypes.NewUUID(),
			TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
			BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
			BlockNumber:      1001,
			TransactionIndex: 5,
			LogIndex:         10,
		},
	}
	bcm.addOrReplaceItem((&Notification{
		Event: eventInfo,
	}).eventPendingItem())
	err := bcm.Notify(&Notification{
		NotificationType: RemovedEventLog,
		Event:            eventInfo,
	})
	assert.NoError(t, err)

	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})

	bcm.confirmationsListener()
	<-bcm.done

	assert.Empty(t, bcm.pending)
	assert.False(t, bcm.CheckInFlight(eventInfo.ID.ListenerID))
	mca.AssertExpectations(t)
}

func TestConfirmationsFailWalkChainAfterBlockGap(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	eventNotification := &Notification{
		NotificationType: NewEventLog,
		Event: &EventInfo{
			ID: &ffcapi.EventID{
				ListenerID:       fftypes.NewUUID(),
				TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
				BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
				BlockNumber:      1001,
				TransactionIndex: 5,
				LogIndex:         10,
			},
		},
	}
	err := bcm.Notify(eventNotification)
	assert.NoError(t, err)

	mca.On("BlockInfoByNumber", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Run(func(args mock.Arguments) {
		bcm.NewBlockHashes() <- &ffcapi.BlockHashEvent{
			GapPotential: true,
		}
	}).Once()

	mca.On("BlockInfoByNumber", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})

	bcm.confirmationsListener()
	<-bcm.done

	assert.Len(t, bcm.pending, 1)
	assert.True(t, bcm.CheckInFlight(eventNotification.Event.ID.ListenerID))
	assert.NotNil(t, eventNotification.eventPendingItem().getKey()) // should be the event in there, the TX should be removed
	mca.AssertExpectations(t)
}

func TestConfirmationsRemoveTransaction(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.done = make(chan struct{})

	txInfo := &TransactionInfo{
		TransactionHash: "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
	}
	eventNotification := &Notification{
		NotificationType: NewEventLog,
		Event: &EventInfo{
			ID: &ffcapi.EventID{
				ListenerID:       fftypes.NewUUID(),
				TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
				BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
				BlockNumber:      1001,
				TransactionIndex: 5,
				LogIndex:         10,
			},
		},
	}
	bcm.addOrReplaceItem((&Notification{
		Transaction: txInfo,
	}).transactionPendingItem())
	go func() {
		// The notification we want to test
		err := bcm.Notify(&Notification{
			NotificationType: RemovedTransaction,
			Transaction:      txInfo,
		})
		assert.NoError(t, err)
		// Another notification that causes BlockInfoByNumber, so we can break the loop
		err = bcm.Notify(eventNotification)
		assert.NoError(t, err)
	}()

	mca.On("BlockInfoByNumber", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Run(func(args mock.Arguments) {
		bcm.cancelFunc()
	})

	bcm.confirmationsListener()
	<-bcm.done

	assert.Len(t, bcm.pending, 1)
	assert.NotNil(t, eventNotification.eventPendingItem().getKey()) // should be the event in there, the TX should be removed
	mca.AssertExpectations(t)
}

func TestWalkChainForEventBlockNotInConfirmationChain(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	pending := (&Notification{
		Event: &EventInfo{
			ID: &ffcapi.EventID{
				ListenerID:       fftypes.NewUUID(),
				TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
				BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
				BlockNumber:      1001,
				TransactionIndex: 5,
				LogIndex:         10,
			},
		},
	}).eventPendingItem()

	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(&ffcapi.BlockInfoByNumberResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(1002),
			BlockHash:   "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8",
			ParentHash:  "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
		},
	}, ffcapi.ErrorReason(""), nil).Once()

	blocks := bcm.newBlockState()
	err := bcm.walkChainForItem(pending, blocks)
	assert.NoError(t, err)

	mca.AssertExpectations(t)
}

func TestWalkChainForEventBlockLookupFail(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	pending := (&Notification{
		Event: &EventInfo{
			ID: &ffcapi.EventID{
				ListenerID:       fftypes.NewUUID(),
				TransactionHash:  "0x531e219d98d81dc9f9a14811ac537479f5d77a74bdba47629bfbebe2d7663ce7",
				BlockHash:        "0x0e32d749a86cfaf551d528b5b121cea456f980a39e5b8136eb8e85dbc744a542",
				BlockNumber:      1001,
				TransactionIndex: 5,
				LogIndex:         10,
			},
		},
	}).eventPendingItem()

	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once()

	blocks := bcm.newBlockState()
	err := bcm.walkChainForItem(pending, blocks)
	assert.Regexp(t, "pop", err)

	mca.AssertExpectations(t)
}

func TestProcessBlockHashesLookupFail(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	blockHash := "0xed21f4f73d150f16f922ae82b7485cd936ae1eca4c027516311b928360a347e8"
	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
		return r.BlockHash == blockHash
	})).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once()

	bcm.processBlockHashes([]string{
		blockHash,
	})

	mca.AssertExpectations(t)
}

func TestProcessNotificationsSwallowsUnknownType(t *testing.T) {

	bcm, _ := newTestBlockConfirmationManager(t, false)
	blocks := bcm.newBlockState()
	bcm.processNotifications([]*Notification{
		{NotificationType: NotificationType(999)},
	}, blocks)
}

func TestGetBlockNotFound(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	mca.On("BlockInfoByHash", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByHashRequest) bool {
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
	assert.Regexp(t, "FF21016", err)

	err = bcm.Notify(&Notification{
		NotificationType: NewEventLog,
	})
	assert.Regexp(t, "FF21016", err)

	err = bcm.Notify(&Notification{
		NotificationType: ListenerRemoved,
	})
	assert.Regexp(t, "FF21016", err)

	bcm.cancelFunc()
	err = bcm.Notify(&Notification{
		NotificationType: NewTransaction,
		Transaction: &TransactionInfo{
			TransactionHash: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
			Confirmed:       func(ctx context.Context, confirmations []BlockInfo) {},
		},
	})
	assert.NoError(t, err)

}

func TestCheckReceiptNotFound(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	mca.On("TransactionReceipt", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found"))

	txHash := "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"
	pending := &pendingItem{
		pType:           pendingTypeTransaction,
		transactionHash: txHash,
	}
	bcm.pending[pending.getKey()] = pending
	bcm.staleReceipts[pendingKeyForTX(txHash)] = true
	blocks := bcm.newBlockState()
	bcm.checkReceipt(pending, blocks)

	assert.False(t, bcm.staleReceipts[pendingKeyForTX(txHash)])

}

func TestCheckReceiptImmediateConfirm(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)
	bcm.requiredConfirmations = 0

	mca.On("TransactionReceipt", mock.Anything, mock.Anything).Return(&ffcapi.TransactionReceiptResponse{
		BlockHash:        fftypes.NewRandB32().String(),
		BlockNumber:      fftypes.NewFFBigInt(1001),
		TransactionIndex: fftypes.NewFFBigInt(0),
		ProtocolID:       fmt.Sprintf("%.12d/%.6d", fftypes.NewFFBigInt(1001).Int64(), fftypes.NewFFBigInt(0).Int64()),
		Success:          true,
	}, ffcapi.ErrorReasonNotFound, nil)

	done := make(chan struct{})
	txHash := "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"
	pending := &pendingItem{
		pType:           pendingTypeTransaction,
		transactionHash: txHash,
		confirmedCallback: func(ctx context.Context, confirmations []BlockInfo) {
			close(done)
		},
	}
	bcm.pending[pending.getKey()] = pending
	blocks := bcm.newBlockState()
	go bcm.checkReceipt(pending, blocks)

	<-done
}

func TestCheckReceiptFail(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	mca.On("TransactionReceipt", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	txHash := "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"
	pending := &pendingItem{
		pType:           pendingTypeTransaction,
		transactionHash: txHash,
	}
	bcm.pending[pending.getKey()] = pending
	bcm.staleReceipts[pendingKeyForTX(txHash)] = true
	blocks := bcm.newBlockState()
	bcm.checkReceipt(pending, blocks)

	assert.True(t, bcm.staleReceipts[pendingKeyForTX(txHash)])

}

func TestCheckReceiptWalkFail(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	mca.On("TransactionReceipt", mock.Anything, mock.Anything).Return(&ffcapi.TransactionReceiptResponse{
		BlockNumber:      fftypes.NewFFBigInt(12345),
		BlockHash:        "0x64fd8179b80dd255d52ce60d7f265c0506be810e2f3df52463fadeb44bb4d2df",
		TransactionIndex: fftypes.NewFFBigInt(10),
		ProtocolID:       fmt.Sprintf("%.12d/%.6d", fftypes.NewFFBigInt(12345).Int64(), fftypes.NewFFBigInt(10).Int64()),
	}, ffcapi.ErrorReason(""), nil)
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 12346
	})).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	txHash := "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"
	pending := &pendingItem{
		pType:           pendingTypeTransaction,
		transactionHash: txHash,
	}
	bcm.pending[pending.getKey()] = pending
	bcm.staleReceipts[pendingKeyForTX(txHash)] = true
	blocks := bcm.newBlockState()
	bcm.checkReceipt(pending, blocks)

	assert.True(t, bcm.staleReceipts[pendingKeyForTX(txHash)])

}

func TestStaleReceiptCheck(t *testing.T) {

	bcm, _ := newTestBlockConfirmationManager(t, false)

	txHash := "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"
	pending := &pendingItem{
		pType:            pendingTypeTransaction,
		lastReceiptCheck: time.Now().Add(-1 * time.Hour),
		transactionHash:  txHash,
	}
	bcm.pending[pending.getKey()] = pending
	bcm.staleReceiptCheck()

	assert.True(t, bcm.staleReceipts[pendingKeyForTX(txHash)])

}

func TestBlockState(t *testing.T) {

	bcm, mca := newTestBlockConfirmationManager(t, false)

	block1002 := &ffcapi.BlockInfoByNumberResponse{
		BlockInfo: ffcapi.BlockInfo{
			BlockNumber: fftypes.NewFFBigInt(1002),
			BlockHash:   fftypes.NewRandB32().String(),
		},
	}
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1002
	})).Return(block1002, ffcapi.ErrorReason(""), nil).Once()
	mca.On("BlockInfoByNumber", mock.Anything, mock.MatchedBy(func(r *ffcapi.BlockInfoByNumberRequest) bool {
		return r.BlockNumber.Uint64() == 1003
	})).Return(nil, ffcapi.ErrorReasonNotFound, fmt.Errorf("not found")).Once()

	blocks := bcm.newBlockState()

	block, err := blocks.getByNumber(1002, "")
	assert.NoError(t, err)
	assert.Equal(t, block1002.BlockHash, block.BlockHash)

	block, err = blocks.getByNumber(1002, "")
	assert.NoError(t, err)
	assert.Equal(t, block1002.BlockHash, block.BlockHash) // cached

	block, err = blocks.getByNumber(1003, "")
	assert.NoError(t, err)
	assert.Nil(t, block)

	block, err = blocks.getByNumber(1004, "")
	assert.NoError(t, err)
	assert.Nil(t, block) // above high water mark

	mca.AssertExpectations(t)
}
