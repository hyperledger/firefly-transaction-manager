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

package apitypes

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/jsonmap"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
)

type DistributionMode = fftypes.FFEnum

var (
	DistributionModeBroadcast   = fftypes.FFEnumValue("distmode", "broadcast")
	DistributionModeLoadBalance = fftypes.FFEnumValue("distmode", "load_balance")
)

type EventStreamType = fftypes.FFEnum

var (
	EventStreamTypeWebhook   = fftypes.FFEnumValue("estype", "webhook")
	EventStreamTypeWebSocket = fftypes.FFEnumValue("estype", "websocket")
)

type ErrorHandlingType = fftypes.FFEnum

var (
	ErrorHandlingTypeBlock = fftypes.FFEnumValue("ehtype", "block")
	ErrorHandlingTypeSkip  = fftypes.FFEnumValue("ehtype", "skip")
)

type EventStream struct {
	ID        *fftypes.UUID    `ffstruct:"eventstream" json:"id"`
	Created   *fftypes.FFTime  `ffstruct:"eventstream" json:"created"`
	Updated   *fftypes.FFTime  `ffstruct:"eventstream" json:"updated"`
	Name      *string          `ffstruct:"eventstream" json:"name,omitempty"`
	Suspended *bool            `ffstruct:"eventstream" json:"suspended,omitempty"`
	Type      *EventStreamType `ffstruct:"eventstream" json:"type,omitempty" ffenum:"estype"`

	ErrorHandling     *ErrorHandlingType  `ffstruct:"eventstream" json:"errorHandling"`
	BatchSize         *uint64             `ffstruct:"eventstream" json:"batchSize"`
	BatchTimeout      *fftypes.FFDuration `ffstruct:"eventstream" json:"batchTimeout"`
	RetryTimeout      *fftypes.FFDuration `ffstruct:"eventstream" json:"retryTimeout"`
	BlockedRetryDelay *fftypes.FFDuration `ffstruct:"eventstream" json:"blockedRetryDelay"`

	EthCompatBatchTimeoutMS       *uint64 `ffstruct:"eventstream" json:"batchTimeoutMS,omitempty"`       // input only, for backwards compatibility
	EthCompatRetryTimeoutSec      *uint64 `ffstruct:"eventstream" json:"retryTimeoutSec,omitempty"`      // input only, for backwards compatibility
	EthCompatBlockedRetryDelaySec *uint64 `ffstruct:"eventstream" json:"blockedRetryDelaySec,omitempty"` // input only, for backwards compatibility

	Webhook   *WebhookConfig   `ffstruct:"eventstream" json:"webhook,omitempty"`
	WebSocket *WebSocketConfig `ffstruct:"eventstream" json:"websocket,omitempty"`
}

type EventStreamStatus string

const (
	EventStreamStatusStarted  EventStreamStatus = "started"
	EventStreamStatusStopping EventStreamStatus = "stopping"
	EventStreamStatusStopped  EventStreamStatus = "stopped"
	EventStreamStatusDeleted  EventStreamStatus = "deleted"
)

type EventStreamWithStatus struct {
	EventStream
	Status EventStreamStatus `ffstruct:"eventstream" json:"status"`
}

type EventStreamCheckpoint struct {
	StreamID  *fftypes.UUID                    `json:"streamId"`
	Time      *fftypes.FFTime                  `json:"time"`
	Listeners map[fftypes.UUID]json.RawMessage `json:"listeners"`
}

type WebhookConfig struct {
	URL                        *string             `ffstruct:"whconfig" json:"url,omitempty"`
	Headers                    map[string]string   `ffstruct:"whconfig" json:"headers,omitempty"`
	TLSkipHostVerify           *bool               `ffstruct:"whconfig" json:"tlsSkipHostVerify,omitempty"`
	RequestTimeout             *fftypes.FFDuration `ffstruct:"whconfig" json:"requestTimeout,omitempty"`
	EthCompatRequestTimeoutSec *int64              `ffstruct:"whconfig" json:"requestTimeoutSec,omitempty"` // input only, for backwards compatibility
}

type WebSocketConfig struct {
	DistributionMode *DistributionMode `ffstruct:"wsconfig" json:"distributionMode,omitempty"`
}

type Listener struct {
	ID               *fftypes.UUID     `ffstruct:"listener" json:"id,omitempty"`
	Created          *fftypes.FFTime   `ffstruct:"listener" json:"created"`
	Updated          *fftypes.FFTime   `ffstruct:"listener" json:"updated"`
	Name             *string           `ffstruct:"listener" json:"name"`
	StreamID         *fftypes.UUID     `ffstruct:"listener" json:"stream" ffexcludeoutput:"true"`
	EthCompatAddress *string           `ffstruct:"listener" json:"address,omitempty"`
	EthCompatEvent   *fftypes.JSONAny  `ffstruct:"listener" json:"event,omitempty"`
	EthCompatMethods *fftypes.JSONAny  `ffstruct:"listener" json:"methods,omitempty"`
	Filters          []fftypes.JSONAny `ffstruct:"listener" json:"filters"`
	Options          *fftypes.JSONAny  `ffstruct:"listener" json:"options"`
	Signature        string            `ffstruct:"listener" json:"signature,omitempty" ffexcludeinput:"true"`
	FromBlock        *string           `ffstruct:"listener" json:"fromBlock,omitempty"`
}

type ListenerWithStatus struct {
	Listener
	ffcapi.EventListenerHWMResponse
}

type LiveStatus struct {
	ffcapi.LiveResponse
}

type ReadyStatus struct {
	ffcapi.ReadyResponse
}

type LiveAddressBalance struct {
	ffcapi.AddressBalanceResponse
}

// CheckUpdateString helper merges supplied configuration, with a base, and applies a default if unset
func CheckUpdateString(changed bool, merged **string, old *string, new *string, defValue string) bool {
	if new != nil {
		*merged = new
	} else {
		*merged = old
	}
	if *merged == nil {
		v := defValue
		*merged = &v
		return true
	}
	return changed || old == nil || *old != **merged
}

// CheckUpdateBool helper merges supplied configuration, with a base, and applies a default if unset
func CheckUpdateBool(changed bool, merged **bool, old *bool, new *bool, defValue bool) bool {
	if new != nil {
		*merged = new
	} else {
		*merged = old
	}
	if *merged == nil {
		v := defValue
		*merged = &v
		return true
	}
	return changed || old == nil || *old != **merged
}

// CheckUpdateUint64 helper merges supplied configuration, with a base, and applies a default if unset
func CheckUpdateUint64(changed bool, merged **uint64, old *uint64, new *uint64, defValue int64) bool {
	if new != nil {
		*merged = new
	} else {
		*merged = old
	}
	if *merged == nil {
		v := uint64(defValue)
		*merged = &v
		return true
	}
	return changed || old == nil || *old != **merged
}

// CheckUpdateDuration helper merges supplied configuration, with a base, and applies a default if unset
func CheckUpdateDuration(changed bool, merged **fftypes.FFDuration, old *fftypes.FFDuration, new *fftypes.FFDuration, defValue fftypes.FFDuration) bool {
	if new != nil {
		*merged = new
	} else {
		*merged = old
	}
	if *merged == nil {
		v := defValue
		*merged = &v
		return true
	}
	return changed || old == nil || *old != **merged
}

// CheckUpdateEnum helper merges supplied configuration, with a base, and applies a default if unset
func CheckUpdateEnum(changed bool, merged **fftypes.FFEnum, old *fftypes.FFEnum, new *fftypes.FFEnum, defValue fftypes.FFEnum) bool {
	if new != nil {
		*merged = new
	} else {
		*merged = old
	}
	if *merged == nil {
		v := defValue
		*merged = &v
		return true
	}
	return changed || old == nil || *old != **merged
}

// CheckUpdateStringMap helper merges supplied configuration, with a base, and applies a default if unset
func CheckUpdateStringMap(changed bool, merged *map[string]string, old map[string]string, new map[string]string) bool {
	if new != nil {
		*merged = new
		changed = changed || (old == nil)
	} else {
		*merged = old
		return false // new was nil, we cannot have changed
	}
	if changed {
		return true
	}
	// We need to compare otherwise
	jsonOld, _ := json.Marshal(old)
	jsonNew, _ := json.Marshal(new)
	return !bytes.Equal(jsonOld, jsonNew)
}

type EventContext struct {
	StreamID       *fftypes.UUID `json:"streamId"`     // the ID of the event stream for this event
	EthCompatSubID *fftypes.UUID `json:"subId"`        // ID of the listener - EthCompat "subscription" naming
	ListenerName   string        `json:"listenerName"` // name of the listener
}

type EventBatch struct {
	BatchNumber int64               `json:"batchNumber"`
	Events      []*EventWithContext `json:"events"`
}

// EventWithContext is what is delivered
// There is custom serialization to flatten the whole structure, so all the custom `info` fields from the
// connector are alongside the required context fields.
// The `data` is kept separate
type EventWithContext struct {
	StandardContext EventContext
	ffcapi.Event
}

func (e *EventWithContext) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	if e.Info != nil {
		jsonmap.AddJSONFieldsToMap(reflect.ValueOf(e.Info), m)
	}
	jsonmap.AddJSONFieldsToMap(reflect.ValueOf(&e.ID), m)
	jsonmap.AddJSONFieldsToMap(reflect.ValueOf(&e.StandardContext), m)
	m["data"] = e.Data
	return json.Marshal(m)
}

// Note on unmarshal info will be a map with all the fields (except "data")
func (e *EventWithContext) UnmarshalJSON(b []byte) error {
	var m fftypes.JSONObject
	err := json.Unmarshal(b, &m)
	if err == nil && m != nil {
		e.Info = m
		data := m["data"]
		delete(m, "data")
		if data != nil {
			b, _ := json.Marshal(&data)
			e.Data = fftypes.JSONAnyPtrBytes(b)
		}
		err = json.Unmarshal(b, &e.ID)
		if err == nil {
			err = json.Unmarshal(b, &e.StandardContext)
		}
	}
	return err
}
