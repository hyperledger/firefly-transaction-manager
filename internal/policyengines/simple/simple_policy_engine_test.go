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

package simple

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/ffresty"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestPolicyEngineFactory(t *testing.T) (*PolicyEngineFactory, config.Prefix) {
	prefix := config.NewPluginConfig("unittest.simple")
	f := &PolicyEngineFactory{}
	f.InitPrefix(prefix)
	assert.Equal(t, "simple", f.Name())
	return f, prefix
}

func TestMissingGasConfig(t *testing.T) {
	f, prefix := newTestPolicyEngineFactory(t)
	_, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.Regexp(t, "FF201020", err)
}

func TestFixedGasOK(t *testing.T) {
	f, prefix := newTestPolicyEngineFactory(t)
	prefix.Set(FixedGas, `{
		"maxPriorityFee":32.146027800733336,
		"maxFee":32.14602781673334
	}`)
	p, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.NoError(t, err)

	mtx := &fftm.ManagedTXOutput{
		Request: &fftm.TransactionRequest{
			TransactionInput: ffcapi.TransactionInput{
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
				},
			},
		},
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI := &ffcapimocks.API{}
	mockFFCAPI.On("SendTransaction", mock.Anything, mock.MatchedBy(func(req *ffcapi.SendTransactionRequest) bool {
		return req.GasPrice.JSONObject().GetString("maxPriorityFee") == "32.146027800733336" &&
			req.GasPrice.JSONObject().GetString("maxFee") == "32.14602781673334" &&
			req.From == "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712" &&
			req.TransactionData == "SOME_RAW_TX_BYTES"
	})).Return(&ffcapi.SendTransactionResponse{}, ffcapi.ErrorReason(""), nil)

	ctx := context.Background()
	updated, err := p.Execute(ctx, mockFFCAPI, mtx)
	assert.NoError(t, err)
	assert.True(t, updated)
	assert.NotNil(t, mtx.FirstSubmit)
	assert.NotNil(t, mtx.LastSubmit)
	assert.Equal(t, `{
		"maxPriorityFee":32.146027800733336,
		"maxFee":32.14602781673334
	}`, mtx.GasPrice.String())

	mockFFCAPI.AssertExpectations(t)
}

func TestGasStationSendOK(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, http.MethodGet)
		w.Write([]byte(`{
			"safeLow": {
			  "maxPriorityFee":30.7611840636,
			  "maxFee":30.7611840796
			  },
			"standard": {
			  "maxPriorityFee":32.146027800733336,
			  "maxFee":32.14602781673334
			  },
			"fast": {
			  "maxPriorityFee":33.284344224133335,
			  "maxFee":33.284344240133336
			  },
			"estimatedBaseFee":1.6e-8,
			"blockTime":6,
			"blockNumber":24962816
		  }`))
	}))
	defer server.Close()

	f, prefix := newTestPolicyEngineFactory(t)
	prefix.SubPrefix(GasStationPrefix).Set(GasStationEnabled, true)
	prefix.SubPrefix(GasStationPrefix).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	prefix.SubPrefix(GasStationPrefix).Set(GasStationGJSON, `{"unit":!"gwei","value":standard.maxPriorityFee}`)
	p, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.NoError(t, err)

	mtx := &fftm.ManagedTXOutput{
		Request: &fftm.TransactionRequest{
			TransactionInput: ffcapi.TransactionInput{
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
				},
			},
		},
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI := &ffcapimocks.API{}
	mockFFCAPI.On("SendTransaction", mock.Anything, mock.MatchedBy(func(req *ffcapi.SendTransactionRequest) bool {
		return req.GasPrice.JSONObject().GetString("unit") == "gwei" &&
			req.GasPrice.JSONObject().GetString("value") == "32.146027800733336" &&
			req.From == "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712" &&
			req.TransactionData == "SOME_RAW_TX_BYTES"
	})).Return(&ffcapi.SendTransactionResponse{}, ffcapi.ErrorReason(""), nil)

	ctx := context.Background()
	updated, err := p.Execute(ctx, mockFFCAPI, mtx)
	assert.NoError(t, err)
	assert.True(t, updated)
	assert.NotNil(t, mtx.FirstSubmit)
	assert.NotNil(t, mtx.LastSubmit)
	assert.Equal(t, `{"unit":"gwei","value":32.146027800733336}`, mtx.GasPrice.String())

	mockFFCAPI.AssertExpectations(t)
}

func TestGasStationSendFail(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`Not the gas station you are looking for`))
	}))
	defer server.Close()

	f, prefix := newTestPolicyEngineFactory(t)
	prefix.SubPrefix(GasStationPrefix).Set(GasStationEnabled, true)
	prefix.SubPrefix(GasStationPrefix).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	p, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.NoError(t, err)

	mtx := &fftm.ManagedTXOutput{
		Request: &fftm.TransactionRequest{
			TransactionInput: ffcapi.TransactionInput{
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
				},
			},
		},
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI := &ffcapimocks.API{}
	ctx := context.Background()
	_, err = p.Execute(ctx, mockFFCAPI, mtx)
	assert.Regexp(t, "FF201021", err)

}

func TestGasStationNonJSON(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	f, prefix := newTestPolicyEngineFactory(t)
	prefix.SubPrefix(GasStationPrefix).Set(GasStationEnabled, true)
	prefix.SubPrefix(GasStationPrefix).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	p, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.NoError(t, err)

	mtx := &fftm.ManagedTXOutput{
		Request: &fftm.TransactionRequest{
			TransactionInput: ffcapi.TransactionInput{
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
				},
			},
		},
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI := &ffcapimocks.API{}
	ctx := context.Background()
	_, err = p.Execute(ctx, mockFFCAPI, mtx)
	assert.Regexp(t, "FF201021", err)

}

func TestTXSendFail(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	f, prefix := newTestPolicyEngineFactory(t)
	prefix.SubPrefix(GasStationPrefix).Set(GasStationEnabled, true)
	prefix.SubPrefix(GasStationPrefix).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	p, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.NoError(t, err)

	mtx := &fftm.ManagedTXOutput{
		Request: &fftm.TransactionRequest{
			TransactionInput: ffcapi.TransactionInput{
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
				},
			},
		},
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI := &ffcapimocks.API{}
	mockFFCAPI.On("SendTransaction", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReasonInvalidInputs, fmt.Errorf("pop"))
	ctx := context.Background()
	_, err = p.Execute(ctx, mockFFCAPI, mtx)
	assert.Regexp(t, "pop", err)

}

func TestWarnStaleWarningCannotParse(t *testing.T) {
	f, prefix := newTestPolicyEngineFactory(t)
	prefix.Set(FixedGas, `12345`)
	p, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.NoError(t, err)

	submitTime := fftypes.FFTime(time.Now().Add(-100 * time.Hour))
	mtx := &fftm.ManagedTXOutput{
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     &submitTime,
		PolicyInfo:      fftypes.JSONAnyPtr("!not json!"),
		Request: &fftm.TransactionRequest{
			TransactionInput: ffcapi.TransactionInput{
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
				},
			},
		},
	}

	mockFFCAPI := &ffcapimocks.API{}

	ctx := context.Background()
	updated, err := p.Execute(ctx, mockFFCAPI, mtx)
	assert.NoError(t, err)
	assert.True(t, updated)
	assert.NotEmpty(t, mtx.PolicyInfo.JSONObject().GetString("lastWarnTime"))

	mockFFCAPI.AssertExpectations(t)
}

func TestWarnStaleAdditionalWarning(t *testing.T) {
	f, prefix := newTestPolicyEngineFactory(t)
	prefix.Set(FixedGas, `12345`)
	p, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.NoError(t, err)

	submitTime := fftypes.FFTime(time.Now().Add(-100 * time.Hour))
	lastWarning := fftypes.FFTime(time.Now().Add(-50 * time.Hour))
	mtx := &fftm.ManagedTXOutput{
		Request: &fftm.TransactionRequest{
			TransactionInput: ffcapi.TransactionInput{
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
				},
			},
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     &submitTime,
		PolicyInfo:      fftypes.JSONAnyPtr(fmt.Sprintf(`{"lastWarnTime": "%s"}`, lastWarning.String())),
	}

	mockFFCAPI := &ffcapimocks.API{}

	ctx := context.Background()
	updated, err := p.Execute(ctx, mockFFCAPI, mtx)
	assert.NoError(t, err)
	assert.True(t, updated)
	assert.NotEmpty(t, mtx.PolicyInfo.JSONObject().GetString("lastWarnTime"))

	mockFFCAPI.AssertExpectations(t)
}

func TestWarnStaleNoWarning(t *testing.T) {
	f, prefix := newTestPolicyEngineFactory(t)
	prefix.Set(FixedGas, `12345`)
	prefix.Set(WarnInterval, "100s")
	p, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.NoError(t, err)

	submitTime := fftypes.FFTime(time.Now().Add(-100 * time.Hour))
	lastWarning := fftypes.Now()
	mtx := &fftm.ManagedTXOutput{
		Request: &fftm.TransactionRequest{
			TransactionInput: ffcapi.TransactionInput{
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
				},
			},
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     &submitTime,
		PolicyInfo:      fftypes.JSONAnyPtr(fmt.Sprintf(`{"lastWarnTime": "%s"}`, lastWarning.String())),
	}

	mockFFCAPI := &ffcapimocks.API{}

	ctx := context.Background()
	updated, err := p.Execute(ctx, mockFFCAPI, mtx)
	assert.NoError(t, err)
	assert.False(t, updated)

	mockFFCAPI.AssertExpectations(t)
}

func TestNoOpWithReceipt(t *testing.T) {
	f, prefix := newTestPolicyEngineFactory(t)
	prefix.Set(FixedGas, `12345`)
	prefix.Set(WarnInterval, "100s")
	p, err := f.NewPolicyEngine(context.Background(), prefix)
	assert.NoError(t, err)

	submitTime := fftypes.Now()
	mtx := &fftm.ManagedTXOutput{
		Request: &fftm.TransactionRequest{
			TransactionInput: ffcapi.TransactionInput{
				TransactionHeaders: ffcapi.TransactionHeaders{
					From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
				},
			},
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     submitTime,
		Receipt: &ffcapi.GetReceiptResponse{
			BlockHash: "0x39e2664effa5ad0651c35f1fe3b4c4b90492b1955fee731c2e9fb4d6518de114",
		},
	}

	mockFFCAPI := &ffcapimocks.API{}

	ctx := context.Background()
	updated, err := p.Execute(ctx, mockFFCAPI, mtx)
	assert.NoError(t, err)
	assert.False(t, updated)

	mockFFCAPI.AssertExpectations(t)
}
