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
	"github.com/hyperledger/firefly-common/pkg/wsclient"
	"github.com/hyperledger/firefly/pkg/core"
	"github.com/spf13/viper"
)

var ffc = config.AddRootKey

var (
	ManagerName                                   = ffc("manager.name")
	ConfirmationsRequired                         = ffc("confirmations.required")
	ConfirmationsBlockQueueLength                 = ffc("confirmations.blockQueueLength")
	ConfirmationsStaleReceiptTimeout              = ffc("confirmations.staleReceiptTimeout")
	ConfirmationsNotificationQueueLength          = ffc("confirmations.notificationQueueLength")
	OperationsTypes                               = ffc("operations.types")
	OperationsFullScanStartupMaxRetries           = ffc("operations.fullScan.startupMaxRetries")
	OperationsFullScanPageSize                    = ffc("operations.fullScan.pageSize")
	OperationsFullScanMinimumDelay                = ffc("operations.fullScan.minimumDelay")
	OperationsErrorHistoryCount                   = ffc("operations.errorHistoryCount")
	OperationsChangeListenerEnabled               = ffc("operations.changeListener.enabled")
	PolicyLoopInterval                            = ffc("policyloop.interval")
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
	FFCoreNamespaces                              = ffc("ffcore.namespaces")
)

var FFCoreConfig config.Section

var APIConfig config.Section

var CorsConfig config.Section

var PolicyEngineBaseConfig config.Section

var WebhookPrefix config.Section

func setDefaults() {
	viper.SetDefault(string(OperationsFullScanPageSize), 100)
	viper.SetDefault(string(OperationsFullScanMinimumDelay), "5s")
	viper.SetDefault(string(OperationsTypes), []string{
		core.OpTypeBlockchainInvoke.String(),
		core.OpTypeBlockchainPinBatch.String(),
		core.OpTypeTokenCreatePool.String(),
	})
	viper.SetDefault(string(OperationsFullScanStartupMaxRetries), 10)
	viper.SetDefault(string(ConfirmationsRequired), 20)
	viper.SetDefault(string(ConfirmationsBlockQueueLength), 50)
	viper.SetDefault(string(ConfirmationsNotificationQueueLength), 50)
	viper.SetDefault(string(ConfirmationsStaleReceiptTimeout), "1m")
	viper.SetDefault(string(OperationsErrorHistoryCount), 25)
	viper.SetDefault(string(PolicyLoopInterval), "1s")
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
	viper.SetDefault(string(PersistenceLevelDBSyncWrites), true)

	viper.SetDefault(string(APIDefaultRequestTimeout), "30s")
	viper.SetDefault(string(APIMaxRequestTimeout), "10m")

	viper.SetDefault(string(FFCoreNamespaces), []string{})
}

func Reset() {
	config.RootConfigReset(setDefaults)

	FFCoreConfig = config.RootSection("ffcore")
	wsclient.InitConfig(FFCoreConfig)
	FFCoreConfig.SetDefault(wsclient.WSConfigKeyPath, "/admin/ws")

	APIConfig = config.RootSection("api")
	httpserver.InitHTTPConfig(APIConfig, 5008)

	CorsConfig = config.RootSection("cors")
	httpserver.InitCORSConfig(CorsConfig)

	WebhookPrefix = config.RootSection("webhooks")
	ffresty.InitConfig(WebhookPrefix)

	PolicyEngineBaseConfig = config.RootSection("policyengine")
	// policy engines must be registered outside of this package

}
