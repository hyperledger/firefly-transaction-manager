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
	"fmt"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-transaction-manager/mocks/policyenginemocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeleteTransaction(t *testing.T) {

	url, m, done := newTestManager(t)
	defer done()

	mpe := &policyenginemocks.PolicyEngine{}
	m.policyEngine = mpe
	mpe.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return(policyengine.UpdateDelete, ffcapi.ErrorReason(""), nil).Maybe()

	err := m.Start()
	assert.NoError(t, err)

	txIn := newTestTxn(t, m, "0xaaaaa", 10001, apitypes.TxStatusSucceeded)

	var txOut *apitypes.ManagedTX
	res, err := resty.New().R().
		SetResult(&txOut).
		Delete(fmt.Sprintf("%s/transactions/%s", url, txIn.ID))
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Equal(t, txIn.ID, txOut.ID)
	assert.NotNil(t, txOut.DeleteRequested)

}
