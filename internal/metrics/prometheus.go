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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	muxprom "gitlab.com/hfuss/mux-prometheus/pkg/middleware"
)

var regMux sync.Mutex
var registry *prometheus.Registry
var evmInstrumentation *muxprom.Instrumentation

// Registry returns FireFly's customized Prometheus registry
func Registry() *prometheus.Registry {
	if registry == nil {
		initMetricsCollectors()
		registry = prometheus.NewRegistry()
		registerMetricsCollectors()
	}

	return registry
}

// GetAdminServerInstrumentation returns the admin server's Prometheus middleware, ensuring its metrics are never
// registered twice
func GetEvmServerInstrumentation() *muxprom.Instrumentation {
	regMux.Lock()
	defer regMux.Unlock()
	if evmInstrumentation == nil {
		evmInstrumentation = NewInstrumentation("fftm")
	}
	return evmInstrumentation
}

func NewInstrumentation(subsystem string) *muxprom.Instrumentation {
	return muxprom.NewCustomInstrumentation(
		true,
		"ff_transaction_manager",
		subsystem,
		prometheus.DefBuckets,
		map[string]string{},
		Registry(),
	)
}

// Clear will reset the Prometheus metrics registry and instrumentations, useful for testing
func Clear() {
	registry = nil
	evmInstrumentation = nil
}

func initMetricsCollectors() {
	InitEvmMetrics()
}

func registerMetricsCollectors() {
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	RegisterEvmMetrics()
}
