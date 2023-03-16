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
	"time"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
)

type metricsManager struct {
	ctx            context.Context
	metricsEnabled bool
	timeMap        map[string]time.Time
}

func NewMetricsManager(ctx context.Context) Metrics {
	mm := &metricsManager{
		ctx:            ctx,
		metricsEnabled: config.GetBool(tmconfig.MetricsEnabled),
		timeMap:        make(map[string]time.Time),
	}

	return mm
}

func (mm *metricsManager) IsMetricsEnabled() bool {
	return mm.metricsEnabled
}

type Metrics interface {
	IsMetricsEnabled() bool

	// functions for transaction handler to define and emit metrics
	TransactionHandlerMetrics
}

// Transaction handler metrics are defined and emitted by transaction handlers
type TransactionHandlerMetrics interface {
	// functions for declaring new metrics
	InitTxHandlerCounterMetric(ctx context.Context, metricName string, helpText string)
	InitTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string)
	InitTxHandlerGaugeMetric(ctx context.Context, metricName string, helpText string)
	InitTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string)
	InitTxHandlerHistogramMetric(ctx context.Context, metricName string, helpText string, buckets []float64)
	InitTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, helpText string, buckets []float64, labelNames []string)

	// functions for use existing metrics
	SetTxHandlerGaugeMetric(ctx context.Context, metricName string, number float64)
	SetTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string)
	IncTxHandlerCounterMetric(ctx context.Context, metricName string)
	IncTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, labels map[string]string)
	ObserveTxHandlerHistogramMetric(ctx context.Context, metricName string, number float64)
	ObserveTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string)
}

func (mm *metricsManager) InitTxHandlerCounterMetric(ctx context.Context, metricName string, helpText string) {
	if mm.metricsEnabled {
		InitTxHandlerCounterMetric(ctx, metricName, helpText)
	}
}
func (mm *metricsManager) InitTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string) {
	if mm.metricsEnabled {
		InitTxHandlerCounterMetricWithLabels(ctx, metricName, helpText, labelNames)
	}
}
func (mm *metricsManager) InitTxHandlerGaugeMetric(ctx context.Context, metricName string, helpText string) {
	if mm.metricsEnabled {
		InitTxHandlerGaugeMetric(ctx, metricName, helpText)
	}
}
func (mm *metricsManager) InitTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string) {
	if mm.metricsEnabled {
		InitTxHandlerGaugeMetricWithLabels(ctx, metricName, helpText, labelNames)
	}
}
func (mm *metricsManager) InitTxHandlerHistogramMetric(ctx context.Context, metricName string, helpText string, buckets []float64) {
	if mm.metricsEnabled {
		InitTxHandlerHistogramMetric(ctx, metricName, helpText, buckets)
	}
}
func (mm *metricsManager) InitTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, helpText string, buckets []float64, labelNames []string) {
	if mm.metricsEnabled {
		InitTxHandlerHistogramMetricWithLabels(ctx, metricName, helpText, buckets, labelNames)
	}
}

// functions for use existing metrics
func (mm *metricsManager) SetTxHandlerGaugeMetric(ctx context.Context, metricName string, number float64) {
	if mm.metricsEnabled {
		SetTxHandlerGaugeMetric(ctx, metricName, number)
	}
}
func (mm *metricsManager) SetTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string) {
	if mm.metricsEnabled {
		SetTxHandlerGaugeMetricWithLabels(ctx, metricName, number, labels)
	}
}

func (mm *metricsManager) IncTxHandlerCounterMetric(ctx context.Context, metricName string) {
	if mm.metricsEnabled {
		IncTxHandlerCounterMetric(ctx, metricName)
	}
}
func (mm *metricsManager) IncTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, labels map[string]string) {
	if mm.metricsEnabled {
		IncTxHandlerCounterMetricWithLabels(ctx, metricName, labels)
	}
}
func (mm *metricsManager) ObserveTxHandlerHistogramMetric(ctx context.Context, metricName string, number float64) {
	if mm.metricsEnabled {
		ObserveTxHandlerHistogramMetric(ctx, metricName, number)
	}
}
func (mm *metricsManager) ObserveTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string) {
	if mm.metricsEnabled {
		ObserveTxHandlerHistogramMetricWithLabels(ctx, metricName, number, labels)
	}
}
