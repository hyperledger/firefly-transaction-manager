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

package tmconfig

import (
	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/ffresty"
	"github.com/hyperledger/firefly-common/pkg/httpserver"
	"github.com/spf13/viper"
)

var ffc = config.AddRootKey

var (
	ConfirmationsRequired                         = ffc("confirmations.required")
	ConfirmationsBlockQueueLength                 = ffc("confirmations.blockQueueLength")
	ConfirmationsStaleReceiptTimeout              = ffc("confirmations.staleReceiptTimeout")
	ConfirmationsNotificationQueueLength          = ffc("confirmations.notificationQueueLength")
	TransactionsErrorHistoryCount                 = ffc("transactions.errorHistoryCount")
	TransactionsMaxInFlight                       = ffc("transactions.maxInFlight")
	TransactionsNonceStateTimeout                 = ffc("transactions.nonceStateTimeout")
	PolicyLoopInterval                            = ffc("policyloop.interval")
	PolicyLoopRetryInitDelay                      = ffc("policyloop.retry.initialDelay")
	PolicyLoopRetryMaxDelay                       = ffc("policyloop.retry.maxDelay")
	PolicyLoopRetryFactor                         = ffc("policyloop.retry.factor")
	PolicyEngineName                              = ffc("policyengine.name")
	EventStreamsDefaultsBatchSize                 = ffc("eventstreams.defaults.batchSize")
	EventStreamsDefaultsBatchTimeout              = ffc("eventstreams.defaults.batchTimeout")
	EventStreamsDefaultsErrorHandling             = ffc("eventstreams.defaults.errorHandling")
	EventStreamsDefaultsRetryTimeout              = ffc("eventstreams.defaults.retryTimeout")
	EventStreamsDefaultsBlockedRetryDelay         = ffc("eventstreams.defaults.blockedRetryDelay")
	EventStreamsDefaultsWebhookRequestTimeout     = ffc("eventstreams.defaults.webhookRequestTimeout")
	EventStreamsDefaultsWebsocketDistributionMode = ffc("eventstreams.defaults.websocketDistributionMode")
	EventStreamsCheckpointInterval                = ffc("eventstreams.checkpointInterval")
	EventStreamsRetryInitDelay                    = ffc("eventstreams.retry.initialDelay")
	EventStreamsRetryMaxDelay                     = ffc("eventstreams.retry.maxDelay")
	EventStreamsRetryFactor                       = ffc("eventstreams.retry.factor")
	WebhooksAllowPrivateIPs                       = ffc("webhooks.allowPrivateIPs")
	PersistenceType                               = ffc("persistence.type")
	PersistenceLevelDBPath                        = ffc("persistence.leveldb.path")
	PersistenceLevelDBMaxHandles                  = ffc("persistence.leveldb.maxHandles")
	PersistenceLevelDBSyncWrites                  = ffc("persistence.leveldb.syncWrites")
	APIDefaultRequestTimeout                      = ffc("api.defaultRequestTimeout")
	APIMaxRequestTimeout                          = ffc("api.maxRequestTimeout")
	DebugPort                                     = ffc("debug.port")
	MetricsEnabled                                = ffc("metrics.enabled")
	MetricsPath                                   = ffc("metrics.path")
)

var APIConfig config.Section

var CorsConfig config.Section

var PolicyEngineBaseConfig config.Section

var WebhookPrefix config.Section

var MetricsConfig config.Section

func setDefaults() {
	viper.SetDefault(string(TransactionsMaxInFlight), 100)
	viper.SetDefault(string(TransactionsErrorHistoryCount), 25)
	viper.SetDefault(string(TransactionsNonceStateTimeout), "1h")
	viper.SetDefault(string(ConfirmationsRequired), 20)
	viper.SetDefault(string(ConfirmationsBlockQueueLength), 50)
	viper.SetDefault(string(ConfirmationsNotificationQueueLength), 50)
	viper.SetDefault(string(ConfirmationsStaleReceiptTimeout), "1m")
	viper.SetDefault(string(PolicyLoopInterval), "10s")
	viper.SetDefault(string(PolicyEngineName), "simple")

	viper.SetDefault(string(EventStreamsDefaultsBatchSize), 50)
	viper.SetDefault(string(EventStreamsDefaultsBatchTimeout), "5s")
	viper.SetDefault(string(EventStreamsDefaultsErrorHandling), "block")
	viper.SetDefault(string(EventStreamsDefaultsRetryTimeout), "30s")
	viper.SetDefault(string(EventStreamsDefaultsBlockedRetryDelay), "30s")
	viper.SetDefault(string(EventStreamsDefaultsWebhookRequestTimeout), "30s")
	viper.SetDefault(string(EventStreamsDefaultsWebsocketDistributionMode), "load_balance")
	viper.SetDefault(string(EventStreamsCheckpointInterval), "1m")
	viper.SetDefault(string(WebhooksAllowPrivateIPs), true)

	viper.SetDefault(string(PersistenceType), "leveldb")
	viper.SetDefault(string(PersistenceLevelDBMaxHandles), 100)
	viper.SetDefault(string(PersistenceLevelDBSyncWrites), false)

	viper.SetDefault(string(APIDefaultRequestTimeout), "30s")
	viper.SetDefault(string(APIMaxRequestTimeout), "10m")

	viper.SetDefault(string(PolicyLoopRetryInitDelay), "250ms")
	viper.SetDefault(string(PolicyLoopRetryMaxDelay), "30s")
	viper.SetDefault(string(PolicyLoopRetryFactor), 2.0)
	viper.SetDefault(string(EventStreamsRetryInitDelay), "250ms")
	viper.SetDefault(string(EventStreamsRetryMaxDelay), "30s")
	viper.SetDefault(string(EventStreamsRetryFactor), 2.0)
	viper.SetDefault(string(DebugPort), -1)
	viper.SetDefault(string(MetricsEnabled), false)
	viper.SetDefault(string(MetricsPath), "/metrics")
}

func Reset() {
	config.RootConfigReset(setDefaults)

	APIConfig = config.RootSection("api")
	httpserver.InitHTTPConfig(APIConfig, 5008)

	CorsConfig = config.RootSection("cors")
	httpserver.InitCORSConfig(CorsConfig)

	WebhookPrefix = config.RootSection("webhooks")
	ffresty.InitConfig(WebhookPrefix)

	PolicyEngineBaseConfig = config.RootSection("policyengine")
	// policy engines must be registered outside of this package

	MetricsConfig = config.RootSection("metrics")
	httpserver.InitHTTPConfig(MetricsConfig, 6000)
}
