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
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/ffresty"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/txhistorymocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestPolicyEngineFactory(t *testing.T) (*PolicyEngineFactory, *policyengine.ToolkitAPI, *ffcapimocks.API, *txhistorymocks.Manager, config.Section) {
	tmconfig.Reset()
	conf := config.RootSection("unittest.simple")
	f := &PolicyEngineFactory{}
	f.InitConfig(conf)
	assert.Equal(t, "simple", f.Name())

	mockHistory := &txhistorymocks.Manager{}
	mockHistory.On("SetSubStatus", mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockHistory.On("AddSubStatusAction", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

	mockFFCAPI := &ffcapimocks.API{}

	return f, &policyengine.ToolkitAPI{
		Connector: mockFFCAPI,
		TXHistory: mockHistory,
	}, mockFFCAPI, mockHistory, conf
}

func TestMissingGasConfig(t *testing.T) {
	f, _, _, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeDisabled)
	_, err := f.NewPolicyEngine(context.Background(), conf)
	assert.Regexp(t, "FF21020", err)
}

func TestFixedGasPriceOK(t *testing.T) {
	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeDisabled)
	conf.Set(FixedGasPrice, `{
		"maxPriorityFee":32.146027800733336,
		"maxFee":32.14602781673334
	}`)
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x12345",
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI.On("TransactionSend", mock.Anything, mock.MatchedBy(func(req *ffcapi.TransactionSendRequest) bool {
		return req.GasPrice.JSONObject().GetString("maxPriorityFee") == "32.146027800733336" &&
			req.GasPrice.JSONObject().GetString("maxFee") == "32.14602781673334" &&
			req.From == "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712" &&
			req.TransactionData == "SOME_RAW_TX_BYTES"
	})).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: "0x12345",
	}, ffcapi.ErrorReason(""), nil)

	ctx := context.Background()
	updated, reason, err := p.Execute(ctx, tk, mtx)
	assert.NoError(t, err)
	assert.Equal(t, policyengine.UpdateYes, updated)
	assert.Empty(t, reason)
	assert.NotNil(t, mtx.FirstSubmit)
	assert.NotNil(t, mtx.LastSubmit)
	assert.Equal(t, `{
		"maxPriorityFee":32.146027800733336,
		"maxFee":32.14602781673334
	}`, mtx.GasPrice.String())

	mockFFCAPI.AssertExpectations(t)
}

func TestGasOracleSendOK(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, http.MethodGet)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"safeLow": {
			  "maxPriorityFee":30.7611840636,
			  "maxFee":30.7611840796
			  },
			"standard": {
			  "maxPriorityFee":32.146027800733336,
			  "maxFee":32.24712781673334
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

	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, `{
		"maxPriorityFeePerGas": {{.standard.maxPriorityFee | mulf 1000000000.0 | int }},
		"maxFeePerGas": {{ .standard.maxFee | mulf 1000000000.0 | int }}
	}`)
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x12345",
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI.On("TransactionSend", mock.Anything, mock.MatchedBy(func(req *ffcapi.TransactionSendRequest) bool {
		return req.GasPrice.JSONObject().GetInteger("maxPriorityFeePerGas").Cmp(big.NewInt(32146027800)) == 0 &&
			req.GasPrice.JSONObject().GetInteger("maxFeePerGas").Cmp(big.NewInt(32247127816)) == 0 &&
			req.From == "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712" &&
			req.TransactionData == "SOME_RAW_TX_BYTES"
	})).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: "0x12345",
	}, ffcapi.ErrorReason(""), nil)

	ctx := context.Background()
	updated, reason, err := p.Execute(ctx, tk, mtx)
	assert.NoError(t, err)
	assert.Empty(t, reason)
	assert.Equal(t, policyengine.UpdateYes, updated)
	assert.NotNil(t, mtx.FirstSubmit)
	assert.NotNil(t, mtx.LastSubmit)
	assert.Equal(t, `{
		"maxPriorityFeePerGas": 32146027800,
		"maxFeePerGas": 32247127816
	}`, mtx.GasPrice.String())

	mockFFCAPI.AssertExpectations(t)

	// Check cache after we close the gas station server
	server.Close()
	gasPrice, err := p.(*simplePolicyEngine).getGasPrice(ctx, mockFFCAPI)
	assert.NoError(t, err)
	assert.NotNil(t, gasPrice)
}

func TestConnectorGasOracleSendOK(t *testing.T) {

	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeConnector)
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x12345",
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI.On("GasPriceEstimate", mock.Anything, mock.Anything).Return(&ffcapi.GasPriceEstimateResponse{
		GasPrice: fftypes.JSONAnyPtr(`"12345"`),
	}, ffcapi.ErrorReason(""), nil).Once()
	mockFFCAPI.On("TransactionSend", mock.Anything, mock.MatchedBy(func(req *ffcapi.TransactionSendRequest) bool {
		return req.From == "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712" &&
			req.TransactionData == "SOME_RAW_TX_BYTES"
	})).Return(&ffcapi.TransactionSendResponse{
		TransactionHash: "0x12345",
	}, ffcapi.ErrorReason(""), nil)

	ctx := context.Background()
	updated, reason, err := p.Execute(ctx, tk, mtx)
	assert.NoError(t, err)
	assert.Empty(t, reason)
	assert.Equal(t, policyengine.UpdateYes, updated)
	assert.NotNil(t, mtx.FirstSubmit)
	assert.NotNil(t, mtx.LastSubmit)
	assert.Equal(t, `"12345"`, mtx.GasPrice.String())

	mockFFCAPI.AssertExpectations(t)

	// Check cache after we close the gas station server
	gasPrice, err := p.(*simplePolicyEngine).getGasPrice(ctx, mockFFCAPI)
	assert.NoError(t, err)
	assert.NotNil(t, gasPrice)
}

func TestConnectorGasOracleFail(t *testing.T) {

	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeConnector)
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x12345",
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI.On("GasPriceEstimate", mock.Anything, mock.Anything).Return(&ffcapi.GasPriceEstimateResponse{
		GasPrice: fftypes.JSONAnyPtr(`"12345"`),
	}, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	ctx := context.Background()
	_, reason, err := p.Execute(ctx, tk, mtx)
	assert.Regexp(t, "pop", err)
	assert.Empty(t, reason)

	mockFFCAPI.AssertExpectations(t)

}

func TestConnectorGasOracleFailStale(t *testing.T) {

	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeConnector)
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	longAgo := time.Now().Add(-1000 * time.Hour)
	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x12345",
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     (*fftypes.FFTime)(&longAgo),
		LastSubmit:      (*fftypes.FFTime)(&longAgo),
	}

	mockFFCAPI.On("GasPriceEstimate", mock.Anything, mock.Anything).Return(&ffcapi.GasPriceEstimateResponse{
		GasPrice: fftypes.JSONAnyPtr(`"12345"`),
	}, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	ctx := context.Background()
	_, reason, err := p.Execute(ctx, tk, mtx)
	assert.Regexp(t, "pop", err)
	assert.Empty(t, reason)

	mockFFCAPI.AssertExpectations(t)

}

func TestGasOracleSendFail(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`Not the gas station you are looking for`))
	}))
	defer server.Close()

	f, tk, _, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ . }}")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	ctx := context.Background()
	_, _, err = p.Execute(ctx, tk, mtx)
	assert.Regexp(t, "FF21021", err)

}

func TestGasOracleMissingTemplate(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	f, _, _, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	_, err := f.NewPolicyEngine(context.Background(), conf)
	assert.Regexp(t, "FF21024", err)

}

func TestGasOracleBadTemplate(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	f, _, _, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ !!! wrong")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	_, err := f.NewPolicyEngine(context.Background(), conf)
	assert.Regexp(t, "FF21025", err)

}

func TestGasOracleTemplateExecuteFail(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	f, tk, _, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ .wrong.thing | len }}")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	ctx := context.Background()
	_, _, err = p.Execute(ctx, tk, mtx)
	assert.Regexp(t, "FF21026", err)

}

func TestGasOracleNonJSON(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	f, tk, _, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ . }}")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	ctx := context.Background()
	_, _, err = p.Execute(ctx, tk, mtx)
	assert.Regexp(t, "FF21021", err)

}

func TestTXSendFail(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ . }}")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
	}

	mockFFCAPI.On("TransactionSend", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReasonInvalidInputs, fmt.Errorf("pop"))
	ctx := context.Background()
	_, _, err = p.Execute(ctx, tk, mtx)
	assert.Regexp(t, "pop", err)

}

func TestWarnStaleWarningCannotParse(t *testing.T) {
	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	submitTime := fftypes.FFTime(time.Now().Add(-100 * time.Hour))
	mtx := &apitypes.ManagedTX{
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     &submitTime,
		PolicyInfo:      fftypes.JSONAnyPtr("!not json!"),
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
	}

	mockFFCAPI.On("GasPriceEstimate", mock.Anything, mock.Anything).Return(&ffcapi.GasPriceEstimateResponse{
		GasPrice: fftypes.JSONAnyPtr(`"12345"`),
	}, ffcapi.ErrorReason(""), nil).Once()
	mockFFCAPI.On("TransactionSend", mock.Anything, mock.Anything).
		Return(nil, ffcapi.ErrorKnownTransaction, fmt.Errorf("Known transaction"))

	ctx := context.Background()
	updated, _, err := p.Execute(ctx, tk, mtx)
	assert.NoError(t, err)
	assert.Equal(t, policyengine.UpdateYes, updated)
	assert.NotEmpty(t, mtx.PolicyInfo.JSONObject().GetString("lastWarnTime"))

	mockFFCAPI.AssertExpectations(t)
}

func TestKnownTransactionHashKnown(t *testing.T) {
	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeDisabled)
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionData: "SOME_RAW_TX_BYTES",
		PolicyInfo:      fftypes.JSONAnyPtr("!not json!"),
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x01020304",
	}

	mockFFCAPI.On("TransactionSend", mock.Anything, mock.Anything).
		Return(nil, ffcapi.ErrorKnownTransaction, fmt.Errorf("Known transaction"))

	ctx := context.Background()
	updated, _, err := p.Execute(ctx, tk, mtx)
	assert.NoError(t, err)
	assert.Equal(t, policyengine.UpdateYes, updated)

	mockFFCAPI.AssertExpectations(t)
}

func TestWarnStaleAdditionalWarningResubmitFail(t *testing.T) {
	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	submitTime := fftypes.FFTime(time.Now().Add(-100 * time.Hour))
	lastWarning := fftypes.FFTime(time.Now().Add(-50 * time.Hour))
	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     &submitTime,
		PolicyInfo:      fftypes.JSONAnyPtr(fmt.Sprintf(`{"lastWarnTime": "%s"}`, lastWarning.String())),
	}

	mockFFCAPI.On("GasPriceEstimate", mock.Anything, mock.Anything).Return(&ffcapi.GasPriceEstimateResponse{
		GasPrice: fftypes.JSONAnyPtr(`"12345"`),
	}, ffcapi.ErrorReason(""), nil).Once()
	mockFFCAPI.On("TransactionSend", mock.Anything, mock.Anything).
		Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	ctx := context.Background()
	updated, reason, err := p.Execute(ctx, tk, mtx)
	assert.Regexp(t, "pop", err)
	assert.Empty(t, reason)
	assert.Equal(t, policyengine.UpdateYes, updated)
	assert.NotEmpty(t, mtx.PolicyInfo.JSONObject().GetString("lastWarnTime"))

	mockFFCAPI.AssertExpectations(t)
}

func TestWarnStaleNoWarning(t *testing.T) {
	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	submitTime := fftypes.FFTime(time.Now().Add(-100 * time.Hour))
	lastWarning := fftypes.Now()
	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     &submitTime,
		PolicyInfo:      fftypes.JSONAnyPtr(fmt.Sprintf(`{"lastWarnTime": "%s"}`, lastWarning.String())),
	}

	ctx := context.Background()
	updated, reason, err := p.Execute(ctx, tk, mtx)
	assert.Empty(t, reason)
	assert.NoError(t, err)
	assert.Equal(t, policyengine.UpdateNo, updated)

	mockFFCAPI.AssertExpectations(t)
}

func TestNoOpWithReceipt(t *testing.T) {
	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	submitTime := fftypes.Now()
	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     submitTime,
		Receipt: &ffcapi.TransactionReceiptResponse{
			BlockHash: "0x39e2664effa5ad0651c35f1fe3b4c4b90492b1955fee731c2e9fb4d6518de114",
		},
	}

	ctx := context.Background()
	updated, reason, err := p.Execute(ctx, tk, mtx)
	assert.Empty(t, reason)
	assert.NoError(t, err)
	assert.Equal(t, policyengine.UpdateNo, updated)

	mockFFCAPI.AssertExpectations(t)
}

func TestAllowsDeleteRequest(t *testing.T) {
	f, tk, mockFFCAPI, _, conf := newTestPolicyEngineFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	p, err := f.NewPolicyEngine(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		DeleteRequested: fftypes.Now(),
	}

	ctx := context.Background()
	updated, reason, err := p.Execute(ctx, tk, mtx)
	assert.Empty(t, reason)
	assert.NoError(t, err)
	assert.Equal(t, policyengine.UpdateDelete, updated)

	mockFFCAPI.AssertExpectations(t)
}
