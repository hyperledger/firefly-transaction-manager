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
	"github.com/prometheus/client_golang/prometheus"
)

var TransactionSubmissionError prometheus.Counter
var TransactionsInFlight prometheus.Gauge

var MetricsTransactionSubmissionError = "ff_transaction_submission_error_total"
var MetricsTransactionsInFlight = "ff_transactions_in_flight"

func InitEvmMetrics() {
	TransactionSubmissionError = prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricsTransactionSubmissionError,
		Help: "Number of times the transaction manager received a submission error from the policy engine",
	})
	TransactionsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricsTransactionsInFlight,
		Help: "Number of transactions currently in flight",
	})
}

func RegisterEvmMetrics() {
	registry.MustRegister(TransactionSubmissionError)
	registry.MustRegister(TransactionsInFlight)
}
