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
	"net/http"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const sampleSendTX = `{
	"headers": {
		"id": "ns1:904F177C-C790-4B01-BDF4-F2B4E52E607E",
		"type": "SendTransaction"
	},
	"from": "0xb480F96c0a3d6E9e9a263e4665a39bFa6c4d01E8",
	"to": "0xe1a078b9e2b145d0a7387f09277c6ae1d9470771",
	"gas": 1000000,
	"method": {
		"inputs": [
			{
				"internalType":" uint256",
				"name": "x",
				"type": "uint256"
			}
		],
		"name":"set",
		"outputs":[],
		"stateMutability":"nonpayable",
		"type":"function"
	},
	"params": [
		{
			"value": 4276993775,
			"type": "uint256"
		}
	]
}`

func TestSendTransactionE2E(t *testing.T) {

	txSent := make(chan struct{})

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	mFFC := m.connector.(*ffcapimocks.API)

	mFFC.On("NextNonceForSigner", mock.Anything, mock.MatchedBy(func(nonceReq *ffcapi.NextNonceForSignerRequest) bool {
		return "0xb480F96c0a3d6E9e9a263e4665a39bFa6c4d01E8" == nonceReq.Signer
	})).Return(&ffcapi.NextNonceForSignerResponse{
		Nonce: fftypes.NewFFBigInt(12345),
	}, ffcapi.ErrorReason(""), nil)

	mFFC.On("TransactionPrepare", mock.Anything, mock.MatchedBy(func(prepTX *ffcapi.TransactionPrepareRequest) bool {
		return "0xb480F96c0a3d6E9e9a263e4665a39bFa6c4d01E8" == prepTX.From &&
			"0xe1a078b9e2b145d0a7387f09277c6ae1d9470771" == prepTX.To &&
			uint64(1000000) == prepTX.Gas.Uint64() &&
			"set" == prepTX.Method.JSONObject().GetString("name") &&
			1 == len(prepTX.Params) &&
			"4276993775" == prepTX.Params[0].JSONObject().GetString("value") &&
			"4276993775" == prepTX.Params[0].JSONObject().GetString("value")
	})).Return(&ffcapi.TransactionPrepareResponse{
		TransactionData: "RAW_UNSIGNED_BYTES",
		Gas:             fftypes.NewFFBigInt(2000000), // gas estimate simulation
	}, ffcapi.ErrorReason(""), nil)

	mFFC.On("TransactionSend", mock.Anything, mock.MatchedBy(func(sendTX *ffcapi.TransactionSendRequest) bool {
		matches := "0xb480F96c0a3d6E9e9a263e4665a39bFa6c4d01E8" == sendTX.From &&
			"0xe1a078b9e2b145d0a7387f09277c6ae1d9470771" == sendTX.To &&
			uint64(2000000) == sendTX.Gas.Uint64() &&
			`223344556677` == sendTX.GasPrice.String() &&
			"RAW_UNSIGNED_BYTES" == sendTX.TransactionData
		if matches {
			// We're at end of job for this test
			close(txSent)
		}
		return matches
	})).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: "0x106215b9c0c9372e3f541beff0cdc3cd061a26f69f3808e28fd139a1abc9d345",
	}, ffcapi.ErrorReason(""), nil)

	mc := m.confirmations.(*confirmationsmocks.Manager)
	mc.On("Notify", mock.MatchedBy(func(n *confirmations.Notification) bool {
		return n.NotificationType == confirmations.NewTransaction
	})).Return(nil)

	m.Start()

	req := strings.NewReader(sampleSendTX)
	res, err := resty.New().R().
		SetBody(req).
		Post(url)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())

	<-txSent

}

func TestSendInvalidRequestBadTXType(t *testing.T) {

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()
	m.Start()

	req := strings.NewReader(`{
		"headers": {
			"type": "SendTransaction"
		},
		"from": {
			"Not": "a string"
		}
	}`)
	var errRes fftypes.RESTError
	res, err := resty.New().R().
		SetBody(req).
		SetError(&errRes).
		Post(url)
	assert.NoError(t, err)
	assert.Equal(t, 400, res.StatusCode())
	assert.Regexp(t, "FF21022", errRes.Error)
}

func TestSwaggerEndpoints(t *testing.T) {

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()
	m.Start()

	res, err := resty.New().R().SetDoNotParseResponse(true).Get(url + "/api/spec.json")
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())

	res, err = resty.New().R().SetDoNotParseResponse(true).Get(url + "/api/spec.yaml")
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())

	res, err = resty.New().R().SetDoNotParseResponse(true).Get(url + "/api")
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
}

func TestSendInvalidRequestWrongType(t *testing.T) {

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()
	m.Start()

	req := strings.NewReader(`{
		"headers": {
			"id": "ns1:` + fftypes.NewUUID().String() + `",
			"type": "wrong"
		}
	}`)
	var errRes fftypes.RESTError
	res, err := resty.New().R().
		SetBody(req).
		SetError(&errRes).
		Post(url)
	assert.NoError(t, err)
	assert.Equal(t, 400, res.StatusCode())
	assert.Regexp(t, "FF21023", errRes.Error)
}

func TestSendTransactionPrepareFail(t *testing.T) {

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()

	mFFC := m.connector.(*ffcapimocks.API)

	mFFC.On("NextNonceForSigner", mock.Anything, mock.MatchedBy(func(nonceReq *ffcapi.NextNonceForSignerRequest) bool {
		return "0xb480F96c0a3d6E9e9a263e4665a39bFa6c4d01E8" == nonceReq.Signer
	})).Return(&ffcapi.NextNonceForSignerResponse{
		Nonce: fftypes.NewFFBigInt(12345),
	}, ffcapi.ErrorReason(""), nil)

	mFFC.On("TransactionPrepare", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	m.Start()

	req := strings.NewReader(sampleSendTX)
	res, err := resty.New().R().
		SetBody(req).
		Post(url)
	assert.NoError(t, err)
	assert.Equal(t, 500, res.StatusCode())

}

func TestSendTransactionUpdateFireFlyFail(t *testing.T) {

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				errRes := fftypes.RESTError{Error: "pop"}
				b, err := json.Marshal(&errRes)
				assert.NoError(t, err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(500)
				w.Write(b)
			} else {
				w.WriteHeader(200)
			}
		},
	)
	defer cancel()

	mFFC := m.connector.(*ffcapimocks.API)

	mFFC.On("NextNonceForSigner", mock.Anything, mock.MatchedBy(func(nonceReq *ffcapi.NextNonceForSignerRequest) bool {
		return "0xb480F96c0a3d6E9e9a263e4665a39bFa6c4d01E8" == nonceReq.Signer
	})).Return(&ffcapi.NextNonceForSignerResponse{
		Nonce: fftypes.NewFFBigInt(12345),
	}, ffcapi.ErrorReason(""), nil)

	mFFC.On("TransactionPrepare", mock.Anything, mock.Anything).Return(&ffcapi.TransactionPrepareResponse{}, ffcapi.ErrorReason(""), nil)

	m.Start()

	req := strings.NewReader(sampleSendTX)
	res, err := resty.New().R().
		SetBody(req).
		Post(url)
	assert.NoError(t, err)
	assert.Equal(t, 500, res.StatusCode())

}

func TestNotFound(t *testing.T) {

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()
	m.Start()

	var errRes fftypes.RESTError
	res, err := resty.New().R().
		SetError(&errRes).
		Post(url + "/not found")
	assert.NoError(t, err)
	assert.Equal(t, 404, res.StatusCode())
	assert.Regexp(t, "FF00167", errRes.Error)
}
