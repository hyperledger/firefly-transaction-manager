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

package manager

import (
	"context"

	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/hyperledger/firefly/pkg/fftypes"
)

func (m *manager) sendManagedTransaction(ctx context.Context, request *fftm.TransactionRequest) (*fftm.ManagedTXOutput, error) {

	// First job is to assign the next nonce to this request.
	// We block any further sends on this nonce until we've got this one successfully into the node, or
	// fail deterministically in a way that allows us to return it.
	lockedNonce, err := m.assignAndLockNonce(ctx, request.Headers.ID, request.From)
	if err != nil {
		return nil, err
	}
	// We will call markSpent() once we reach the point the nonce has been used
	defer lockedNonce.complete(ctx)

	// Prepare the transaction, which will mean we have a transaction that should be submittable.
	// If we fail at this stage, we don't need to write any state as we are sure we haven't submitted
	// anything to the blockchain itself.
	prepared, _, err := m.connectorAPI.PrepareTransaction(ctx, &ffcapi.PrepareTransactionRequest{
		TransactionHeaders:       request.TransactionHeaders,
		TransactionPrepareInputs: request.TransactionPrepareInputs,
	})
	if err != nil {
		return nil, err
	}

	// Next we update FireFly core with the pre-submitted record pending record, with the allocated nonce.
	// From this point on, we will guide this transaction through to submission.
	// We return an "ack" at this point, and dispatch the work of getting the transaction submitted
	// to the background worker.
	mtx := &fftm.ManagedTXOutput{
		FFTMName:        m.name,
		ID:              request.Headers.ID, // on input the request ID must be the Operation ID
		Nonce:           fftypes.NewFFBigInt(int64(lockedNonce.nonce)),
		Gas:             prepared.Gas,
		TransactionHash: prepared.TransactionHash,
		TransactionData: prepared.TransactionData,
		Request:         request,
	}
	if err = m.writeManagedTX(ctx, &opUpdate{
		ID:     mtx.ID,
		Status: fftypes.OpStatusPending,
		Output: mtx,
	}); err != nil {
		return nil, err
	}

	// Ok - we've spent it. The rest of the processing will be triggered off of lockedNonce
	// completion adding this transaction to the pool (and/or the change event that comes in from
	// FireFly core from the update to the transaction)
	lockedNonce.spent = mtx
	return mtx, nil
}
