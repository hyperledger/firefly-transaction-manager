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

package simple

import (
	"net/http"

	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/ffresty"
)

const (
	FixedGasPrice           = "fixedGasPrice" // when not using a gas station - will be treated as a raw JSON string, so can be numeric 123, or string "123", or object {"maxPriorityFeePerGas":123})
	WarnInterval            = "warnInterval"  // warnings will be written to the log at this interval if mining has not occurred
	GasStationPrefix        = "gasstation"
	GasStationMethod        = "method"
	GasStationEnabled       = "enabled"
	GasStationGJSON         = "gjson" // executes a GJSON query against the returned output: https://github.com/tidwall/gjson/blob/master/SYNTAX.md
	GasStationQueryInterval = "queryInterval"
)

const (
	defaultWarnInterval            = "15m"
	defaultGasStationMethod        = http.MethodGet
	defaultGasStationEnabled       = false
	defaultGasStationQueryInterval = "5m"
)

func (f *PolicyEngineFactory) InitPrefix(prefix config.Prefix) {
	prefix.AddKnownKey(FixedGasPrice)
	prefix.AddKnownKey(WarnInterval, defaultWarnInterval)

	gasStationPrefix := prefix.SubPrefix(GasStationPrefix)
	ffresty.InitPrefix(gasStationPrefix)
	gasStationPrefix.AddKnownKey(GasStationMethod, defaultGasStationMethod)
	gasStationPrefix.AddKnownKey(GasStationEnabled, defaultGasStationEnabled)
	gasStationPrefix.AddKnownKey(GasStationQueryInterval, defaultGasStationQueryInterval)
	gasStationPrefix.AddKnownKey(GasStationGJSON)

}
