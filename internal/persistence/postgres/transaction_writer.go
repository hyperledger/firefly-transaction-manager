// Copyright © 2023 Kaleido, Inc.
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

package postgres

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/dbsql"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

type transactionOperation struct {
	txID string
	done chan error

	txInsert           *apitypes.ManagedTX
	txUpdate           *apitypes.TXUpdates
	txDelete           *string
	clearConfirmations bool
	confirmation       *apitypes.ConfirmationRecord
	receipt            *apitypes.ReceiptRecord
	historyRecord      *apitypes.TXHistoryRecord
}

type txCacheEntry struct {
	// lastCompacted time.Time
	// count         int
}

type transactionWriter struct {
	p            *sqlPersistence
	txMetaCache  *lru.Cache[string, *txCacheEntry]
	bgCtx        context.Context
	cancelCtx    context.CancelFunc
	batchTimeout time.Duration
	batchMaxSize int
	workerCount  uint32
	workQueues   []chan *transactionOperation
	workersDone  []chan struct{}
}

type transactionWriterBatch struct {
	id             string
	ops            []*transactionOperation
	timeoutContext context.Context
	timeoutCancel  func()

	txInserts           []*apitypes.ManagedTX
	txUpdates           []*transactionOperation
	txDeletes           []string
	receiptInserts      map[string]*apitypes.ReceiptRecord
	historyInserts      []*apitypes.TXHistoryRecord
	confirmationInserts []*apitypes.ConfirmationRecord
	confirmationResets  map[string]bool
}

func newTransactionWriter(bgCtx context.Context, p *sqlPersistence, conf config.Section) (*transactionWriter, error) {
	txMetaCache, err := lru.New[string, *txCacheEntry](
		conf.GetInt(ConfigTXWriterHistoryCacheSlots),
	)
	if err != nil {
		return nil, err
	}
	workerCount := conf.GetInt(ConfigTXWriterCount)
	batchMaxSize := conf.GetInt(ConfigTXWriterBatchSize)
	tw := &transactionWriter{
		p:            p,
		txMetaCache:  txMetaCache,
		workerCount:  uint32(workerCount),
		batchTimeout: conf.GetDuration(ConfigTXWriterBatchTimeout),
		batchMaxSize: batchMaxSize,
		workersDone:  make([]chan struct{}, workerCount),
		workQueues:   make([]chan *transactionOperation, workerCount),
	}
	tw.bgCtx, tw.cancelCtx = context.WithCancel(bgCtx)
	for i := 0; i < workerCount; i++ {
		tw.workersDone[i] = make(chan struct{})
		tw.workQueues[i] = make(chan *transactionOperation, batchMaxSize)
		go tw.worker(i)
	}
	return tw, nil
}

func newTransactionOperation(txID string) *transactionOperation {
	return &transactionOperation{
		txID: txID,
		done: make(chan error, 1), // 1 slot to ensure we don't block the writer
	}
}

func (op *transactionOperation) flush(ctx context.Context) error {
	select {
	case err := <-op.done:
		return err
	case <-ctx.Done():
		return i18n.NewError(ctx, i18n.MsgContextCanceled)
	}
}

func (tw *transactionWriter) queue(ctx context.Context, op *transactionOperation) {
	// All operations for a given transaction, deterministically go to the same worker.
	// This ensures that all sequenced items (like history records) are written in the right order.
	//
	// Note the insertion order of the transactions themselves does not matter, as the
	// caller waits for "done" on these inserts before returning to the caller (FireFly Core).
	// So if multiple transactions are queued for insert concurrently with different IDs,
	// then there is no deterministic ordering guarantee possible regardless.
	h := fnv.New32a() // simple non-cryptographic hash algo
	_, _ = h.Write([]byte(op.txID))
	routine := h.Sum32() % tw.workerCount
	select {
	case tw.workQueues[routine] <- op: // it's queued
	case <-ctx.Done(): // timeout of caller context
		// Just return, as they are giving up on the request so there's no need to queue it
		// If they flush they will get an error
	case <-tw.bgCtx.Done(): // shutdown
		// Push an error back to the operator before we return (note we allocate a slot to make this safe)
		op.done <- i18n.NewError(ctx, tmmsgs.MsgShuttingDown)
	}
}

func (tw *transactionWriter) worker(i int) {
	defer close(tw.workersDone[i])
	workerID := fmt.Sprintf("tx_writer_%.4d", i)
	ctx := log.WithLogField(tw.bgCtx, "job", workerID)
	var batch *transactionWriterBatch
	batchCount := 0
	workQueue := tw.workQueues[i]
	for {
		var timeoutContext context.Context
		var timedOut bool
		if batch != nil {
			timeoutContext = batch.timeoutContext
		} else {
			timeoutContext = ctx
		}
		select {
		case op := <-workQueue:
			if batch == nil {
				batch = &transactionWriterBatch{
					id: fmt.Sprintf("%.4d_%.9d", i, batchCount),
				}
				batch.timeoutContext, batch.timeoutCancel = context.WithTimeout(ctx, tw.batchTimeout)
				batchCount++
			}
			batch.ops = append(batch.ops, op)
		case <-timeoutContext.Done():
			timedOut = true
			select {
			case <-ctx.Done():
				log.L(ctx).Debugf("Transaction writer ending")
				return
			default:
			}
		}

		if batch != nil && (timedOut || (len(batch.ops) >= tw.batchMaxSize)) {
			batch.timeoutCancel()
			tw.runBatch(ctx, batch)
			batch = nil
		}
	}
}

func (tw *transactionWriter) runBatch(ctx context.Context, b *transactionWriterBatch) {
	err := tw.p.db.RunAsGroup(ctx, func(ctx context.Context) error {
		// Build all the batch insert operations
		b.confirmationResets = make(map[string]bool)
		b.receiptInserts = make(map[string]*apitypes.ReceiptRecord)
		for _, op := range b.ops {
			switch {
			case op.txInsert != nil:
				b.txInserts = append(b.txInserts, op.txInsert)
			case op.txUpdate != nil:
				b.txUpdates = append(b.txUpdates, op)
			case op.txDelete != nil:
				b.txDeletes = append(b.txDeletes, *op.txDelete)
			case op.receipt != nil:
				// Last one wins in the receipts (can't insert the same TXID twice in one InsertMany)
				b.receiptInserts[op.txID] = op.receipt
			case op.historyRecord != nil:
				b.historyInserts = append(b.historyInserts, op.historyRecord)
			case op.confirmation != nil:
				if op.clearConfirmations {
					// We need to purge any previous confirmation inserts for the same TX,
					// as we will only do one clear operation for this batch (before the insert-many).
					newConfirmationInserts := make([]*apitypes.ConfirmationRecord, 0, len(b.confirmationInserts))
					for _, c := range b.confirmationInserts {
						if c.TransactionID != op.confirmation.TransactionID {
							newConfirmationInserts = append(newConfirmationInserts, c)
						}
					}
					b.confirmationInserts = newConfirmationInserts
					// Add the reset
					b.confirmationResets[op.confirmation.TransactionID] = true
				}
				b.confirmationInserts = append(b.confirmationInserts, op.confirmation)
			}
		}
		if err := tw.executeBatchOps(ctx, b); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.L(ctx).Errorf("Transaction persistence batch failed: %s", err)
		// All ops in the batch get a single generic error
		err = i18n.NewError(ctx, tmmsgs.MsgTransactionPersistenceError)
	}
	for _, op := range b.ops {
		op.done <- err
	}

}

func (tw *transactionWriter) executeBatchOps(ctx context.Context, b *transactionWriterBatch) error {
	// Insert all the transactions
	if len(b.txInserts) > 0 {
		if err := tw.p.transactions.InsertMany(ctx, b.txInserts, false); err != nil {
			log.L(ctx).Errorf("InsertMany transactions (%d) failed: %s", len(b.historyInserts), err)
			return err
		}
	}
	// Do all the transaction updates
	for _, op := range b.txUpdates {
		if err := tw.p.updateTransaction(ctx, op.txID, op.txUpdate); err != nil {
			log.L(ctx).Errorf("Updata transaction %s failed: %s", op.txID, err)
			return err
		}
	}
	// Then the receipts - which need to be an upsert
	receipts := make([]*apitypes.ReceiptRecord, 0, len(b.receiptInserts))
	for _, r := range b.receiptInserts {
		receipts = append(receipts, r)
	}
	if len(receipts) > 0 {
		// Try optimized insert first, allowing partial success so we can fall back
		err := tw.p.receipts.InsertMany(ctx, receipts, true /* fallback */)
		if err != nil {
			log.L(ctx).Debugf("Batch receipt insert optimization failed: %s", err)
			for _, receipt := range b.receiptInserts {
				// FAll back to individual upserts
				if _, err := tw.p.receipts.Upsert(ctx, receipt, dbsql.UpsertOptimizationExisting); err != nil {
					log.L(ctx).Errorf("Upsert receipt %s failed: %s", receipt.TransactionID, err)
					return err
				}
			}
		}
	}
	// Then the history entries
	if len(b.historyInserts) > 0 {
		if err := tw.p.txHistory.InsertMany(ctx, b.historyInserts, false); err != nil {
			log.L(ctx).Errorf("InsertMany history records (%d) failed: %s", len(b.historyInserts), err)
			return err
		}
	}
	// Then do any confirmation clears
	for txID := range b.confirmationResets {
		if err := tw.p.confirmations.DeleteMany(ctx, persistence.ConfirmationFilters.NewFilter(ctx).Eq("transaction", txID)); err != nil {
			log.L(ctx).Errorf("DeleteMany confirmation records for transaction %s failed: %s", txID, err)
			return err
		}
	}
	// Then insert the new confirmation records
	if len(b.confirmationInserts) > 0 {
		if err := tw.p.confirmations.InsertMany(ctx, b.confirmationInserts, false); err != nil {
			log.L(ctx).Errorf("InsertMany confirmation records (%d) failed: %s", len(b.confirmationInserts), err)
			return err
		}
	}
	// Do all the transaction deletes
	for _, txID := range b.txDeletes {
		// Delete any receipt
		if err := tw.p.receipts.Delete(ctx, txID); err != nil && err != fftypes.DeleteRecordNotFound {
			log.L(ctx).Errorf("Delete receipt for transaction %s failed: %s", txID, err)
			return err
		}
		// Clear confirmations
		if err := tw.p.confirmations.DeleteMany(ctx, persistence.ConfirmationFilters.NewFilter(ctx).Eq("transaction", txID)); err != nil {
			log.L(ctx).Errorf("DeleteMany confirmation records for transaction %s failed: %s", txID, err)
			return err
		}
		// Clear history
		if err := tw.p.txHistory.DeleteMany(ctx, persistence.TXHistoryFilters.NewFilter(ctx).Eq("transaction", txID)); err != nil {
			log.L(ctx).Errorf("DeleteMany history records for transaction %s failed: %s", txID, err)
			return err
		}
		// Delete the transaction
		if err := tw.p.transactions.Delete(ctx, txID); err != nil {
			log.L(ctx).Errorf("Delete transaction %s failed: %s", txID, err)
			return err
		}
	}
	return nil
}

func (tw *transactionWriter) stop() {
	tw.cancelCtx()
	for _, workerDone := range tw.workersDone {
		<-workerDone
	}
}
