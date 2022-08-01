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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/httpserver"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/persistencemocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengines"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengines/simple"
	"github.com/hyperledger/firefly/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testManagerName = "unittest"

func strPtr(s string) *string { return &s }

func newTestManager(t *testing.T) (string, *manager, func()) {
	InitConfig()
	policyengines.RegisterEngine(&simple.PolicyEngineFactory{})
	dir, err := ioutil.TempDir("", "ldb_*")
	assert.NoError(t, err)
	config.Set(tmconfig.PersistenceLevelDBPath, dir)
	tmconfig.PolicyEngineBaseConfig.SubSection("simple").SubSection(simple.GasOracleConfig).Set(simple.GasOracleMode, simple.GasOracleModeDisabled)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	managerPort := strings.Split(ln.Addr().String(), ":")[1]
	ln.Close()
	tmconfig.APIConfig.Set(httpserver.HTTPConfPort, managerPort)
	tmconfig.APIConfig.Set(httpserver.HTTPConfAddress, "127.0.0.1")

	config.Set(tmconfig.PolicyLoopInterval, "1ms")
	tmconfig.PolicyEngineBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	mm, err := NewManager(context.Background(), &ffcapimocks.API{})
	assert.NoError(t, err)
	m := mm.(*manager)
	mcm := &confirmationsmocks.Manager{}
	mcm.On("Start").Return().Maybe()
	m.confirmations = mcm

	return fmt.Sprintf("http://127.0.0.1:%s", managerPort),
		m,
		func() {
			m.Close()
			os.RemoveAll(dir)
		}

}

func newMockPersistenceManager(t *testing.T) (*persistencemocks.Persistence, *ffcapimocks.API, *manager) {
	InitConfig()
	mca := &ffcapimocks.API{}
	mps := &persistencemocks.Persistence{}
	m := newManager(context.Background(), mca)
	m.persistence = mps
	return mps, mca, m
}

func newTestOperation(t *testing.T, mtx *apitypes.ManagedTX, status core.OpStatus) *core.Operation {
	b, err := json.Marshal(&mtx)
	assert.NoError(t, err)
	op := &core.Operation{
		Namespace: strings.Split(mtx.ID, ":")[0],
		ID:        fftypes.MustParseUUID(strings.Split(mtx.ID, ":")[1]),
		Status:    status,
	}
	err = json.Unmarshal(b, &op.Output)
	assert.NoError(t, err)
	return op
}

func TestNewManagerBadHttpConfig(t *testing.T) {

	tmconfig.Reset()
	tmconfig.APIConfig.Set(httpserver.HTTPConfAddress, "::::")

	policyengines.RegisterEngine(&simple.PolicyEngineFactory{})
	tmconfig.PolicyEngineBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	_, err := NewManager(context.Background(), nil)
	assert.Regexp(t, "FF00151", err)

}

func TestNewManagerBadLevelDBConfig(t *testing.T) {

	tmpFile, err := ioutil.TempFile("", "ut-*")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	tmconfig.Reset()
	config.Set(tmconfig.PersistenceLevelDBPath, tmpFile.Name)
	tmconfig.APIConfig.Set(httpserver.HTTPConfPort, "0")

	policyengines.RegisterEngine(&simple.PolicyEngineFactory{})
	tmconfig.PolicyEngineBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	_, err = NewManager(context.Background(), nil)
	assert.Regexp(t, "FF21049", err)

}

func TestNewManagerBadPersistenceConfig(t *testing.T) {

	tmconfig.Reset()
	config.Set(tmconfig.PersistenceType, "wrong")
	tmconfig.APIConfig.Set(httpserver.HTTPConfPort, "0")

	policyengines.RegisterEngine(&simple.PolicyEngineFactory{})
	tmconfig.PolicyEngineBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	_, err := NewManager(context.Background(), nil)
	assert.Regexp(t, "FF21043", err)

}

func TestNewManagerBadPolicyEngine(t *testing.T) {

	tmconfig.Reset()
	config.Set(tmconfig.PolicyEngineName, "wrong")

	_, err := NewManager(context.Background(), nil)
	assert.Regexp(t, "FF21019", err)

}

func TestAddErrorMessageMax(t *testing.T) {

	var m *manager
	_, m, cancel := newTestManager(t)
	defer cancel()

	m.errorHistoryCount = 2
	mtx := &apitypes.ManagedTX{}
	m.addError(mtx, ffcapi.ErrorReasonTransactionUnderpriced, fmt.Errorf("snap"))
	m.addError(mtx, ffcapi.ErrorReasonTransactionUnderpriced, fmt.Errorf("crackle"))
	m.addError(mtx, ffcapi.ErrorReasonTransactionUnderpriced, fmt.Errorf("pop"))
	assert.Len(t, mtx.ErrorHistory, 2)
	assert.Equal(t, "pop", mtx.ErrorHistory[0].Error)
	assert.Equal(t, "crackle", mtx.ErrorHistory[1].Error)

}

func TestStartRestoreFail(t *testing.T) {
	_, m, cancel := newTestManager(t)
	cancel()

	mp := &persistencemocks.Persistence{}
	m.persistence = mp
	mp.On("ListStreams", mock.Anything, mock.Anything, startupPaginationLimit, persistence.SortDirectionAscending).
		Return(nil, fmt.Errorf("pop"))

	err := m.Start()
	assert.Regexp(t, "pop", err)
}
