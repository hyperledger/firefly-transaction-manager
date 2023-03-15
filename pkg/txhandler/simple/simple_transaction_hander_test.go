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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/ffresty"
	"github.com/hyperledger/firefly-common/pkg/fftypes"

	// Internal packages are used in the tests for e2e tests with more coverage
	// If you are developing a customized transaction handler, you'll need to mock the toolkit APIs instead
	"github.com/hyperledger/firefly-transaction-manager/internal/metrics"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/metricsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/persistencemocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/txhandlermocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhandler"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhistory"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestTransactionHandlerFactory(t *testing.T) (*TransactionHandlerFactory, *txhandler.Toolkit, *ffcapimocks.API, config.Section) {
	tmconfig.Reset()
	conf := config.RootSection("unittest.simple")

	f := &TransactionHandlerFactory{}
	f.InitConfig(conf)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeDisabled)
	assert.Equal(t, "simple", f.Name())

	mockPersistence := &persistencemocks.TransactionPersistence{}

	mockFFCAPI := &ffcapimocks.API{}

	return f, &txhandler.Toolkit{
		Connector:      mockFFCAPI,
		TXHistory:      txhistory.NewTxHistoryManager(context.Background()),
		TXPersistence:  mockPersistence,
		MetricsManager: metrics.NewMetricsManager(context.Background()),
	}, mockFFCAPI, conf
}

func newTestTransactionHandlerFactoryWithFilePersistence(t *testing.T) (*TransactionHandlerFactory, *txhandler.Toolkit, *ffcapimocks.API, config.Section, func()) {
	tmconfig.Reset()
	conf := config.RootSection("unittest.simple")

	f := &TransactionHandlerFactory{}
	f.InitConfig(conf)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeDisabled)
	assert.Equal(t, "simple", f.Name())

	dir, err := ioutil.TempDir("", "ldb_*")
	assert.NoError(t, err)
	config.Set(tmconfig.PersistenceLevelDBPath, dir)
	filePersistence, err := persistence.NewLevelDBPersistence(context.Background())
	assert.NoError(t, err)

	mockEventHandler := &txhandlermocks.ManagedTxEventHandler{}

	mockFFCAPI := &ffcapimocks.API{}

	return f, &txhandler.Toolkit{
			Connector:      mockFFCAPI,
			TXHistory:      txhistory.NewTxHistoryManager(context.Background()),
			TXPersistence:  filePersistence,
			MetricsManager: metrics.NewMetricsManager(context.Background()),
			EventHandler:   mockEventHandler,
		}, mockFFCAPI, conf,
		func() {
			os.RemoveAll(dir)
		}
}

func newTestTransactionHandler(t *testing.T) txhandler.TransactionHandler {
	f, _, _, conf := newTestTransactionHandlerFactory(t)
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)
	return th
}

func TestSupportDeprecatedPolicyEngineConfiguration(t *testing.T) {
	f, _, _, conf := newTestTransactionHandlerFactory(t)
	viper.SetDefault(string(tmconfig.DeprecatedPolicyEngineName), "simple")
	viper.SetDefault(string(tmconfig.DeprecatedTransactionsMaxInFlight), 23412412)

	conf.Set(FixedGasPrice, `12345`)

	th, err := f.NewTransactionHandler(context.Background(), conf)

	sth := th.(*simpleTransactionHandler)
	assert.Equal(t, 23412412, sth.maxInFlight)
	assert.NoError(t, err)
}

func TestMissingGasConfig(t *testing.T) {
	f, _, _, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeDisabled)
	_, err := f.NewTransactionHandler(context.Background(), conf)
	assert.Regexp(t, "FF21071", err)
}

func TestFixedGasPriceOK(t *testing.T) {
	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeDisabled)
	conf.Set(FixedGasPrice, `{
		"maxPriorityFee":32.146027800733336,
		"maxFee":32.14602781673334
	}`)
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x12345",
		TransactionData: "SOME_RAW_TX_BYTES",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
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

	mmm := &metricsmocks.TransactionHandlerMetrics{}
	mmm.On("InitTxHandlerGaugeMetric", mock.Anything, metricsTransactionsInflightCurrent, metricsTransactionsInflightCurrentDescription).Return(fmt.Errorf("fail")).Once()
	mmm.On("InitTxHandlerCounterMetricWithLabels", mock.Anything, metricsTransactionProcessEventsTotal, metricsTransactionProcessEventsTotalDescription, []string{metricsLabelNameEvent}).Return(fmt.Errorf("fail")).Once()
	mmm.On("InitTxHandlerHistogramMetricWithLabels", mock.Anything, metricsTransactionProcessDuration, metricsTransactionProcessDurationDescription, []float64{}, []string{metricsLabelNameEvent}).Return(fmt.Errorf("fail")).Once()
	mmm.On("IncTxHandlerCounterMetricWithLabels", mock.Anything, metricsTransactionProcessEventsTotal, mock.Anything, mock.Anything).Return().Maybe()
	mmm.On("ObserveTxHandlerHistogramMetricWithLabels", mock.Anything, metricsTransactionProcessEventsTotal, mock.Anything, mock.Anything).Return().Maybe()

	tk.MetricsManager = mmm

	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	updated, reason, err := sth.processTransaction(ctx, mtx)
	assert.NoError(t, err)
	assert.Equal(t, UpdateYes, updated)
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

	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, `{
		"maxPriorityFeePerGas": {{.standard.maxPriorityFee | mulf 1000000000.0 | int }},
		"maxFeePerGas": {{ .standard.maxFee | mulf 1000000000.0 | int }}
	}`)
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x12345",
		TransactionData: "SOME_RAW_TX_BYTES",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
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
	th.Init(ctx, tk)
	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	updated, reason, err := sth.processTransaction(ctx, mtx)
	assert.NoError(t, err)
	assert.Empty(t, reason)
	assert.Equal(t, UpdateYes, updated)
	assert.NotNil(t, mtx.FirstSubmit)
	assert.NotNil(t, mtx.LastSubmit)
	assert.Equal(t, `{
		"maxPriorityFeePerGas": 32146027800,
		"maxFeePerGas": 32247127816
	}`, mtx.GasPrice.String())

	mockFFCAPI.AssertExpectations(t)

	// Check cache after we close the gas station server
	server.Close()
	gasPrice, err := th.(*simpleTransactionHandler).getGasPrice(ctx, mockFFCAPI)
	assert.NoError(t, err)
	assert.NotNil(t, gasPrice)
}

func TestConnectorGasOracleSendOK(t *testing.T) {

	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeConnector)
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x12345",
		TransactionData: "SOME_RAW_TX_BYTES",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
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
	th.Init(ctx, tk)
	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	updated, reason, err := sth.processTransaction(ctx, mtx)
	assert.NoError(t, err)
	assert.Empty(t, reason)
	assert.Equal(t, UpdateYes, updated)
	assert.NotNil(t, mtx.FirstSubmit)
	assert.NotNil(t, mtx.LastSubmit)
	assert.Equal(t, `"12345"`, mtx.GasPrice.String())

	mockFFCAPI.AssertExpectations(t)

	// Check cache after we close the gas station server
	gasPrice, err := th.(*simpleTransactionHandler).getGasPrice(ctx, mockFFCAPI)
	assert.NoError(t, err)
	assert.NotNil(t, gasPrice)
}

func TestConnectorGasOracleFail(t *testing.T) {

	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeConnector)
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x12345",
		TransactionData: "SOME_RAW_TX_BYTES",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	mockFFCAPI.On("GasPriceEstimate", mock.Anything, mock.Anything).Return(&ffcapi.GasPriceEstimateResponse{
		GasPrice: fftypes.JSONAnyPtr(`"12345"`),
	}, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	ctx := context.Background()
	th.Init(ctx, tk)
	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	_, reason, err := sth.processTransaction(ctx, mtx)
	assert.Regexp(t, "pop", err)
	assert.Empty(t, reason)

	mockFFCAPI.AssertExpectations(t)

}

func TestConnectorGasOracleFailStale(t *testing.T) {

	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeConnector)
	th, err := f.NewTransactionHandler(context.Background(), conf)
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
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	mockFFCAPI.On("GasPriceEstimate", mock.Anything, mock.Anything).Return(&ffcapi.GasPriceEstimateResponse{
		GasPrice: fftypes.JSONAnyPtr(`"12345"`),
	}, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	_, reason, err := sth.processTransaction(ctx, mtx)
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

	f, tk, _, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ . }}")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	_, _, err = sth.processTransaction(ctx, mtx)
	assert.Regexp(t, "FF21021", err)

}

func TestGasOracleInvalidJSON(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`invalid JSON`))
	}))
	defer server.Close()

	f, tk, _, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ . }}")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	_, _, err = sth.processTransaction(ctx, mtx)
	assert.Regexp(t, "FF21076", err)

}

func TestGasOracleMissingTemplate(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	f, _, _, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	_, err := f.NewTransactionHandler(context.Background(), conf)
	assert.Regexp(t, "FF21024", err)

}

func TestGasOracleBadTemplate(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	f, _, _, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ !!! wrong")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	_, err := f.NewTransactionHandler(context.Background(), conf)
	assert.Regexp(t, "FF21025", err)

}

func TestGasOracleTemplateExecuteFail(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	f, tk, _, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ .wrong.thing | len }}")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	_, _, err = sth.processTransaction(ctx, mtx)
	assert.Regexp(t, "FF21026", err)

}

func TestGasOracleNonJSON(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	f, tk, _, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ . }}")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	_, _, err = sth.processTransaction(ctx, mtx)
	assert.Regexp(t, "FF21021", err)

}

func TestTXSendFail(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"gasPrice":32}`))
	}))
	defer server.Close()

	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeRESTAPI)
	conf.SubSection(GasOracleConfig).Set(GasOracleTemplate, "{{ .gasPrice }}")
	conf.SubSection(GasOracleConfig).Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", server.Listener.Addr()))
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionData: "SOME_RAW_TX_BYTES",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	mockFFCAPI.On("TransactionSend", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReasonInvalidInputs, fmt.Errorf("pop"))
	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	_, _, err = sth.processTransaction(ctx, mtx)
	assert.Regexp(t, "pop", err)

}

func TestWarnStaleWarningCannotParse(t *testing.T) {
	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeConnector)
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	submitTime := fftypes.FFTime(time.Now().Add(-100 * time.Hour))
	mtx := &apitypes.ManagedTX{
		TransactionData: "SOME_RAW_TX_BYTES",
		FirstSubmit:     &submitTime,
		PolicyInfo:      fftypes.JSONAnyPtr("!not json!"),
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		History: []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	mockFFCAPI.On("GasPriceEstimate", mock.Anything, mock.Anything).Return(&ffcapi.GasPriceEstimateResponse{
		GasPrice: fftypes.JSONAnyPtr(`"12345"`),
	}, ffcapi.ErrorReason(""), nil).Once()
	mockFFCAPI.On("TransactionSend", mock.Anything, mock.Anything).
		Return(nil, ffcapi.ErrorKnownTransaction, fmt.Errorf("Known transaction"))

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	updated, _, err := sth.processTransaction(ctx, mtx)
	assert.NoError(t, err)
	assert.Equal(t, UpdateYes, updated)
	assert.NotEmpty(t, mtx.PolicyInfo.JSONObject().GetString("lastWarnTime"))

	mockFFCAPI.AssertExpectations(t)
}

func TestKnownTransactionHashKnown(t *testing.T) {
	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeDisabled)
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		TransactionData: "SOME_RAW_TX_BYTES",
		PolicyInfo:      fftypes.JSONAnyPtr("!not json!"),
		TransactionHeaders: ffcapi.TransactionHeaders{
			From: "0x6b7cfa4cf9709d3b3f5f7c22de123d2e16aee712",
		},
		TransactionHash: "0x01020304",
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	mockFFCAPI.On("TransactionSend", mock.Anything, mock.Anything).
		Return(nil, ffcapi.ErrorKnownTransaction, fmt.Errorf("Known transaction"))

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	updated, _, err := sth.processTransaction(ctx, mtx)
	assert.NoError(t, err)
	assert.Equal(t, UpdateYes, updated)

	mockFFCAPI.AssertExpectations(t)
}

func TestWarnStaleAdditionalWarningResubmitFail(t *testing.T) {
	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.SubSection(GasOracleConfig).Set(GasOracleMode, GasOracleModeConnector)
	th, err := f.NewTransactionHandler(context.Background(), conf)
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
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	mockFFCAPI.On("GasPriceEstimate", mock.Anything, mock.Anything).Return(&ffcapi.GasPriceEstimateResponse{
		GasPrice: fftypes.JSONAnyPtr(`"12345"`),
	}, ffcapi.ErrorReason(""), nil).Once()
	mockFFCAPI.On("TransactionSend", mock.Anything, mock.Anything).
		Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	updated, reason, err := sth.processTransaction(ctx, mtx)
	assert.Regexp(t, "pop", err)
	assert.Empty(t, reason)
	assert.Equal(t, UpdateYes, updated)
	assert.NotEmpty(t, mtx.PolicyInfo.JSONObject().GetString("lastWarnTime"))

	mockFFCAPI.AssertExpectations(t)
}

func TestWarnStaleNoWarning(t *testing.T) {
	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	th, err := f.NewTransactionHandler(context.Background(), conf)
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
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	updated, reason, err := sth.processTransaction(ctx, mtx)
	assert.Empty(t, reason)
	assert.NoError(t, err)
	assert.Equal(t, UpdateNo, updated)

	mockFFCAPI.AssertExpectations(t)
}

func TestNoOpWithReceipt(t *testing.T) {
	f, _, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	th, err := f.NewTransactionHandler(context.Background(), conf)
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
		History: []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	ctx := context.Background()

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	updated, reason, err := sth.processTransaction(ctx, mtx)
	assert.Empty(t, reason)
	assert.NoError(t, err)
	assert.Equal(t, UpdateNo, updated)

	mockFFCAPI.AssertExpectations(t)
}

func TestAllowsDeleteRequest(t *testing.T) {
	f, tk, mockFFCAPI, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	th, err := f.NewTransactionHandler(context.Background(), conf)
	assert.NoError(t, err)

	mtx := &apitypes.ManagedTX{
		DeleteRequested: fftypes.Now(),
		History:         []*apitypes.TxHistoryStateTransitionEntry{{Status: apitypes.TxSubStatusReceived, Time: fftypes.Now(), Actions: []*apitypes.TxHistoryActionEntry{}}},
	}

	ctx := context.Background()
	th.Init(ctx, tk)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	updated, reason, err := sth.processTransaction(ctx, mtx)
	assert.Empty(t, reason)
	assert.NoError(t, err)
	assert.Equal(t, UpdateDelete, updated)

	mockFFCAPI.AssertExpectations(t)
}

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

func TestSendTXPersistFail(t *testing.T) {

	f, tk, _, conf := newTestTransactionHandlerFactory(t)
	conf.Set(FixedGasPrice, `12345`)
	conf.Set(ResubmitInterval, "100s")
	th, err := f.NewTransactionHandler(context.Background(), conf)

	sth := th.(*simpleTransactionHandler)
	sth.ctx = context.Background()
	mp := tk.TXPersistence.(*persistencemocks.TransactionPersistence)
	mp.On("ListTransactionsByNonce", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*apitypes.ManagedTX{
			{ID: "id12345", Created: fftypes.Now(), Status: apitypes.TxStatusSucceeded, Nonce: fftypes.NewFFBigInt(1000)},
		}, nil)
	mp.On("WriteTransaction", sth.ctx, mock.Anything, true).Return(fmt.Errorf("pop"))
	sth.Init(sth.ctx, tk)
	var txReq *ffcapi.TransactionSendRequest
	err = json.Unmarshal([]byte(sampleSendTX), &txReq)
	assert.NoError(t, err)

	_, err = sth.createManagedTx(sth.ctx, "id1", &txReq.TransactionHeaders, fftypes.NewFFBigInt(12345), "0x123456")
	assert.Regexp(t, "pop", err)

}
