// Copyright © 2025 Kaleido, Inc.
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

package simple

import (
	"context"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

// prepareFunc is a function type for preparing a transaction or deployment.
// It takes the context and index, and returns the prepared transaction, rejection status, and error.
type prepareFunc func(ctx context.Context, idx int) (*apitypes.ManagedTX, bool, error)

// getItemIDFunc is a function type for getting the ID of an item at a given index.
type getItemIDFunc func(idx int) string

// setItemIDFunc is a function type for setting the ID of an item at a given index.
type setItemIDFunc func(idx int, id string)

// HandleNewTransactions handles a batch of new transaction requests.
// Transactions are prepared first, then inserted in batch using InsertTransactionsWithNextNonce for efficient nonce allocation.
// Each transaction in the batch is processed independently, and results are returned in arrays matching the input order.
// If a transaction fails during preparation, it does not affect the processing of other transactions in the batch.
func (sth *simpleTransactionHandler) HandleNewTransactions(ctx context.Context, txReqs []*apitypes.TransactionRequest) (mtxs []*apitypes.ManagedTX, submissionRejected []bool, errs []error) {
	log.L(ctx).Debugf("HandleNewTransactions processing batch of %d transaction requests", len(txReqs))

	prepareFn := func(ctx context.Context, idx int) (*apitypes.ManagedTX, bool, error) {
		return sth.prepareTransaction(ctx, txReqs[idx])
	}

	getItemIDFn := func(idx int) string {
		if idx < len(txReqs) && txReqs[idx] != nil {
			return txReqs[idx].Headers.ID
		}
		return ""
	}

	setItemIDFn := func(idx int, id string) {
		if idx < len(txReqs) && txReqs[idx] != nil {
			txReqs[idx].Headers.ID = id
		}
	}

	return sth.handleBatch(ctx, len(txReqs), "HandleNewTransactions", "transaction request", "transactions", prepareFn, getItemIDFn, setItemIDFn)
}

// HandleNewContractDeployments handles a batch of new contract deployment requests.
// Deployments are prepared first, then inserted in batch using InsertTransactionsWithNextNonce for efficient nonce allocation.
// Each deployment in the batch is processed independently, and results are returned in arrays matching the input order.
// If a deployment fails during preparation, it does not affect the processing of other deployments in the batch.
func (sth *simpleTransactionHandler) HandleNewContractDeployments(ctx context.Context, txReqs []*apitypes.ContractDeployRequest) (mtxs []*apitypes.ManagedTX, submissionRejected []bool, errs []error) {
	log.L(ctx).Debugf("HandleNewContractDeployments processing batch of %d contract deployment requests", len(txReqs))

	prepareFn := func(ctx context.Context, idx int) (*apitypes.ManagedTX, bool, error) {
		return sth.prepareContractDeployment(ctx, txReqs[idx])
	}

	getItemIDFn := func(idx int) string {
		if idx < len(txReqs) && txReqs[idx] != nil {
			return txReqs[idx].Headers.ID
		}
		return ""
	}

	setItemIDFn := func(idx int, id string) {
		if idx < len(txReqs) && txReqs[idx] != nil {
			txReqs[idx].Headers.ID = id
		}
	}

	return sth.handleBatch(ctx, len(txReqs), "HandleNewContractDeployments", "contract deployment request", "contract deployments", prepareFn, getItemIDFn, setItemIDFn)
}

// handleBatch is a generic batch processing function that handles the common logic for both transactions and contract deployments.
func (sth *simpleTransactionHandler) handleBatch(
	ctx context.Context,
	totalCount int,
	operationName string,
	itemNameSingular string,
	itemNamePluralShort string,
	prepareFn prepareFunc,
	getItemIDFn getItemIDFunc,
	setItemIDFn setItemIDFunc,
) (mtxs []*apitypes.ManagedTX, submissionRejected []bool, errs []error) {
	// Initialize result arrays with the same length as input
	mtxs = make([]*apitypes.ManagedTX, totalCount)
	submissionRejected = make([]bool, totalCount)
	errs = make([]error, totalCount)

	// Prepare all items first
	preparedTxs := make([]*apitypes.ManagedTX, 0, totalCount)
	preparedIndices := make([]int, 0, totalCount)

	for i := 0; i < totalCount; i++ {
		itemID := getItemIDFn(i)

		if itemID == "" {
			itemID = fftypes.NewUUID().String()
			setItemIDFn(i, itemID)
		}

		log.L(ctx).Tracef("%s preparing %s %d/%d with ID %s", operationName, itemNameSingular, i+1, totalCount, itemID)

		// Prepare the item
		preparedMtx, rejected, err := prepareFn(ctx, i)
		if rejected || err != nil {
			log.L(ctx).Errorf("%s failed to prepare %s %d/%d with ID %s: %+v", operationName, itemNameSingular, i+1, totalCount, itemID, err)
			errs[i] = err
			submissionRejected[i] = rejected
			continue
		}

		// Store prepared transaction for batch insertion
		preparedTxs = append(preparedTxs, preparedMtx)
		preparedIndices = append(preparedIndices, i)
	}

	// Batch insert all prepared transactions using InsertTransactionsWithNextNonce
	// This optimizes nonce allocation for transactions from the same signer and batches database operations
	if len(preparedTxs) > 0 {
		log.L(ctx).Debugf("%s batch inserting %d prepared %s", operationName, len(preparedTxs), itemNamePluralShort)
		insertErrs := sth.toolkit.TXPersistence.InsertTransactionsWithNextNonce(ctx, preparedTxs, func(ctx context.Context, signer string) (uint64, error) {
			nextNonceRes, _, err := sth.toolkit.Connector.NextNonceForSigner(ctx, &ffcapi.NextNonceForSignerRequest{
				Signer: signer,
			})
			if err != nil {
				return 0, err
			}
			return nextNonceRes.Nonce.Uint64(), nil
		})

		// Process insertion results and add history entries for successful transactions
		sth.processBatchInsertionResults(ctx, preparedTxs, preparedIndices, insertErrs, totalCount, operationName, mtxs, errs)

		// Mark inflight stale once after batch insertion
		sth.markInflightStale()
	}

	// Log summary of batch processing results
	sth.logBatchProcessingSummary(ctx, totalCount, errs, submissionRejected, operationName)

	return mtxs, submissionRejected, errs
}

// processBatchInsertionResults processes the results of a batch insertion operation.
// It adds history entries for successful transactions and updates the result arrays.
func (sth *simpleTransactionHandler) processBatchInsertionResults(ctx context.Context, preparedTxs []*apitypes.ManagedTX, preparedIndices []int, insertErrs []error, totalCount int, operationName string, mtxs []*apitypes.ManagedTX, errs []error) {
	for j, preparedIdx := range preparedIndices {
		if insertErrs[j] != nil {
			log.L(ctx).Errorf("%s failed to insert transaction %d/%d with ID %s: %+v", operationName, preparedIdx+1, totalCount, preparedTxs[j].ID, insertErrs[j])
			errs[preparedIdx] = insertErrs[j]
			continue
		}

		// Transaction successfully inserted, add history entry
		log.L(ctx).Tracef("%s persisted transaction with ID: %s, using nonce %s", operationName, preparedTxs[j].ID, preparedTxs[j].Nonce.String())
		err := sth.toolkit.TXHistory.AddSubStatusAction(ctx, preparedTxs[j].ID, apitypes.TxSubStatusReceived, apitypes.TxActionAssignNonce, fftypes.JSONAnyPtr(`{"nonce":"`+preparedTxs[j].Nonce.String()+`"}`), nil, fftypes.Now())
		if err != nil {
			log.L(ctx).Errorf("%s failed to add history entry for transaction %s: %+v", operationName, preparedTxs[j].ID, err)
			errs[preparedIdx] = err
			continue
		}

		mtxs[preparedIdx] = preparedTxs[j]
		log.L(ctx).Tracef("%s successfully processed transaction request %d/%d with ID %s", operationName, preparedIdx+1, totalCount, preparedTxs[j].ID)
	}
}

// logBatchProcessingSummary logs a summary of batch processing results.
func (sth *simpleTransactionHandler) logBatchProcessingSummary(ctx context.Context, totalCount int, errs []error, submissionRejected []bool, operationName string) {
	successCount := 0
	rejectedCount := 0
	errorCount := 0
	for i := range errs {
		switch {
		case errs[i] != nil:
			errorCount++
		case submissionRejected[i]:
			rejectedCount++
		default:
			successCount++
		}
	}
	log.L(ctx).Debugf("%s batch processing complete: %d succeeded, %d rejected, %d errors out of %d total requests", operationName, successCount, rejectedCount, errorCount, totalCount)
}
