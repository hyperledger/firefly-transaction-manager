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
	DistributionModeBroadcast = fftypes.FFEnumValue("distmode", "broadcast")
	DistributionModeWLD       = fftypes.FFEnumValue("distmode", "workloadDistribution")
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

	BatchSize         *uint64            `ffstruct:"eventstream" json:"batchSize,omitempty"`
	BatchTimeout      *uint64            `ffstruct:"eventstream" json:"batchTimeout,omitempty"`
	ErrorHandling     *ErrorHandlingType `ffstruct:"eventstream" json:"errorHandling,omitempty"`
	RetryTimeout      *uint64            `ffstruct:"eventstream" json:"retryTimeout,omitempty"`
	BlockedRetryDelay *uint64            `ffstruct:"eventstream" json:"blockedRetryDelay,omitempty"`

	DeprecatedBatchTimeoutMS       *uint64 `ffstruct:"eventstream" json:"batchTimeoutMS,omitempty"`       // we now allow duration units like 100ms / 10s
	DeprecatedRetryTimeoutSec      *uint64 `ffstruct:"eventstream" json:"retryTimeoutSec,omitempty"`      // we now allow duration units like 100ms / 10s
	DeprecatedBlockedRetryDelaySec *uint64 `ffstruct:"eventstream" json:"blockedRetryDelaySec,omitempty"` // we now allow duration units like 100ms / 10s

	Webhook   *WebhookConfig     `ffstruct:"eventstream" json:"webhook,omitempty"`
	WebSocket *WebSocketConfig   `ffstruct:"eventstream" json:"websocket,omitempty"`
	Options   fftypes.JSONObject `ffstruct:"eventstream" json:"options,omitempty"`
}

type WebhookConfig struct {
	URL                         string            `ffstruct:"whconfig" json:"url,omitempty"`
	Headers                     map[string]string `ffstruct:"whconfig" json:"headers,omitempty"`
	TLSkipHostVerify            bool              `ffstruct:"whconfig" json:"tlsSkipHostVerify,omitempty"`
	RequestTimeoutSec           uint32            `ffstruct:"whconfig" json:"requestTimeout,omitempty"`
	DeprecatedRequestTimeoutSec uint32            `ffstruct:"whconfig" json:"requestTimeoutSec,omitempty"` // we now allow duration units like 100ms / 10s
}

type WebSocketConfig struct {
	Topic            string           `ffstruct:"wsconfig" json:"topic,omitempty"`
	DistributionMode DistributionMode `ffstruct:"wsconfig" json:"distributionMode,omitempty"`
}

type Listener struct {
	ID        *fftypes.UUID      `ffstruct:"listener" json:"id,omitempty"`
	Name      string             `ffstruct:"listener" json:"name"`
	Stream    string             `ffstruct:"listener" json:"stream"`
	Event     *fftypes.JSONAny   `ffstruct:"listener" json:"event"`
	Options   fftypes.JSONObject `ffstruct:"listener" json:"options"`
	FromBlock string             `ffstruct:"listener" json:"fromBlock,omitempty"`
}

func CheckUpdateString(target **string, old *string, new *string, defValue string) bool {
	if new == nil {
		*target = new
	} else {
		*target = old
	}
	if *target == nil {
		v := defValue
		*target = &v
		return true
	}
	return *old != *new
}

func CheckUpdateBool(target **bool, old *bool, new *bool, defValue bool) bool {
	if new == nil {
		*target = new
	} else {
		*target = old
	}
	if *target == nil {
		v := defValue
		*target = &v
		return true
	}
	return *old != *new
}

func CheckUpdateUint64(target **uint64, old *uint64, new *uint64, defValue uint64) bool {
	if new == nil {
		*target = new
	} else {
		*target = old
	}
	if *target == nil {
		v := defValue
		*target = &v
		return true
	}
	return *old != *new
}

func CheckUpdateEnum(target **fftypes.FFEnum, old *fftypes.FFEnum, new *fftypes.FFEnum, defValue fftypes.FFEnum) bool {
	if new == nil {
		*target = new
	} else {
		*target = old
	}
	if *target == nil {
		v := defValue
		*target = &v
		return true
	}
	return *old != *new
}

func CheckUpdateObject(target *fftypes.JSONObject, old fftypes.JSONObject, new fftypes.JSONObject) bool {
	if new == nil {
		*target = old
		return false
	}
	*target = new
	if old == nil {
		return true
	}
	jsonOld, _ := json.Marshal(old)
	jsonNew, _ := json.Marshal(old)
	return !bytes.Equal(jsonOld, jsonNew)
}
