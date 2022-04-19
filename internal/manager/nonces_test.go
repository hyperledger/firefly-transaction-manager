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
	"net/http"
	"testing"
	"time"

	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/stretchr/testify/assert"
)

func TestNonceCached(t *testing.T) {

	_, m, cancel := newTestManager(t,
		testFFCAPIHandler(t, func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int) {
			return &ffcapi.GetNextNonceResponse{
				Nonce: fftypes.NewFFBigInt(1111),
			}, 200
		}),
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	locked1 := make(chan struct{})
	done1 := make(chan struct{})
	done2 := make(chan struct{})

	go func() {
		defer close(done1)

		ln, err := m.assignAndLockNonce(context.Background(), fftypes.NewUUID(), "0x12345")
		assert.NoError(t, err)
		assert.Equal(t, uint64(1111), ln.nonce)
		close(locked1)

		time.Sleep(1 * time.Millisecond)
		ln.spent = &fftm.ManagedTXOutput{
			ID:    fftypes.NewUUID(),
			Nonce: fftypes.NewFFBigInt(int64(ln.nonce)),
			Request: &fftm.TransactionRequest{
				TransactionInput: ffcapi.TransactionInput{
					TransactionHeaders: ffcapi.TransactionHeaders{
						From: "0x12345",
					},
				},
			},
		}
		ln.complete(context.Background())
	}()

	go func() {
		defer close(done2)

		<-locked1
		ln, err := m.assignAndLockNonce(context.Background(), fftypes.NewUUID(), "0x12345")
		assert.NoError(t, err)

		assert.Equal(t, uint64(1112), ln.nonce)

		ln.complete(context.Background())

	}()

	<-done1
	<-done2

}
