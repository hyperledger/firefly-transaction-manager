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
var TransactionAPIRequestDurationSecond *prometheus.HistogramVec

var MetricsTransactionRequestCount = "ff_tm_api_requests_total"
var MetricsTransactionResponseCount = "ff_tm_api_responses_total"
var MetricsTransactionRequestDurationSecond = "ff_tm_api_request_duration"

func InitTxManagementMetrics() {
	TransactionAPIRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricsTransactionRequestCount,
		Help: "Number of transaction API requests",
	}, []string{"operation"})

	TransactionAPIResponsesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricsTransactionResponseCount,
		Help: "Number of transaction API responses",
	}, []string{"operation", "status"})

	TransactionAPIRequestDurationSecond = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: MetricsTransactionRequestDurationSecond,
		Help: "Time of processing transaction API requests",
	}, []string{"operation", "status"})
}

func RegisterTXManagerMetrics() {
	registry.MustRegister(TransactionAPIRequestsTotal)
	registry.MustRegister(TransactionAPIResponsesTotal)
	registry.MustRegister(TransactionAPIRequestDurationSecond)
}

func CountNewTransactionRequest(ctx context.Context, operationName string) {
	TransactionAPIRequestsTotal.With(prometheus.Labels{
		"operation": operationName,
	}).Inc()
}

func CountSuccessTransactionResponse(ctx context.Context, operationName string) {
	TransactionAPIResponsesTotal.With(prometheus.Labels{
		"operation": operationName,
		"status":    "success",
	}).Inc()
}

func CountErrorTransactionResponse(ctx context.Context, operationName string) {
	TransactionAPIResponsesTotal.With(prometheus.Labels{
		"operation": operationName,
		"status":    "error",
	}).Inc()
}

func RecordSuccessTransactionRequestDuration(ctx context.Context, operationName string, ms time.Duration) {
	TransactionAPIRequestDurationSecond.With(prometheus.Labels{
		"operation": operationName,
		"status":    "success",
	}).Observe(ms.Seconds())
}

func RecordErrorTransactionRequestDuration(ctx context.Context, operationName string, ms time.Duration) {
	TransactionAPIRequestDurationSecond.With(prometheus.Labels{
		"operation": operationName,
		"status":    "error",
	}).Observe(ms.Seconds())
}
