// Copyright © 2024 Kaleido, Inc.
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

type EventListenerHWMRequest struct {
	StreamID   *fftypes.UUID `json:"streamId"`
	ListenerID *fftypes.UUID `json:"listenerId"`
}

type EventListenerHWMResponse struct {
	Checkpoint EventListenerCheckpoint `json:"checkpoint"`
	Catchup    bool                    `json:"catchup,omitempty"` // informational only - informs an operator that the stream is catching up
	Synced     bool                    `json:"synced,omitempty"`  // derived from the distance of the checkpoint block from chain head
}
