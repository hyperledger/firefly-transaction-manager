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
	"encoding/json"
	"sync"
	"time"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-common/pkg/retry"
	"github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/internal/ws"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

type Stream interface {
	AddOrUpdateListener(ctx context.Context, id *fftypes.UUID,
		updates *apitypes.Listener, reset bool) (*apitypes.Listener, error) // Add or update a listener
	RemoveListener(ctx context.Context, id *fftypes.UUID) error          // Stop and remove a listener
	UpdateSpec(ctx context.Context, updates *apitypes.EventStream) error // Apply definition updates (if there are changes)
	Spec() *apitypes.EventStream                                         // Retrieve the merged definition to persist
	Status() apitypes.EventStreamStatus                                  // Get the current status
	Start(ctx context.Context) error                                     // Start delivery
	Stop(ctx context.Context) error                                      // Stop delivery (does not remove checkpoints)
	Delete(ctx context.Context) error                                    // Stop delivery, and clean up any checkpoint
}

// esDefaults are the defaults for new event streams, read from the config once in InitDefaults()
var esDefaults struct {
	initialized               bool
	batchSize                 int64
	batchTimeout              fftypes.FFDuration
	errorHandling             apitypes.ErrorHandlingType
	retryTimeout              fftypes.FFDuration
	blockedRetryDelay         fftypes.FFDuration
	webhookRequestTimeout     fftypes.FFDuration
	websocketDistributionMode apitypes.DistributionMode
	retry                     *retry.Retry
}

func InitDefaults() {
	esDefaults.batchSize = config.GetInt64(tmconfig.EventStreamsDefaultsBatchSize)
	esDefaults.batchTimeout = fftypes.FFDuration(config.GetDuration(tmconfig.EventStreamsDefaultsBatchTimeout))
	esDefaults.errorHandling = fftypes.FFEnum(config.GetString(tmconfig.EventStreamsDefaultsErrorHandling))
	esDefaults.retryTimeout = fftypes.FFDuration(config.GetDuration(tmconfig.EventStreamsDefaultsRetryTimeout))
	esDefaults.blockedRetryDelay = fftypes.FFDuration(config.GetDuration(tmconfig.EventStreamsDefaultsBlockedRetryDelay))
	esDefaults.webhookRequestTimeout = fftypes.FFDuration(config.GetDuration(tmconfig.EventStreamsDefaultsWebhookRequestTimeout))
	esDefaults.websocketDistributionMode = fftypes.FFEnum(config.GetString(tmconfig.EventStreamsDefaultsWebsocketDistributionMode))
	esDefaults.retry = &retry.Retry{
		InitialDelay: config.GetDuration(tmconfig.EventStreamsRetryInitDelay),
		MaximumDelay: config.GetDuration(tmconfig.EventStreamsRetryMaxDelay),
		Factor:       config.GetFloat64(tmconfig.EventStreamsRetryFactor),
	}
}

type eventStreamAction func(ctx context.Context, batchNumber, attempt int, events []*ffcapi.EventWithContext) error

type eventStreamBatch struct {
	number      int
	events      []*ffcapi.EventWithContext
	checkpoints map[fftypes.UUID]ffcapi.EventListenerCheckpoint
	timeout     *time.Timer
}

type startedStreamState struct {
	ctx               context.Context
	cancelCtx         func()
	startTime         *fftypes.FFTime
	action            eventStreamAction
	eventLoopDone     chan struct{}
	batchLoopDone     chan struct{}
	blockListenerDone chan struct{}
	updates           chan *ffcapi.ListenerEvent
	blocks            chan *ffcapi.BlockHashEvent
}

type eventStream struct {
	bgCtx              context.Context
	spec               *apitypes.EventStream
	mux                sync.Mutex
	status             apitypes.EventStreamStatus
	connector          ffcapi.API
	persistence        persistence.Persistence
	confirmations      confirmations.Manager
	listeners          map[fftypes.UUID]*listener
	wsChannels         ws.WebSocketChannels
	retry              *retry.Retry
	currentState       *startedStreamState
	checkpointInterval time.Duration
	batchChannel       chan *ffcapi.ListenerEvent
}

func NewEventStream(
	bgCtx context.Context,
	persistedSpec *apitypes.EventStream,
	connector ffcapi.API,
	persistence persistence.Persistence,
	wsChannels ws.WebSocketChannels,
	initialListeners []*apitypes.Listener,
) (ees Stream, err error) {
	esCtx := log.WithLogField(bgCtx, "eventstream", persistedSpec.ID.String())
	es := &eventStream{
		bgCtx:              esCtx,
		status:             apitypes.EventStreamStatusStopped,
		spec:               persistedSpec,
		connector:          connector,
		persistence:        persistence,
		listeners:          make(map[fftypes.UUID]*listener),
		wsChannels:         wsChannels,
		retry:              esDefaults.retry,
		checkpointInterval: config.GetDuration(tmconfig.EventStreamsCheckpointInterval),
		confirmations:      confirmations.NewBlockConfirmationManager(esCtx, connector),
	}
	// The configuration we have in memory, applies all the defaults to what is passed in
	// to ensure there are no nil fields on the configuration object.
	if es.spec, _, err = mergeValidateEsConfig(esCtx, nil, persistedSpec); err != nil {
		return nil, err
	}
	es.batchChannel = make(chan *ffcapi.ListenerEvent, *es.spec.BatchSize)
	for _, existing := range initialListeners {
		spec, err := es.verifyListenerOptions(esCtx, existing.ID, existing)
		if err != nil {
			return nil, err
		}
		es.listeners[*spec.ID] = &listener{
			es:   es,
			spec: spec,
		}
	}
	log.L(esCtx).Infof("Initialized Event Stream")
	return es, nil
}

func (es *eventStream) initAction(startedState *startedStreamState) {
	ctx := startedState.ctx
	switch *es.spec.Type {
	case apitypes.EventStreamTypeWebhook:
		startedState.action = newWebhookAction(ctx, es.spec.Webhook).attemptBatch
	case apitypes.EventStreamTypeWebSocket:
		startedState.action = newWebSocketAction(es.wsChannels, es.spec.WebSocket, *es.spec.Name).attemptBatch
	default:
		// mergeValidateEsConfig always be called previous to this
		panic(i18n.NewError(ctx, tmmsgs.MsgInvalidStreamType, *es.spec.Type))
	}
}

func mergeValidateEsConfig(ctx context.Context, base *apitypes.EventStream, updates *apitypes.EventStream) (merged *apitypes.EventStream, changed bool, err error) {

	// Merged is assured to not have any unset values (default set in all cases), or any deprecated fields
	if base == nil {
		base = &apitypes.EventStream{}
	}
	merged = &apitypes.EventStream{
		ID:      base.ID,
		Created: base.Created,
		Updated: fftypes.Now(),
	}
	if merged.Created == nil || merged.ID == nil {
		merged.Created = updates.Created
		merged.ID = updates.ID
		if merged.Created == nil {
			merged.Created = merged.Updated
		}
		if merged.ID == nil {
			return nil, false, i18n.NewError(ctx, tmmsgs.MsgMissingID)
		}
	}
	// Name (no default - must be set)
	// - Note we do not check for uniqueness of the name at this layer in the code, but we do require unique names.
	//   That's the responsibility of the calling code that manages the persistence of the configured streams.
	changed = apitypes.CheckUpdateString(changed, &merged.Name, base.Name, updates.Name, "")
	if *merged.Name == "" {
		return nil, false, i18n.NewError(ctx, tmmsgs.MsgMissingName)
	}

	// Suspended
	changed = apitypes.CheckUpdateBool(changed, &merged.Suspended, base.Suspended, updates.Suspended, false)

	// Batch size
	changed = apitypes.CheckUpdateUint64(changed, &merged.BatchSize, base.BatchSize, updates.BatchSize, esDefaults.batchSize)

	// Error handling mode
	changed = apitypes.CheckUpdateEnum(changed, &merged.ErrorHandling, base.ErrorHandling, updates.ErrorHandling, esDefaults.errorHandling)

	// Batch timeout
	if updates.DeprecatedBatchTimeoutMS != nil {
		dv := fftypes.FFDuration(*updates.DeprecatedBatchTimeoutMS) * fftypes.FFDuration(time.Millisecond)
		changed = apitypes.CheckUpdateDuration(changed, &merged.BatchTimeout, base.BatchTimeout, &dv, esDefaults.batchTimeout)
	} else {
		changed = apitypes.CheckUpdateDuration(changed, &merged.BatchTimeout, base.BatchTimeout, updates.BatchTimeout, esDefaults.batchTimeout)
	}

	// Retry timeout
	if updates.DeprecatedRetryTimeoutSec != nil {
		dv := fftypes.FFDuration(*updates.DeprecatedRetryTimeoutSec) * fftypes.FFDuration(time.Second)
		changed = apitypes.CheckUpdateDuration(changed, &merged.RetryTimeout, base.RetryTimeout, &dv, esDefaults.retryTimeout)
	} else {
		changed = apitypes.CheckUpdateDuration(changed, &merged.RetryTimeout, base.RetryTimeout, updates.RetryTimeout, esDefaults.retryTimeout)
	}

	// Blocked retry delay
	if updates.DeprecatedBlockedRetryDelaySec != nil {
		dv := fftypes.FFDuration(*updates.DeprecatedBlockedRetryDelaySec) * fftypes.FFDuration(time.Second)
		changed = apitypes.CheckUpdateDuration(changed, &merged.BlockedRetryDelay, base.BlockedRetryDelay, &dv, esDefaults.blockedRetryDelay)
	} else {
		changed = apitypes.CheckUpdateDuration(changed, &merged.BlockedRetryDelay, base.BlockedRetryDelay, updates.BlockedRetryDelay, esDefaults.blockedRetryDelay)
	}

	// Type
	changed = apitypes.CheckUpdateEnum(changed, &merged.Type, base.Type, updates.Type, apitypes.EventStreamTypeWebSocket)
	switch *merged.Type {
	case apitypes.EventStreamTypeWebSocket:
		if merged.WebSocket, changed, err = mergeValidateWsConfig(ctx, changed, base.WebSocket, updates.WebSocket); err != nil {
			return nil, false, err
		}
	case apitypes.EventStreamTypeWebhook:
		if merged.Webhook, changed, err = mergeValidateWhConfig(ctx, changed, base.Webhook, updates.Webhook); err != nil {
			return nil, false, err
		}
	default:
		return nil, false, i18n.NewError(ctx, tmmsgs.MsgInvalidStreamType, *merged.Type)
	}

	return merged, changed, nil
}

func (es *eventStream) Spec() *apitypes.EventStream {
	return es.spec
}

func (es *eventStream) UpdateSpec(ctx context.Context, updates *apitypes.EventStream) error {
	merged, changed, err := mergeValidateEsConfig(ctx, es.spec, updates)
	if err != nil {
		return err
	}

	es.mux.Lock()
	es.spec = merged
	isStarted := es.status == apitypes.EventStreamStatusStarted
	es.mux.Unlock()

	if changed && isStarted {
		if err := es.Stop(ctx); err != nil {
			return i18n.NewError(ctx, tmmsgs.MsgStopFailedUpdatingESConfig, err)
		}
		if err := es.Start(ctx); err != nil {
			return i18n.NewError(ctx, tmmsgs.MsgStartFailedUpdatingESConfig, err)
		}
	}

	return nil
}

func (es *eventStream) mergeListenerOptions(id *fftypes.UUID, updates *apitypes.Listener) *apitypes.Listener {

	es.mux.Lock()
	l := es.listeners[*id]
	var base *apitypes.Listener
	now := fftypes.Now()
	if l != nil {
		base = l.spec
	} else {
		latest := ffcapi.FromBlockLatest
		base = &apitypes.Listener{
			ID:        id,
			Created:   now,
			FromBlock: &latest,
			StreamID:  es.spec.ID,
		}
	}
	es.mux.Unlock()

	merged := *base

	if updates.Name != nil {
		merged.Name = updates.Name
	}

	if updates.FromBlock != nil {
		merged.FromBlock = updates.FromBlock
	}

	if updates.Options != nil {
		merged.Options = updates.Options
	} else {
		merged.Options = base.Options
	}

	if updates.Filters != nil {
		merged.Filters = updates.Filters
	} else {
		// Allow a single "event" object to be specified instead of a filter, with an optional "address".
		// This is migrated to the new syntax: `"filters":[{"address":"0x1235","event":{...}}]`
		// (only expected to work for the eth connector that supports address/event)
		if updates.DeprecatedEvent != nil {
			migrationFilter := fftypes.JSONObject{
				"event": updates.DeprecatedEvent,
			}
			if updates.DeprecatedAddress != nil {
				migrationFilter["address"] = *updates.DeprecatedAddress
			}
			merged.Filters = []fftypes.JSONAny{fftypes.JSONAny(migrationFilter.String())}
		} else {
			merged.Filters = base.Filters
		}
	}

	return &merged

}

func (es *eventStream) verifyListenerOptions(ctx context.Context, id *fftypes.UUID, updatesOrNew *apitypes.Listener) (*apitypes.Listener, error) {
	// Merge the supplied options with defaults and any existing config.
	spec := es.mergeListenerOptions(id, updatesOrNew)

	// The connector needs to validate the options, building a set of options that are assured to be non-nil
	res, _, err := es.connector.EventListenerVerifyOptions(ctx, &ffcapi.EventListenerVerifyOptionsRequest{
		EventListenerOptions: listenerSpecToOptions(spec),
	})
	if err != nil {
		return nil, i18n.NewError(ctx, tmmsgs.MsgBadListenerOptions, err)
	}

	// We update the spec object in-place for the signature and resolved options
	spec.Signature = res.ResolvedSignature
	spec.Options = &res.ResolvedOptions
	if spec.Name == nil || *spec.Name == "" {
		sig := spec.Signature
		spec.Name = &sig
	}
	log.L(ctx).Infof("Listener %s signature: %s", spec.ID, spec.Signature)
	return spec, nil
}

func (es *eventStream) AddOrUpdateListener(ctx context.Context, id *fftypes.UUID, updates *apitypes.Listener, reset bool) (merged *apitypes.Listener, err error) {
	log.L(ctx).Warnf("Adding/updating listener %s", id)

	// Ask the connector to verify the options, and apply defaults
	spec, err := es.verifyListenerOptions(ctx, id, updates)
	if err != nil {
		return nil, err
	}

	// Do the locked part - which checks if this is a new listener, or just an update to the options.
	new, l, startedState, err := es.lockedListenerUpdate(ctx, spec, reset)
	if err != nil {
		return nil, err
	}
	if reset {
		// Only safe to do the reset with the event stream stopped
		if startedState != nil {
			if err := es.Stop(ctx); err != nil {
				return nil, err
			}
		}
		// Clear out the checkpoint for this listener
		if err := es.resetListenerCheckpoint(ctx, l); err != nil {
			return nil, err
		}
		// Restart if we were started
		if startedState != nil {
			if err := es.Start(ctx); err != nil {
				return nil, err
			}
		}
	} else if new && startedState != nil {
		// Start the new listener - no checkpoint needed here
		return spec, l.start(startedState, nil)
	}
	return spec, nil
}

func (es *eventStream) resetListenerCheckpoint(ctx context.Context, l *listener) error {
	cp, err := es.persistence.GetCheckpoint(ctx, es.spec.ID)
	if err != nil || cp == nil {
		return err
	}
	delete(cp.Listeners, *l.spec.ID)
	return es.persistence.WriteCheckpoint(ctx, cp)
}

func (es *eventStream) lockedListenerUpdate(ctx context.Context, spec *apitypes.Listener, reset bool) (bool, *listener, *startedStreamState, error) {
	es.mux.Lock()
	defer es.mux.Unlock()

	l, exists := es.listeners[*spec.ID]
	switch {
	case exists:
		if spec.Signature != l.spec.Signature {
			// We do not allow the filters to be updated, because that would lead to a confusing situation
			// where the previously emitted events are a subset/mismatch to the filters configured now.
			return false, nil, nil, i18n.NewError(ctx, tmmsgs.MsgFilterUpdateNotAllowed, l.spec.Signature, spec.Signature)
		}
		l.spec = spec
	case reset:
		return false, nil, nil, i18n.NewError(ctx, tmmsgs.MsgResetStreamNotFound, spec.ID, es.spec.ID)
	default:
		l = &listener{
			es:   es,
			spec: spec,
		}
		es.listeners[*spec.ID] = l
	}
	// Take a copy of the current started status, before unlocking
	return !exists, l, es.currentState, nil
}

func (es *eventStream) RemoveListener(ctx context.Context, id *fftypes.UUID) (err error) {
	es.mux.Lock()
	l, exists := es.listeners[*id]
	if !exists {
		log.L(ctx).Warnf("Removing listener not in map: %s", id)
		es.mux.Unlock()
		return nil
	}
	startedState := es.currentState
	delete(es.listeners, *id)
	es.mux.Unlock()

	log.L(ctx).Warnf("Removing listener: %s", id)
	if startedState != nil {
		err = l.stop(startedState)
	}
	return err
}

func (es *eventStream) String() string {
	return es.spec.ID.String()
}

// checkSetStatus - caller must have locked the mux when calling this
func (es *eventStream) checkSetStatus(ctx context.Context, requiredState apitypes.EventStreamStatus, newState ...apitypes.EventStreamStatus) error {
	if es.status != requiredState {
		return i18n.NewError(ctx, tmmsgs.MsgStreamStateError, es.status)
	}
	if len(newState) == 1 {
		es.status = newState[0]
	}
	return nil
}

func (es *eventStream) Start(ctx context.Context) error {
	es.mux.Lock()
	defer es.mux.Unlock()
	if err := es.checkSetStatus(ctx, apitypes.EventStreamStatusStopped, apitypes.EventStreamStatusStarted); err != nil {
		return err
	}
	log.L(ctx).Infof("Starting event stream %s", es)

	startedState := &startedStreamState{
		startTime:         fftypes.Now(),
		eventLoopDone:     make(chan struct{}),
		batchLoopDone:     make(chan struct{}),
		blockListenerDone: make(chan struct{}),
		updates:           make(chan *ffcapi.ListenerEvent, int(*es.spec.BatchSize)),
		blocks:            make(chan *ffcapi.BlockHashEvent), // we promise to consume immediately
	}
	startedState.ctx, startedState.cancelCtx = context.WithCancel(es.bgCtx)
	es.currentState = startedState
	es.initAction(startedState)

	cp, err := es.persistence.GetCheckpoint(ctx, es.spec.ID)
	if err != nil {
		return err
	}

	initialListeners := make([]*ffcapi.EventListenerAddRequest, 0)
	for _, l := range es.listeners {
		initialListeners = append(initialListeners, l.buildAddRequest(ctx, cp))
	}
	_, _, err = es.connector.EventStreamStart(startedState.ctx, &ffcapi.EventStreamStartRequest{
		ID:               es.spec.ID,
		EventStream:      startedState.updates,
		StreamContext:    startedState.ctx,
		BlockListener:    startedState.blocks,
		InitialListeners: initialListeners,
	})
	if err != nil {
		_ = es.checkSetStatus(ctx, apitypes.EventStreamStatusStarted, apitypes.EventStreamStatusStopped)
		return err
	}

	// Kick off the loops
	go es.eventLoop(startedState)
	go es.batchLoop(startedState)
	go es.blockListener(startedState)

	// Start the confirmations manager
	es.confirmations.Start()

	return err
}

func (es *eventStream) requestStop(ctx context.Context) (*startedStreamState, error) {
	es.mux.Lock()
	defer es.mux.Unlock()
	startedState := es.currentState
	if es.status == apitypes.EventStreamStatusStopping {
		// Already stopping, just return
		return startedState, nil
	}
	if err := es.checkSetStatus(ctx, apitypes.EventStreamStatusStarted, apitypes.EventStreamStatusStopping); err != nil {
		return nil, err
	}
	log.L(ctx).Infof("Stopping event stream %s", es)

	// Cancel the context, stop stop the event loop, and shut down the action (WebSockets in particular)
	startedState.cancelCtx()

	return startedState, nil
}

func (es *eventStream) Status() apitypes.EventStreamStatus {
	es.mux.Lock()
	defer es.mux.Unlock()
	return es.status
}

func (es *eventStream) Stop(ctx context.Context) error {

	// Request the stop - this phase is locked, and gives us a safe copy of the listeners array to use outside the lock
	startedState, err := es.requestStop(ctx)
	if err != nil || startedState == nil {
		return err
	}

	// Inform the connector explicitly of the stream stop (it should be shutting down all it's listeners anyway
	// due to the cancelled context)
	if _, _, err = es.connector.EventStreamStopped(ctx, &ffcapi.EventStreamStoppedRequest{
		ID: es.spec.ID,
	}); err != nil {
		log.L(ctx).Errorf("Connector returned error when notified of stopped stream: %s", err)
		return err
	}

	// Stop the confirmations manager
	es.confirmations.Stop()

	// Wait for our event loop to stop
	<-startedState.eventLoopDone

	// Wait for our batch loop to stop
	<-startedState.batchLoopDone

	// Wait for our block listener to stop
	<-startedState.blockListenerDone

	// Transition to stopped (takes the lock again)
	es.mux.Lock()
	es.currentState = nil
	defer es.mux.Unlock()
	return es.checkSetStatus(ctx, apitypes.EventStreamStatusStopping, apitypes.EventStreamStatusStopped)
}

func (es *eventStream) Delete(ctx context.Context) error {
	// Check we are stopped
	if err := es.checkSetStatus(ctx, apitypes.EventStreamStatusStopped); err != nil {
		if err := es.Stop(ctx); err != nil {
			return err
		}
	}
	log.L(ctx).Infof("Deleting event stream %s", es)

	// Hold the lock for the whole of delete, rather than transitioning into a deleting status.
	// If we error out, that way the caller can retry.
	es.mux.Lock()
	defer es.mux.Unlock()
	if err := es.persistence.DeleteCheckpoint(ctx, es.spec.ID); err != nil {
		return err
	}
	return es.checkSetStatus(ctx, apitypes.EventStreamStatusStopped, apitypes.EventStreamStatusDeleted)
}

func (es *eventStream) processNewEvent(ctx context.Context, fev *ffcapi.ListenerEvent) {
	event := fev.Event
	if event == nil || event.ListenerID == nil || fev.Checkpoint == nil {
		log.L(ctx).Warnf("Invalid event from connector: %+v", fev)
		return
	}
	es.mux.Lock()
	l := es.listeners[*fev.Event.ListenerID]
	es.mux.Unlock()
	if l != nil {
		log.L(ctx).Debugf("%s event detected: %s", l.spec.ID, event)
		if es.confirmations == nil {
			// Updates that are just a checkpoint update, go straight to the batch loop.
			// Or if the confirmation manager is disabled.
			// - Note this will block the eventLoop when the event stream is blocked
			es.batchChannel <- fev
		} else {
			// Notify will block, when the confirmation manager is blocked, which per below
			// will flow back from when the event stream is blocked
			err := es.confirmations.Notify(&confirmations.Notification{
				NotificationType: confirmations.NewEventLog,
				Event: &confirmations.EventInfo{
					EventID: event.EventID,
					Confirmed: func(confirmations []confirmations.BlockInfo) {
						// Push it to the batch when confirmed
						// - Note this will block the confirmation manager when the event stream is blocked
						es.batchChannel <- fev
					},
				},
			})
			if err != nil {
				log.L(ctx).Warnf("Failed to notify confirmation manager for event '%s': %s", event, err)
			}
		}
	}
}

func (es *eventStream) processRemovedEvent(ctx context.Context, fev *ffcapi.ListenerEvent) {
	if fev.Event != nil && fev.Event.ListenerID != nil && es.confirmations != nil {
		err := es.confirmations.Notify(&confirmations.Notification{
			NotificationType: confirmations.RemovedEventLog,
			Event: &confirmations.EventInfo{
				EventID: fev.Event.EventID,
			},
		})
		if err != nil {
			log.L(ctx).Warnf("Failed to notify confirmation manager for removed event '%s': %s", fev.Event, err)
		}
	}
}

func (es *eventStream) eventLoop(startedState *startedStreamState) {
	defer close(startedState.eventLoopDone)
	ctx := startedState.ctx

	for {
		select {
		case lev := <-startedState.updates:
			if lev.Removed {
				es.processRemovedEvent(ctx, lev)
			} else {
				es.processNewEvent(ctx, lev)
			}
		case <-ctx.Done():
			log.L(ctx).Debugf("Event loop exiting")
			return
		}
	}
}

// batchLoop receives confirmed events from the confirmation manager,
// batches them together, and drives the actions.
func (es *eventStream) batchLoop(startedState *startedStreamState) {
	defer close(startedState.batchLoopDone)
	ctx := startedState.ctx
	maxSize := int(*es.spec.BatchSize)
	batchNumber := 0

	var batch *eventStreamBatch
	var checkpointTimer = time.NewTimer(es.checkpointInterval)
	for {
		var timeoutChannel <-chan time.Time
		if batch != nil {
			// Once a batch has started, the batch timeout always takes precedence (even if it slows down the checkpoint slightly)
			timeoutChannel = batch.timeout.C
		} else {
			// If we don't have a batch in-flight, then the (longer) checkpoint timer is used
			timeoutChannel = checkpointTimer.C
		}
		timedOut := false
		select {
		case fev := <-es.batchChannel:
			if fev.Event != nil {
				es.mux.Lock()
				l := es.listeners[*fev.Event.ListenerID]
				es.mux.Unlock()
				if l != nil {
					currentCheckpoint := l.checkpoint
					if currentCheckpoint != nil && !currentCheckpoint.LessThan(fev.Checkpoint) {
						// This event is behind the current checkpoint - this is a re-detection.
						// We're perfectly happy to accept re-detections from the connector, as it can be
						// very efficient to batch operations between listeners that cause re-detections.
						// However, we need to protect the application from receiving the re-detections.
						// This loop is the right place for this check, as we are responsible for writing the checkpoints and
						// delivering to the application. So we are the one source of truth.
						log.L(es.bgCtx).Debugf("%s '%s' event re-detected behind checkpoint: %s", l.spec.ID, l.spec.Signature, fev.Event)
						continue
					}

					if batch == nil {
						batchNumber++
						batch = &eventStreamBatch{
							number:      batchNumber,
							timeout:     time.NewTimer(time.Duration(*es.spec.BatchTimeout)),
							checkpoints: make(map[fftypes.UUID]ffcapi.EventListenerCheckpoint),
						}
					}
					if fev.Checkpoint != nil {
						batch.checkpoints[*fev.Event.ListenerID] = fev.Checkpoint
					}

					log.L(es.bgCtx).Debugf("%s '%s' event confirmed: %s", l.spec.ID, l.spec.Signature, fev.Event)
					batch.events = append(batch.events, &ffcapi.EventWithContext{
						StreamID: es.spec.ID,
						Event:    *fev.Event,
					})
				}
			}
		case <-timeoutChannel:
			timedOut = true
			if batch == nil {
				checkpointTimer = time.NewTimer(es.checkpointInterval)
			}
		case <-ctx.Done():
			// The started context exited, we are stopping
			if checkpointTimer != nil {
				checkpointTimer.Stop()
			}
			log.L(ctx).Debugf("Batch loop exiting")
			return
		}

		if timedOut || len(batch.events) >= maxSize {
			var err error
			if batch != nil {
				batch.timeout.Stop()
				err = es.performActionsWithRetry(startedState, batch)
			}
			if err == nil {
				checkpointTimer = time.NewTimer(es.checkpointInterval) // Reset the checkpoint timeout
				err = es.writeCheckpoint(startedState, batch)
			}
			if err != nil {
				log.L(ctx).Debugf("Batch loop exiting: %s", err)
				return
			}
			batch = nil
		}
	}
}

// performActionWithRetry performs an action, with exponential back-off retry up
// to a given threshold. Only returns error in the case that the context is closed.
func (es *eventStream) performActionsWithRetry(startedState *startedStreamState, batch *eventStreamBatch) (err error) {
	// We may not have anything to do, if we only had checkpoints in the batch timeout cycle
	if len(batch.events) == 0 {
		return nil
	}

	ctx := startedState.ctx
	startTime := time.Now()
	for {
		// Short exponential back-off retry
		err := es.retry.Do(ctx, "action", func(attempt int) (retry bool, err error) {
			err = startedState.action(ctx, batch.number, attempt, batch.events)
			if err != nil {
				log.L(ctx).Errorf("Batch %d attempt %d failed. err=%s",
					batch.number, attempt, err)
				return time.Since(startTime) < time.Duration(*es.spec.RetryTimeout), err
			}
			return false, nil
		})
		if err == nil {
			return nil
		}
		// We're in blocked retry delay
		log.L(ctx).Errorf("Batch failed short retry after %.2fs secs. ErrorHandling=%s BlockedRetryDelay=%.2fs ",
			time.Since(startTime).Seconds(), *es.spec.ErrorHandling, time.Duration(*es.spec.BlockedRetryDelay).Seconds())
		if *es.spec.ErrorHandling == apitypes.ErrorHandlingTypeSkip {
			// Swallow the error now we have logged it
			return nil
		}
		select {
		case <-time.After(time.Duration(*es.spec.BlockedRetryDelay)):
		case <-ctx.Done():
			// Only way we exit with error, is if the context is cancelled
			return i18n.NewError(ctx, i18n.MsgContextCanceled)
		}
	}
}

func (es *eventStream) checkUpdateHWMCheckpoint(ctx context.Context, l *listener) ffcapi.EventListenerCheckpoint {

	checkpoint := l.checkpoint

	inFlight := false
	if es.confirmations != nil {
		inFlight = es.confirmations.CheckInFlight(l.spec.ID)
	}

	// If there's in-flight messages in the confirmation manager, we wait for these to be confirmed or purged before
	// writing a checkpoint.
	if inFlight {
		log.L(ctx).Infof("Stale checkpoint for listener '%s' will not be updated as events are in-flight", l.spec.ID)
	} else {
		res, _, err := es.connector.EventListenerHWM(ctx, &ffcapi.EventListenerHWMRequest{
			StreamID:   es.spec.ID,
			ListenerID: l.spec.ID,
		})
		if err != nil {
			log.L(ctx).Errorf("Failed to obtain high watermark checkpoint for listener '%s': %s", l.spec.ID, err)
			return checkpoint
		}
		es.mux.Lock()
		if l.checkpoint == checkpoint /* double check it hasn't changed */ {
			checkpoint = res.Checkpoint
			l.checkpoint = checkpoint
			l.lastCheckpoint = fftypes.Now()
		}
		es.mux.Unlock()
	}

	return checkpoint

}

func (es *eventStream) writeCheckpoint(startedState *startedStreamState, batch *eventStreamBatch) (err error) {
	// We update the checkpoints (under lock) for all listeners with events in this batch.
	// The last event for any listener in the batch wins.
	es.mux.Lock()
	cp := &apitypes.EventStreamCheckpoint{
		StreamID:  es.spec.ID,
		Time:      fftypes.Now(),
		Listeners: make(map[fftypes.UUID]json.RawMessage),
	}
	if batch != nil {
		for lID, lCP := range batch.checkpoints {
			if l, ok := es.listeners[lID]; ok {
				l.checkpoint = lCP
				l.lastCheckpoint = fftypes.Now()
				log.L(es.bgCtx).Tracef("%s (%s) checkpoint: %s", l.spec.Signature, l.spec.ID, lCP)
			}
		}
	}
	staleCheckpoints := make([]*listener, 0)
	for lID, l := range es.listeners {
		cp.Listeners[lID], _ = json.Marshal(l.checkpoint)
		if l.checkpoint == nil || l.lastCheckpoint == nil || time.Since(*l.lastCheckpoint.Time()) > es.checkpointInterval {
			staleCheckpoints = append(staleCheckpoints, l)
		}
	}
	es.mux.Unlock()

	// Ask the connector for any updated high watermark checkpoints - checking we don't have any in-flight confirmations
	for _, l := range staleCheckpoints {
		cpb, _ := json.Marshal(es.checkUpdateHWMCheckpoint(startedState.ctx, l))
		cp.Listeners[*l.spec.ID] = cpb
	}

	// We only return if the context is cancelled, or the checkpoint succeeds
	return es.retry.Do(startedState.ctx, "checkpoint", func(attempt int) (retry bool, err error) {
		return true, es.persistence.WriteCheckpoint(startedState.ctx, cp)
	})
}
