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

package fftm

import (
	"context"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/events"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
)

const (
	startupPaginationLimit = 25
)

func (m *manager) restoreStreams() error {
	var lastInPage *fftypes.UUID
	for {
		streamDefs, err := m.persistence.ListStreams(m.ctx, lastInPage, startupPaginationLimit)
		if err != nil {
			return err
		}
		if len(streamDefs) == 0 {
			break
		}
		for _, def := range streamDefs {
			lastInPage = def.ID
			s, err := events.NewEventStream(m.ctx, def, m.connector, m.persistence, m.confirmations, m.wsChannels)
			if err != nil {
				return err
			}
			m.mux.Lock()
			m.eventStreams[*s.Definition().ID] = s
			m.mux.Unlock()
		}
	}
	return m.restoreListeners()
}

func (m *manager) restoreListeners() error {
	var lastInPage *fftypes.UUID
	for {
		listenerDefs, err := m.persistence.ListListeners(m.ctx, lastInPage, startupPaginationLimit)
		if err != nil {
			return err
		}
		if len(listenerDefs) == 0 {
			break
		}
		for _, def := range listenerDefs {
			lastInPage = def.ID
			m.mux.Lock()
			s := m.eventStreams[*def.StreamID]
			m.mux.Unlock()
			if s != nil {
				if err := s.AddOrUpdateListener(m.ctx, def); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *manager) newStream(ctx context.Context, es *apitypes.EventStream) (*apitypes.EventStream, error) {
	return nil, nil
}
