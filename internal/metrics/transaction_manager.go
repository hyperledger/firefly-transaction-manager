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

	"github.com/prometheus/client_golang/prometheus"
)

var TransactionAPIRequestsTotal *prometheus.CounterVec
var TransactionAPIResponsesTotal *prometheus.CounterVec
var TransactionAPIRequestDurationMs *prometheus.HistogramVec

var MetricsTransactionRequestCount = "ff_tm_api_requests_total"
var MetricsTransactionResponseCount = "ff_tm_api_responses_total"
var MetricsTransactionRequestDurationMs = "ff_tm_api_request_duration_ms"

func InitTxManagementMetrics() {
	TransactionAPIRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricsTransactionRequestCount,
		Help: "Number of transaction API requests",
	}, []string{"operation"})

	TransactionAPIResponsesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricsTransactionResponseCount,
		Help: "Number of transaction API responses",
	}, []string{"operation", "status"})

	TransactionAPIRequestDurationMs = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: MetricsTransactionRequestDurationMs,
		Help: "Time of processing transaction API requests",
	}, []string{"operation", "status"})
}

func RegisterTXManagerMetrics() {
	registry.MustRegister(TransactionAPIRequestsTotal)
	registry.MustRegister(TransactionAPIResponsesTotal)
	registry.MustRegister(TransactionAPIRequestDurationMs)
}

func CountNewTransactionRequest(ctx context.Context, operationName string) {
	TransactionAPIRequestsTotal.With(prometheus.Labels{
		"operation": operationName,
	}).Inc()
}

func CountNewTransactionResponse(ctx context.Context, operationName string, status string) {
	TransactionAPIResponsesTotal.With(prometheus.Labels{
		"operation": operationName,
		"status":    status,
	}).Inc()
}

func RecordTransactionRequestDurationMs(ctx context.Context, operationName string, status string, ms time.Duration) {
	TransactionAPIRequestDurationMs.With(prometheus.Labels{
		"operation": operationName,
		"status":    status,
	}).Observe(float64(ms.Milliseconds()))
}
