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
	"context"
	"encoding/json"

	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/log"
	"github.com/hyperledger/firefly/pkg/wsclient"
)

func (m *manager) startChangeListener(ctx context.Context, w wsclient.WSClient) error {
	cmd := fftypes.WSChangeEventCommand{
		Type:        fftypes.WSChangeEventCommandTypeStart,
		Collections: []string{"operations"},
		Filter: fftypes.ChangeEventFilter{
			Types: []fftypes.ChangeEventType{fftypes.ChangeEventTypeUpdated},
		},
	}
	b, _ := json.Marshal(&cmd)
	log.L(m.ctx).Infof("Change listener connected. Sent: %s", b)
	return w.Send(ctx, b)
}

func (m *manager) handleEvent(ce *fftypes.ChangeEvent) {
	log.L(m.ctx).Debugf("%s:%s/%s operation change event received", ce.Namespace, ce.ID, ce.Type)
	if ce.Collection == "operations" && ce.Type == fftypes.ChangeEventTypeUpdated {
		m.mux.Lock()
		_, knownID := m.pendingOpsByID[*ce.ID]
		m.mux.Unlock()
		if !knownID {
			// Currently the only action taken for change events, is to check we are
			// tracking the transaction. However, as only transactions we submitted are
			// valid and we do a full query on startup - the change listener is a little
			// redundant (and disabled by default)
			m.queryAndAddPending(ce.ID)
		}
	}
}

func (m *manager) changeEventLoop() {
	defer close(m.changeEventLoopDone)
	for {
		select {
		case b := <-m.wsClient.Receive():
			var ce *fftypes.ChangeEvent
			_ = json.Unmarshal(b, &ce)
			m.handleEvent(ce)
		case <-m.ctx.Done():
			log.L(m.ctx).Infof("Change event loop exiting")
			return
		}
	}
}

func (m *manager) startWS() error {
	if m.enableChangeListener {
		m.changeEventLoopDone = make(chan struct{})
		if err := m.wsClient.Connect(); err != nil {
			return err
		}
		go m.changeEventLoop()
	}
	return nil
}

func (m *manager) waitWSStop() {
	if m.changeEventLoopDone != nil {
		<-m.changeEventLoopDone
	}
}
