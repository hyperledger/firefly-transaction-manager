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

import (
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"golang.org/x/text/language"
)

var ffm = func(key, translation string) i18n.MessageKey {
	return i18n.FFM(language.AmericanEnglish, key, translation)
}

//revive:disable
var (
	APIEndpointPostRoot                  = ffm("api.endpoints.post.root", "")
	APIEndpointPostEventStream           = ffm("api.endpoints.post.eventstreams", "Create a new event stream")
	APIEndpointPatchEventStream          = ffm("api.endpoints.patch.eventstreams", "Update an existing event stream")
	APIEndpointPostEventStreamSuspend    = ffm("api.endpoints.post.eventstream.suspend", "Suspend an event stream")
	APIEndpointPostEventStreamResume     = ffm("api.endpoints.post.eventstream.resume", "Resume an event stream")
	APIEndpointGetEventStreams           = ffm("api.endpoints.get.eventstreams", "List event streams")
	APIEndpointGetEventStream            = ffm("api.endpoints.get.eventstream", "Get an event stream with status")
	APIEndpointDeleteEventStream         = ffm("api.endpoints.delete.eventstream", "Delete an event stream")
	APIEndpointGetSubscriptions          = ffm("api.endpoints.get.subscriptions", "Get listeners - route deprecated in favor of /eventstreams/{streamId}/listeners")
	APIEndpointGetSubscription           = ffm("api.endpoints.get.subscription", "Get listener - route deprecated in favor of /eventstreams/{streamId}/listeners/{listenerId}")
	APIEndpointPostSubscriptions         = ffm("api.endpoints.post.subscriptions", "Create new listener - route deprecated in favor of /eventstreams/{streamId}/listeners")
	APIEndpointPatchSubscription         = ffm("api.endpoints.patch.subscription", "Update listener - route deprecated in favor of /eventstreams/{streamId}/listeners/{listenerId}")
	APIEndpointDeleteSubscription        = ffm("api.endpoints.delete.subscription", "Delete listener - route deprecated in favor of /eventstreams/{streamId}/listeners/{listenerId}")
	APIEndpointGetEventStreamListeners   = ffm("api.endpoints.get.eventstream.listeners", "List event stream listeners")
	APIEndpointGetEventStreamListener    = ffm("api.endpoints.get.eventstream.listener", "Get event stream listener")
	APIEndpointPostEventStreamListener   = ffm("api.endpoints.post.eventstream.listener", "Create event stream listener")
	APIEndpointPatchEventStreamListener  = ffm("api.endpoints.patch.eventstream.listener", "Update event stream listener")
	APIEndpointDeleteEventStreamListener = ffm("api.endpoints.delete.eventstream.listener", "Delete event stream listener")

	APIParamStreamID      = ffm("api.params.streamId", "Event Stream ID")
	APIParamListenerID    = ffm("api.params.listenerId", "Listener ID")
	APIParamLimit         = ffm("api.params.limit", "Maximum number of entries to return")
	APIParamAfter         = ffm("api.params.after", "Return entries after this ID - for pagination (non-inclusive)")
	APIParamTXSigner      = ffm("api.params.txSigner", "Return only transactions for a specific signing address, in reverse nonce order")
	APIParamTXPending     = ffm("api.params.txPending", "Return only pending transactions, in reverse submission sequence (a 'sequenceId' is assigned to each transaction to determine its sequence")
	APIParamSortDirection = ffm("api.params.sortDirection", "Sort direction: 'asc'/'ascending' or 'desc'/'descending'")
)
