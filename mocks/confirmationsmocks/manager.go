// Code generated by mockery v2.18.0. DO NOT EDIT.

package confirmationsmocks

import (
	confirmations "github.com/hyperledger/firefly-transaction-manager/internal/confirmations"
	ffcapi "github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"

	fftypes "github.com/hyperledger/firefly-common/pkg/fftypes"

	mock "github.com/stretchr/testify/mock"
)

// Manager is an autogenerated mock type for the Manager type
type Manager struct {
	mock.Mock
}

// CheckInFlight provides a mock function with given fields: listenerID
func (_m *Manager) CheckInFlight(listenerID *fftypes.UUID) bool {
	ret := _m.Called(listenerID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*fftypes.UUID) bool); ok {
		r0 = rf(listenerID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// NewBlockHashes provides a mock function with given fields:
func (_m *Manager) NewBlockHashes() chan<- *ffcapi.BlockHashEvent {
	ret := _m.Called()

	var r0 chan<- *ffcapi.BlockHashEvent
	if rf, ok := ret.Get(0).(func() chan<- *ffcapi.BlockHashEvent); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan<- *ffcapi.BlockHashEvent)
		}
	}

	return r0
}

// Notify provides a mock function with given fields: n
func (_m *Manager) Notify(n *confirmations.Notification) error {
	ret := _m.Called(n)

	var r0 error
	if rf, ok := ret.Get(0).(func(*confirmations.Notification) error); ok {
		r0 = rf(n)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Start provides a mock function with given fields:
func (_m *Manager) Start() {
	_m.Called()
}

// Stop provides a mock function with given fields:
func (_m *Manager) Stop() {
	_m.Called()
}

type mockConstructorTestingTNewManager interface {
	mock.TestingT
	Cleanup(func())
}

// NewManager creates a new instance of Manager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewManager(t mockConstructorTestingTNewManager) *Manager {
	mock := &Manager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
