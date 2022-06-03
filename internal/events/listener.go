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

package events

import (
	"context"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
)

type listener struct {
	definition *fftm.Listener
}

func (l *listener) ID() *fftypes.UUID {
	return l.definition.ID
}

func (l *listener) Start(ctx context.Context) error {
	return nil
}

func (l *listener) RequestStop(ctx context.Context) {

}

func (l *listener) WaitStopped(ctx context.Context) {

}
