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

package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/stretchr/testify/assert"
)

func newTestMetricsManager(t *testing.T) (*metricsManager, func()) {
	tmconfig.Reset()
	Clear()
	Registry()
	ctx, cancel := context.WithCancel(context.Background())
	mmi := NewMetricsManager(ctx)
	mm := mmi.(*metricsManager)
	assert.Equal(t, len(mm.timeMap), 0)
	return mm, cancel
}

func TestTransactionAPIRequestsTotal(t *testing.T) {
	ctx := context.Background()
	mm, cancel := newTestMetricsManager(t)
	defer cancel()
	mm.metricsEnabled = true
	mm.CountNewTransactionRequest(ctx, "newTransaction")
}

func TestTransactionAPIResponseTotal(t *testing.T) {
	ctx := context.Background()
	mm, cancel := newTestMetricsManager(t)
	defer cancel()
	mm.metricsEnabled = true
	mm.CountNewTransactionResponse(ctx, "newTransaction", "200")
}

func TestTransactionAPIRequestDuration(t *testing.T) {
	ctx := context.Background()
	mm, cancel := newTestMetricsManager(t)
	defer cancel()
	mm.metricsEnabled = true
	mm.RecordTransactionRequestDurationMs(ctx, "newTransaction", "200", 1*time.Millisecond)
}

func TestTransactionHandlerMetricsToolkit(t *testing.T) {
	ctx := context.Background()
	mm, cancel := newTestMetricsManager(t)
	defer cancel()
	mm.metricsEnabled = true
	mm.InitTxHandlerCounterMetric(ctx, "tx_request", "Transactions requests handled")
	mm.InitTxHandlerCounterMetricWithLabels(ctx, "tx_process", "Transaction processed", []string{"status"})
	mm.InitTxHandlerGaugeMetric(ctx, "tx_stalled", "Transactions that are stuck in a loop")
	mm.InitTxHandlerGaugeMetricWithLabels(ctx, "tx_inflight", "Transactions that are in flight", []string{"stage"})
	mm.InitTxHandlerHistogramMetric(ctx, "tx_timeout_ms", "Duration of timed out transactions", []float64{})
	mm.InitTxHandlerHistogramMetricWithLabels(ctx, "tx_stage_ms", "Duration of each transaction stage", []float64{}, []string{"stage"})

	mm.IncTxHandlerCounterMetric(ctx, "tx_request")
	mm.IncTxHandlerCounterMetricWithLabels(ctx, "tx_process", map[string]string{"status": "success"})
	mm.SetTxHandlerGaugeMetric(ctx, "tx_stalled", 2)
	mm.SetTxHandlerGaugeMetricWithLabels(ctx, "tx_inflight", 2, map[string]string{"stage": "singing:)"})
	mm.ObserveTxHandlerHistogramMetric(ctx, "tx_timeout_ms", 2000)
	mm.ObserveTxHandlerHistogramMetricWithLabels(ctx, "tx_stage_ms", 2000, map[string]string{"stage": "singing:)"})
}

func TestTransactionHandlerMetricsToolkitSwallowErrors(t *testing.T) {
	ctx := context.Background()
	mm, cancel := newTestMetricsManager(t)
	defer cancel()
	mm.metricsEnabled = true

	// swallow init errors
	mm.InitTxHandlerCounterMetric(ctx, "txInvalidName", "Invalid name")
	mm.InitTxHandlerGaugeMetric(ctx, "tx_duplicate", "Duplicate registration")
	mm.InitTxHandlerGaugeMetric(ctx, "tx_duplicate", "Duplicate registration")
	mm.InitTxHandlerGaugeMetric(ctx, "tx_no_help_text", "")

	// swallow emit errors, none of the metrics below are registered
	mm.IncTxHandlerCounterMetric(ctx, "tx_not_exist")
	mm.IncTxHandlerCounterMetricWithLabels(ctx, "tx_not_exist", map[string]string{"status": "success"})
	mm.SetTxHandlerGaugeMetric(ctx, "tx_not_exist", 2)
	mm.SetTxHandlerGaugeMetricWithLabels(ctx, "tx_not_exist", 2, map[string]string{"stage": "singing:)"})
	mm.ObserveTxHandlerHistogramMetric(ctx, "tx_not_exist", 2000)
	mm.ObserveTxHandlerHistogramMetricWithLabels(ctx, "tx_not_exist", 2000, map[string]string{"stage": "singing:)"})
}

func TestIsMetricsEnabledTrue(t *testing.T) {
	mm, cancel := newTestMetricsManager(t)
	defer cancel()
	mm.metricsEnabled = true
	assert.Equal(t, mm.IsMetricsEnabled(), true)
}

func TestIsMetricsEnabledFalse(t *testing.T) {
	mm, cancel := newTestMetricsManager(t)
	defer cancel()
	mm.metricsEnabled = false
	assert.Equal(t, mm.IsMetricsEnabled(), false)
}
