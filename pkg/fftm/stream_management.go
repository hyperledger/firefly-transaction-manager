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
	"strconv"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/events"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
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
			streamListeners, err := m.persistence.ListStreamListeners(m.ctx, nil, 0, def.ID)
			if err != nil {
				return err
			}
			// check to see if it's already started
			if m.eventStreams[*def.ID] == nil {
				closeoutName, err := m.reserveStreamName(m.ctx, *def.Name, def.ID)
				var s events.Stream
				if err == nil {
					s, err = m.addRuntimeStream(def, streamListeners)
				}
				if err == nil && !*def.Suspended {

					err = s.Start(m.ctx)
				}
				closeoutName(err == nil)
				if err != nil {
					return err
				}
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

func (m *manager) addRuntimeStream(def *apitypes.EventStream, listeners []*apitypes.Listener) (events.Stream, error) {
	s, err := events.NewEventStream(m.ctx, def, m.connector, m.persistence, m.wsServer, listeners)
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
	if s != nil {
		delete(m.streamsByName, *s.Spec().Name)
	}
	m.mux.Unlock()
	if err := m.deleteAllStreamListeners(ctx, id); err != nil {
		return err
	}
	if err := m.persistence.DeleteStream(ctx, id); err != nil {
		return err
	}
	if s != nil {
		return s.Delete(ctx)
	}
	return nil
}

func (m *manager) reserveStreamName(ctx context.Context, name string, id *fftypes.UUID) (func(bool), error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	oldName := ""
	s := m.eventStreams[*id]
	if s != nil {
		oldName = *s.Spec().Name
	}
	existing := m.streamsByName[name]
	if existing != nil {
		if !existing.Equals(id) {
			return nil, i18n.NewError(ctx, tmmsgs.MsgDuplicateStreamName, name, existing)
		}
	}
	m.streamsByName[name] = id

	return func(succeeded bool) {
		// Release the name on failure, but only if it wasn't existing
		if !succeeded && (existing == nil) {
			m.mux.Lock()
			delete(m.streamsByName, name)
			m.mux.Unlock()
		} else if succeeded && oldName != name {
			// Delete the old name on success
			delete(m.streamsByName, oldName)
		}
	}, nil
}

func (m *manager) createAndStoreNewStream(ctx context.Context, def *apitypes.EventStream) (*apitypes.EventStream, error) {
	def.ID = apitypes.UUIDVersion1()
	def.Created = nil // set to updated time by events.NewEventStream
	if def.Name == nil || *def.Name == "" {
		return nil, i18n.NewError(ctx, tmmsgs.MsgMissingName)
	}

	stored := false
	closeoutName, err := m.reserveStreamName(ctx, *def.Name, def.ID)
	if err != nil {
		return nil, err
	}
	defer func() { closeoutName(stored) }()

	s, err := m.addRuntimeStream(def, nil /* no listeners when a new stream is first created */)
	if err != nil {
		return nil, err
	}
	spec := s.Spec()
	err = m.persistence.WriteStream(ctx, spec)
	if err != nil {
		m.mux.Lock()
		delete(m.eventStreams, *def.ID)
		m.mux.Unlock()
		err1 := s.Delete(ctx)
		log.L(ctx).Infof("Cleaned up runtime stream after write failed (err?=%v)", err1)
		return nil, err
	}
	stored = true
	if !*spec.Suspended {
		return spec, s.Start(ctx)
	}
	return spec, nil
}

func (m *manager) createAndStoreNewStreamListener(ctx context.Context, idStr string, def *apitypes.Listener) (*apitypes.Listener, error) {
	streamID, err := fftypes.ParseUUID(ctx, idStr)
	if err != nil {
		return nil, err
	}
	def.StreamID = streamID
	return m.createAndStoreNewListener(ctx, def)
}

func (m *manager) createAndStoreNewListener(ctx context.Context, def *apitypes.Listener) (*apitypes.Listener, error) {
	return m.createOrUpdateListener(ctx, apitypes.UUIDVersion1(), def, false)
}

func (m *manager) updateExistingListener(ctx context.Context, streamIDStr, listenerIDStr string, updates *apitypes.Listener, reset bool) (*apitypes.Listener, error) {
	l, err := m.getListener(ctx, streamIDStr, listenerIDStr) // Verify the listener exists in storage
	if err != nil {
		return nil, err
	}
	updates.StreamID = l.StreamID
	return m.createOrUpdateListener(ctx, l.ID, updates, reset)
}

func (m *manager) createOrUpdateListener(ctx context.Context, id *fftypes.UUID, newOrUpdates *apitypes.Listener, reset bool) (*apitypes.Listener, error) {
	var s events.Stream
	if newOrUpdates.StreamID != nil {
		m.mux.Lock()
		s = m.eventStreams[*newOrUpdates.StreamID]
		m.mux.Unlock()
	}
	if s == nil {
		return nil, i18n.NewError(ctx, tmmsgs.MsgStreamNotFound, newOrUpdates.StreamID)
	}
	def, err := s.AddOrUpdateListener(ctx, id, newOrUpdates, reset)
	if err != nil {
		return nil, err
	}
	if err := m.persistence.WriteListener(ctx, def); err != nil {
		err1 := s.RemoveListener(ctx, def.ID)
		log.L(ctx).Infof("Cleaned up runtime listener after write failed (err?=%v)", err1)
		return nil, err
	}
	return def, nil
}

func (m *manager) deleteListener(ctx context.Context, streamIDStr, listenerIDStr string) error {
	spec, err := m.getListener(ctx, streamIDStr, listenerIDStr) // Verify the listener exists in storage
	if err != nil {
		return err
	}
	m.mux.Lock()
	s := m.eventStreams[*spec.StreamID]
	m.mux.Unlock()
	if s == nil {
		return i18n.NewError(ctx, tmmsgs.MsgStreamNotFound, spec.StreamID)
	}
	if err := s.RemoveListener(ctx, spec.ID); err != nil {
		return err
	}
	return m.persistence.DeleteListener(ctx, spec.ID)
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
		return nil, i18n.NewError(ctx, tmmsgs.MsgStreamNotFound, id)
	}

	nameChanged := false
	if updates.Name != nil && *updates.Name != "" {
		closeoutName, err := m.reserveStreamName(ctx, *updates.Name, id)
		if err != nil {
			return nil, err
		}
		defer func() { closeoutName(nameChanged) }()
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
	nameChanged = true

	// We might need to start or stop
	if *spec.Suspended && s.Status() != apitypes.EventStreamStatusStopped {
		return nil, s.Stop(ctx)
	} else if !*spec.Suspended && s.Status() != apitypes.EventStreamStatusStarted {
		return nil, s.Start(ctx)
	}
	return spec, nil
}

func (m *manager) getStream(ctx context.Context, idStr string) (*apitypes.EventStreamWithStatus, error) {
	id, err := fftypes.ParseUUID(ctx, idStr)
	if err != nil {
		return nil, err
	}
	m.mux.Lock()
	s := m.eventStreams[*id]
	m.mux.Unlock()
	if s == nil {
		return nil, i18n.NewError(ctx, tmmsgs.MsgStreamNotFound, idStr)
	}
	return &apitypes.EventStreamWithStatus{
		EventStream: *s.Spec(),
		Status:      s.Status(),
	}, nil
}

func (m *manager) parseAfterAndLimit(ctx context.Context, afterStr, limitStr string) (after *fftypes.UUID, limit int, err error) {
	if limitStr != "" {
		if limit, err = strconv.Atoi(limitStr); err != nil {
			return nil, -1, i18n.NewError(ctx, tmmsgs.MsgInvalidLimit, limitStr, err)
		}
	}
	if afterStr != "" {
		if after, err = fftypes.ParseUUID(ctx, afterStr); err != nil {
			return nil, -1, err
		}
	}
	return after, limit, nil
}

func (m *manager) getStreams(ctx context.Context, afterStr, limitStr string) (streams []*apitypes.EventStream, err error) {
	after, limit, err := m.parseAfterAndLimit(ctx, afterStr, limitStr)
	if err != nil {
		return nil, err
	}
	return m.persistence.ListStreams(ctx, after, limit)
}

func (m *manager) getListener(ctx context.Context, streamIDStr, listenerIDStr string) (spec *apitypes.Listener, err error) {
	var streamID *fftypes.UUID
	if streamIDStr != "" {
		streamID, err = fftypes.ParseUUID(ctx, streamIDStr)
		if err != nil {
			return nil, err
		}

	}
	listenerID, err := fftypes.ParseUUID(ctx, listenerIDStr)
	if err != nil {
		return nil, err
	}
	spec, err = m.persistence.GetListener(ctx, listenerID)
	if err != nil {
		return nil, err
	}
	// Check we found the listener, and it's owned by the correct stream ID (if we're on a path that specifies a stream ID)
	if spec == nil || (streamID != nil && !streamID.Equals(spec.StreamID)) {
		return nil, i18n.NewError(ctx, tmmsgs.MsgListenerNotFound, listenerID)
	}
	return spec, nil
}

func (m *manager) getListeners(ctx context.Context, afterStr, limitStr string) (streams []*apitypes.Listener, err error) {
	after, limit, err := m.parseAfterAndLimit(ctx, afterStr, limitStr)
	if err != nil {
		return nil, err
	}
	return m.persistence.ListListeners(ctx, after, limit)
}

func (m *manager) getStreamListeners(ctx context.Context, afterStr, limitStr, idStr string) (streams []*apitypes.Listener, err error) {
	after, limit, err := m.parseAfterAndLimit(ctx, afterStr, limitStr)
	if err != nil {
		return nil, err
	}
	id, err := fftypes.ParseUUID(ctx, idStr)
	if err != nil {
		return nil, err
	}
	return m.persistence.ListStreamListeners(ctx, after, limit, id)
}
