// Code generated by mockery v2.16.0. DO NOT EDIT.

package metricsmocks

import mock "github.com/stretchr/testify/mock"

// Metrics is an autogenerated mock type for the Metrics type
type Metrics struct {
	mock.Mock
}

// IsMetricsEnabled provides a mock function with given fields:
func (_m *Metrics) IsMetricsEnabled() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// TransactionSubmissionError provides a mock function with given fields:
func (_m *Metrics) TransactionSubmissionError() {
	_m.Called()
}

// TransactionsInFlightSet provides a mock function with given fields: count
func (_m *Metrics) TransactionsInFlightSet(count float64) {
	_m.Called(count)
}

type mockConstructorTestingTNewMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewMetrics creates a new instance of Metrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMetrics(t mockConstructorTestingTNewMetrics) *Metrics {
	mock := &Metrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
