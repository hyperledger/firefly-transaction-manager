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

package fftm

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-transaction-manager/mocks/txhandlermocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPostBatch(t *testing.T) {
	url, m, done := newTestManager(t)
	defer done()

	err := m.Start()
	assert.NoError(t, err)

	// Mock the transaction handler directly
	mth := txhandlermocks.TransactionHandler{}
	mtx1 := &apitypes.ManagedTX{
		ID: "tx1",
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x111111",
			To:   "0x222222",
		},
	}
	mtx2 := &apitypes.ManagedTX{
		ID: "tx2",
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x111111",
			To:   "0x222222",
		},
	}
	mth.On("HandleNewTransactions", mock.Anything, mock.MatchedBy(func(txReqs []*apitypes.TransactionRequest) bool {
		return len(txReqs) == 2 && txReqs[0].Headers.ID == "tx1" && txReqs[1].Headers.ID == "tx2"
	})).Return([]*apitypes.ManagedTX{mtx1, mtx2}, []bool{false, false}, []error{nil, nil})
	m.txHandler = &mth

	// Create a batch request with multiple SendTransaction requests as JSON
	batchReqJSON := `{
		"requests": [
			{
				"headers": {
					"id": "tx1",
					"type": "SendTransaction"
				},
				"from": "0x111111",
				"to": "0x222222",
				"method": {"type":"function","name":"test"},
				"params": ["value1"]
			},
			{
				"headers": {
					"id": "tx2",
					"type": "SendTransaction"
				},
				"from": "0x111111",
				"to": "0x222222",
				"method": {"type":"function","name":"test"},
				"params": ["value2"]
			}
		]
	}`

	var batchResp apitypes.BatchResponse
	res, err := resty.New().
		SetTimeout(10*time.Second).
		R().
		SetHeader("Content-Type", "application/json").
		SetBody(batchReqJSON).
		SetResult(&batchResp).
		Post(url + "/batch")
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Len(t, batchResp.Responses, 2)

	// Both should succeed
	for i, resp := range batchResp.Responses {
		assert.True(t, resp.Success, "Request %d should succeed", i)
		assert.Nil(t, resp.Error, "Request %d should have no error", i)
		assert.NotNil(t, resp.Output, "Request %d should have output", i)
		assert.Equal(t, fmt.Sprintf("tx%d", i+1), resp.ID)
	}
}

func TestPostBatchMixedTypes(t *testing.T) {
	url, m, done := newTestManager(t)
	defer done()

	err := m.Start()
	assert.NoError(t, err)

	// Mock the transaction handler directly
	mth := txhandlermocks.TransactionHandler{}
	mtx1 := &apitypes.ManagedTX{
		ID: "tx1",
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x111111",
			To:   "0x222222",
		},
	}
	mtx2 := &apitypes.ManagedTX{
		ID: "tx2",
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x111111",
			To:   "0x222222",
		},
	}
	deploy1 := &apitypes.ManagedTX{
		ID: "deploy1",
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x111111",
		},
	}
	mth.On("HandleNewTransactions", mock.Anything, mock.MatchedBy(func(txReqs []*apitypes.TransactionRequest) bool {
		return len(txReqs) == 2 && txReqs[0].Headers.ID == "tx1" && txReqs[1].Headers.ID == "tx2"
	})).Return([]*apitypes.ManagedTX{mtx1, mtx2}, []bool{false, false}, []error{nil, nil})
	mth.On("HandleNewContractDeployments", mock.Anything, mock.MatchedBy(func(txReqs []*apitypes.ContractDeployRequest) bool {
		return len(txReqs) == 1 && txReqs[0].Headers.ID == "deploy1"
	})).Return([]*apitypes.ManagedTX{deploy1}, []bool{false}, []error{nil})
	m.txHandler = &mth

	// Create a batch request with both SendTransaction and Deploy requests as JSON
	batchReqJSON := `{
		"requests": [
			{
				"headers": {
					"id": "tx1",
					"type": "SendTransaction"
				},
				"from": "0x111111",
				"to": "0x222222",
				"method": {"type":"function","name":"test"},
				"params": ["value1"]
			},
			{
				"headers": {
					"id": "deploy1",
					"type": "DeployContract"
				},
				"from": "0x111111",
				"definition": {"abi":[]},
				"contract": "0xbytecode"
			},
			{
				"headers": {
					"id": "tx2",
					"type": "SendTransaction"
				},
				"from": "0x111111",
				"to": "0x222222",
				"method": {"type":"function","name":"test"},
				"params": ["value2"]
			}
		]
	}`

	var batchResp apitypes.BatchResponse
	res, err := resty.New().
		SetTimeout(10*time.Second).
		R().
		SetHeader("Content-Type", "application/json").
		SetBody(batchReqJSON).
		SetResult(&batchResp).
		Post(url + "/batch")
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Len(t, batchResp.Responses, 3)

	// All should succeed
	for i, resp := range batchResp.Responses {
		assert.True(t, resp.Success, "Request %d should succeed", i)
		assert.Nil(t, resp.Error, "Request %d should have no error", i)
		assert.NotNil(t, resp.Output, "Request %d should have output", i)
	}
}

func TestPostBatchUnsupportedType(t *testing.T) {
	url, m, done := newTestManager(t)
	defer done()

	err := m.Start()
	assert.NoError(t, err)

	// Mock the transaction handler directly
	mth := txhandlermocks.TransactionHandler{}
	mtx1 := &apitypes.ManagedTX{
		ID: "tx1",
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x111111",
			To:   "0x222222",
		},
	}
	mth.On("HandleNewTransactions", mock.Anything, mock.MatchedBy(func(txReqs []*apitypes.TransactionRequest) bool {
		return len(txReqs) == 1 && txReqs[0].Headers.ID == "tx1"
	})).Return([]*apitypes.ManagedTX{mtx1}, []bool{false}, []error{nil})
	m.txHandler = &mth

	// Create a batch request with unsupported request type as JSON
	batchReqJSON := `{
		"requests": [
			{
				"headers": {
					"id": "tx1",
					"type": "SendTransaction"
				},
				"from": "0x111111",
				"to": "0x222222",
				"method": {"type":"function","name":"test"},
				"params": ["value1"]
			},
			{
				"headers": {
					"id": "unsupported1",
					"type": "UnsupportedType"
				}
			}
		]
	}`

	var batchResp apitypes.BatchResponse
	res, err := resty.New().
		SetTimeout(10*time.Second).
		R().
		SetHeader("Content-Type", "application/json").
		SetBody(batchReqJSON).
		SetResult(&batchResp).
		Post(url + "/batch")
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Len(t, batchResp.Responses, 2)

	// First should succeed, second should fail
	assert.True(t, batchResp.Responses[0].Success)
	assert.False(t, batchResp.Responses[1].Success)
	assert.NotNil(t, batchResp.Responses[1].Error)

	// Error should be a SubmissionError object
	var submissionErr ffcapi.SubmissionError
	errBytes, _ := json.Marshal(batchResp.Responses[1].Error)
	err = json.Unmarshal(errBytes, &submissionErr)
	assert.NoError(t, err)
	assert.Contains(t, submissionErr.Error, "UnsupportedType")
	assert.True(t, submissionErr.SubmissionRejected)
}

func TestPostBatchInvalidRequest(t *testing.T) {
	url, m, done := newTestManager(t)
	defer done()

	err := m.Start()
	assert.NoError(t, err)

	// Mock the transaction handler directly
	mth := txhandlermocks.TransactionHandler{}
	mtx1 := &apitypes.ManagedTX{
		ID: "tx1",
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x111111",
			To:   "0x222222",
		},
	}
	// First transaction succeeds, second fails (empty From address)
	mth.On("HandleNewTransactions", mock.Anything, mock.MatchedBy(func(txReqs []*apitypes.TransactionRequest) bool {
		return len(txReqs) == 2 && txReqs[0].Headers.ID == "tx1" && txReqs[1].Headers.ID == "tx2"
	})).Return([]*apitypes.ManagedTX{mtx1, nil}, []bool{false, false}, []error{nil, fmt.Errorf("transaction operation invalid")})
	m.txHandler = &mth

	// Create a batch request with missing required fields in one request
	batchReqJSON := `{
		"requests": [
			{
				"headers": {
					"id": "tx1",
					"type": "SendTransaction"
				},
				"from": "0x111111",
				"to": "0x222222",
				"method": {"type":"function","name":"test"},
				"params": ["value1"]
			},
			{
				"headers": {
					"id": "tx2",
					"type": "SendTransaction"
				},
				"from": ""
			}
		]
	}`

	var batchResp apitypes.BatchResponse
	res, err := resty.New().
		SetTimeout(10*time.Second).
		R().
		SetHeader("Content-Type", "application/json").
		SetBody(batchReqJSON).
		SetResult(&batchResp).
		Post(url + "/batch")
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Len(t, batchResp.Responses, 2)

	// First should succeed, second should fail
	assert.True(t, batchResp.Responses[0].Success)
	assert.False(t, batchResp.Responses[1].Success)
	assert.NotNil(t, batchResp.Responses[1].Error)

	// Error should be a SubmissionError object
	var submissionErr ffcapi.SubmissionError
	errBytes, _ := json.Marshal(batchResp.Responses[1].Error)
	err = json.Unmarshal(errBytes, &submissionErr)
	assert.NoError(t, err)
	assert.NotEmpty(t, submissionErr.Error)
}

func TestPostBatchEmpty(t *testing.T) {
	url, m, done := newTestManager(t)
	defer done()
	err := m.Start()
	assert.NoError(t, err)

	// Create an empty batch request as JSON
	batchReqJSON := `{
		"requests": []
	}`

	var batchResp apitypes.BatchResponse
	res, err := resty.New().
		SetTimeout(10*time.Second).
		R().
		SetHeader("Content-Type", "application/json").
		SetBody(batchReqJSON).
		SetResult(&batchResp).
		Post(url + "/batch")
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Len(t, batchResp.Responses, 0)
}
