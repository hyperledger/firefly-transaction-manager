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

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/ffresty"
)

const (
	FixedGasPrice          = "fixedGasPrice" // when not using a gas station - will be treated as a raw JSON string, so can be numeric 123, or string "123", or object {"maxPriorityFeePerGas":123})
	WarnInterval           = "warnInterval"  // warnings will be written to the log at this interval if mining has not occurred
	GasOraclePrefix        = "gasOracle"
	GasOracleMode          = "mode"
	GasOracleMethod        = "method"
	GasOracleTemplate      = "template"
	GasOracleQueryInterval = "queryInterval"
)

const (
	GasOracleModeDisabled  = "disabled"
	GasOracleModeRESTAPI   = "restapi"
	GasOracleModeConnector = "connector"
)

const (
	defaultWarnInterval           = "15m"
	defaultGasOracleQueryInterval = "5m"
	defaultGasOracleMethod        = http.MethodGet
	defaultGasOracleMode          = GasOracleModeDisabled
)

func (f *PolicyEngineFactory) InitPrefix(prefix config.Prefix) {
	prefix.AddKnownKey(FixedGasPrice)
	prefix.AddKnownKey(WarnInterval, defaultWarnInterval)

	gasOraclePrefix := prefix.SubPrefix(GasOraclePrefix)
	ffresty.InitPrefix(gasOraclePrefix)
	gasOraclePrefix.AddKnownKey(GasOracleMethod, defaultGasOracleMethod)
	gasOraclePrefix.AddKnownKey(GasOracleMode, defaultGasOracleMode)
	gasOraclePrefix.AddKnownKey(GasOracleQueryInterval, defaultGasOracleQueryInterval)
	gasOraclePrefix.AddKnownKey(GasOracleTemplate)

}
