// Code generated by mockery v1.0.0. DO NOT EDIT.

package policyenginemocks

import (
	context "context"

	ffcapi "github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	fftm "github.com/hyperledger/firefly-transaction-manager/pkg/fftm"

	mock "github.com/stretchr/testify/mock"
)

// PolicyEngine is an autogenerated mock type for the PolicyEngine type
type PolicyEngine struct {
	mock.Mock
}

// Execute provides a mock function with given fields: ctx, cAPI, mtx
func (_m *PolicyEngine) Execute(ctx context.Context, cAPI ffcapi.API, mtx *fftm.ManagedTXOutput) (bool, error) {
	ret := _m.Called(ctx, cAPI, mtx)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, ffcapi.API, *fftm.ManagedTXOutput) bool); ok {
		r0 = rf(ctx, cAPI, mtx)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, ffcapi.API, *fftm.ManagedTXOutput) error); ok {
		r1 = rf(ctx, cAPI, mtx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
