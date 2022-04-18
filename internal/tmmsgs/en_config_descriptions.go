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

package tmmsgs

import "github.com/hyperledger/firefly/pkg/i18n"

var ffc = i18n.FFC

//revive:disable
var (
	ConfigAPIAddress      = ffc("config.api.address", "Listener address for API", "string")
	ConfigAPIPort         = ffc("config.api.port", "Listener port for API", "number")
	ConfigAPIPublicURL    = ffc("config.api.publicURL", "External address callers should access API over", "string")
	ConfigAPIReadTimeout  = ffc("config.api.readTimeout", "The maximum time to wait when reading from an HTTP connection", "duration")
	ConfigAPIWriteTimeout = ffc("config.api.writeTimeout", "The maximum time to wait when writing to a HTTP connection", "duration")

	ConfigConfirmationsBlockCacheSize           = ffc("config.confirmations.blockCacheSize", "The maximum number of block headers to keep in the cache", "number")
	ConfigConfirmationsBlockPollingInterval     = ffc("config.confirmations.blockPollingInterval", "How often to poll for new block headers", "duration")
	ConfigConfirmationsNotificationsQueueLength = ffc("config.confirmations.notificationQueueLength", "Internal queue length for notifying the confirmations manager of new transactions/events", "number")
	ConfigConfirmationsRequired                 = ffc("config.confirmations.required", "Number of confirmations required to consider a transaction/event final", "number")

	ConfigConnectorURL      = ffc("config.connector.url", "The URL of the blockchain connector", "string")
	ConfigConnectorVariant  = ffc("config.connector.variant", "The variant is the overall category of blockchain connector, defining things like how input/output definitions are passed", "string")
	ConfigConnectorProxyURL = ffc("config.connector.proxy.url", "Optional HTTP proxy URL to use for the blockchain connector", "string")

	ConfigFFCoreURL      = ffc("config.ffcore.url", "The URL of the FireFly core admin API server to connect to", "string")
	ConfigFFCoreProxyURL = ffc("config.ffcore.proxy.url", "Optional HTTP proxy URL to use for the FireFly core admin API server", "string")

	ConfigManagerName = ffc("config.manager.name", "The name of this Transaction Manager, used in operation metadata to track which operations are to be updated", "string")

	ConfigOperationsTypes                     = ffc("config.operations.types", "The operation types to query in FireFly core, that might have been submitted via this Transaction Manager", "string[]")
	ConfigOperationsFullScanMinimumDelay      = ffc("config.operations.fullScan.minimumDelay", "The minimum delay between full scans of the FireFly core API, when reconnecting, or recovering from missed events / errors", "duration")
	ConfigOperationsFullScanPageSize          = ffc("config.operations.fullScan.pageSize", "The page size to use when performing a full scan of the ForeFly core API on startup, or recovery", "number")
	ConfigOperationsFullScanStartupMaxRetries = ffc("config.operations.fullScan.startupMaxRetries", "The page size to use when performing a full scan of the ForeFly core API on startup, or recovery", "number")

	ConfigPolicyEngineName = ffc("config.policyengine.name", "The name of the policy engine to use", "string")

	ConfigPolicyEngineSimpleFixedGas           = ffc("config.policyengine.simple.fixedGas", "A fixed gasPrice value/structure to pass to the connector", "Raw JSON")
	ConfigPolicyEngineSimpleWarnInterval       = ffc("config.policyengine.simple.warnInterval", "The time between warnings when a blockchain transaction has not been allocated a receipt", "duration")
	ConfigPolicyEngineSimpleGasStationEnabled  = ffc("config.policyengine.simple.gasstation.enabled", "When true the configured gasstation URL will be queried before submitting each", "boolean")
	ConfigPolicyEngineSimpleGasStationGJSON    = ffc("config.policyengine.simple.gasstation.gjson", "A GJSON query to execute against the response from the Gas Station API. The raw json will then be passed as the gasPrice to the connector", "see [GJSON syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)")
	ConfigPolicyEngineSimpleGasStationURL      = ffc("config.policyengine.simple.gasstation.url", "The URL of a Gas Station API to call", "string")
	ConfigPolicyEngineSimpleGasStationProxyURL = ffc("config.policyengine.simple.gasstation.proxy.url", "Optional HTTP proxy URL to use for the Gas Station API", "string")
	ConfigPolicyEngineSimpleGasStationMethod   = ffc("config.policyengine.simple.gasstation.method", "The HTTP Method to use when invoking the Gas STation API", "string")

	ConfigReceiptsPollingInterval = ffc("config.receipts.pollingInteval", "Interval between queries for receipts for all in-flight transactions that have not met the confirmation threshold", "duration")
)
