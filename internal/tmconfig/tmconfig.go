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
	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/ffresty"
	"github.com/spf13/viper"
)

var ffc = config.AddRootKey

var (
	// MonitorPollingInterval frequency of polling against FireFly (note polling is only a backup for event notification)
	MonitorPollingInterval = ffc("monitor.pollingInterval")
	// ConnectorVariant is the variant setting to add to all requests to the backend connector
	ConnectorVariant = ffc("connector.variant")
	// ConfirmationsRequired is the number of confirmations required for a transaction to be considered final
	ConfirmationsRequired = ffc("confirmations.required")
	// ConfirmationsBlockCacheSize is the size of the block cache
	ConfirmationsBlockCacheSize = ffc("confirmations.blockCacheSize")
	// ConfirmationsBlockPollingInterval is the time between block polling
	ConfirmationsBlockPollingInterval = ffc("confirmations.blockPollingInterval")
	// ConfirmationsNotificationQueueLength is the length of the internal queue to the block confirmations manager
	ConfirmationsNotificationQueueLength = ffc("confirmations.notificationQueueLength")
)

var ConnectorPrefix config.Prefix

func setDefaults() {
	viper.SetDefault(string(MonitorPollingInterval), "15m")
	viper.SetDefault(string(ConnectorVariant), "evm")
	viper.SetDefault(string(ConfirmationsRequired), 20)
	viper.SetDefault(string(ConfirmationsBlockCacheSize), 1000)
	viper.SetDefault(string(ConfirmationsBlockPollingInterval), "3s")
	viper.SetDefault(string(ConfirmationsNotificationQueueLength), 50)
}

func Reset() {
	config.RootConfigReset(setDefaults)

	ConnectorPrefix = config.NewPluginConfig("connector")
	ffresty.InitPrefix(ConnectorPrefix)
}
