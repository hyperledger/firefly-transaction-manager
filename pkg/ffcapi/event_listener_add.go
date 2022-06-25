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

package ffcapi

import (
	"github.com/hyperledger/firefly-common/pkg/fftypes"
)

const (
	FromBlockEarliest = "earliest"
	FromBlockLatest   = "latest"
)

type EventListenerOptions struct {
	FromBlock string            // The instruction for the first block to index from (when there is no previous checkpoint). Special "earliest" and "latest" strings should be supported as well as blockchain specific block ID (like a decimal number etc.)
	Filters   []fftypes.JSONAny // The blockchain specific list of filters. The top-level array is an OR list. The semantics within each entry is defined by the blockchain
	Options   *fftypes.JSONAny  // Blockchain specific set of options, such as the first block to detect events from (can be null)
}

type EventListenerVerifyOptionsRequest struct {
	EventListenerOptions
}

type EventListenerVerifyOptionsResponse struct {
	ResolvedSignature string
	ResolvedOptions   fftypes.JSONAny
}

type EventListenerAddRequest struct {
	EventListenerOptions
	ID          *fftypes.UUID          // Unique UUID for the event listener, that should be included in each event
	Name        string                 // Descriptive name of the listener, provided by the user, or defaulted to the signature. Not guaranteed to be unique. Should be included in the event info
	Checkpoint  *fftypes.JSONAny       // The last persisted checkpoint for this event stream
	Done        <-chan struct{}        // Channel that will be closed when the event listener needs to stop - the event listener should stop pushing events
	EventStream chan<- *ListenerUpdate // The event stream to push events to as they are detected, and checkpoints regularly even if there are no events - remember to select on Done as well when pushing events
}

type EventListenerAddResponse struct {
}
