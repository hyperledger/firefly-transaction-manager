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
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hyperledger/firefly-transaction-manager/internal/policyengines"
	"github.com/hyperledger/firefly-transaction-manager/internal/policyengines/simple"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/ffresty"
	"github.com/hyperledger/firefly/pkg/httpserver"
	"github.com/stretchr/testify/assert"
)

func newTestManager(t *testing.T, cAPIHandler http.HandlerFunc, ffCoreHandler http.HandlerFunc) (string, *manager, func()) {
	tmconfig.Reset()
	policyengines.RegisterEngine(tmconfig.PolicyEngineBasePrefix, &simple.PolicyEngineFactory{})

	cAPIServer := httptest.NewServer(cAPIHandler)
	tmconfig.ConnectorPrefix.Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", cAPIServer.Listener.Addr()))

	ffCoreServer := httptest.NewServer(ffCoreHandler)
	tmconfig.FFCorePrefix.Set(ffresty.HTTPConfigURL, fmt.Sprintf("http://%s", ffCoreServer.Listener.Addr()))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	managerPort := strings.Split(ln.Addr().String(), ":")[1]
	ln.Close()
	tmconfig.APIPrefix.Set(httpserver.HTTPConfPort, managerPort)
	tmconfig.APIPrefix.Set(httpserver.HTTPConfAddress, "127.0.0.1")

	config.Set(tmconfig.ManagerName, "unittest")
	config.Set(tmconfig.ReceiptsPollingInterval, "1ms")
	tmconfig.PolicyEngineBasePrefix.SubPrefix("simple").Set(simple.FixedGas, "223344556677")

	mm, err := NewManager(context.Background())
	assert.NoError(t, err)
	m := mm.(*manager)
	m.confirmations = &confirmationsmocks.Manager{}

	return fmt.Sprintf("http://127.0.0.1:%s", managerPort),
		m,
		func() {
			cAPIServer.Close()
			ffCoreServer.Close()
			_ = m.WaitStop()
		}

}
