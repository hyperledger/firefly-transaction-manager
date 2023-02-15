// Code generated by mockery v2.18.0. DO NOT EDIT.

package txhistorymocks

import (
	context "context"

	apitypes "github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"

	fftypes "github.com/hyperledger/firefly-common/pkg/fftypes"

	mock "github.com/stretchr/testify/mock"
)

// Manager is an autogenerated mock type for the Manager type
type Manager struct {
	mock.Mock
}

// AddSubStatusAction provides a mock function with given fields: ctx, mtx, action, info, err
func (_m *Manager) AddSubStatusAction(ctx context.Context, mtx *apitypes.ManagedTX, action apitypes.TxAction, info *fftypes.JSONAny, err *fftypes.JSONAny) {
	_m.Called(ctx, mtx, action, info, err)
}

// CurrentSubStatus provides a mock function with given fields: ctx, mtx
func (_m *Manager) CurrentSubStatus(ctx context.Context, mtx *apitypes.ManagedTX) *apitypes.TxHistoryStateTransitionEntry {
	ret := _m.Called(ctx, mtx)

	var r0 *apitypes.TxHistoryStateTransitionEntry
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.ManagedTX) *apitypes.TxHistoryStateTransitionEntry); ok {
		r0 = rf(ctx, mtx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apitypes.TxHistoryStateTransitionEntry)
		}
	}

	return r0
}

// SetSubStatus provides a mock function with given fields: ctx, mtx, subStatus
func (_m *Manager) SetSubStatus(ctx context.Context, mtx *apitypes.ManagedTX, subStatus apitypes.TxSubStatus) {
	_m.Called(ctx, mtx, subStatus)
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
