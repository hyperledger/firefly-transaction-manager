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
	"testing"
	"time"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/stretchr/testify/assert"
)

func testESConf(t *testing.T, j string) (es *fftm.EventStream) {
	err := json.Unmarshal([]byte(j), &es)
	assert.NoError(t, err)
	return es
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
			"distributionMode":"loadbalanced"
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

func TestConfigNewMissingName(t *testing.T) {
	tmconfig.Reset()
	InitDefaults()

	_, _, err := mergeValidateEsConfig(context.Background(), nil, testESConf(t, `{}`))
	assert.Regexp(t, "FF21028", err)

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
