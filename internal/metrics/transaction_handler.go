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
	"regexp"

	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/prometheus/client_golang/prometheus"
)

var allowedNameStringRegex = `^[a-z_]+$`
var prefix = `ff_th_`

var thMetrics = make(map[string]prometheus.Collector)

var supportedTypes = []string{"counter", "counterVec", "gauge", "gaugeVec", "histogram", "histogramVec"}

type regInfo struct {
	Type       string
	Name       string
	HelpText   string
	LabelNames []string  // should only be provided for metrics types with vectors
	Buckets    []float64 // only applicable for histogram
}

func initTxHandlerMetrics(ctx context.Context, mr regInfo) {

	nameRegex := regexp.MustCompile(allowedNameStringRegex)

	isValidNameString := nameRegex.MatchString(mr.Name)
	if !isValidNameString {
		err := i18n.NewError(ctx, tmmsgs.MsgTHMetricsInvalidName, mr.Name)
		log.L(ctx).Warnf("Failed to initialize metric %s due to error: %s", mr.Name, err.Error())
		return
	}

	for _, typeName := range supportedTypes {
		if _, ok := thMetrics[prefix+mr.Name+"_"+typeName]; ok {
			err := i18n.NewError(ctx, tmmsgs.MsgTHMetricsDuplicateName, mr.Name)
			log.L(ctx).Warnf("Failed to initialize metric %s due to error: %s", mr.Name, err.Error())
			return

		}
	}
	if mr.HelpText == "" {
		err := i18n.NewError(ctx, tmmsgs.MsgTHMetricsHelpTextMissing)
		log.L(ctx).Warnf("Failed to initialize metric %s due to error: %s", mr.Name, err.Error())
		return
	}
	switch mr.Type {
	case "counter":
		thMetrics[prefix+mr.Name+"_"+mr.Type] = prometheus.NewCounter(prometheus.CounterOpts{
			Name: mr.Name,
			Help: mr.HelpText,
		})
	case "counterVec":
		thMetrics[prefix+mr.Name+"_"+mr.Type] = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: mr.Name,
			Help: mr.HelpText,
		}, mr.LabelNames)
	case "gauge":
		thMetrics[prefix+mr.Name+"_"+mr.Type] = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: mr.Name,
			Help: mr.HelpText,
		})
	case "gaugeVec":
		thMetrics[prefix+mr.Name+"_"+mr.Type] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: mr.Name,
			Help: mr.HelpText,
		}, mr.LabelNames)
	case "histogram":
		thMetrics[prefix+mr.Name+"_"+mr.Type] = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    mr.Name,
			Help:    mr.HelpText,
			Buckets: mr.Buckets,
		})
	case "histogramVec":
		thMetrics[prefix+mr.Name+"_"+mr.Type] = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    mr.Name,
			Help:    mr.HelpText,
			Buckets: mr.Buckets,
		}, mr.LabelNames)
		// deliberately no support of Summary type until it's necessary. Use histogram instead.
	}
}
func InitTxHandlerCounterMetric(ctx context.Context, metricName string, helpText string) {
	initTxHandlerMetrics(ctx, regInfo{
		Type:     "counter",
		Name:     metricName,
		HelpText: helpText,
	})
}

func InitTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string) {
	initTxHandlerMetrics(ctx, regInfo{
		Type:       "counterVec",
		Name:       metricName,
		HelpText:   helpText,
		LabelNames: labelNames,
	})
}

func InitTxHandlerGaugeMetric(ctx context.Context, metricName string, helpText string) {
	initTxHandlerMetrics(ctx, regInfo{
		Type:     "gauge",
		Name:     metricName,
		HelpText: helpText,
	})
}

func InitTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string) {
	initTxHandlerMetrics(ctx, regInfo{
		Type:       "gaugeVec",
		Name:       metricName,
		HelpText:   helpText,
		LabelNames: labelNames,
	})
}
func InitTxHandlerHistogramMetric(ctx context.Context, metricName string, helpText string, buckets []float64) {
	initTxHandlerMetrics(ctx, regInfo{
		Type:     "histogram",
		Name:     metricName,
		HelpText: helpText,
		Buckets:  buckets,
	})
}

func InitTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, helpText string, buckets []float64, labelNames []string) {
	initTxHandlerMetrics(ctx, regInfo{
		Type:       "histogramVec",
		Name:       metricName,
		HelpText:   helpText,
		Buckets:    buckets,
		LabelNames: labelNames,
	})
}

func SetTxHandlerGaugeMetric(ctx context.Context, metricName string, number float64) {
	collector, ok := thMetrics[prefix+metricName+"_gauge"]
	if !ok {
		log.L(ctx).Warnf("Transaction handler metric with name: '%s' and type: '%s' is not found", metricName, "gauge")
	} else {
		collector.(prometheus.Gauge).Set(number)
	}
}
func SetTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string) {
	collector, ok := thMetrics[prefix+metricName+"_gaugeVec"]
	if !ok {
		log.L(ctx).Warnf("Transaction handler metric with name: '%s' and type: '%s' is not found", metricName, "gaugeVec")
	} else {
		collector.(*prometheus.GaugeVec).With(labels).Set(number)
	}
}

func IncTxHandlerCounterMetric(ctx context.Context, metricName string) {
	collector, ok := thMetrics[prefix+metricName+"_counter"]
	if !ok {
		log.L(ctx).Warnf("Transaction handler metric with name: '%s' and type: '%s' is not found", metricName, "counter")
	} else {
		collector.(prometheus.Counter).Inc()
	}
}

func IncTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, labels map[string]string) {
	collector, ok := thMetrics[prefix+metricName+"_counterVec"]
	if !ok {
		log.L(ctx).Warnf("Transaction handler metric with name: '%s' and type: '%s' is not found", metricName, "counterVec")
	} else {
		collector.(*prometheus.CounterVec).With(labels).Inc()
	}
}

func ObserveTxHandlerHistogramMetric(ctx context.Context, metricName string, number float64) {
	collector, ok := thMetrics[prefix+metricName+"_histogram"]
	if !ok {
		log.L(ctx).Warnf("Transaction handler metric with name: '%s' and type: '%s' is not found", metricName, "histogram")
	} else {
		collector.(prometheus.Histogram).Observe(number)
	}
}

func ObserveTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string) {
	collector, ok := thMetrics[prefix+metricName+"_histogramVec"]
	if !ok {
		log.L(ctx).Warnf("Transaction handler metric with name: '%s' and type: '%s' is not found", metricName, "histogramVec")
	} else {
		collector.(*prometheus.HistogramVec).With(labels).Observe(number)
	}
}

func RegisterTxHandlerMetrics() {
	for _, collector := range thMetrics {
		registry.MustRegister(collector)
	}
}
