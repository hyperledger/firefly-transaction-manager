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
	APIEndpointPostEventStream        = ffm("api.endpoints.post.eventstreams", "Create a new event stream")
	APIEndpointPatchEventStream       = ffm("api.endpoints.patch.eventstreams", "Update an existing event stream")
	APIEndpointPostEventStreamSuspend = ffm("api.endpoints.post.eventstream.suspend", "Suspend an event stream")
	APIEndpointPostEventStreamResume  = ffm("api.endpoints.post.eventstream.resume", "Resume an event stream")

	APIParamStreamID = ffm("api.params.streamId", "Event Stream ID")
)
