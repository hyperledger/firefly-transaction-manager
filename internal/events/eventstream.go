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
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
)

type Stream interface {
	AddOrUpdateListener(ctx context.Context, s *fftm.Listener) error       // Add or update a listener
	RemoveListener(ctx context.Context, id *fftypes.UUID) error            // Stop and remove a listener
	UpdateDefinition(ctx context.Context, updates *fftm.EventStream) error // Apply definition updates (if there are changes)
	Definition() *fftm.EventStream                                         // Retrieve the current definition to persist
	Start(ctx context.Context) error                                       // Start delivery
	Stop(ctx context.Context) error                                        // Stop delivery (does not remove checkpoints)
	Delete(ctx context.Context) error                                      // Stop delivery, and clean up any checkpoint
}

type streamState string

const (
	streamStateStarted  = "started"
	streamStateStopping = "stopping"
	streamStateStopped  = "stopped"
	streamStateDeleted  = "deleted"
)

// esDefaults are the defaults for new event streams, read from the config once in InitDefaults()
var esDefaults struct {
	initialized               bool
	batchSize                 int64
	batchTimeout              fftypes.FFDuration
	errorHandling             fftm.ErrorHandlingType
	retryTimeout              fftypes.FFDuration
	blockedRetryDelay         fftypes.FFDuration
	webhookRequestTimeout     fftypes.FFDuration
	websocketDistributionMode fftm.DistributionMode
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

type eventStreamAction interface {
	attemptBatch(ctx context.Context, batchNumber, attempt int, events []*ffcapi.EventWithContext) error
}

type eventStreamBatch struct {
	number         int
	events         []*ffcapi.EventWithContext
	checkpoints    map[fftypes.UUID]*fftypes.JSONAny
	timeoutContext context.Context
	timeoutCancel  func()
}

type eventStream struct {
	bgCtx         context.Context
	spec          *fftm.EventStream
	mux           sync.Mutex
	state         streamState
	connector     ffcapi.API
	persistence   persistence.EventStreamPersistence
	confirmations confirmations.Manager
	listeners     map[fftypes.UUID]*listener
	wsChannels    ws.WebSocketChannels
	retry         *retry.Retry
	startedState  struct {
		ctx           context.Context
		cancelCtx     func()
		action        eventStreamAction
		eventLoopDone chan struct{}
		updates       chan *ffcapi.ListenerUpdate
	}
}

func NewEventStream(
	bgCtx context.Context,
	persistedSpec *fftm.EventStream,
	connector ffcapi.API,
	persistence persistence.EventStreamPersistence,
	confirmations confirmations.Manager,
	wsChannels ws.WebSocketChannels,
) (ees Stream, err error) {
	es := &eventStream{
		bgCtx:         log.WithLogField(bgCtx, "eventstream", persistedSpec.ID.String()),
		state:         streamStateStopped,
		spec:          persistedSpec,
		connector:     connector,
		persistence:   persistence,
		confirmations: confirmations,
		listeners:     make(map[fftypes.UUID]*listener),
		wsChannels:    wsChannels,
		retry:         esDefaults.retry,
	}
	// The configuration we have in memory, applies all the defaults to what is passed in
	// to ensure there are no nil fields on the configuration object.
	if es.spec, _, err = mergeValidateEsConfig(es.bgCtx, nil, persistedSpec); err != nil {
		return nil, err
	}
	return es, nil
}

func (es *eventStream) initAction() {
	ctx := es.startedState.ctx
	switch *es.spec.Type {
	case fftm.EventStreamTypeWebhook:
		es.startedState.action = newWebhookAction(ctx, es.spec.Webhook)
	case fftm.EventStreamTypeWebSocket:
		es.startedState.action = newWebSocketAction(ctx, es.wsChannels, es.spec.WebSocket, *es.spec.Name)
	}
	// mergeValidateEsConfig always be called previous to this
	panic(i18n.NewError(ctx, tmmsgs.MsgInvalidStreamType, *es.spec.Type))
}

func mergeValidateEsConfig(ctx context.Context, base *fftm.EventStream, updates *fftm.EventStream) (merged *fftm.EventStream, changed bool, err error) {

	// Merged is assured to not have any unset values (default set in all cases), or any deprecated fields
	if base == nil {
		base = &fftm.EventStream{}
	}
	merged = &fftm.EventStream{
		ID:      base.ID,
		Created: base.Created,
		Updated: fftypes.Now(),
	}
	if merged.Created == nil || merged.ID == nil {
		merged.Created = merged.Updated
		merged.ID = fftypes.NewUUID()
	}
	// Name (no default - must be set)
	// - Note we do not check for uniqueness of the name at this layer in the code, but we do require unique names.
	//   That's the responsibility of the calling code that manages the persistence of the configured streams.
	changed = fftm.CheckUpdateString(changed, &merged.Name, base.Name, updates.Name, "")
	if *merged.Name == "" {
		return nil, false, i18n.NewError(ctx, tmmsgs.MsgMissingName)
	}

	// Suspended
	changed = fftm.CheckUpdateBool(changed, &merged.Suspended, base.Suspended, updates.Suspended, false)

	// Batch size
	changed = fftm.CheckUpdateUint64(changed, &merged.BatchSize, base.BatchSize, updates.BatchSize, esDefaults.batchSize)

	// Error handling mode
	changed = fftm.CheckUpdateEnum(changed, &merged.ErrorHandling, base.ErrorHandling, updates.ErrorHandling, esDefaults.errorHandling)

	// Batch timeout
	if updates.DeprecatedBatchTimeoutMS != nil {
		dv := fftypes.FFDuration(*updates.DeprecatedBatchTimeoutMS) * fftypes.FFDuration(time.Millisecond)
		changed = fftm.CheckUpdateDuration(changed, &merged.BatchTimeout, base.BatchTimeout, &dv, esDefaults.batchTimeout)
	} else {
		changed = fftm.CheckUpdateDuration(changed, &merged.BatchTimeout, base.BatchTimeout, updates.BatchTimeout, esDefaults.batchTimeout)
	}

	// Retry timeout
	if updates.DeprecatedRetryTimeoutSec != nil {
		dv := fftypes.FFDuration(*updates.DeprecatedRetryTimeoutSec) * fftypes.FFDuration(time.Second)
		changed = fftm.CheckUpdateDuration(changed, &merged.RetryTimeout, base.RetryTimeout, &dv, esDefaults.retryTimeout)
	} else {
		changed = fftm.CheckUpdateDuration(changed, &merged.RetryTimeout, base.RetryTimeout, updates.RetryTimeout, esDefaults.retryTimeout)
	}

	// Blocked retry delay
	if updates.DeprecatedBlockedRetryDelaySec != nil {
		dv := fftypes.FFDuration(*updates.DeprecatedBlockedRetryDelaySec) * fftypes.FFDuration(time.Second)
		changed = fftm.CheckUpdateDuration(changed, &merged.BlockedRetryDelay, base.BlockedRetryDelay, &dv, esDefaults.blockedRetryDelay)
	} else {
		changed = fftm.CheckUpdateDuration(changed, &merged.BlockedRetryDelay, base.BlockedRetryDelay, updates.BlockedRetryDelay, esDefaults.blockedRetryDelay)
	}

	// Type
	changed = fftm.CheckUpdateEnum(changed, &merged.Type, base.Type, updates.Type, fftm.EventStreamTypeWebSocket)
	switch *merged.Type {
	case fftm.EventStreamTypeWebSocket:
		if merged.WebSocket, changed, err = mergeValidateWsConfig(ctx, changed, base.WebSocket, updates.WebSocket); err != nil {
			return nil, false, err
		}
	case fftm.EventStreamTypeWebhook:
		if merged.Webhook, changed, err = mergeValidateWhConfig(ctx, changed, base.Webhook, updates.Webhook); err != nil {
			return nil, false, err
		}
	default:
		return nil, false, i18n.NewError(ctx, tmmsgs.MsgInvalidStreamType, *merged.Type)
	}

	return merged, changed, nil
}

func (es *eventStream) Definition() *fftm.EventStream {
	return es.spec
}

func (es *eventStream) UpdateDefinition(ctx context.Context, updates *fftm.EventStream) error {
	merged, changed, err := mergeValidateEsConfig(ctx, es.spec, updates)
	if err != nil {
		return err
	}

	es.mux.Lock()
	es.spec = merged
	defer es.mux.Unlock()

	if changed {
		if err := es.Stop(ctx); err != nil {
			return i18n.NewError(ctx, tmmsgs.MsgStopFailedUpdatingESConfig)
		}
		if err := es.Start(ctx); err != nil {
			return i18n.NewError(ctx, tmmsgs.MsgStartFailedUpdatingESConfig)
		}
	}

	return nil
}

func (es *eventStream) AddOrUpdateListener(ctx context.Context, s *fftm.Listener) error {
	return nil
}

func (es *eventStream) RemoveListener(ctx context.Context, id *fftypes.UUID) error {
	return nil
}

func (es *eventStream) String() string {
	return es.spec.ID.String()
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

	es.startedState.ctx, es.startedState.cancelCtx = context.WithCancel(es.bgCtx)
	es.initAction()
	es.startedState.eventLoopDone = make(chan struct{})
	es.startedState.updates = make(chan *ffcapi.ListenerUpdate, int(*es.spec.BatchSize))
	go es.eventLoop()
	return nil
}

func (es *eventStream) requestStop(ctx context.Context) ([]*listener, error) {
	es.mux.Lock()
	defer es.mux.Unlock()
	if err := es.checkSetStateLocked(ctx, streamStateStarted, streamStateStopping); err != nil {
		return nil, err
	}
	log.L(ctx).Infof("Stopping event stream %s", es)

	// Cancel the context, stop stop the event loop, and shut down the action (WebSockets in particular)
	es.startedState.cancelCtx()

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
	<-es.startedState.eventLoopDone

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
	if err := es.persistence.DeleteCheckpoint(ctx, es.spec.ID); err != nil {
		return err
	}
	return es.checkSetStateLocked(ctx, streamStateStopped, streamStateDeleted)
}

func (es *eventStream) eventLoop() {
	defer close(es.startedState.eventLoopDone)
	ctx := es.startedState.ctx
	batchTimeout := time.Duration(*es.spec.BatchTimeout)
	maxSize := int(*es.spec.BatchSize)
	batchNumber := 0

	var batch *eventStreamBatch
	for {
		var timeoutContext context.Context
		var timedOut bool
		if batch != nil {
			timeoutContext = batch.timeoutContext
		} else {
			timeoutContext = ctx
		}
		select {
		case update := <-es.startedState.updates:
			if batch == nil {
				batchNumber++
				batch = &eventStreamBatch{
					number:      batchNumber,
					checkpoints: make(map[fftypes.UUID]*fftypes.JSONAny),
				}
				batch.timeoutContext, batch.timeoutCancel = context.WithTimeout(ctx, batchTimeout)
			}
			if update.Checkpoint != nil {
				batch.checkpoints[*update.ListenerID] = update.Checkpoint
			}
			for _, event := range update.Events {
				batch.events = append(batch.events, &ffcapi.EventWithContext{
					StreamID:   es.spec.ID,
					ListenerID: update.ListenerID,
					Event:      *event,
				})
			}
		case <-timeoutContext.Done():
			if batch == nil {
				// The started context exited, we are stopping
				log.L(ctx).Debugf("Event poller exiting")
				return
			}
			// Otherwise we timed out
			timedOut = true
		}

		if batch != nil && (timedOut || len(batch.events) >= maxSize) {
			batch.timeoutCancel()
			err := es.performActionsWithRetry(batch)
			if err == nil {
				err = es.writeCheckpoint(batch)
			}
			if err != nil {
				log.L(ctx).Debugf("Event poller exiting: %s", err)
				return
			}
			batch = nil
		}
	}
}

// performActionWithRetry performs an action, with exponential back-off retry up
// to a given threshold. Only returns error in the case that the context is closed.
func (es *eventStream) performActionsWithRetry(batch *eventStreamBatch) (err error) {
	// We may not have anything to do, if we only had checkpoints in the batch timeout cycle
	if len(batch.events) == 0 {
		return nil
	}

	ctx := es.startedState.ctx
	startTime := time.Now()
	for {
		// Short exponential back-off retry
		err := es.retry.Do(ctx, "action", func(attempt int) (retry bool, err error) {
			err = es.startedState.action.attemptBatch(ctx, batch.number, attempt, batch.events)
			if err != nil {
				log.L(ctx).Errorf("Batch %d attempt %d failed. err=%s",
					batch.number, attempt, err)
				return time.Since(startTime) > time.Duration(*es.spec.RetryTimeout), err
			}
			return false, nil
		})
		if err == nil {
			return nil
		}
		// We're in blocked retry delay
		log.L(ctx).Errorf("Batch failed short retry after %.2fs secs. ErrorHandling=%s BlockedRetryDelay=%.2fs ",
			time.Since(startTime), *es.spec.ErrorHandling, time.Duration(*es.spec.BlockedRetryDelay).Seconds())
		if *es.spec.ErrorHandling == fftm.ErrorHandlingTypeSkip {
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

func (es *eventStream) writeCheckpoint(batch *eventStreamBatch) (err error) {
	// We update the checkpoints (under lock) for all listeners with events in this batch.
	// The last event for any listener in the batch wins.
	es.mux.Lock()
	cp := &persistence.EventStreamCheckpoint{
		StreamID:  es.spec.ID,
		Time:      fftypes.Now(),
		Listeners: make(map[fftypes.UUID]*fftypes.JSONAny),
	}
	for lID, lCP := range batch.checkpoints {
		if l, ok := es.listeners[lID]; ok {
			l.UpdateCheckpoint(lCP)
		}
	}
	for lID, l := range es.listeners {
		cp.Listeners[lID] = l.checkpoint
	}
	es.mux.Unlock()

	// We only return if the context is cancelled, or the checkpoint succeeds
	return es.retry.Do(es.startedState.ctx, "action", func(attempt int) (retry bool, err error) {
		return true, es.persistence.StoreCheckpoint(es.startedState.ctx, cp)
	})
}
