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

package manager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/wsclient"
	"github.com/hyperledger/firefly/pkg/core"
	"github.com/stretchr/testify/assert"
)

func TestWSChangeDeliveryLookup(t *testing.T) {

	opID := fftypes.NewUUID()
	lookedUp := make(chan struct{})
	toServer, fromServer, wsURL, done := wsclient.NewTestWSServer(
		func(req *http.Request) {
			switch req.URL.Path {
			case `/admin/ws`:
				return
			case fmt.Sprintf("/spi/v1/operations/ns1:%s", opID):
				close(lookedUp)
			default:
				assert.Fail(t, fmt.Sprintf("Unexpected path: %s", req.URL.Path))
			}
		},
	)
	defer done()

	httpURL, err := url.Parse(wsURL)
	assert.NoError(t, err)
	httpURL.Scheme = "http"

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
		httpURL.String(),
	)
	defer cancel()

	m.startWS()

	cmdJSON := <-toServer
	var startCmd core.WSChangeEventCommand
	err = json.Unmarshal([]byte(cmdJSON), &startCmd)
	assert.NoError(t, err)
	assert.Equal(t, core.WSChangeEventCommandTypeStart, startCmd.Type)

	change := &core.ChangeEvent{
		Collection: "operations",
		Type:       core.ChangeEventTypeUpdated,
		Namespace:  "ns1",
		ID:         opID,
	}
	changeJSON, err := json.Marshal(&change)
	assert.NoError(t, err)
	fromServer <- string(changeJSON)

	<-lookedUp

	m.cancelCtx()
	m.waitWSStop()

}

func TestWSConnectFail(t *testing.T) {

	_, m, cancel := newTestManager(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	cancel()

	m.enableChangeListener = true
	err := m.startWS()
	assert.Regexp(t, "FF00154", err)

}
