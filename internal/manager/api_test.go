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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/stretchr/testify/assert"
)

const sampleSendTX = `{
	"headers": {
		"id": "904F177C-C790-4B01-BDF4-F2B4E52E607E",
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

func testFFCAPIHandler(t *testing.T, fn func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var reqHeader ffcapi.RequestBase
		b, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		err = json.Unmarshal(b, &reqHeader)
		assert.NoError(t, err)

		assert.NotNil(t, reqHeader.FFCAPI.RequestID)
		assert.Equal(t, ffcapi.VersionCurrent, reqHeader.FFCAPI.Version)
		assert.Equal(t, ffcapi.Variant("evm"), reqHeader.FFCAPI.Variant)

		res, status := fn(reqHeader.FFCAPI.RequestType, b)

		b, err = json.Marshal(res)
		assert.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(b)

	}
}

func TestSendTransactionE2E(t *testing.T) {

	txSent := make(chan struct{})

	url, m, cancel := newTestManager(t,
		testFFCAPIHandler(t, func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int) {
			status = 200
			switch reqType {

			case ffcapi.RequestTypeGetNextNonce:
				var nonceReq ffcapi.GetNextNonceRequest
				err := json.Unmarshal(b, &nonceReq)
				assert.NoError(t, err)
				assert.Equal(t, "0xb480F96c0a3d6E9e9a263e4665a39bFa6c4d01E8", nonceReq.Signer)
				res = ffcapi.GetNextNonceResponse{
					Nonce: fftypes.NewFFBigInt(12345),
				}

			case ffcapi.RequestTypePrepareTransaction:
				var prepTX ffcapi.PrepareTransactionRequest
				err := json.Unmarshal(b, &prepTX)
				assert.NoError(t, err)
				assert.Equal(t, "0xe1a078b9e2b145d0a7387f09277c6ae1d9470771", prepTX.To)
				assert.Equal(t, uint64(1000000), prepTX.Gas.Uint64())
				assert.Equal(t, "set", prepTX.Method.JSONObject().GetString("name"))
				assert.Len(t, prepTX.Params, 1)
				assert.Equal(t, "4276993775", prepTX.Params[0].JSONObject().GetString("value"))
				res = ffcapi.PrepareTransactionResponse{
					TransactionHash: "0x106215b9c0c9372e3f541beff0cdc3cd061a26f69f3808e28fd139a1abc9d345",
					RawTransaction:  "RAW_UNSIGNED_BYTES",
					Gas:             fftypes.NewFFBigInt(2000000), // gas estimate simulation
				}

			case ffcapi.RequestTypeSendTransaction:
				var sendTX ffcapi.SendTransactionRequest
				err := json.Unmarshal(b, &sendTX)
				assert.NoError(t, err)
				assert.Equal(t, "0xb480F96c0a3d6E9e9a263e4665a39bFa6c4d01E8", sendTX.From)
				assert.Equal(t, `223344556677`, sendTX.GasPrice.String())
				assert.Equal(t, "RAW_UNSIGNED_BYTES", sendTX.RawTransaction)

				// We're at end of job for this test
				close(txSent)

			default:
				assert.Fail(t, fmt.Sprintf("Unexpected type: %s", reqType))
				status = 500
			}
			return res, status
		}),
		func(w http.ResponseWriter, r *http.Request) {

		},
	)
	defer cancel()

	m.Start()

	req := strings.NewReader(sampleSendTX)
	res, err := resty.New().R().
		SetBody(req).
		Post(url)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())

	<-txSent

}

func TestSendInvalidRequestNoHeaders(t *testing.T) {

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()
	m.Start()

	req := strings.NewReader(`{
		"noHeaders": true
	}`)
	var errRes fftypes.RESTError
	res, err := resty.New().R().
		SetBody(req).
		SetError(&errRes).
		Post(url)
	assert.NoError(t, err)
	assert.Equal(t, 400, res.StatusCode())
	assert.Regexp(t, "FF201022", errRes.Error)
}

func TestSendInvalidRequestWrongType(t *testing.T) {

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()
	m.Start()

	req := strings.NewReader(`{
		"headers": {
			"id": "` + fftypes.NewUUID().String() + `",
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
	assert.Regexp(t, "FF201023", errRes.Error)
}

func TestSendInvalidRequestFail(t *testing.T) {

	url, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {
			backendError := &fftypes.RESTError{Error: "pop"}
			b, err := json.Marshal(&backendError)
			assert.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			w.Write(b)
		},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cancel()
	m.Start()

	req := strings.NewReader(`{
		"headers": {
			"id": "` + fftypes.NewUUID().String() + `",
			"type": "SendTransaction"
		}
	}`)
	var errRes fftypes.RESTError
	res, err := resty.New().R().
		SetBody(req).
		SetError(&errRes).
		Post(url)
	assert.NoError(t, err)
	assert.Equal(t, 500, res.StatusCode())
	assert.Regexp(t, "FF201012", errRes.Error)
}

func TestSendTransactionPrepareFail(t *testing.T) {

	url, m, cancel := newTestManager(t,
		testFFCAPIHandler(t, func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int) {
			status = 200
			switch reqType {
			case ffcapi.RequestTypeGetNextNonce:
				res = ffcapi.GetNextNonceResponse{
					Nonce: fftypes.NewFFBigInt(12345),
				}

			case ffcapi.RequestTypePrepareTransaction:
				res = ffcapi.ErrorResponse{
					Error: "pop",
				}
				status = 500
			}
			return res, status
		}),
		func(w http.ResponseWriter, r *http.Request) {

		},
	)
	defer cancel()

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
		testFFCAPIHandler(t, func(reqType ffcapi.RequestType, b []byte) (res interface{}, status int) {
			status = 200
			switch reqType {
			case ffcapi.RequestTypeGetNextNonce:
				res = ffcapi.GetNextNonceResponse{
					Nonce: fftypes.NewFFBigInt(12345),
				}

			case ffcapi.RequestTypePrepareTransaction:
				res = ffcapi.PrepareTransactionResponse{}
				status = 200
			}
			return res, status
		}),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
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

	m.Start()

	req := strings.NewReader(sampleSendTX)
	res, err := resty.New().R().
		SetBody(req).
		Post(url)
	assert.NoError(t, err)
	assert.Equal(t, 500, res.StatusCode())

}
