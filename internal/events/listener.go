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
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

type listener struct {
	es         *eventStream
	spec       *apitypes.Listener
	checkpoint *fftypes.JSONAny
}

func listenerSpecToOptions(spec *apitypes.Listener) ffcapi.EventListenerOptions {
	return ffcapi.EventListenerOptions{
		FromBlock: *spec.FromBlock,
		Filters:   spec.Filters,
		Options:   spec.Options,
	}
}

func (l *listener) stop(startedState *startedStreamState) error {
	_, _, err := l.es.connector.EventListenerRemove(startedState.ctx, &ffcapi.EventListenerRemoveRequest{
		ID: l.spec.ID,
	})
	return err
}

func (l *listener) start(startedState *startedStreamState) error {
	_, _, err := l.es.connector.EventListenerAdd(startedState.ctx, &ffcapi.EventListenerAddRequest{
		EventListenerOptions: listenerSpecToOptions(l.spec),
		Name:                 *l.spec.Name,
		ID:                   l.spec.ID,
		EventStream:          startedState.updates,
		Done:                 startedState.ctx.Done(),
	})
	return err
}
