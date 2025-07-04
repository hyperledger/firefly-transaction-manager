// Code generated by mockery v2.53.4. DO NOT EDIT.

package metricsmocks

import (
	context "context"

	metric "github.com/hyperledger/firefly-common/pkg/metric"

	mock "github.com/stretchr/testify/mock"
)

// TransactionHandlerMetrics is an autogenerated mock type for the TransactionHandlerMetrics type
type TransactionHandlerMetrics struct {
	mock.Mock
}

// IncTxHandlerCounterMetric provides a mock function with given fields: ctx, metricName, defaultLabels
func (_m *TransactionHandlerMetrics) IncTxHandlerCounterMetric(ctx context.Context, metricName string, defaultLabels *metric.FireflyDefaultLabels) {
	_m.Called(ctx, metricName, defaultLabels)
}

// IncTxHandlerCounterMetricWithLabels provides a mock function with given fields: ctx, metricName, labels, defaultLabels
func (_m *TransactionHandlerMetrics) IncTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels) {
	_m.Called(ctx, metricName, labels, defaultLabels)
}

// InitTxHandlerCounterMetric provides a mock function with given fields: ctx, metricName, helpText, withDefaultLabels
func (_m *TransactionHandlerMetrics) InitTxHandlerCounterMetric(ctx context.Context, metricName string, helpText string, withDefaultLabels bool) {
	_m.Called(ctx, metricName, helpText, withDefaultLabels)
}

// InitTxHandlerCounterMetricWithLabels provides a mock function with given fields: ctx, metricName, helpText, labelNames, withDefaultLabels
func (_m *TransactionHandlerMetrics) InitTxHandlerCounterMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string, withDefaultLabels bool) {
	_m.Called(ctx, metricName, helpText, labelNames, withDefaultLabels)
}

// InitTxHandlerGaugeMetric provides a mock function with given fields: ctx, metricName, helpText, withDefaultLabels
func (_m *TransactionHandlerMetrics) InitTxHandlerGaugeMetric(ctx context.Context, metricName string, helpText string, withDefaultLabels bool) {
	_m.Called(ctx, metricName, helpText, withDefaultLabels)
}

// InitTxHandlerGaugeMetricWithLabels provides a mock function with given fields: ctx, metricName, helpText, labelNames, withDefaultLabels
func (_m *TransactionHandlerMetrics) InitTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string, withDefaultLabels bool) {
	_m.Called(ctx, metricName, helpText, labelNames, withDefaultLabels)
}

// InitTxHandlerHistogramMetric provides a mock function with given fields: ctx, metricName, helpText, buckets, withDefaultLabels
func (_m *TransactionHandlerMetrics) InitTxHandlerHistogramMetric(ctx context.Context, metricName string, helpText string, buckets []float64, withDefaultLabels bool) {
	_m.Called(ctx, metricName, helpText, buckets, withDefaultLabels)
}

// InitTxHandlerHistogramMetricWithLabels provides a mock function with given fields: ctx, metricName, helpText, buckets, labelNames, withDefaultLabels
func (_m *TransactionHandlerMetrics) InitTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, helpText string, buckets []float64, labelNames []string, withDefaultLabels bool) {
	_m.Called(ctx, metricName, helpText, buckets, labelNames, withDefaultLabels)
}

// InitTxHandlerSummaryMetric provides a mock function with given fields: ctx, metricName, helpText, withDefaultLabels
func (_m *TransactionHandlerMetrics) InitTxHandlerSummaryMetric(ctx context.Context, metricName string, helpText string, withDefaultLabels bool) {
	_m.Called(ctx, metricName, helpText, withDefaultLabels)
}

// InitTxHandlerSummaryMetricWithLabels provides a mock function with given fields: ctx, metricName, helpText, labelNames, withDefaultLabels
func (_m *TransactionHandlerMetrics) InitTxHandlerSummaryMetricWithLabels(ctx context.Context, metricName string, helpText string, labelNames []string, withDefaultLabels bool) {
	_m.Called(ctx, metricName, helpText, labelNames, withDefaultLabels)
}

// ObserveTxHandlerHistogramMetric provides a mock function with given fields: ctx, metricName, number, defaultLabels
func (_m *TransactionHandlerMetrics) ObserveTxHandlerHistogramMetric(ctx context.Context, metricName string, number float64, defaultLabels *metric.FireflyDefaultLabels) {
	_m.Called(ctx, metricName, number, defaultLabels)
}

// ObserveTxHandlerHistogramMetricWithLabels provides a mock function with given fields: ctx, metricName, number, labels, defaultLabels
func (_m *TransactionHandlerMetrics) ObserveTxHandlerHistogramMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels) {
	_m.Called(ctx, metricName, number, labels, defaultLabels)
}

// ObserveTxHandlerSummaryMetric provides a mock function with given fields: ctx, metricName, number, defaultLabels
func (_m *TransactionHandlerMetrics) ObserveTxHandlerSummaryMetric(ctx context.Context, metricName string, number float64, defaultLabels *metric.FireflyDefaultLabels) {
	_m.Called(ctx, metricName, number, defaultLabels)
}

// ObserveTxHandlerSummaryMetricWithLabels provides a mock function with given fields: ctx, metricName, number, labels, defaultLabels
func (_m *TransactionHandlerMetrics) ObserveTxHandlerSummaryMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels) {
	_m.Called(ctx, metricName, number, labels, defaultLabels)
}

// SetTxHandlerGaugeMetric provides a mock function with given fields: ctx, metricName, number, defaultLabels
func (_m *TransactionHandlerMetrics) SetTxHandlerGaugeMetric(ctx context.Context, metricName string, number float64, defaultLabels *metric.FireflyDefaultLabels) {
	_m.Called(ctx, metricName, number, defaultLabels)
}

// SetTxHandlerGaugeMetricWithLabels provides a mock function with given fields: ctx, metricName, number, labels, defaultLabels
func (_m *TransactionHandlerMetrics) SetTxHandlerGaugeMetricWithLabels(ctx context.Context, metricName string, number float64, labels map[string]string, defaultLabels *metric.FireflyDefaultLabels) {
	_m.Called(ctx, metricName, number, labels, defaultLabels)
}

// NewTransactionHandlerMetrics creates a new instance of TransactionHandlerMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewTransactionHandlerMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *TransactionHandlerMetrics {
	mock := &TransactionHandlerMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
