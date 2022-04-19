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

	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/log"
)

type lockedNonce struct {
	m        *manager
	opID     *fftypes.UUID
	signer   string
	unlocked chan struct{}
	nonce    uint64
	spent    *fftm.ManagedTXOutput
}

// complete must be called for any lockedNonce returned from a successful assignAndLockNonce call
func (ln *lockedNonce) complete(ctx context.Context) {
	if ln.spent != nil {
		log.L(ctx).Debugf("Next nonce %d for signer %s spent", ln.nonce, ln.signer)
		ln.m.trackManaged(ln.spent)
	} else {
		log.L(ctx).Debugf("Returning next nonce %d for signer %s unspent", ln.nonce, ln.signer)
		// Do not
	}
	ln.m.mux.Lock()
	delete(ln.m.lockedNonces, ln.signer)
	close(ln.unlocked)
	ln.m.mux.Unlock()
}

func (m *manager) assignAndLockNonce(ctx context.Context, opID *fftypes.UUID, signer string) (*lockedNonce, error) {

	for {
		// Take the lock to query our nonce cache, and check if we are already locked
		m.mux.Lock()
		doLookup := false
		locked, isLocked := m.lockedNonces[signer]
		if !isLocked {
			locked = &lockedNonce{
				m:        m,
				opID:     opID,
				signer:   signer,
				unlocked: make(chan struct{}),
			}
			m.lockedNonces[signer] = locked
			// We might know the highest nonce straight away
			nextNonce, nonceCached := m.nextNonces[signer]
			if nonceCached {
				locked.nonce = nextNonce
				log.L(ctx).Debugf("Locking next nonce %d from cache for signer %s", locked.nonce, signer)
				// We can return the nonce to use without any query
				m.mux.Unlock()
				return locked, nil
			}
			// Otherwise, defer a lookup to outside of the mutex
			doLookup = true
		}
		m.mux.Unlock()

		// If we're locked, then wait
		if isLocked {
			log.L(ctx).Debugf("Contention for next nonce for signer %s", signer)
			<-locked.unlocked
		} else if doLookup {
			// We have to ensure we either successfully return a nonce,
			// or otherwise we unlock when we send the error
			nextNonceRes, _, err := m.connectorAPI.GetNextNonce(ctx, &ffcapi.GetNextNonceRequest{
				Signer: signer,
			})
			if err != nil {
				close(locked.unlocked)
				return nil, err
			}
			nextNonce := nextNonceRes.Nonce.Uint64()
			m.mux.Lock()
			m.nextNonces[signer] = nextNonce
			locked.nonce = nextNonce
			m.mux.Unlock()
			return locked, nil
		}
	}

}
