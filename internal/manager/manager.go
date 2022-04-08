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

	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/log"
)

type Manager interface {
	Start()
	WaitStop()
}

type manager struct {
	ctx          context.Context
	done         chan struct{}
	changeEvents chan *fftypes.ChangeEvent
}

func NewManager(ctx context.Context) Manager {
	return &manager{
		ctx:          ctx,
		changeEvents: make(chan *fftypes.ChangeEvent),
	}
}

func (m *manager) handleEvent(ce *fftypes.ChangeEvent) {
	log.L(m.ctx).Debugf("%s:%s/%s operation event received", ce.Namespace, ce.ID, ce.Type)
}

func (m *manager) managerLoop() {
	defer close(m.done)
	for {
		select {
		case ce := <-m.changeEvents:
			m.handleEvent(ce)
		case <-m.ctx.Done():
			log.L(m.ctx).Infof("Manager exiting")
		}
	}
}

func (m *manager) Start() {
	m.done = make(chan struct{})
	go m.managerLoop()
}

func (m *manager) WaitStop() {
	// Will return when the context has cancelled
	<-m.done
}
