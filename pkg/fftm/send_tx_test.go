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

package fftm

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/mocks/persistencemocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSendTXPersistFail(t *testing.T) {

	_, m, close := newTestManagerMockPersistence(t)
	defer close()

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("ListTransactionsByNonce", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*apitypes.ManagedTX{
			{ID: "id12345", Created: fftypes.Now(), Status: apitypes.TxStatusSucceeded, Nonce: fftypes.NewFFBigInt(1000)},
		}, nil)
	mp.On("WriteTransaction", m.ctx, mock.Anything, true).Return(fmt.Errorf("pop"))

	var txReq *ffcapi.TransactionSendRequest
	err := json.Unmarshal([]byte(sampleSendTX), &txReq)
	assert.NoError(t, err)

	_, err = m.submitPreparedTX(m.ctx, "id1", &txReq.TransactionHeaders, fftypes.NewFFBigInt(12345), "0x123456", false)
	assert.Regexp(t, "pop", err)

}
