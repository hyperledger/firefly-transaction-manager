// Copyright © 2023 Kaleido, Inc.
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
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/dbsql"
	"github.com/hyperledger/firefly-common/pkg/httpserver"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/mocks/confirmationsmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/ffcapimocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/fftmmocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/persistencemocks"
	"github.com/hyperledger/firefly-transaction-manager/mocks/txhandlermocks"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhandler"
	txRegistry "github.com/hyperledger/firefly-transaction-manager/pkg/txhandler/registry"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhandler/simple"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testManagerName = "unittest"

func strPtr(s string) *string { return &s }

func testManagerCommonInit(t *testing.T, withMetrics bool) string {

	InitConfig()
	viper.SetDefault(string(tmconfig.TransactionsHandlerName), "simple")
	txRegistry.RegisterHandler(&simple.TransactionHandlerFactory{})
	tmconfig.TransactionHandlerBaseConfig.SubSection("simple").SubSection(simple.GasOracleConfig).Set(simple.GasOracleMode, simple.GasOracleModeDisabled)

	if withMetrics {
		tmconfig.MetricsConfig.Set("enabled", true)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	managerPort := strings.Split(ln.Addr().String(), ":")[1]
	ln.Close()
	tmconfig.APIConfig.Set(httpserver.HTTPConfPort, managerPort)
	tmconfig.APIConfig.Set(httpserver.HTTPConfAddress, "127.0.0.1")

	if withMetrics {
		tmconfig.MetricsConfig.Set(httpserver.HTTPConfPort, 6010)
		tmconfig.MetricsConfig.Set(httpserver.HTTPConfAddress, "127.0.0.1")
	}

	// config.Set(tmconfig.PolicyLoopInterval, "1ns") //TODO: fix this
	tmconfig.TransactionHandlerBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	return fmt.Sprintf("http://127.0.0.1:%s", managerPort)
}

func newTestManager(t *testing.T) (string, *manager, func()) {
	logrus.SetLevel(logrus.TraceLevel)

	url := testManagerCommonInit(t, false)

	dir := t.TempDir()
	config.Set(tmconfig.PersistenceLevelDBPath, dir)

	mca := &ffcapimocks.API{}
	mca.On("NewBlockListener", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), nil).Maybe()
	mm, err := NewManager(context.Background(), mca, nil)
	assert.NoError(t, err)

	m := mm.(*manager)
	mcm := &confirmationsmocks.Manager{}
	mcm.On("Start").Return().Maybe()
	m.confirmations = mcm

	return url,
		m,
		func() {
			m.Close()
		}
}

func newTestManagerMockNoRichDB(t *testing.T) (string, *manager, func()) {

	url := testManagerCommonInit(t, false)

	m := newManager(context.Background(), &ffcapimocks.API{}, nil)

	mpm := &persistencemocks.Persistence{}
	mpm.On("Close", mock.Anything).Return(nil)
	m.persistence = mpm
	m.richQueryEnabled = false
	mcm := &confirmationsmocks.Manager{}
	m.confirmations = mcm

	err := m.initServices(m.ctx)
	assert.NoError(t, err)

	go m.runAPIServer()

	return url,
		m,
		func() {
			m.Close()
			mpm.AssertExpectations(t)
		}
}

func newTestManagerMockRichDB(t *testing.T) (string, *manager, *persistencemocks.RichQuery, func()) {

	url := testManagerCommonInit(t, false)

	m := newManager(context.Background(), &ffcapimocks.API{}, nil)

	mpm := &persistencemocks.Persistence{}
	mpm.On("Close", mock.Anything).Return(nil)
	mrq := &persistencemocks.RichQuery{}
	mpm.On("RichQuery").Return(mrq)
	m.persistence = mpm
	m.richQueryEnabled = true
	mcm := &confirmationsmocks.Manager{}
	m.confirmations = mcm

	err := m.initServices(m.ctx)
	assert.NoError(t, err)

	go m.runAPIServer()

	return url,
		m,
		mrq,
		func() {
			m.Close()
			mpm.AssertExpectations(t)
			mrq.AssertExpectations(t)
		}
}

func newTestManagerWithMetrics(t *testing.T) (string, *manager, func()) {

	url := testManagerCommonInit(t, true)

	dir := t.TempDir()
	config.Set(tmconfig.PersistenceLevelDBPath, dir)

	mca := &ffcapimocks.API{}
	mca.On("NewBlockListener", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), nil).Maybe()

	mm, err := NewManager(context.Background(), mca, nil)
	assert.NoError(t, err)

	m := mm.(*manager)
	mcm := &confirmationsmocks.Manager{}
	mcm.On("Start").Return().Maybe()
	m.confirmations = mcm

	return url,
		m,
		func() {
			m.Close()
		}
}

func newTestManagerMockPersistence(t *testing.T) (string, *manager, func()) {

	url := testManagerCommonInit(t, false)
	m := newManager(context.Background(), &ffcapimocks.API{}, nil)
	mp := &persistencemocks.Persistence{}
	mp.On("Close", mock.Anything).Return(nil).Maybe()
	m.persistence = mp

	err := m.initServices(context.Background())
	assert.NoError(t, err)

	return url, m, func() {
		m.Close()
	}
}

func TestNewManagerBadPersistencePathConfig(t *testing.T) {

	tmconfig.Reset()
	tmconfig.APIConfig.Set(httpserver.HTTPConfAddress, "::::")

	txRegistry.RegisterHandler(&simple.TransactionHandlerFactory{})
	tmconfig.TransactionHandlerBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	_, err := NewManager(context.Background(), nil, nil)
	assert.Error(t, err)
	assert.Regexp(t, "FF21050", err)

}

func TestNewManagerWithLegacyConfiguration(t *testing.T) {

	InitConfig()
	viper.SetDefault(string(tmconfig.DeprecatedPolicyEngineName), "simple")

	txRegistry.RegisterHandler(&simple.TransactionHandlerFactory{})
	tmconfig.DeprecatedPolicyEngineBaseConfig.SubSection("simple").SubSection(simple.GasOracleConfig).Set(simple.GasOracleMode, simple.GasOracleModeDisabled)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	managerPort := strings.Split(ln.Addr().String(), ":")[1]
	ln.Close()
	tmconfig.APIConfig.Set(httpserver.HTTPConfPort, managerPort)
	tmconfig.APIConfig.Set(httpserver.HTTPConfAddress, "127.0.0.1")

	tmconfig.DeprecatedPolicyEngineBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	m := newManager(context.Background(), &ffcapimocks.API{}, nil)
	mp := &persistencemocks.Persistence{}
	mp.On("Close", mock.Anything).Return(nil).Maybe()
	m.persistence = mp

	err = m.initServices(context.Background())
	assert.NoError(t, err)

}

func TestNewManagerBadHttpConfig(t *testing.T) {

	tmconfig.Reset()
	tmconfig.APIConfig.Set(httpserver.HTTPConfAddress, "::::")
	dir := t.TempDir()
	config.Set(tmconfig.PersistenceLevelDBPath, dir)

	txRegistry.RegisterHandler(&simple.TransactionHandlerFactory{})
	tmconfig.TransactionHandlerBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	_, err := NewManager(context.Background(), nil, nil)
	assert.Error(t, err)
	assert.Regexp(t, "FF00151", err)

}

func TestNewManagerBadLevelDBConfig(t *testing.T) {

	tmpFile, err := ioutil.TempFile("", "ut-*")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	tmconfig.Reset()
	config.Set(tmconfig.PersistenceLevelDBPath, tmpFile.Name)
	tmconfig.APIConfig.Set(httpserver.HTTPConfPort, "0")

	txRegistry.RegisterHandler(&simple.TransactionHandlerFactory{})
	tmconfig.TransactionHandlerBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	_, err = NewManager(context.Background(), nil, nil)
	assert.Regexp(t, "FF21049", err)

}

func TestNewManagerBadPersistenceConfig(t *testing.T) {

	tmconfig.Reset()
	config.Set(tmconfig.PersistenceType, "wrong")
	tmconfig.APIConfig.Set(httpserver.HTTPConfPort, "0")

	txRegistry.RegisterHandler(&simple.TransactionHandlerFactory{})
	tmconfig.TransactionHandlerBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	_, err := NewManager(context.Background(), nil, nil)
	assert.Regexp(t, "FF21043", err)

}

func TestNewManagerInvalidTransactionHandlerName(t *testing.T) {

	tmconfig.Reset()
	dir := t.TempDir()
	config.Set(tmconfig.PersistenceLevelDBPath, dir)
	config.Set(tmconfig.TransactionsHandlerName, "wrong")

	_, err := NewManager(context.Background(), nil, nil)
	assert.Regexp(t, "FF21070", err)

}

func TestNewManagerMetricsOffByDefault(t *testing.T) {

	tmconfig.Reset()

	m := newManager(context.Background(), nil, nil)
	assert.False(t, m.metricsEnabled)
}

func TestNewManagerWithMetrics(t *testing.T) {

	_, m, close := newTestManagerWithMetrics(t)
	defer close()
	_ = m.Start()

	assert.True(t, m.metricsEnabled)
}

func TestNewManagerWithMetricsBadConfig(t *testing.T) {

	tmconfig.Reset()
	viper.SetDefault(string(tmconfig.TransactionsHandlerName), "simple")

	tmconfig.MetricsConfig.Set("enabled", true)
	tmconfig.MetricsConfig.Set(httpserver.HTTPConfAddress, "::::")
	dir := t.TempDir()
	config.Set(tmconfig.PersistenceLevelDBPath, dir)

	txRegistry.RegisterHandler(&simple.TransactionHandlerFactory{})
	tmconfig.TransactionHandlerBaseConfig.SubSection("simple").Set(simple.FixedGasPrice, "223344556677")

	_, err := NewManager(context.Background(), nil, nil)
	assert.Error(t, err)
	assert.Regexp(t, "FF00151", err)
}

func TestStartListListenersFail(t *testing.T) {
	_, m, close := newTestManagerMockPersistence(t)
	defer close()

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("ListStreamsByCreateTime", mock.Anything, mock.Anything, startupPaginationLimit, txhandler.SortDirectionAscending).Return(nil, fmt.Errorf("pop"))

	err := m.Start()
	assert.Regexp(t, "pop", err)

}

func TestStartTransactionHandlerFail(t *testing.T) {
	_, m, close := newTestManager(t)
	defer close()
	mth := &txhandlermocks.TransactionHandler{}
	mth.On("Start", m.ctx).Return(nil, fmt.Errorf("pop"))
	m.txHandler = mth
	err := m.Start()
	assert.Regexp(t, "pop", err)

}

func TestStartBlockListenerFail(t *testing.T) {
	_, m, close := newTestManagerMockPersistence(t)
	defer close()

	mp := m.persistence.(*persistencemocks.Persistence)
	mp.On("ListStreamsByCreateTime", mock.Anything, mock.Anything, startupPaginationLimit, txhandler.SortDirectionAscending).Return(nil, nil)

	mca := m.connector.(*ffcapimocks.API)
	mca.On("NewBlockListener", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), fmt.Errorf("pop"))

	err := m.Start()
	assert.Regexp(t, "pop", err)

}

func TestPSQLInitFail(t *testing.T) {

	_ = testManagerCommonInit(t, false)
	config.Set(tmconfig.PersistenceType, "postgres")

	m := newManager(context.Background(), &ffcapimocks.API{}, nil)

	err := m.initPersistence(context.Background())
	assert.Regexp(t, "FF21049", err)
}

func TestPSQLInitRichQueryEnabled(t *testing.T) {

	_ = testManagerCommonInit(t, false)
	config.Set(tmconfig.PersistenceType, "postgres")
	tmconfig.PostgresSection.Set(dbsql.SQLConfDatasourceURL, "unused")

	m := newManager(context.Background(), &ffcapimocks.API{}, nil)

	err := m.initPersistence(context.Background())
	assert.NoError(t, err)
	defer m.Close()

	assert.True(t, m.richQueryEnabled)
	assert.NotNil(t, m.toolkit.RichQuery)
}

func TestModuleMode(t *testing.T) {

	_ = testManagerCommonInit(t, false)

	dir := t.TempDir()
	config.Set(tmconfig.PersistenceLevelDBPath, dir)

	m := newManager(context.Background(), &ffcapimocks.API{}, &fftmmocks.ModuleFunctions{})
	mpm := &persistencemocks.Persistence{}
	mpm.On("Close", mock.Anything).Return(nil)
	mpm.On("ListStreamsByCreateTime", mock.Anything, mock.Anything, startupPaginationLimit, txhandler.SortDirectionAscending).Return(nil, nil)
	mrq := &persistencemocks.RichQuery{}
	mpm.On("RichQuery").Return(mrq)
	m.persistence = mpm
	m.richQueryEnabled = true
	mcm := &confirmationsmocks.Manager{}
	m.confirmations = mcm
	mcm.On("Start").Return().Maybe()
	mca := m.connector.(*ffcapimocks.API)
	mca.On("NewBlockListener", mock.Anything, mock.Anything).Return(nil, ffcapi.ErrorReason(""), nil).Maybe()

	err := m.initPersistence(context.Background())
	assert.NoError(t, err)

	err = m.initServices(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, m.apiServer)
	assert.Nil(t, m.metricsServer)
	assert.Nil(t, m.wsServer)

	err = m.Start()
	assert.NoError(t, err)
}
