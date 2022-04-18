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

var ffe = i18n.FFE

//revive:disable
var (
	MsgInvalidOutputType             = ffe("FF201010", "Invalid output type: %s")
	MsgConnectorError                = ffe("FF201012", "Connector failed request. requestId=%s reason=%s error: %s")
	MsgConnectorInvalidConentType    = ffe("FF201013", "Connector failed request. requestId=%s invalid response content type: %s")
	MsgConnectorFailInvoke           = ffe("FF201014", "Connector failed request. requestId=%s failed to call connector API")
	MsgCacheInitFail                 = ffe("FF201015", "Failed to initialize cache")
	MsgInvalidConfirmationRequest    = ffe("FF201016", "Invalid confirmation request %+v")
	MsgCoreError                     = ffe("FF201017", "Error from core status=%d: %s")
	MsgConfigParamNotSet             = ffe("FF201018", "Configuration parameter '%s' must be set")
	MsgPolicyEngineNotRegistered     = ffe("FF201019", "No policy engine registered with name '%s'")
	MsgNoGasConfigSetForPolicyEngine = ffe("FF201020", "No gas configuration has been set for policy engine")
	MsgErrorQueryingGasStationAPI    = ffe("FF201021", "Error from gas station API [%d]: %s")
	MsgErrorInvalidRequest           = ffe("FF201022", "Invalid request")
	MsgUnsupportedRequestType        = ffe("FF201023", "Unsupported request type: %s")
)
