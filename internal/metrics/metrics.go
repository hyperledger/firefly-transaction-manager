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

func (mm *metricsManager) TransactionSubmissionError() {
	TransactionSubmissionError.Inc()
}
func (mm *metricsManager) TransactionsInFlightSet(count float64) {
	TransactionsInFlight.Set(count)
}

func (mm *metricsManager) IsMetricsEnabled() bool {
	return mm.metricsEnabled
}

type Metrics interface {
	TransactionSubmissionError()
	TransactionsInFlightSet(count float64)
	IsMetricsEnabled() bool
}
