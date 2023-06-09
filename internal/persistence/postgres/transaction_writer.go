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
					id: fmt.Sprintf("%s/%.9d", batch.id, batchCount),
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

func (tw *transactionWriter) runBatch(ctx context.Context, batch *transactionWriterBatch) {

	err := tw.p.db.RunAsGroup(ctx, func(ctx context.Context) error {
		// Build all the batch insert operations
		var txInserts []*apitypes.ManagedTX
		var receiptInserts []*apitypes.ReceiptRecord
		var historyInserts []*apitypes.TXHistoryRecord
		var confirmationInserts []*apitypes.ConfirmationRecord
		confirmationResets := make(map[string]bool)
		for _, op := range batch.ops {
			switch {
			case op.txInsert != nil:
				txInserts = append(txInserts, op.txInsert)
			case op.receipt != nil:
				receiptInserts = append(receiptInserts, op.receipt)
			case op.historyRecord != nil:
				historyInserts = append(historyInserts, op.historyRecord)
			case op.confirmation != nil:
				if op.clearConfirmations {
					// We need to purge any previous confirmation inserts for the same TX,
					// as we will only do one clear operation for this batch (before the insert-many).
					newConfirmationInserts := make([]*apitypes.ConfirmationRecord, 0, len(confirmationInserts))
					for _, c := range confirmationInserts {
						if c.TransactionID != op.confirmation.TransactionID {
							newConfirmationInserts = append(newConfirmationInserts, c)
						}
					}
					confirmationInserts = newConfirmationInserts
					// Add the reset
					confirmationResets[op.confirmation.TransactionID] = true
				}
				confirmationInserts = append(confirmationInserts, op.confirmation)
			}
		}
		// Insert all the transactions
		if err := tw.p.transactions.InsertMany(ctx, txInserts, false); err != nil {
			return err
		}
		// Then the receipts - which need to be an upsert
		for _, receipt := range receiptInserts {
			if _, err := tw.p.receipts.Upsert(ctx, receipt, dbsql.UpsertOptimizationNew); err != nil {
				return err
			}
		}
		// Then the history entries
		if err := tw.p.txHistory.InsertMany(ctx, historyInserts, false); err != nil {
			return err
		}
		// Then do any confirmation clears
		for txID := range confirmationResets {
			if err := tw.p.confirmations.DeleteMany(ctx, persistence.ConfirmationFilters.NewFilter(ctx).Eq("transaction", txID)); err != nil {
				return err
			}
		}
		// Then insert the new confirmation records
		if err := tw.p.confirmations.InsertMany(ctx, confirmationInserts, false); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.L(ctx).Errorf("Transaction persistence batch failed: %s", err)
		// All ops in the batch get a single generic error
		err = i18n.NewError(ctx, tmmsgs.MsgTransactionPersistenceError)
	}
	for _, op := range batch.ops {
		op.done <- err
	}

}

func (tw *transactionWriter) stop() {
	tw.cancelCtx()
	for _, workerDone := range tw.workersDone {
		<-workerDone
	}
}
