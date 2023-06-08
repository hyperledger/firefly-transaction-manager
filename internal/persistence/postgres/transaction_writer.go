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

	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

type transactionOperation struct {
	txID string
	done chan error

	txInsert           *apitypes.ManagedTX
	txUpdate           *apitypes.TXUpdates
	clearConfirmations bool
	confirmation       *apitypes.ConfirmationRecord
	receipt            *ffcapi.TransactionReceiptResponse
	historyRecord      *apitypes.TXHistoryRecord
}

type transactionWriter struct {
	p            *sqlPersistence
	bgCtx        context.Context
	routines     uint32
	batchTimeout time.Duration
	batchMaxSize int
	workQueues   []chan *transactionOperation
}

type transactionWriterBatch struct {
	id             string
	ops            []*transactionOperation
	timeoutContext context.Context
	timeoutCancel  func()
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
	routine := h.Sum32() % tw.routines
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
					id: fmt.Sprintf("%s/%.9d", batchCount),
				}
				batch.timeoutContext, batch.timeoutCancel = context.WithTimeout(ctx, tw.batchTimeout)
				batchCount++
			}
			batch.ops = append(batch.ops, op)
		case <-timeoutContext.Done():
			timedOut = true
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
		var receiptInserts []*ffcapi.TransactionReceiptResponse
		var historyInserts []*apitypes.TXHistoryRecord
		var confirmationInserts []*apitypes.ConfirmationRecord
		var confirmationResets map[string]bool // note complexity below
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
		// Then the receipts
		if err := tw.p.receipts.InsertMany(ctx, receiptInserts, false); err != nil {
			return err
		}
		// Then the history entries
		if err := tw.p.txHistory.InsertMany(ctx, historyInserts, false); err != nil {
			return err
		}
		// Then do any confirmation clears
		for txID := range confirmationResets {
			tw.p.confirmations.Delete
		}
		if err := tw.p.txHistory.InsertMany(ctx, historyInserts, false); err != nil {
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
