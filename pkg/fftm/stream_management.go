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
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
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
			if _, err := m.addRuntimeStream(def); err != nil {
				return err
			}
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
			if _, err := m.addRuntimeListener(m.ctx, def); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *manager) deleteAllStreamListeners(ctx context.Context, streamID *fftypes.UUID) error {
	var lastInPage *fftypes.UUID
	for {
		listenerDefs, err := m.persistence.ListStreamListeners(ctx, lastInPage, startupPaginationLimit, streamID)
		if err != nil {
			return err
		}
		if len(listenerDefs) == 0 {
			break
		}
		for _, def := range listenerDefs {
			lastInPage = def.ID
			if err := m.persistence.DeleteListener(ctx, def.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *manager) addRuntimeListener(ctx context.Context, def *apitypes.Listener) (*apitypes.Listener, error) {
	m.mux.Lock()
	s := m.eventStreams[*def.StreamID]
	m.mux.Unlock()
	if s != nil {
		// The definition is updated in-place by the event stream code
		if err := s.AddOrUpdateListener(ctx, def); err != nil {
			return nil, err
		}
	}
	return def, nil
}

func (m *manager) addRuntimeStream(def *apitypes.EventStream) (events.Stream, error) {
	s, err := events.NewEventStream(m.ctx, def, m.connector, m.persistence, m.confirmations, m.wsChannels)
	if err != nil {
		return nil, err
	}
	spec := s.Spec()
	m.mux.Lock()
	m.eventStreams[*spec.ID] = s
	m.mux.Unlock()
	return s, nil
}

func (m *manager) deleteStream(ctx context.Context, idStr string) error {
	id, err := fftypes.ParseUUID(ctx, idStr)
	if err != nil {
		return err
	}
	m.mux.Lock()
	s := m.eventStreams[*id]
	delete(m.eventStreams, *id)
	m.mux.Unlock()
	if s != nil {
		if err := m.deleteAllStreamListeners(ctx, id); err != nil {
			return err
		}
		if err := m.persistence.DeleteStream(ctx, id); err != nil {
			return err
		}
		return s.Delete(ctx)
	}
	return nil
}

func (m *manager) createAndStoreNewStream(ctx context.Context, def *apitypes.EventStream) (*apitypes.EventStream, error) {
	def.ID = nil // set by addRuntimeStream
	def.Created = nil
	s, err := m.addRuntimeStream(def)
	if err != nil {
		return nil, err
	}
	spec := s.Spec()
	err = m.persistence.WriteStream(ctx, spec)
	if err != nil {
		err1 := m.deleteStream(ctx, spec.ID.String())
		log.L(ctx).Infof("Cleaned up runtime stream after write failed (err?=%v)", err1)
		return nil, err
	}
	if !*spec.Suspended {
		if err = s.Start(ctx); err != nil {
			return nil, err
		}
	}
	return spec, nil
}

func (m *manager) updateStream(ctx context.Context, idStr string, updates *apitypes.EventStream) (*apitypes.EventStream, error) {
	id, err := fftypes.ParseUUID(ctx, idStr)
	if err != nil {
		return nil, err
	}
	m.mux.Lock()
	s := m.eventStreams[*id]
	m.mux.Unlock()
	if s == nil {
		return nil, i18n.NewError(ctx, i18n.Msg404NotFound)
	}
	err = s.UpdateSpec(ctx, updates)
	if err != nil {
		return nil, err
	}
	spec := s.Spec()
	err = m.persistence.WriteStream(ctx, spec)
	if err != nil {
		return nil, err
	}
	// We might need to start or stop
	if *spec.Suspended && s.State() != events.StreamStateStopped {
		if err = s.Stop(ctx); err != nil {
			return nil, err
		}
	} else if !*spec.Suspended && s.State() != events.StreamStateStarted {
		if err = s.Start(ctx); err != nil {
			return nil, err
		}
	}
	return spec, nil
}
