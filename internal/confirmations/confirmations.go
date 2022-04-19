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
package confirmations

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/i18n"
	"github.com/hyperledger/firefly/pkg/log"
)

// blockConfirmationManager listens to the blocks on the chain, and attributes confirmations to
// pending events. Once those events meet a threshold they are considered final and
// dispatched to the relevant listener.
type BlockConfirmationManager interface {
	Notify(n *Notification) error
	Start()
	Stop()
}

type NotificationType int

const (
	NewEventLog NotificationType = iota
	RemovedEventLog
	NewTransaction
	StopStream
)

type Notification struct {
	NotificationType NotificationType
	Event            *EventInfo
	Transaction      *TransactionInfo
	StoppedStream    *StoppedStreamInfo
}

type EventInfo struct {
	StreamID         string
	BlockHash        string
	BlockNumber      uint64
	TransactionHash  string
	TransactionIndex uint64
	LogIndex         uint64
	Confirmed        func(confirmations []BlockInfo)
}

type TransactionInfo struct {
	BlockHash       string
	BlockNumber     uint64
	TransactionHash string
	Confirmed       func(confirmations []BlockInfo)
}

type StoppedStreamInfo struct {
	StreamID  string
	Completed chan struct{}
}

type BlockInfo struct {
	BlockNumber uint64
	BlockHash   string
	ParentHash  string
}

type blockConfirmationManager struct {
	ctx                   context.Context
	cancelFunc            func()
	blockListenerID       string
	blockListenerStale    bool
	connectorAPI          ffcapi.API
	requiredConfirmations int
	pollingInterval       time.Duration
	blockCache            *lru.Cache
	bcmNotifications      chan *Notification
	highestBlockSeen      uint64
	pending               map[string]*pendingItem
	done                  chan struct{}
}

func NewBlockConfirmationManager(ctx context.Context, connectorAPI ffcapi.API) (BlockConfirmationManager, error) {
	var err error
	bcm := &blockConfirmationManager{
		connectorAPI:          connectorAPI,
		requiredConfirmations: config.GetInt(tmconfig.ConfirmationsRequired),
		pollingInterval:       config.GetDuration(tmconfig.ConfirmationsBlockPollingInterval),
		blockListenerStale:    true,
		bcmNotifications:      make(chan *Notification, config.GetInt(tmconfig.ConfirmationsNotificationQueueLength)),
		pending:               make(map[string]*pendingItem),
	}
	bcm.ctx, bcm.cancelFunc = context.WithCancel(ctx)
	bcm.blockCache, err = lru.New(config.GetInt(tmconfig.ConfirmationsBlockCacheSize))
	if err != nil {
		return nil, i18n.WrapError(bcm.ctx, err, tmmsgs.MsgCacheInitFail)
	}
	return bcm, nil
}

type pendingType int

const (
	pendingTypeEvent pendingType = iota
	pendingTypeTransaction
)

// pendingItem could be a specific event that has been detected, but not confirmed yet.
// Or it could be a transaction
type pendingItem struct {
	pType            pendingType
	added            time.Time
	confirmations    []*BlockInfo
	confirmed        func(confirmations []BlockInfo)
	transactionHash  string
	streamID         string // events only
	blockHash        string // can be notified of changes to this for receipts
	blockNumber      uint64 // known at creation time for event logs
	transactionIndex uint64 // known at creation time for event logs
	logIndex         uint64 // events only
}

func (pi *pendingItem) getKey() string {
	switch pi.pType {
	case pendingTypeEvent:
		// For events they are identified by their hash, blockNumber, transactionIndex and logIndex
		// If any of those change, it's a new new event - and as such we should get informed of it separately by the blockchain connector.
		return fmt.Sprintf("Event[%s]:th=%s,bh=%s,bn=%d,ti=%d,li=%d", pi.streamID, pi.transactionHash, pi.blockHash, pi.blockNumber, pi.transactionIndex, pi.logIndex)
	case pendingTypeTransaction:
		// For transactions, it's simply the transaction hash that identifies it. It can go into any block
		return fmt.Sprintf("TX:th=%s", pi.transactionHash)
	default:
		panic("invalid pending item type")
	}
}

func (pi *pendingItem) copyConfirmations() []BlockInfo {
	copy := make([]BlockInfo, len(pi.confirmations))
	for i, c := range pi.confirmations {
		copy[i] = *c
	}
	return copy
}

func (n *Notification) eventPendingItem() *pendingItem {
	return &pendingItem{
		pType:            pendingTypeEvent,
		blockNumber:      n.Event.BlockNumber,
		blockHash:        n.Event.BlockHash,
		streamID:         n.Event.StreamID,
		transactionHash:  n.Event.TransactionHash,
		transactionIndex: n.Event.TransactionIndex,
		logIndex:         n.Event.LogIndex,
		confirmed:        n.Event.Confirmed,
	}
}

func (n *Notification) transactionPendingItem() *pendingItem {
	return &pendingItem{
		pType:           pendingTypeTransaction,
		blockNumber:     n.Transaction.BlockNumber,
		blockHash:       n.Transaction.BlockHash,
		transactionHash: n.Transaction.TransactionHash,
		confirmed:       n.Transaction.Confirmed,
	}
}

type pendingItems []*pendingItem

func (pi pendingItems) Len() int      { return len(pi) }
func (pi pendingItems) Swap(i, j int) { pi[i], pi[j] = pi[j], pi[i] }
func (pi pendingItems) Less(i, j int) bool {
	// At the point we emit the confirmations, we ensure to sort them by:
	// - Block number
	// - Transaction index within the block
	// - Log index within the transaction (only for events)
	return pi[i].blockNumber < pi[j].blockNumber ||
		(pi[i].blockNumber == pi[j].blockNumber && (pi[i].transactionIndex < pi[j].transactionIndex ||
			(pi[i].transactionIndex == pi[j].transactionIndex && pi[i].logIndex < pi[j].logIndex)))
}

func (bcm *blockConfirmationManager) Start() {
	bcm.done = make(chan struct{})
	go bcm.confirmationsListener()
}

func (bcm *blockConfirmationManager) Stop() {
	bcm.cancelFunc()
	<-bcm.done
}

// Notify is used to notify the confirmation manager of detection of a new logEntry addition or removal
func (bcm *blockConfirmationManager) Notify(n *Notification) error {
	switch n.NotificationType {
	case NewEventLog, RemovedEventLog:
		if n.Event == nil || n.Event.StreamID == "" || n.Event.TransactionHash == "" || n.Event.BlockHash == "" {
			return i18n.NewError(bcm.ctx, tmmsgs.MsgInvalidConfirmationRequest, n)
		}
	case NewTransaction:
		if n.Transaction == nil || n.Transaction.TransactionHash == "" || n.Transaction.BlockHash == "" {
			return i18n.NewError(bcm.ctx, tmmsgs.MsgInvalidConfirmationRequest, n)
		}
	case StopStream:
		if n.StoppedStream == nil || n.StoppedStream.Completed == nil {
			return i18n.NewError(bcm.ctx, tmmsgs.MsgInvalidConfirmationRequest, n)
		}
	}
	select {
	case bcm.bcmNotifications <- n:
	case <-bcm.ctx.Done():
		log.L(bcm.ctx).Debugf("Shut down while queuing notification")
		return nil
	}
	return nil
}

func (bcm *blockConfirmationManager) createBlockListener() error {
	res, _, err := bcm.connectorAPI.CreateBlockListener(bcm.ctx, &ffcapi.CreateBlockListenerRequest{})
	if err != nil {
		return err
	}
	bcm.blockListenerStale = false
	bcm.blockListenerID = res.ListenerID
	log.L(bcm.ctx).Infof("Created blockListener: %s", bcm.blockListenerID)
	return err
}

func (bcm *blockConfirmationManager) pollBlockListener() ([]string, error) {
	ctx, cancel := context.WithTimeout(bcm.ctx, 30*time.Second)
	defer cancel()
	res, reason, err := bcm.connectorAPI.GetNewBlockHashes(ctx, &ffcapi.GetNewBlockHashesRequest{
		ListenerID: bcm.blockListenerID,
	})
	if err != nil {
		if reason == ffcapi.ErrorReasonNotFound {
			bcm.blockListenerStale = true
		}
		return nil, err
	}
	return res.BlockHashes, nil
}

func (bcm *blockConfirmationManager) addToCache(blockInfo *BlockInfo) {
	bcm.blockCache.Add(blockInfo.BlockHash, blockInfo)
	bcm.blockCache.Add(strconv.FormatUint(blockInfo.BlockNumber, 10), blockInfo)
}

func (bcm *blockConfirmationManager) getBlockByHash(blockHash string) (*BlockInfo, error) {
	cached, ok := bcm.blockCache.Get(blockHash)
	if ok {
		return cached.(*BlockInfo), nil
	}

	res, reason, err := bcm.connectorAPI.GetBlockInfoByHash(bcm.ctx, &ffcapi.GetBlockInfoByHashRequest{
		BlockHash: blockHash,
	})
	if err != nil {
		if reason == ffcapi.ErrorReasonNotFound {
			return nil, nil
		}
		return nil, err
	}
	blockInfo := &BlockInfo{
		BlockNumber: res.BlockNumber.Uint64(),
		BlockHash:   res.BlockHash,
		ParentHash:  res.ParentHash,
	}
	log.L(bcm.ctx).Debugf("Downloaded block header by hash: %d / %s parent=%s", blockInfo.BlockNumber, blockInfo.BlockHash, blockInfo.ParentHash)

	bcm.addToCache(blockInfo)
	return blockInfo, nil
}

func (bcm *blockConfirmationManager) getBlockByNumber(blockNumber uint64, expectedParentHash string) (*BlockInfo, error) {
	cached, ok := bcm.blockCache.Get(strconv.FormatUint(blockNumber, 10))
	if ok {
		blockInfo := cached.(*BlockInfo)
		if blockInfo.ParentHash != expectedParentHash {
			// Treat a missing block, or a mismatched block, both as a cache miss and query the node
			log.L(bcm.ctx).Debugf("Block cache miss due to parent hash mismatch: %d / %s parent=%s required=%s ", blockInfo.BlockNumber, blockInfo.BlockHash, blockInfo.ParentHash, expectedParentHash)
		} else {
			return blockInfo, nil
		}
	}
	res, reason, err := bcm.connectorAPI.GetBlockInfoByNumber(bcm.ctx, &ffcapi.GetBlockInfoByNumberRequest{
		BlockNumber: fftypes.NewFFBigInt(int64(blockNumber)),
	})
	if err != nil {
		if reason == ffcapi.ErrorReasonNotFound {
			return nil, nil
		}
		return nil, err
	}
	blockInfo := &BlockInfo{
		BlockNumber: res.BlockNumber.Uint64(),
		BlockHash:   res.BlockHash,
		ParentHash:  res.ParentHash,
	}
	log.L(bcm.ctx).Debugf("Downloaded block header by number: %d / %s parent=%s", blockInfo.BlockNumber, blockInfo.BlockHash, blockInfo.ParentHash)

	bcm.addToCache(blockInfo)
	return blockInfo, nil
}

func (bcm *blockConfirmationManager) confirmationsListener() {
	defer close(bcm.done)
	pollTimer := time.NewTimer(0)
	notifications := make([]*Notification, 0)
	for {
		popped := false
		for !popped {
			select {
			case <-pollTimer.C:
				popped = true
			case <-bcm.ctx.Done():
				log.L(bcm.ctx).Debugf("Block confirmation listener stopping")
				return
			case notification := <-bcm.bcmNotifications:
				if notification.NotificationType == StopStream {
					// Handle stream notifications immediately
					bcm.streamStopped(notification)
				} else {
					// Defer until after we've got new logs
					notifications = append(notifications, notification)
				}
			}
		}
		pollTimer = time.NewTimer(bcm.pollingInterval)

		// Setup a blockListener if we're missing one
		if bcm.blockListenerStale {
			if err := bcm.createBlockListener(); err != nil {
				log.L(bcm.ctx).Errorf("Failed to create blockListener: %s", err)
				continue
			}

			if err := bcm.walkChain(); err != nil {
				log.L(bcm.ctx).Errorf("Failed to create walk chain after restoring blockListener: %s", err)
				continue
			}
		}

		// Do the poll
		blockHashes, err := bcm.pollBlockListener()
		if err != nil {
			log.L(bcm.ctx).Errorf("Failed to retrieve blocks from blockListener: %s", err)
			continue
		}

		// Process each new block
		bcm.processBlockHashes(blockHashes)

		// Process any new notifications - we do this at the end, so it can benefit
		// from knowing the latest highestBlockSeen
		if err := bcm.processNotifications(notifications); err != nil {
			log.L(bcm.ctx).Errorf("Failed processing notifications: %s", err)
			continue
		}

		// Clear the notifications array now we've processed them (we keep the slice memory)
		notifications = notifications[:0]

	}

}

func (bcm *blockConfirmationManager) processNotifications(notifications []*Notification) error {

	var newItem *pendingItem
	for _, n := range notifications {
		switch n.NotificationType {
		case NewEventLog:
			newItem = n.eventPendingItem()
		case NewTransaction:
			newItem = n.transactionPendingItem()
		case RemovedEventLog:
			bcm.removeItem(n.eventPendingItem())
		default:
			// Note that streamStopped is handled in the polling loop directly
			log.L(bcm.ctx).Warnf("Unexpected notification type: %d", n.NotificationType)
		}
	}

	if newItem != nil {
		bcm.addOrReplaceItem(newItem)
		if err := bcm.walkChainForItem(newItem); err != nil {
			return err
		}

	}

	return nil
}

// streamStopped removes all pending work for a given stream, and notifies once done
func (bcm *blockConfirmationManager) streamStopped(notification *Notification) {
	for eventKey, pending := range bcm.pending {
		if pending.streamID == notification.StoppedStream.StreamID {
			delete(bcm.pending, eventKey)
		}
	}
	close(notification.StoppedStream.Completed)
}

// addEvent is called by the goroutine on receipt of a new event/transaction notification
func (bcm *blockConfirmationManager) addOrReplaceItem(pending *pendingItem) {
	pending.added = time.Now()
	pending.confirmations = make([]*BlockInfo, 0, bcm.requiredConfirmations)
	eventKey := pending.getKey()
	bcm.pending[eventKey] = pending
	log.L(bcm.ctx).Infof("Added pending item %s", eventKey)
}

// removeEvent is called by the goroutine on receipt of a remove event notification
func (bcm *blockConfirmationManager) removeItem(pending *pendingItem) {
	eventKey := pending.getKey()
	log.L(bcm.ctx).Infof("Removing stale item %s", eventKey)
	delete(bcm.pending, eventKey)
}

func (bcm *blockConfirmationManager) processBlockHashes(blockHashes []string) {
	if len(blockHashes) > 0 {
		log.L(bcm.ctx).Debugf("New block notifications %v", blockHashes)
	}

	for _, blockHash := range blockHashes {
		// Get the block header
		block, err := bcm.getBlockByHash(blockHash)
		if err != nil || block == nil {
			log.L(bcm.ctx).Errorf("Failed to retrieve block %s: %v", blockHash, err)
			continue
		}

		// Process the block for confirmations
		bcm.processBlock(block)

		// Update the highest block (used for efficiency in chain walks)
		if block.BlockNumber > bcm.highestBlockSeen {
			bcm.highestBlockSeen = block.BlockNumber
		}
	}
}

func (bcm *blockConfirmationManager) processBlock(block *BlockInfo) {

	// Go through all the events, adding in the confirmations, and popping any out
	// that have reached their threshold. Then drop the log before logging/processing them.
	blockNumber := block.BlockNumber
	var confirmed pendingItems
	for eventKey, pending := range bcm.pending {
		// The block might appear at any point in the confirmation list
		expectedParentHash := pending.blockHash
		expectedBlockNumber := pending.blockNumber + 1
		for i := 0; i < (len(pending.confirmations) + 1); i++ {
			if block.ParentHash == expectedParentHash && blockNumber == expectedBlockNumber {
				pending.confirmations = append(pending.confirmations[0:i], block)
				log.L(bcm.ctx).Infof("Confirmation %d at block %d / %s item=%s",
					len(pending.confirmations), block.BlockNumber, block.BlockHash, pending.getKey())
				break
			}
			if i < len(pending.confirmations) {
				expectedParentHash = pending.confirmations[i].BlockHash
			}
			expectedBlockNumber++
		}
		if len(pending.confirmations) >= bcm.requiredConfirmations {
			delete(bcm.pending, eventKey)
			confirmed = append(confirmed, pending)
		}
	}

	// Sort the events to dispatch them in the correct order
	sort.Sort(confirmed)
	for _, c := range confirmed {
		bcm.dispatchConfirmed(c)
	}

}

// dispatchConfirmed drive the event stream for any events that are confirmed, and prunes the state
func (bcm *blockConfirmationManager) dispatchConfirmed(item *pendingItem) {
	eventKey := item.getKey()
	log.L(bcm.ctx).Infof("Confirmed with %d confirmations event=%s", len(item.confirmations), eventKey)
	delete(bcm.pending, eventKey)
	item.confirmed(item.copyConfirmations() /* a safe copy outside of our cache */)
}

// walkChain goes through each event and sees whether it's valid,
// purging any stale confirmations - or whole events if the blockListener is invalid
// We do this each time our blockListener is invalidated
func (bcm *blockConfirmationManager) walkChain() error {

	// Grab a copy of all the pending in order
	pendingItems := make(pendingItems, 0, len(bcm.pending))
	for _, pending := range bcm.pending {
		pendingItems = append(pendingItems, pending)
	}
	sort.Sort(pendingItems)

	// Go through them in order - using the cache for efficiency
	for _, pending := range pendingItems {
		if err := bcm.walkChainForItem(pending); err != nil {
			return err
		}
	}

	return nil

}

func (bcm *blockConfirmationManager) walkChainForItem(pending *pendingItem) (err error) {

	eventKey := pending.getKey()

	blockNumber := pending.blockNumber + 1
	expectedParentHash := pending.blockHash
	pending.confirmations = pending.confirmations[:0]
	for {
		// No point in walking past the highest block we've seen via the notifier
		if bcm.highestBlockSeen > 0 && blockNumber > bcm.highestBlockSeen {
			log.L(bcm.ctx).Debugf("Waiting for confirmation after block %d event=%s", bcm.highestBlockSeen, eventKey)
			return nil
		}
		block, err := bcm.getBlockByNumber(blockNumber, expectedParentHash)
		if err != nil {
			return err
		}
		if block == nil {
			log.L(bcm.ctx).Infof("Block %d unavailable walking chain event=%s", blockNumber, eventKey)
			return nil
		}
		candidateParentHash := block.ParentHash
		if candidateParentHash != expectedParentHash {
			log.L(bcm.ctx).Infof("Block mismatch in confirmations: block=%d expected=%s actual=%s confirmations=%d event=%s", blockNumber, expectedParentHash, candidateParentHash, len(pending.confirmations), eventKey)
			return nil
		}
		pending.confirmations = append(pending.confirmations, block)
		if len(pending.confirmations) >= bcm.requiredConfirmations {
			// Ready for dispatch
			bcm.dispatchConfirmed(pending)
			return nil
		}
		blockNumber++
		expectedParentHash = block.BlockHash
	}

}
