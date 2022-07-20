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
	"sync"
	"time"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

// Manager listens to the blocks on the chain, and attributes confirmations to
// pending events. Once those events meet a threshold they are considered final and
// dispatched to the relevant listener.
type Manager interface {
	Notify(n *Notification) error
	Start()
	Stop()
	NewBlockHashes() chan<- *ffcapi.BlockHashEvent
	CheckInFlight(listenerID *fftypes.UUID) bool
}

type NotificationType int

const (
	NewEventLog NotificationType = iota
	RemovedEventLog
	NewTransaction
	RemovedTransaction
	ListenerRemoved
)

type Notification struct {
	NotificationType NotificationType
	Event            *EventInfo
	Transaction      *TransactionInfo
	RemovedListener  *RemovedListenerInfo
}

type EventInfo struct {
	ffcapi.EventID
	Confirmed func(confirmations []BlockInfo)
}

type TransactionInfo struct {
	TransactionHash string
	Receipt         func(receipt *ffcapi.TransactionReceiptResponse)
	Confirmed       func(confirmations []BlockInfo)
}

type RemovedListenerInfo struct {
	ListenerID *fftypes.UUID
	Completed  chan struct{}
}

type BlockInfo struct {
	BlockNumber       uint64   `json:"blockNumber"`
	BlockHash         string   `json:"blockHash"`
	ParentHash        string   `json:"parentHash"`
	TransactionHashes []string `json:"transactionHashes,omitempty"`
}

type blockConfirmationManager struct {
	baseContext           context.Context
	ctx                   context.Context
	cancelFunc            func()
	newBlockHashes        chan *ffcapi.BlockHashEvent
	connector             ffcapi.API
	blockListenerStale    bool
	requiredConfirmations int
	staleReceiptTimeout   time.Duration
	bcmNotifications      chan *Notification
	highestBlockSeen      uint64
	pending               map[string]*pendingItem
	pendingMux            sync.Mutex
	staleReceipts         map[string]bool
	done                  chan struct{}
}

func NewBlockConfirmationManager(baseContext context.Context, connector ffcapi.API) Manager {
	bcm := &blockConfirmationManager{
		baseContext:           baseContext,
		connector:             connector,
		blockListenerStale:    true,
		requiredConfirmations: config.GetInt(tmconfig.ConfirmationsRequired),
		staleReceiptTimeout:   config.GetDuration(tmconfig.ConfirmationsStaleReceiptTimeout),
		bcmNotifications:      make(chan *Notification, config.GetInt(tmconfig.ConfirmationsNotificationQueueLength)),
		pending:               make(map[string]*pendingItem),
		staleReceipts:         make(map[string]bool),
		newBlockHashes:        make(chan *ffcapi.BlockHashEvent, config.GetInt(tmconfig.ConfirmationsBlockQueueLength)),
	}
	bcm.ctx, bcm.cancelFunc = context.WithCancel(baseContext)
	return bcm
}

type pendingType int

const (
	pendingTypeEvent pendingType = iota
	pendingTypeTransaction
)

// pendingItem could be a specific event that has been detected, but not confirmed yet.
// Or it could be a transaction
type pendingItem struct {
	pType             pendingType
	added             time.Time
	confirmations     []*BlockInfo
	lastReceiptCheck  time.Time
	receiptCallback   func(receipt *ffcapi.TransactionReceiptResponse)
	confirmedCallback func(confirmations []BlockInfo)
	transactionHash   string
	blockHash         string        // can be notified of changes to this for receipts
	blockNumber       uint64        // known at creation time for event logs
	transactionIndex  uint64        // known at creation time for event logs
	logIndex          uint64        // events only
	listenerID        *fftypes.UUID // events only
}

func pendingKeyForTX(txHash string) string {
	return fmt.Sprintf("TX:th=%s", txHash)
}

func (pi *pendingItem) getKey() string {
	switch pi.pType {
	case pendingTypeEvent:
		// For events they are identified by their hash, blockNumber, transactionIndex and logIndex
		// If any of those change, it's a new new event - and as such we should get informed of it separately by the blockchain connector.
		return fmt.Sprintf("Event:l=%s,th=%s,bh=%s,bn=%d,ti=%d,li=%d", pi.listenerID, pi.transactionHash, pi.blockHash, pi.blockNumber, pi.transactionIndex, pi.logIndex)
	case pendingTypeTransaction:
		// For transactions, it's simply the transaction hash that identifies it. It can go into any block
		return pendingKeyForTX(pi.transactionHash)
	default:
		panic("invalid pending item type")
	}
}

func (pi *pendingItem) copyConfirmations() []BlockInfo {
	copy := make([]BlockInfo, len(pi.confirmations))
	for i, c := range pi.confirmations {
		copy[i] = BlockInfo{
			BlockNumber: c.BlockNumber,
			BlockHash:   c.BlockHash,
			ParentHash:  c.ParentHash,
			// Don't include transaction hash array
		}
	}
	return copy
}

func (n *Notification) eventPendingItem() *pendingItem {
	return &pendingItem{
		pType:             pendingTypeEvent,
		listenerID:        n.Event.ListenerID,
		blockNumber:       n.Event.BlockNumber,
		blockHash:         n.Event.BlockHash,
		transactionHash:   n.Event.TransactionHash,
		transactionIndex:  n.Event.TransactionIndex,
		logIndex:          n.Event.LogIndex,
		confirmedCallback: n.Event.Confirmed,
	}
}

func (n *Notification) transactionPendingItem() *pendingItem {
	return &pendingItem{
		pType:             pendingTypeTransaction,
		lastReceiptCheck:  time.Now(),
		transactionHash:   n.Transaction.TransactionHash,
		receiptCallback:   n.Transaction.Receipt,
		confirmedCallback: n.Transaction.Confirmed,
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
	if bcm.done != nil {
		bcm.cancelFunc()
		<-bcm.done
		bcm.done = nil
		// Reset context ready for restart
		bcm.ctx, bcm.cancelFunc = context.WithCancel(bcm.baseContext)
	}
}

func (bcm *blockConfirmationManager) NewBlockHashes() chan<- *ffcapi.BlockHashEvent {
	return bcm.newBlockHashes
}

// Notify is used to notify the confirmation manager of detection of a new logEntry addition or removal
func (bcm *blockConfirmationManager) Notify(n *Notification) error {
	switch n.NotificationType {
	case NewEventLog, RemovedEventLog:
		if n.Event == nil || n.Event.ListenerID == nil || n.Event.TransactionHash == "" || n.Event.BlockHash == "" {
			return i18n.NewError(bcm.ctx, tmmsgs.MsgInvalidConfirmationRequest, n)
		}
	case NewTransaction, RemovedTransaction:
		if n.Transaction == nil || n.Transaction.TransactionHash == "" {
			return i18n.NewError(bcm.ctx, tmmsgs.MsgInvalidConfirmationRequest, n)
		}
	case ListenerRemoved:
		if n.RemovedListener == nil || n.RemovedListener.Completed == nil {
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

func (bcm *blockConfirmationManager) CheckInFlight(listenerID *fftypes.UUID) bool {
	bcm.pendingMux.Lock()
	defer bcm.pendingMux.Unlock()
	for _, p := range bcm.pending {
		if listenerID.Equals(p.listenerID) {
			return true
		}
	}
	return false
}

func (bcm *blockConfirmationManager) getBlockByHash(blockHash string) (*BlockInfo, error) {
	res, reason, err := bcm.connector.BlockInfoByHash(bcm.ctx, &ffcapi.BlockInfoByHashRequest{
		BlockHash: blockHash,
	})
	if err != nil {
		if reason == ffcapi.ErrorReasonNotFound {
			return nil, nil
		}
		return nil, err
	}
	blockInfo := transformBlockInfo(&res.BlockInfo)
	log.L(bcm.ctx).Debugf("Downloaded block header by hash: %d / %s parent=%s", blockInfo.BlockNumber, blockInfo.BlockHash, blockInfo.ParentHash)

	return blockInfo, nil
}

func (bcm *blockConfirmationManager) getBlockByNumber(blockNumber uint64, expectedParentHash string) (*BlockInfo, error) {
	res, reason, err := bcm.connector.BlockInfoByNumber(bcm.ctx, &ffcapi.BlockInfoByNumberRequest{
		BlockNumber:        fftypes.NewFFBigInt(int64(blockNumber)),
		ExpectedParentHash: expectedParentHash,
	})
	if err != nil {
		if reason == ffcapi.ErrorReasonNotFound {
			return nil, nil
		}
		return nil, err
	}
	blockInfo := transformBlockInfo(&res.BlockInfo)
	log.L(bcm.ctx).Debugf("Downloaded block header by number: %d / %s parent=%s", blockInfo.BlockNumber, blockInfo.BlockHash, blockInfo.ParentHash)
	return blockInfo, nil
}

func transformBlockInfo(res *ffcapi.BlockInfo) *BlockInfo {
	return &BlockInfo{
		BlockNumber:       res.BlockNumber.Uint64(),
		BlockHash:         res.BlockHash,
		ParentHash:        res.ParentHash,
		TransactionHashes: res.TransactionHashes,
	}
}

func (bcm *blockConfirmationManager) confirmationsListener() {
	defer close(bcm.done)
	notifications := make([]*Notification, 0)
	blockHashes := make([]string, 0)
	for {
		select {
		case bhe := <-bcm.newBlockHashes:
			if bhe.GapPotential {
				bcm.blockListenerStale = true
			}
			blockHashes = append(blockHashes, bhe.BlockHashes...)
		case <-bcm.ctx.Done():
			log.L(bcm.ctx).Debugf("Block confirmation listener stopping")
			return
		case notification := <-bcm.bcmNotifications:
			if notification.NotificationType == ListenerRemoved {
				// Handle listener notifications immediately
				bcm.listenerRemoved(notification)
			} else {
				// Defer until after we've got new logs
				notifications = append(notifications, notification)
			}
		}

		if bcm.blockListenerStale {
			if err := bcm.walkChain(); err != nil {
				log.L(bcm.ctx).Errorf("Failed to create walk chain after restoring blockListener: %s", err)
				continue
			}
			bcm.blockListenerStale = false
		}

		// Process each new block
		bcm.processBlockHashes(blockHashes)
		// Truncate the block hashes now we've processed them
		blockHashes = blockHashes[:0]

		// Process any new notifications - we do this at the end, so it can benefit
		// from knowing the latest highestBlockSeen
		if err := bcm.processNotifications(notifications); err != nil {
			log.L(bcm.ctx).Errorf("Failed processing notifications: %s", err)
			continue
		}
		// Clear the notifications array now we've processed them (we keep the slice memory)
		notifications = notifications[:0]

		// Mark receipts stale after duration
		bcm.staleReceiptCheck()

		// Perform any receipt checks required, due to new notifications, previously failed
		// receipt checks, or processing block headers
		for pendingKey := range bcm.staleReceipts {
			if pending, ok := bcm.pending[pendingKey]; ok {
				bcm.checkReceipt(pending)
			}
		}

	}

}

func (bcm *blockConfirmationManager) staleReceiptCheck() {
	now := time.Now()
	for _, pending := range bcm.pending {
		if pending.pType == pendingTypeTransaction && now.Sub(pending.lastReceiptCheck) > bcm.staleReceiptTimeout {
			pendingKey := pending.getKey()
			log.L(bcm.ctx).Infof("Marking receipt check stale for %s", pendingKey)
			bcm.staleReceipts[pendingKey] = true
		}
	}
}

func (bcm *blockConfirmationManager) processNotifications(notifications []*Notification) error {

	for _, n := range notifications {
		switch n.NotificationType {
		case NewEventLog:
			newItem := n.eventPendingItem()
			bcm.addOrReplaceItem(newItem)
			if err := bcm.walkChainForItem(newItem); err != nil {
				return err
			}
		case NewTransaction:
			newItem := n.transactionPendingItem()
			bcm.addOrReplaceItem(newItem)
			bcm.staleReceipts[newItem.getKey()] = true
		case RemovedEventLog:
			bcm.removeItem(n.eventPendingItem())
		case RemovedTransaction:
			bcm.removeItem(n.transactionPendingItem())
		default:
			// Note that streamStopped is handled in the polling loop directly
			log.L(bcm.ctx).Warnf("Unexpected notification type: %d", n.NotificationType)
		}
	}

	return nil
}

func (bcm *blockConfirmationManager) checkReceipt(pending *pendingItem) {
	res, reason, err := bcm.connector.TransactionReceipt(bcm.ctx, &ffcapi.TransactionReceiptRequest{
		TransactionHash: pending.transactionHash,
	})

	if err != nil {
		if reason == ffcapi.ErrorReasonNotFound {
			log.L(bcm.ctx).Debugf("Receipt for transaction %s not yet available", pending.transactionHash)
		} else {
			// We need to keep checking this receipt until we've got a good return code
			log.L(bcm.ctx).Debugf("Failed to query receipt for transaction %s: %s", pending.transactionHash, err)
			return
		}
	} else {
		pending.blockNumber = res.BlockNumber.Uint64()
		pending.blockHash = res.BlockHash
		log.L(bcm.ctx).Infof("Receipt for transaction %s downloaded. BlockNumber=%d BlockHash=%s", pending.transactionHash, pending.blockNumber, pending.blockHash)
		// Notify of the receipt
		if pending.receiptCallback != nil {
			pending.receiptCallback(res)
		}
		// Need to walk the chain for this new receipt
		if err = bcm.walkChainForItem(pending); err != nil {
			log.L(bcm.ctx).Debugf("Failed to walk chain for transaction %s: %s", pending.transactionHash, err)
			return
		}
	}
	// No need to keep polling - either we now have a receipt, or normal block header monitoring will pick this one up
	delete(bcm.staleReceipts, pending.getKey())
}

// listenerRemoved removes all pending work for a given listener, and notifies once done
func (bcm *blockConfirmationManager) listenerRemoved(notification *Notification) {
	bcm.pendingMux.Lock()
	defer bcm.pendingMux.Unlock()
	for pendingKey, pending := range bcm.pending {
		if notification.RemovedListener.ListenerID.Equals(pending.listenerID) {
			delete(bcm.pending, pendingKey)
		}
	}
	close(notification.RemovedListener.Completed)
}

// addEvent is called by the goroutine on receipt of a new event/transaction notification
func (bcm *blockConfirmationManager) addOrReplaceItem(pending *pendingItem) {
	bcm.pendingMux.Lock()
	defer bcm.pendingMux.Unlock()
	pending.added = time.Now()
	pending.confirmations = make([]*BlockInfo, 0, bcm.requiredConfirmations)
	pendingKey := pending.getKey()
	bcm.pending[pendingKey] = pending
	log.L(bcm.ctx).Infof("Added pending item %s", pendingKey)
}

// removeEvent is called by the goroutine on receipt of a remove event notification
func (bcm *blockConfirmationManager) removeItem(pending *pendingItem) {
	bcm.pendingMux.Lock()
	defer bcm.pendingMux.Unlock()
	pendingKey := pending.getKey()
	log.L(bcm.ctx).Infof("Removing stale item %s", pendingKey)
	delete(bcm.pending, pendingKey)
	delete(bcm.staleReceipts, pendingKey)
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

	// For any transactions in the block that are known to us, we need to mark them
	// stale to go query the receipt
	bcm.pendingMux.Lock()
	for _, txHash := range block.TransactionHashes {
		txKey := pendingKeyForTX(txHash)
		if pending, ok := bcm.pending[txKey]; ok {
			if pending.blockHash != block.BlockHash {
				log.L(bcm.ctx).Infof("Detected transaction %s added to block %d / %s - receipt check scheduled", txHash, block.BlockNumber, block.BlockHash)
				bcm.staleReceipts[txKey] = true
			}
		}
	}
	bcm.pendingMux.Unlock()

	// Go through all the events, adding in the confirmations, and popping any out
	// that have reached their threshold. Then drop the log before logging/processing them.
	blockNumber := block.BlockNumber
	var confirmed pendingItems
	for pendingKey, pending := range bcm.pending {
		if pending.blockHash != "" {

			// The block might appear at any point in the confirmation list
			expectedParentHash := pending.blockHash
			expectedBlockNumber := pending.blockNumber
			for i := 0; i < (len(pending.confirmations) + 1); i++ {
				log.L(bcm.ctx).Tracef("Comparing block number=%d parent=%s to %d / %s for %s", blockNumber, block.ParentHash, expectedBlockNumber, expectedParentHash, pendingKey)
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
				bcm.pendingMux.Lock()
				delete(bcm.pending, pendingKey)
				bcm.pendingMux.Unlock()
				confirmed = append(confirmed, pending)
			}

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
	pendingKey := item.getKey()
	log.L(bcm.ctx).Infof("Confirmed with %d confirmations event=%s", len(item.confirmations), pendingKey)
	item.confirmedCallback(item.copyConfirmations() /* a safe copy outside of our cache */)
}

// walkChain goes through each event and sees whether it's valid,
// purging any stale confirmations - or whole events if the blockListener is invalid
// We do this each time our blockListener is invalidated
func (bcm *blockConfirmationManager) walkChain() error {

	// Grab a copy of all the pending in order
	bcm.pendingMux.Lock()
	pendingItems := make(pendingItems, 0, len(bcm.pending))
	for _, pending := range bcm.pending {
		pendingItems = append(pendingItems, pending)
	}
	bcm.pendingMux.Unlock()
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

	if pending.blockHash == "" {
		// This is a transaction that we don't yet have the receipt for
		log.L(bcm.ctx).Debugf("Transaction %s still awaiting receipt", pending.transactionHash)
		return nil
	}

	pendingKey := pending.getKey()

	blockNumber := pending.blockNumber + 1
	expectedParentHash := pending.blockHash
	pending.confirmations = pending.confirmations[:0]
	for {
		// No point in walking past the highest block we've seen via the notifier
		if bcm.highestBlockSeen > 0 && blockNumber > bcm.highestBlockSeen {
			log.L(bcm.ctx).Debugf("Waiting for confirmation after block %d event=%s", bcm.highestBlockSeen, pendingKey)
			return nil
		}
		block, err := bcm.getBlockByNumber(blockNumber, expectedParentHash)
		if err != nil {
			return err
		}
		if block == nil {
			log.L(bcm.ctx).Infof("Block %d unavailable walking chain event=%s", blockNumber, pendingKey)
			return nil
		}
		candidateParentHash := block.ParentHash
		if candidateParentHash != expectedParentHash {
			log.L(bcm.ctx).Infof("Block mismatch in confirmations: block=%d expected=%s actual=%s confirmations=%d event=%s", blockNumber, expectedParentHash, candidateParentHash, len(pending.confirmations), pendingKey)
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
