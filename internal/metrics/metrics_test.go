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

func TestTransactionSubmissionError(t *testing.T) {
	mm, cancel := newTestMetricsManager(t)
	defer cancel()
	mm.TransactionSubmissionError()
}

func TestTransactionsInFlight(t *testing.T) {
	mm, cancel := newTestMetricsManager(t)
	defer cancel()
	mm.TransactionsInFlightSet(float64(12))
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
