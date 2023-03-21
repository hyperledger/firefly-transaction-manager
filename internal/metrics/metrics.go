// Copyright © 2023 Kaleido, Inc.
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
	"fmt"
	"net/http"
	"time"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/metric"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsTransactionManagerPrefix = "tm"

// REST api-server and transaction handler are sub-subsystem
var metricsTransactionHandlerSubsystemName = fmt.Sprintf("%s_%s", metricsTransactionManagerPrefix, "th")
var metricsRESTAPIServerSubSystemName = fmt.Sprintf("%s_%s", metricsTransactionManagerPrefix, "api_server_rest")

type metricsManager struct {
	ctx                     context.Context
	metricsEnabled          bool
	metricsRegistry         metric.MetricsRegistry
	txHandlerMetricsManager metric.MetricsManager
	timeMap                 map[string]time.Time
}

func NewMetricsManager(ctx context.Context) Metrics {
	metricsRegistry := metric.NewPrometheusMetricsRegistry()
	txHandlerMetricsManager, _ := metricsRegistry.NewMetricsManagerForSubsystem(ctx, metricsTransactionHandlerSubsystemName)
	_ = metricsRegistry.NewHTTPMetricsInstrumentationsForSubsystem(
		ctx,
		metricsRESTAPIServerSubSystemName,
		true,
		prometheus.DefBuckets,
		map[string]string{},
	)
	mm := &metricsManager{
		ctx:                     ctx,
		metricsEnabled:          config.GetBool(tmconfig.MetricsEnabled),
		timeMap:                 make(map[string]time.Time),
		metricsRegistry:         metricsRegistry,
		txHandlerMetricsManager: txHandlerMetricsManager,
	}

	return mm
}

func (mm *metricsManager) IsMetricsEnabled() bool {
	return mm.metricsEnabled
}

func (mm *metricsManager) HTTPHandler() http.Handler {
	httpHandler, _ := mm.metricsRegistry.HTTPHandler(mm.ctx, promhttp.HandlerOpts{})
	return httpHandler
}

func (mm *metricsManager) GetAPIServerRESTHTTPMiddleware() func(next http.Handler) http.Handler {
	httpMiddleware, _ := mm.metricsRegistry.GetHTTPMetricsInstrumentationsMiddlewareForSubsystem(mm.ctx, metricsRESTAPIServerSubSystemName)
	return httpMiddleware
}

type Metrics interface {
	IsMetricsEnabled() bool

	// HTTPHandler returns the HTTP handler of this metrics registry
	HTTPHandler() http.Handler

	GetAPIServerRESTHTTPMiddleware() func(next http.Handler) http.Handler

	// functions for transaction handler to define and emit metrics
	TransactionHandlerMetrics
}

// Transaction handler metrics are defined and emitted by transaction handlers
type TransactionHandlerMetrics interface {
	// functions for declaring new metrics
	InitTxHandlerCounterMetric(ctx context.Context, metricName string, helpText string, withDefaultLabels bool)
	InitTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string, withDefaultLabels bool)
	InitTxHandlerGaugeMetric(ctx context.Context, metricName string, helpText string, withDefaultLabels bool)
	InitTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string, withDefaultLabels bool)
	InitTxHandlerHistogramMetric(ctx context.Context, metricName string, helpText string, buckets []float64, withDefaultLabels bool)
	InitTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, helpText string, buckets []float64, labelNames []string, withDefaultLabels bool)
	InitTxHandlerSummaryMetric(ctx context.Context, metricName string, helpText string, withDefaultLabels bool)
	InitTxHandlerSummaryMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string, withDefaultLabels bool)

	// functions for use existing metrics
	SetTxHandlerGaugeMetric(ctx context.Context, metricName string, number float64, defaultLabels *metric.FireflyDefaultLabels)
	SetTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels)
	IncTxHandlerCounterMetric(ctx context.Context, metricName string, defaultLabels *metric.FireflyDefaultLabels)
	IncTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels)
	ObserveTxHandlerHistogramMetric(ctx context.Context, metricName string, number float64, defaultLabels *metric.FireflyDefaultLabels)
	ObserveTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels)
	ObserveTxHandlerSummaryMetric(ctx context.Context, metricName string, number float64, defaultLabels *metric.FireflyDefaultLabels)
	ObserveTxHandlerSummaryMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels)
}

func (mm *metricsManager) InitTxHandlerCounterMetric(ctx context.Context, metricName string, helpText string, withDefaultLabels bool) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.NewCounterMetric(ctx, metricName, helpText, withDefaultLabels)
	}
}
func (mm *metricsManager) InitTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string, withDefaultLabels bool) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.NewCounterMetricWithLabels(ctx, metricName, helpText, labelNames, withDefaultLabels)
	}
}
func (mm *metricsManager) InitTxHandlerGaugeMetric(ctx context.Context, metricName string, helpText string, withDefaultLabels bool) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.NewGaugeMetric(ctx, metricName, helpText, withDefaultLabels)
	}
}
func (mm *metricsManager) InitTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string, withDefaultLabels bool) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.NewGaugeMetricWithLabels(ctx, metricName, helpText, labelNames, withDefaultLabels)
	}
}
func (mm *metricsManager) InitTxHandlerHistogramMetric(ctx context.Context, metricName string, helpText string, buckets []float64, withDefaultLabels bool) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.NewHistogramMetric(ctx, metricName, helpText, buckets, withDefaultLabels)
	}
}
func (mm *metricsManager) InitTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, helpText string, buckets []float64, labelNames []string, withDefaultLabels bool) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.NewHistogramMetricWithLabels(ctx, metricName, helpText, buckets, labelNames, withDefaultLabels)
	}
}
func (mm *metricsManager) InitTxHandlerSummaryMetric(ctx context.Context, metricName string, helpText string, withDefaultLabels bool) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.NewSummaryMetric(ctx, metricName, helpText, withDefaultLabels)
	}
}
func (mm *metricsManager) InitTxHandlerSummaryMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string, withDefaultLabels bool) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.NewSummaryMetricWithLabels(ctx, metricName, helpText, labelNames, withDefaultLabels)
	}
}

// functions for use existing metrics
func (mm *metricsManager) SetTxHandlerGaugeMetric(ctx context.Context, metricName string, number float64, defaultLabels *metric.FireflyDefaultLabels) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.SetGaugeMetric(ctx, metricName, number, defaultLabels)
	}
}
func (mm *metricsManager) SetTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.SetGaugeMetricWithLabels(ctx, metricName, number, labels, defaultLabels)
	}
}

func (mm *metricsManager) IncTxHandlerCounterMetric(ctx context.Context, metricName string, defaultLabels *metric.FireflyDefaultLabels) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.IncCounterMetric(ctx, metricName, defaultLabels)
	}
}
func (mm *metricsManager) IncTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.IncCounterMetricWithLabels(ctx, metricName, labels, defaultLabels)
	}
}
func (mm *metricsManager) ObserveTxHandlerHistogramMetric(ctx context.Context, metricName string, number float64, defaultLabels *metric.FireflyDefaultLabels) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.ObserveHistogramMetric(ctx, metricName, number, defaultLabels)
	}
}
func (mm *metricsManager) ObserveTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.ObserveHistogramMetricWithLabels(ctx, metricName, number, labels, defaultLabels)
	}
}

func (mm *metricsManager) ObserveTxHandlerSummaryMetric(ctx context.Context, metricName string, number float64, defaultLabels *metric.FireflyDefaultLabels) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.ObserveSummaryMetric(ctx, metricName, number, defaultLabels)
	}
}
func (mm *metricsManager) ObserveTxHandlerSummaryMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels) {
	if mm.metricsEnabled {
		mm.txHandlerMetricsManager.ObserveSummaryMetricWithLabels(ctx, metricName, number, labels, defaultLabels)
	}
}
