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
	"sync"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
)

type EventStreamPersistence interface {
	StoreCheckpoint(ctx context.Context, streamID *fftypes.UUID, listenerID *fftypes.UUID, checkpoint *fftypes.JSONAny) error
	ReadCheckpoint(ctx context.Context, streamID *fftypes.UUID, listenerID *fftypes.UUID) (*fftypes.JSONAny, error)
	DeleteCheckpoint(ctx context.Context, streamID *fftypes.UUID, listenerID *fftypes.UUID) error
}

type EventStream interface {
	AddOrUpdateListener(ctx context.Context, s *fftm.Listener) error                            // Add or update a listener
	RemoveListener(ctx context.Context, id *fftypes.UUID) error                                 // Stop and remove a listener
	UpdateDefinition(ctx context.Context, updates *fftm.EventStream) (*fftm.EventStream, error) // Apply definition updates (if there are changes) and return new object
	Start(ctx context.Context) error                                                            // Start delivery
	Stop(ctx context.Context) error                                                             // Stop delivery (does not remove checkpoints)
	Delete(ctx context.Context) error                                                           // Stop delivery, and clean up any checkpoint
}

type streamState string

const (
	streamStateStarted  = "started"
	streamStateStopping = "stopping"
	streamStateStopped  = "stopped"
	streamStateDeleted  = "deleted"
)

type eventStreamAction interface {
	attemptBatch(batchNumber, attempt uint64, events []*ffcapi.Event) error
}

// eventStreamDefaults are all customizable via the configuration
type eventStreamDefaults struct {
}

type eventStream struct {
	bgCtx           context.Context
	definition      *fftm.EventStream
	mux             sync.Mutex
	state           streamState
	connector       ffcapi.API
	persistence     EventStreamPersistence
	confirmations   confirmations.Manager
	listeners       map[fftypes.UUID]*listener
	action          eventStreamAction
	cancelEventLoop func()
	eventLoopDone   chan struct{}

	eventStreamDefaults
}

func NewEventStream(
	bgCtx context.Context,
	definition *fftm.EventStream,
	connector ffcapi.API,
	persistence EventStreamPersistence,
	confirmations confirmations.Manager,
) (EventStream, error) {
	es := &eventStream{
		bgCtx:         log.WithLogField(bgCtx, "eventstream", definition.ID.String()),
		state:         streamStateStopped,
		definition:    definition,
		connector:     connector,
		persistence:   persistence,
		confirmations: confirmations,
		listeners:     make(map[fftypes.UUID]*listener),
	}
	if err := es.initAction(); err != nil {
		return nil, err
	}
	return es, nil
}

func (es *eventStream) initAction() error {
	// TODO: Implement websocket/webhook
	return nil
}

func (es *eventStream) updateVerify(ctx context.Context, updates *fftm.EventStream) (merged *fftm.EventStream, changed bool, err error) {

	if updates == nil {
		updates = &fftm.EventStream{}
	}
	old := es.definition
	merged = &fftm.EventStream{
		Updated: fftypes.Now(),
		Created: old.Created,
	}

	changed = changed || fftm.CheckUpdateString(&merged.Name, old.Name, updates.Name, "")
	if merged.Name == nil || *merged.Name == "" {
		return nil, false, i18n.NewError(ctx, tmmsgs.MsgMissingName)
	}
	changed = changed || fftm.CheckUpdateBool(&merged.Suspended, old.Suspended, updates.Suspended, false)
	changed = changed || fftm.CheckUpdateEnum(&merged.Type, old.Type, updates.Type, fftm.EventStreamTypeWebSocket)
	changed = changed || fftm.CheckUpdateUint64(&merged.BatchSize, old.BatchSize, updates.BatchSize, fftm.EventStreamTypeWebSocket)

	return merged, changed, nil
}

func (es *eventStream) UpdateDefinition(ctx context.Context, updates *fftm.EventStream) (*fftm.EventStream, error) {
	merged, changed, err := es.updateVerify(ctx, updates)
	if !changed || err != nil {
		return es.definition, err
	}

	es.mux.Lock()
	es.definition = merged
	defer es.mux.Unlock()

	return es.definition, nil
}

func (es *eventStream) AddOrUpdateListener(ctx context.Context, s *fftm.Listener) error {
	return nil
}

func (es *eventStream) RemoveListener(ctx context.Context, id *fftypes.UUID) error {
	return nil
}

func (es *eventStream) String() string {
	return es.definition.ID.String()
}

func (es *eventStream) checkSetState(ctx context.Context, requiredState streamState, newState ...streamState) error {
	es.mux.Lock()
	defer es.mux.Unlock()
	return es.checkSetStateLocked(ctx, requiredState, newState...)
}

func (es *eventStream) checkSetStateLocked(ctx context.Context, requiredState streamState, newState ...streamState) error {
	if es.state != requiredState {
		return i18n.NewError(ctx, tmmsgs.MsgStreamStateError, es.state)
	}
	if len(newState) == 1 {
		es.state = newState[0]
	}
	return nil
}

func (es *eventStream) Start(ctx context.Context) error {
	es.mux.Lock()
	defer es.mux.Unlock()
	if err := es.checkSetStateLocked(ctx, streamStateStopped, streamStateStarted); err != nil {
		return err
	}
	log.L(ctx).Infof("Starting event stream %s", es)

	elCtx, cancelELCtx := context.WithCancel(es.bgCtx)
	es.cancelEventLoop = cancelELCtx
	es.eventLoopDone = make(chan struct{})
	go es.eventLoop(elCtx)
	return nil
}

func (es *eventStream) requestStop(ctx context.Context) ([]*listener, error) {
	es.mux.Lock()
	defer es.mux.Unlock()
	if err := es.checkSetStateLocked(ctx, streamStateStarted, streamStateStopping); err != nil {
		return nil, err
	}
	log.L(ctx).Infof("Stopping event stream %s", es)

	// Cancel the event loop
	es.cancelEventLoop()

	// Stop all the listeners - we hold the lock during this
	listeners := make([]*listener, 0, len(es.listeners))
	for _, l := range es.listeners {
		l.RequestStop(ctx)
		listeners = append(listeners, l)
	}
	return listeners, nil
}

func (es *eventStream) Stop(ctx context.Context) error {

	// Request the stop - this phase is locked, and gives us a safe copy of the listeners array to use outside the lock
	listeners, err := es.requestStop(ctx)
	if err != nil {
		return err
	}

	// Wait for each listener to stop
	for _, l := range listeners {
		l.WaitStopped(ctx)
	}

	// Wait for our event loop to stop
	<-es.eventLoopDone

	// Transition to stopped (takes the lock again)
	return es.checkSetState(ctx, streamStateStopping, streamStateStopped)
}

func (es *eventStream) Delete(ctx context.Context) error {
	// Check we are stopped
	if err := es.checkSetState(ctx, streamStateStopped); err != nil {
		if err := es.Stop(ctx); err != nil {
			return err
		}
	}
	log.L(ctx).Infof("Deleting event stream %s", es)

	// Hold the lock for the whole of delete, rather than transitioning into a deleting state.
	// If we error out, that way the caller can retry.
	es.mux.Lock()
	defer es.mux.Unlock()
	for _, l := range es.listeners {
		if err := es.persistence.DeleteCheckpoint(ctx, es.definition.ID, l.ID()); err != nil {
			return err
		}
	}
	return es.checkSetStateLocked(ctx, streamStateStopped, streamStateDeleted)
}

func (es *eventStream) eventLoop(ctx context.Context) {

}
