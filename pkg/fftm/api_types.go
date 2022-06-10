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
	"bytes"
	"encoding/json"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
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

	DeprecatedBatchTimeoutMS       *uint64 `ffstruct:"eventstream" json:"batchTimeoutMS,omitempty"`       // input only, for backwards compatibility
	DeprecatedRetryTimeoutSec      *uint64 `ffstruct:"eventstream" json:"retryTimeoutSec,omitempty"`      // input only, for backwards compatibility
	DeprecatedBlockedRetryDelaySec *uint64 `ffstruct:"eventstream" json:"blockedRetryDelaySec,omitempty"` // input only, for backwards compatibility

	Webhook   *WebhookConfig   `ffstruct:"eventstream" json:"webhook,omitempty"`
	WebSocket *WebSocketConfig `ffstruct:"eventstream" json:"websocket,omitempty"`
}

type WebhookConfig struct {
	URL                         *string             `ffstruct:"whconfig" json:"url,omitempty"`
	Headers                     map[string]string   `ffstruct:"whconfig" json:"headers,omitempty"`
	TLSkipHostVerify            *bool               `ffstruct:"whconfig" json:"tlsSkipHostVerify,omitempty"`
	RequestTimeout              *fftypes.FFDuration `ffstruct:"whconfig" json:"requestTimeout,omitempty"`
	DeprecatedRequestTimeoutSec *int64              `ffstruct:"whconfig" json:"requestTimeoutSec,omitempty"` // input only, for backwards compatibility
}

type WebSocketConfig struct {
	DistributionMode *DistributionMode `ffstruct:"wsconfig" json:"distributionMode,omitempty"`
}

type Listener struct {
	ID                *fftypes.UUID     `ffstruct:"listener" json:"id,omitempty"`
	Name              string            `ffstruct:"listener" json:"name"`
	Stream            string            `ffstruct:"listener" json:"stream" ffexcludeoutput:"true"`
	DeprecatedAddress *string           `ffstruct:"listener" json:"address,omitempty"`
	DeprecatedEvent   *fftypes.JSONAny  `ffstruct:"listener" json:"event,omitempty"`
	Filters           []fftypes.JSONAny `ffstruct:"listener" json:"filters"`
	Options           *fftypes.JSONAny  `ffstruct:"listener" json:"options"`
	FromBlock         string            `ffstruct:"listener" json:"fromBlock,omitempty"`
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
		*merged = old
		return false
	}
	*merged = new
	if old == nil || changed {
		return true
	}
	jsonOld, _ := json.Marshal(old)
	jsonNew, _ := json.Marshal(old)
	return !bytes.Equal(jsonOld, jsonNew)
}
