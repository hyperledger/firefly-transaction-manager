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

import "github.com/hyperledger/firefly-common/pkg/i18n"

var ffe = i18n.FFE

//revive:disable
var (
	MsgInvalidOutputType             = ffe("FF21010", "Invalid output type: %s")
	MsgConnectorError                = ffe("FF21012", "Connector failed request. requestId=%s reason=%s error: %s")
	MsgConnectorInvalidConentType    = ffe("FF21013", "Connector failed request. requestId=%s invalid response content type: %s")
	MsgCacheInitFail                 = ffe("FF21015", "Failed to initialize cache")
	MsgInvalidConfirmationRequest    = ffe("FF21016", "Invalid confirmation request %+v")
	MsgCoreError                     = ffe("FF21017", "Error from core status=%d: %s")
	MsgConfigParamNotSet             = ffe("FF21018", "Configuration parameter '%s' must be set")
	MsgPolicyEngineNotRegistered     = ffe("FF21019", "No policy engine registered with name '%s'")
	MsgNoGasConfigSetForPolicyEngine = ffe("FF21020", "A fixed gas price must be set when not using a gas oracle")
	MsgErrorQueryingGasOracleAPI     = ffe("FF21021", "Error from gas station API [%d]: %s")
	MsgErrorInvalidRequest           = ffe("FF21022", "Invalid request")
	MsgUnsupportedRequestType        = ffe("FF21023", "Unsupported request type: %s")
	MsgMissingGOTemplate             = ffe("FF21024", "Missing template for processing response from Gas Oracle REST API")
	MsgBadGOTemplate                 = ffe("FF21025", "Invalid Go template: %s")
	MsgGasOracleResultError          = ffe("FF21026", "Error processing result from gas station API via template")
)
