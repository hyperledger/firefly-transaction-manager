// Code generated by mockery v2.20.0. DO NOT EDIT.

package txhandlermocks

import (
	context "context"

	apitypes "github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"

	ffcapi "github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"

	mock "github.com/stretchr/testify/mock"

	txhandler "github.com/hyperledger/firefly-transaction-manager/pkg/txhandler"
)

// TransactionHandler is an autogenerated mock type for the TransactionHandler type
type TransactionHandler struct {
	mock.Mock
}

// HandleCancelTransaction provides a mock function with given fields: ctx, txID
func (_m *TransactionHandler) HandleCancelTransaction(ctx context.Context, txID string) (*apitypes.ManagedTX, error) {
	ret := _m.Called(ctx, txID)

	var r0 *apitypes.ManagedTX
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*apitypes.ManagedTX, error)); ok {
		return rf(ctx, txID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *apitypes.ManagedTX); ok {
		r0 = rf(ctx, txID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apitypes.ManagedTX)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, txID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// HandleNewContractDeployment provides a mock function with given fields: ctx, txReq
func (_m *TransactionHandler) HandleNewContractDeployment(ctx context.Context, txReq *apitypes.ContractDeployRequest) (*apitypes.ManagedTX, error) {
	ret := _m.Called(ctx, txReq)

	var r0 *apitypes.ManagedTX
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.ContractDeployRequest) (*apitypes.ManagedTX, error)); ok {
		return rf(ctx, txReq)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.ContractDeployRequest) *apitypes.ManagedTX); ok {
		r0 = rf(ctx, txReq)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apitypes.ManagedTX)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *apitypes.ContractDeployRequest) error); ok {
		r1 = rf(ctx, txReq)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// HandleNewTransaction provides a mock function with given fields: ctx, txReq
func (_m *TransactionHandler) HandleNewTransaction(ctx context.Context, txReq *apitypes.TransactionRequest) (*apitypes.ManagedTX, error) {
	ret := _m.Called(ctx, txReq)

	var r0 *apitypes.ManagedTX
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.TransactionRequest) (*apitypes.ManagedTX, error)); ok {
		return rf(ctx, txReq)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.TransactionRequest) *apitypes.ManagedTX); ok {
		r0 = rf(ctx, txReq)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apitypes.ManagedTX)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *apitypes.TransactionRequest) error); ok {
		r1 = rf(ctx, txReq)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// HandleTransactionConfirmed provides a mock function with given fields: ctx, txID, confirmations
func (_m *TransactionHandler) HandleTransactionConfirmed(ctx context.Context, txID string, confirmations []apitypes.BlockInfo) error {
	ret := _m.Called(ctx, txID, confirmations)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []apitypes.BlockInfo) error); ok {
		r0 = rf(ctx, txID, confirmations)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// HandleTransactionReceiptReceived provides a mock function with given fields: ctx, txID, receipt
func (_m *TransactionHandler) HandleTransactionReceiptReceived(ctx context.Context, txID string, receipt *ffcapi.TransactionReceiptResponse) error {
	ret := _m.Called(ctx, txID, receipt)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *ffcapi.TransactionReceiptResponse) error); ok {
		r0 = rf(ctx, txID, receipt)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Init provides a mock function with given fields: ctx, toolkit
func (_m *TransactionHandler) Init(ctx context.Context, toolkit *txhandler.Toolkit) {
	_m.Called(ctx, toolkit)
}

// Start provides a mock function with given fields: ctx
func (_m *TransactionHandler) Start(ctx context.Context) (<-chan struct{}, error) {
	ret := _m.Called(ctx)

	var r0 <-chan struct{}
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (<-chan struct{}, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) <-chan struct{}); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewTransactionHandler interface {
	mock.TestingT
	Cleanup(func())
}

// NewTransactionHandler creates a new instance of TransactionHandler. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewTransactionHandler(t mockConstructorTestingTNewTransactionHandler) *TransactionHandler {
	mock := &TransactionHandler{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
