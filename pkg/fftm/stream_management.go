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

package fftm

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/events"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhandler"
)

const (
	startupPaginationLimit = 25
)

type StreamManager interface {
	CreateAndStoreNewStream(ctx context.Context, def *apitypes.EventStream) (*apitypes.EventStream, error)
	GetStreams(ctx context.Context, afterStr, limitStr string) (streams []*apitypes.EventStream, err error)
	GetStream(ctx context.Context, idStr string) (*apitypes.EventStreamWithStatus, error)
	UpdateStream(ctx context.Context, idStr string, updates *apitypes.EventStream) (*apitypes.EventStream, error)
	DeleteStream(ctx context.Context, idStr string) error
}

type ListenerManager interface {
	CreateAndStoreNewListener(ctx context.Context, idStr string, def *apitypes.Listener) (*apitypes.Listener, error)
	GetListeners(ctx context.Context, afterStr, limitStr string) (streams []*apitypes.Listener, err error)
	GetListener(ctx context.Context, streamIDStr, listenerIDStr string) (l *apitypes.ListenerWithStatus, err error)
	UpdateListener(ctx context.Context, streamIDStr, listenerIDStr string, updates *apitypes.Listener, reset bool) (*apitypes.Listener, error)
	DeleteListener(ctx context.Context, streamIDStr, listenerIDStr string) error
}

// Event stream functions
func (m *manager) _restoreStreams() error {
	var lastInPage *fftypes.UUID
	for {
		streamDefs, err := m.persistence.ListStreamsByCreateTime(m.ctx, lastInPage, startupPaginationLimit, txhandler.SortDirectionAscending)
		if err != nil {
			return err
		}
		if len(streamDefs) == 0 {
			break
		}
		for _, def := range streamDefs {
			lastInPage = def.ID
			streamListeners, err := m.persistence.ListStreamListenersByCreateTime(m.ctx, nil, 0, txhandler.SortDirectionAscending, def.ID)
			if err != nil {
				return err
			}
			// check to see if it's already started
			if _, ok := m.eventStreams[*def.ID]; !ok {
				closeoutName, err := m._reserveStreamName(m.ctx, *def.Name, def.ID)
				var s events.Stream
				if err == nil {
					s, err = m._addRuntimeStream(def, streamListeners)
				}
				if err == nil && !*def.Suspended {
					err = s.Start(m.ctx)
				}
				if err != nil {
					return err
				}
				closeoutName(err == nil)
			}
		}
	}
	return nil
}

func (m *manager) _deleteAllStreamListeners(ctx context.Context, streamID *fftypes.UUID) error {
	for {
		// Do not specify after as we just delete everything
		listenerDefs, err := m.persistence.ListStreamListenersByCreateTime(ctx, nil, startupPaginationLimit, txhandler.SortDirectionAscending, streamID)
		if err != nil {
			return err
		}
		if len(listenerDefs) == 0 {
			break
		}
		for _, def := range listenerDefs {
			if err := m.persistence.DeleteListener(ctx, def.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *manager) _addRuntimeStream(def *apitypes.EventStream, listeners []*apitypes.Listener) (events.Stream, error) {
	s, err := events.NewEventStream(m.ctx, def, m.connector, m.persistence, m.wsServer, listeners, m.metricsManager, m.moduleFunctions)
	if err != nil {
		return nil, err
	}
	spec := s.Spec()
	m.mux.Lock()
	m.eventStreams[*spec.ID] = s
	m.mux.Unlock()
	return s, nil
}

func (m *manager) _reserveStreamName(ctx context.Context, name string, id *fftypes.UUID) (func(bool), error) {
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

func (m *manager) CreateAndStoreNewStream(ctx context.Context, def *apitypes.EventStream) (*apitypes.EventStream, error) {
	def.ID = apitypes.NewULID()
	def.Created = nil // set to updated time by events.NewEventStream
	if def.Name == nil || *def.Name == "" {
		return nil, i18n.NewError(ctx, tmmsgs.MsgMissingName)
	}

	stored := false
	closeoutName, err := m._reserveStreamName(ctx, *def.Name, def.ID)
	if err != nil {
		return nil, err
	}
	defer func() { closeoutName(stored) }()

	s, err := m._addRuntimeStream(def, nil /* no listeners when a new stream is first created */)
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

func (m *manager) GetStreams(ctx context.Context, afterStr, limitStr string) (streams []*apitypes.EventStream, err error) {
	after, limit, err := m._parseAfterAndLimit(ctx, afterStr, limitStr)
	if err != nil {
		return nil, err
	}
	return m.persistence.ListStreamsByCreateTime(ctx, after, limit, txhandler.SortDirectionDescending)
}

func (m *manager) GetStream(ctx context.Context, idStr string) (*apitypes.EventStreamWithStatus, error) {
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

func (m *manager) UpdateStream(ctx context.Context, idStr string, updates *apitypes.EventStream) (*apitypes.EventStream, error) {
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
		closeoutName, err := m._reserveStreamName(ctx, *updates.Name, id)
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

func (m *manager) DeleteStream(ctx context.Context, idStr string) error {
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
	if err := m._deleteAllStreamListeners(ctx, id); err != nil {
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

// Stream listener functions

func (m *manager) _createOrUpdateListener(ctx context.Context, id *fftypes.UUID, newOrUpdates *apitypes.Listener, reset bool) (*apitypes.Listener, error) {
	if err := _mergeEthCompatMethods(ctx, newOrUpdates); err != nil {
		return nil, err
	}
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

func (m *manager) createAndStoreNewListenerDeprecated(ctx context.Context, def *apitypes.Listener) (*apitypes.Listener, error) {
	return m._createOrUpdateListener(ctx, apitypes.NewULID(), def, false)
}

func (m *manager) CreateAndStoreNewListener(ctx context.Context, idStr string, def *apitypes.Listener) (*apitypes.Listener, error) {
	streamID, err := fftypes.ParseUUID(ctx, idStr)
	if err != nil {
		return nil, err
	}
	def.StreamID = streamID
	return m.createAndStoreNewListenerDeprecated(ctx, def)
}

func (m *manager) _getListenerSpec(ctx context.Context, streamIDStr, listenerIDStr string) (spec *apitypes.Listener, err error) {
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

func (m *manager) getStreamListenersByCreateTime(ctx context.Context, afterStr, limitStr, idStr string) (streams []*apitypes.Listener, err error) {
	after, limit, err := m._parseAfterAndLimit(ctx, afterStr, limitStr)
	if err != nil {
		return nil, err
	}
	id, err := fftypes.ParseUUID(ctx, idStr)
	if err != nil {
		return nil, err
	}
	return m.persistence.ListStreamListenersByCreateTime(ctx, after, limit, txhandler.SortDirectionDescending, id)
}

func (m *manager) getStreamListenersRich(ctx context.Context, streamID string, filter ffapi.AndFilter) ([]*apitypes.Listener, *ffapi.FilterResult, error) {
	id, err := fftypes.ParseUUID(ctx, streamID)
	if err != nil {
		return nil, nil, err
	}
	return m.persistence.RichQuery().ListStreamListeners(ctx, id, filter)
}

func (m *manager) GetListeners(ctx context.Context, afterStr, limitStr string) (streams []*apitypes.Listener, err error) {
	after, limit, err := m._parseAfterAndLimit(ctx, afterStr, limitStr)
	if err != nil {
		return nil, err
	}
	return m.persistence.ListListenersByCreateTime(ctx, after, limit, txhandler.SortDirectionDescending)
}

func (m *manager) GetListener(ctx context.Context, streamIDStr, listenerIDStr string) (l *apitypes.ListenerWithStatus, err error) {
	spec, err := m._getListenerSpec(ctx, streamIDStr, listenerIDStr)
	if err != nil {
		return nil, err
	}
	l = &apitypes.ListenerWithStatus{Listener: *spec}
	status, _, err := m.connector.EventListenerHWM(ctx, &ffcapi.EventListenerHWMRequest{
		StreamID:   spec.StreamID,
		ListenerID: spec.ID,
	})
	if err == nil {
		l.EventListenerHWMResponse = *status
	} else {
		log.L(ctx).Warnf("Failed to query status for listener %s/%s: %s", spec.StreamID, spec.ID, err)
	}
	return l, nil
}

func (m *manager) UpdateListener(ctx context.Context, streamIDStr, listenerIDStr string, updates *apitypes.Listener, reset bool) (*apitypes.Listener, error) {
	l, err := m._getListenerSpec(ctx, streamIDStr, listenerIDStr) // Verify the listener exists in storage
	if err != nil {
		return nil, err
	}
	updates.StreamID = l.StreamID
	return m._createOrUpdateListener(ctx, l.ID, updates, reset)
}

func (m *manager) DeleteListener(ctx context.Context, streamIDStr, listenerIDStr string) error {
	spec, err := m._getListenerSpec(ctx, streamIDStr, listenerIDStr) // Verify the listener exists in storage
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

// other internal functions

func _mergeEthCompatMethods(ctx context.Context, listener *apitypes.Listener) error {
	if listener.EthCompatMethods != nil {
		if listener.Options == nil {
			listener.Options = fftypes.JSONAnyPtr("{}")
		}
		var optionsMap map[string]interface{}
		if err := listener.Options.Unmarshal(ctx, &optionsMap); err != nil {
			return err
		}
		var methodList []interface{}
		if err := listener.EthCompatMethods.Unmarshal(ctx, &methodList); err != nil {
			return err
		}
		optionsMap["methods"] = methodList
		optionsMap["signer"] = true // the EthCompat support extracts the signer automatically when you choose methods (was just one option)
		b, _ := json.Marshal(optionsMap)
		listener.Options = fftypes.JSONAnyPtrBytes(b)
		listener.EthCompatMethods = nil
	}
	return nil
}

func (m *manager) _parseLimit(ctx context.Context, limitStr string) (limit int, err error) {
	if limitStr != "" {
		if limit, err = strconv.Atoi(limitStr); err != nil {
			return -1, i18n.NewError(ctx, tmmsgs.MsgInvalidLimit, limitStr, err)
		}
	}
	return limit, nil
}

func (m *manager) _parseAfterAndLimit(ctx context.Context, afterStr, limitStr string) (after *fftypes.UUID, limit int, err error) {
	if limit, err = m._parseLimit(ctx, limitStr); err != nil {
		return nil, -1, i18n.NewError(ctx, tmmsgs.MsgInvalidLimit, limitStr, err)
	}
	if afterStr != "" {
		if after, err = fftypes.ParseUUID(ctx, afterStr); err != nil {
			return nil, -1, err
		}
	}
	return after, limit, nil
}
