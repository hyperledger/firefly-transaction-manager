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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/persistencemocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/wsmocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func testESConf(t *testing.T, j string) (spec *fftm.EventStream) {
	err := json.Unmarshal([]byte(j), &spec)
	assert.NoError(t, err)
	return spec
}

func newTestEventStream(t *testing.T, conf string) (es *eventStream) {
	tmconfig.Reset()
	config.Set(tmconfig.EventStreamsDefaultsBatchTimeout, "1us")
	InitDefaults()
	ees, err := NewEventStream(context.Background(), testESConf(t, conf),
		&ffcapimocks.API{},
		&persistencemocks.EventStreamPersistence{},
		&confirmationsmocks.Manager{},
		&wsmocks.WebSocketChannels{})
	assert.NoError(t, err)
	return ees.(*eventStream)
}

func mockWSChannels(es *eventStream) (chan interface{}, chan interface{}, chan error) {
	wsc := es.wsChannels.(*wsmocks.WebSocketChannels)
	senderChannel := make(chan interface{}, 1)
	broadcastChannel := make(chan interface{}, 1)
	receiverChannel := make(chan error, 1)
	wsc.On("GetChannels", "ut_stream").Return((chan<- interface{})(senderChannel), (chan<- interface{})(broadcastChannel), (<-chan error)(receiverChannel))
	return senderChannel, broadcastChannel, receiverChannel
}

func TestNewTestEventStreamBadConfig(t *testing.T) {
	tmconfig.Reset()
	InitDefaults()
	_, err := NewEventStream(context.Background(), testESConf(t, `{}`),
		&ffcapimocks.API{},
		&persistencemocks.EventStreamPersistence{},
		&confirmationsmocks.Manager{},
		&wsmocks.WebSocketChannels{})
	assert.Regexp(t, "FF21028", err)
}

func TestConfigNewDefaultsUpdate(t *testing.T) {
	tmconfig.Reset()
	InitDefaults()

	es := testESConf(t, `{
		"name": "test1"
	}`)
	es, changed, err := mergeValidateEsConfig(context.Background(), nil, es)
	assert.NoError(t, err)
	assert.True(t, changed)

	b, err := json.Marshal(&es)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"id":"`+es.ID.String()+`",
		"created":"`+es.Created.String()+`",
		"updated":"`+es.Created.String()+`",
		"batchSize": 50,
		"batchTimeout": "5s",
		"blockedRetryDelay": "30s",
		"errorHandling":"block",
		"name":"test1",
		"retryTimeout":"30s",
		"suspended":false,
		"type":"websocket",
		"websocket": {
			"distributionMode":"load_balance"
		}
	}`, string(b))

	es, changed, err = mergeValidateEsConfig(context.Background(), es, es)
	assert.NoError(t, err)
	assert.False(t, changed)

	es2, changed, err := mergeValidateEsConfig(context.Background(), es, testESConf(t, `{
		"id": "4023945d-ea5d-43aa-ab4f-f39f8c055c7e",`+ /* ignored */ `
		"batchSize": 111,
		"batchTimeoutMS": 222,
		"blockedRetryDelaySec": 333,
		"errorHandling": "skip",
		"name": "test2",
		"retryTimeoutSec": 444,
		"suspended": true,
		"type": "webhook",
		"webhook": {
			"url": "http://test.example.com"
		}
	}`))

	assert.NoError(t, err)
	assert.True(t, changed)

	b, err = json.Marshal(&es2)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"id":"`+es.ID.String()+`",
		"created":"`+es.Created.String()+`",
		"updated":"`+es2.Updated.String()+`",
		"batchSize": 111,
		"batchTimeout": "222ms",
		"blockedRetryDelay": "5m33s",
		"errorHandling":"skip",
		"name":"test2",
		"retryTimeout":"7m24s",
		"suspended":true,
		"type":"webhook",
		"webhook": {
			"tlsSkipHostVerify": false,
			"requestTimeout": "30s",
			"url": "http://test.example.com"
		}
	}`, string(b))

}

func TestConfigNewMissingWebhookConf(t *testing.T) {
	tmconfig.Reset()
	InitDefaults()

	_, _, err := mergeValidateEsConfig(context.Background(), nil, testESConf(t, `{
		"name": "test",
		"type": "webhook",
		"websocket": {}
	}`))
	assert.Regexp(t, "FF21030", err)

}

func TestConfigBadWebSocketDistModeConf(t *testing.T) {
	tmconfig.Reset()
	InitDefaults()

	_, _, err := mergeValidateEsConfig(context.Background(), nil, testESConf(t, `{
		"name": "test",
		"type": "websocket",
		"websocket": {
			"distributionMode":"wrong"
		}
	}`))
	assert.Regexp(t, "FF21034", err)

}

func TestConfigWebSocketBroadcast(t *testing.T) {
	tmconfig.Reset()
	InitDefaults()

	es, _, err := mergeValidateEsConfig(context.Background(), nil, testESConf(t, `{
		"name": "test",
		"type": "websocket",
		"websocket": {
			"distributionMode":"broadcast"
		}
	}`))
	assert.NoError(t, err)
	assert.Equal(t, fftm.DistributionModeBroadcast, *es.WebSocket.DistributionMode)

}

func TestConfigNewBadType(t *testing.T) {
	tmconfig.Reset()
	InitDefaults()

	_, _, err := mergeValidateEsConfig(context.Background(), nil, testESConf(t, `{
		"name": "test",
		"type": "wrong"
	}`))
	assert.Regexp(t, "FF21029", err)

}

func TestConfigNewWebhookRetryMigration(t *testing.T) {
	tmconfig.Reset()
	InitDefaults()

	es, changed, err := mergeValidateEsConfig(context.Background(), nil, testESConf(t, `{
		"name": "test",
		"type": "webhook",
		"webhook": {
			"urL": "http://www.example.com",
			"requestTimeoutSec": 5
		}
	}`))
	assert.NoError(t, err)
	assert.True(t, changed)

	assert.Equal(t, fftypes.FFDuration(5*time.Second), *es.Webhook.RequestTimeout)

}

func TestInitActionBadAction(t *testing.T) {
	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)
	badType := fftm.EventStreamType("wrong")
	es.spec.Type = &badType
	assert.Panics(t, func() {
		es.initAction(&startedStreamState{
			ctx: context.Background(),
		})
	})
}

func TestWebSocketEventStreamsE2EMigrationThenStart(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)

	addr := "0x12345"
	l := &fftm.Listener{
		ID:                fftypes.NewUUID(),
		Name:              "ut_listener",
		DeprecatedAddress: &addr,
		DeprecatedEvent:   fftypes.JSONAnyPtr(`{"event":"definition"}`),
		Options:           fftypes.JSONAnyPtr(`{"option1":"value1"}`),
		FromBlock:         "12345",
	}

	mfc := es.connector.(*ffcapimocks.API)

	mfc.On("EventListenerVerifyOptions", mock.Anything, mock.MatchedBy(func(standard *ffcapi.ListenerOptions) bool {
		return standard.FromBlock == "12345"
	}), mock.MatchedBy(func(customOptions *fftypes.JSONAny) bool {
		return customOptions.JSONObject().GetString("option1") == "value1"
	})).Return(*fftypes.JSONAnyPtr(`{"option1":"value1","option2":"value2"}`), nil)

	started := make(chan *ffcapi.EventListenerAddRequest, 1)
	mfc.On("EventListenerAdd", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerAddRequest) bool {
		started <- r
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerAddResponse{}, ffcapi.ErrorReason(""), nil)

	mfc.On("EventListenerRemove", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerRemoveRequest) bool {
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerRemoveResponse{}, ffcapi.ErrorReason(""), nil)

	msp := es.persistence.(*persistencemocks.EventStreamPersistence)
	msp.On("StoreCheckpoint", mock.Anything, mock.MatchedBy(func(cp *persistence.EventStreamCheckpoint) bool {
		return cp.StreamID.Equals(es.spec.ID) && cp.Listeners[*l.ID].JSONObject().GetString("cp1data") == "stuff"
	})).Return(nil)

	senderChannel, _, receiverChannel := mockWSChannels(es)

	err := es.AddOrUpdateListener(es.bgCtx, l)
	assert.NoError(t, err)

	err = es.Start(es.bgCtx)
	assert.NoError(t, err)

	r := <-started

	assert.JSONEq(t, `{
			"event": {"event":"definition"},
			"address": "0x12345"
		}`, r.Filters[0].String())
	assert.JSONEq(t, `{
			"option1":"value1",
			"option2":"value2"
		}`, r.Options.String())

	r.EventStream <- &ffcapi.ListenerUpdate{
		ListenerID: l.ID,
		Checkpoint: fftypes.JSONAnyPtr(`{"cp1data": "stuff"}`),
		Events: []*ffcapi.Event{
			{
				Data:       fftypes.JSONAnyPtr(`{"k1":"v1"}`),
				ProtocolID: "000000000042/000013/000001",
				Info:       fftypes.JSONAnyPtr(`{"blockNumber":"42","transactionIndex":"13","logIndex":"1"}`),
			},
		},
	}

	batch1 := (<-senderChannel).([]*ffcapi.EventWithContext)
	assert.Len(t, batch1, 1)
	assert.Equal(t, "v1", batch1[0].Data.JSONObject().GetString("k1"))

	receiverChannel <- nil // ack

	err = es.Stop(es.bgCtx)
	assert.NoError(t, err)

	<-r.Done

	mfc.AssertExpectations(t)
}

func TestWebhookEventStreamsE2EAddAfterStart(t *testing.T) {

	receivedWebhook := make(chan []*ffcapi.EventWithContext, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test/path", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("content-type"))
		var events []*ffcapi.EventWithContext
		err := json.NewDecoder(r.Body).Decode(&events)
		assert.NoError(t, err)
		receivedWebhook <- events
	}))
	defer s.Close()

	es := newTestEventStream(t, `{
		"name": "ut_stream",
		"type": "webhook",
		"webhook": {
			"url": "`+fmt.Sprintf("http://%s/test/path", s.Listener.Addr())+`"
		}
	}`)

	l := &fftm.Listener{
		ID:   fftypes.NewUUID(),
		Name: "ut_listener",
		Filters: []fftypes.JSONAny{
			`{"event":"definition1"}`,
			`{"event":"definition2"}`,
		},
		Options:   fftypes.JSONAnyPtr(`{"option1":"value1"}`),
		FromBlock: "12345",
	}

	mfc := es.connector.(*ffcapimocks.API)

	mfc.On("EventListenerVerifyOptions", mock.Anything, mock.MatchedBy(func(standard *ffcapi.ListenerOptions) bool {
		return standard.FromBlock == "12345"
	}), mock.MatchedBy(func(customOptions *fftypes.JSONAny) bool {
		return customOptions.JSONObject().GetString("option1") == "value1"
	})).Return(*fftypes.JSONAnyPtr(`{"option1":"value1","option2":"value2"}`), nil)

	started := make(chan *ffcapi.EventListenerAddRequest, 1)
	mfc.On("EventListenerAdd", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerAddRequest) bool {
		started <- r
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerAddResponse{}, ffcapi.ErrorReason(""), nil)

	mfc.On("EventListenerRemove", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerRemoveRequest) bool {
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerRemoveResponse{}, ffcapi.ErrorReason(""), nil)

	msp := es.persistence.(*persistencemocks.EventStreamPersistence)
	msp.On("StoreCheckpoint", mock.Anything, mock.MatchedBy(func(cp *persistence.EventStreamCheckpoint) bool {
		return cp.StreamID.Equals(es.spec.ID) && cp.Listeners[*l.ID].JSONObject().GetString("cp1data") == "stuff"
	})).Return(nil)

	err := es.AddOrUpdateListener(es.bgCtx, l)
	assert.NoError(t, err)

	err = es.Start(es.bgCtx)
	assert.NoError(t, err)

	r := <-started

	assert.JSONEq(t, `{"event":"definition1"}`, r.Filters[0].String())
	assert.JSONEq(t, `{"event":"definition2"}`, r.Filters[1].String())
	assert.JSONEq(t, `{
			"option1":"value1",
			"option2":"value2"
		}`, r.Options.String())

	r.EventStream <- &ffcapi.ListenerUpdate{
		ListenerID: l.ID,
		Checkpoint: fftypes.JSONAnyPtr(`{"cp1data": "stuff"}`),
		Events: []*ffcapi.Event{
			{
				Data:       fftypes.JSONAnyPtr(`{"k1":"v1"}`),
				ProtocolID: "000000000042/000013/000001",
				Info:       fftypes.JSONAnyPtr(`{"blockNumber":"42","transactionIndex":"13","logIndex":"1"}`),
			},
		},
	}

	batch1 := <-receivedWebhook
	assert.Len(t, batch1, 1)
	assert.Equal(t, "v1", batch1[0].Data.JSONObject().GetString("k1"))

	err = es.Stop(es.bgCtx)
	assert.NoError(t, err)

	<-r.Done

	mfc.AssertExpectations(t)
}

func TestConnectorRejectListener(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)

	l := &fftm.Listener{
		ID:      fftypes.NewUUID(),
		Name:    "ut_listener",
		Filters: []fftypes.JSONAny{`badness`},
	}

	mfc := es.connector.(*ffcapimocks.API)

	mfc.On("EventListenerVerifyOptions", mock.Anything, mock.Anything, mock.Anything).Return(*fftypes.JSONAnyPtr(`{}`), fmt.Errorf("pop"))

	err := es.AddOrUpdateListener(es.bgCtx, l)
	assert.Regexp(t, "FF21040.*pop", err)

	mfc.AssertExpectations(t)
}

func TestUpdateStreamStarted(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)

	l := &fftm.Listener{
		ID:      fftypes.NewUUID(),
		Name:    "ut_listener",
		Filters: []fftypes.JSONAny{`{"event":"definition1"}`},
	}

	mfc := es.connector.(*ffcapimocks.API)

	mfc.On("EventListenerVerifyOptions", mock.Anything, mock.Anything, mock.Anything).Return(*fftypes.JSONAnyPtr(`{}`), nil)

	started := make(chan *ffcapi.EventListenerAddRequest, 1)
	mfc.On("EventListenerAdd", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerAddRequest) bool {
		started <- r
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerAddResponse{}, ffcapi.ErrorReason(""), nil).Twice()

	mfc.On("EventListenerRemove", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerRemoveRequest) bool {
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerRemoveResponse{}, ffcapi.ErrorReason(""), nil).Twice()

	err := es.AddOrUpdateListener(es.bgCtx, l)
	assert.NoError(t, err)

	err = es.Start(es.bgCtx)
	assert.NoError(t, err)

	r := <-started

	defNoChange := testESConf(t, `{
		"name": "ut_stream"
	}`)
	err = es.UpdateDefinition(context.Background(), defNoChange)
	assert.NoError(t, err)

	defChanged := testESConf(t, `{
		"name": "ut_stream2"
	}`)
	err = es.UpdateDefinition(context.Background(), defChanged)
	assert.NoError(t, err)

	assert.Equal(t, "ut_stream2", *es.Definition().Name)

	<-r.Done
	r = <-started

	err = es.Stop(es.bgCtx)
	assert.NoError(t, err)

	<-r.Done

	mfc.AssertExpectations(t)
}

func TestUpdateListenerStarted(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)

	l1 := &fftm.Listener{
		ID:      fftypes.NewUUID(),
		Name:    "ut_listener",
		Filters: []fftypes.JSONAny{`{"event":"definition1"}`},
	}

	mfc := es.connector.(*ffcapimocks.API)

	mfc.On("EventListenerVerifyOptions", mock.Anything, mock.Anything, mock.Anything).Return(*fftypes.JSONAnyPtr(`{}`), nil).Times(3)

	started := make(chan *ffcapi.EventListenerAddRequest, 1)
	mfc.On("EventListenerAdd", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerAddRequest) bool {
		started <- r
		return r.ID.Equals(l1.ID)
	})).Return(&ffcapi.EventListenerAddResponse{}, ffcapi.ErrorReason(""), nil).Twice()

	mfc.On("EventListenerRemove", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerRemoveRequest) bool {
		return r.ID.Equals(l1.ID)
	})).Return(&ffcapi.EventListenerRemoveResponse{}, ffcapi.ErrorReason(""), nil).Twice()

	err := es.AddOrUpdateListener(es.bgCtx, l1)
	assert.NoError(t, err)

	err = es.Start(es.bgCtx)
	assert.NoError(t, err)

	r := <-started

	// Double add the same
	err = es.AddOrUpdateListener(es.bgCtx, l1)
	assert.NoError(t, err)

	l2 := &fftm.Listener{
		ID:      l1.ID,
		Name:    "ut_listener",
		Filters: []fftypes.JSONAny{`{"event":"definition2"}`},
	}

	// Change the event definition
	err = es.AddOrUpdateListener(es.bgCtx, l2)
	assert.NoError(t, err)

	r = <-started

	err = es.Stop(es.bgCtx)
	assert.NoError(t, err)

	<-r.Done

	mfc.AssertExpectations(t)
}

func TestUpdateListenerFail(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)

	l1 := &fftm.Listener{
		ID:      fftypes.NewUUID(),
		Name:    "ut_listener",
		Filters: []fftypes.JSONAny{`{"event":"definition1"}`},
	}

	mfc := es.connector.(*ffcapimocks.API)

	mfc.On("EventListenerVerifyOptions", mock.Anything, mock.Anything, mock.Anything).Return(*fftypes.JSONAnyPtr(`{}`), nil).Times(3)

	started := make(chan *ffcapi.EventListenerAddRequest, 1)
	mfc.On("EventListenerAdd", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerAddRequest) bool {
		started <- r
		return r.ID.Equals(l1.ID)
	})).Return(&ffcapi.EventListenerAddResponse{}, ffcapi.ErrorReason(""), nil).Once()

	mfc.On("EventListenerRemove", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once()
	mfc.On("EventListenerRemove", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerRemoveRequest) bool {
		return r.ID.Equals(l1.ID)
	})).Return(&ffcapi.EventListenerRemoveResponse{}, ffcapi.ErrorReason(""), nil).Once()

	err := es.AddOrUpdateListener(es.bgCtx, l1)
	assert.NoError(t, err)

	err = es.Start(es.bgCtx)
	assert.NoError(t, err)

	r := <-started

	// Double add the same
	err = es.AddOrUpdateListener(es.bgCtx, l1)
	assert.NoError(t, err)

	l2 := &fftm.Listener{
		ID:      l1.ID,
		Name:    "ut_listener",
		Filters: []fftypes.JSONAny{`{"event":"definition2"}`},
	}

	err = es.AddOrUpdateListener(es.bgCtx, l2)
	assert.Regexp(t, "pop", err)

	err = es.Stop(es.bgCtx)
	assert.NoError(t, err)

	<-r.Done

	mfc.AssertExpectations(t)
}

func TestUpdateEventStreamBad(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "old_name"
	}`)

	defNoChange := testESConf(t, `{
		"name": "new_name",
		"type": "wrong"
	}`)
	err := es.UpdateDefinition(context.Background(), defNoChange)
	assert.Regexp(t, "FF21029", err)

	assert.Equal(t, "old_name", *es.Definition().Name)

}

func TestUpdateStreamRestartFail(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)

	l := &fftm.Listener{
		ID:      fftypes.NewUUID(),
		Name:    "ut_listener",
		Filters: []fftypes.JSONAny{`{"event":"definition1"}`},
	}

	mfc := es.connector.(*ffcapimocks.API)

	mfc.On("EventListenerVerifyOptions", mock.Anything, mock.Anything, mock.Anything).Return(*fftypes.JSONAnyPtr(`{}`), nil)

	started := make(chan *ffcapi.EventListenerAddRequest, 1)
	mfc.On("EventListenerAdd", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerAddRequest) bool {
		started <- r
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerAddResponse{}, ffcapi.ErrorReason(""), nil).Once()

	mfc.On("EventListenerAdd", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once()

	mfc.On("EventListenerRemove", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerRemoveRequest) bool {
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerRemoveResponse{}, ffcapi.ErrorReason(""), nil).Twice()

	err := es.AddOrUpdateListener(es.bgCtx, l)
	assert.NoError(t, err)

	err = es.Start(es.bgCtx)
	assert.NoError(t, err)

	r := <-started

	defChanged := testESConf(t, `{
		"name": "ut_stream2"
	}`)
	err = es.UpdateDefinition(context.Background(), defChanged)
	assert.Regexp(t, "FF21032.*pop", err)

	<-r.Done
	r = <-started

	err = es.Stop(es.bgCtx)
	assert.NoError(t, err)

	<-r.Done

	mfc.AssertExpectations(t)
}

func TestUpdateStreamStopFail(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)

	l := &fftm.Listener{
		ID:      fftypes.NewUUID(),
		Name:    "ut_listener",
		Filters: []fftypes.JSONAny{`{"event":"definition1"}`},
	}

	mfc := es.connector.(*ffcapimocks.API)

	mfc.On("EventListenerVerifyOptions", mock.Anything, mock.Anything, mock.Anything).Return(*fftypes.JSONAnyPtr(`{}`), nil)

	started := make(chan *ffcapi.EventListenerAddRequest, 1)
	mfc.On("EventListenerAdd", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerAddRequest) bool {
		started <- r
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerAddResponse{}, ffcapi.ErrorReason(""), nil).Once()

	mfc.On("EventListenerRemove", mock.Anything, mock.Anything).Return(&ffcapi.EventListenerRemoveResponse{}, ffcapi.ErrorReason(""), fmt.Errorf("pop")).Once()

	mfc.On("EventListenerRemove", mock.Anything, mock.MatchedBy(func(r *ffcapi.EventListenerRemoveRequest) bool {
		return r.ID.Equals(l.ID)
	})).Return(&ffcapi.EventListenerRemoveResponse{}, ffcapi.ErrorReason(""), nil).Once()

	err := es.AddOrUpdateListener(es.bgCtx, l)
	assert.NoError(t, err)

	err = es.Start(es.bgCtx)
	assert.NoError(t, err)

	r := <-started

	defChanged := testESConf(t, `{
		"name": "ut_stream2"
	}`)
	err = es.UpdateDefinition(context.Background(), defChanged)
	assert.Regexp(t, "FF21031.*pop", err)

	<-r.Done

	err = es.Stop(es.bgCtx)
	assert.NoError(t, err)

	<-r.Done

	mfc.AssertExpectations(t)
}

func TestStopWhenNotStarted(t *testing.T) {

	es := newTestEventStream(t, `{
		"name": "ut_stream"
	}`)

	err := es.Stop(es.bgCtx)
	assert.Regexp(t, "FF21027", err)

}

func TestSafeCompareFilterListDiffLen(t *testing.T) {
	assert.False(t, safeCompareFilterList([]fftypes.JSONAny{}, []fftypes.JSONAny{`{}`}))
}
