// Code generated by mockery v2.32.4. DO NOT EDIT.

package persistencemocks

import (
	context "context"

	apitypes "github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"

	ffcapi "github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"

	fftypes "github.com/hyperledger/firefly-common/pkg/fftypes"

	mock "github.com/stretchr/testify/mock"

	persistence "github.com/hyperledger/firefly-transaction-manager/internal/persistence"

	txhandler "github.com/hyperledger/firefly-transaction-manager/pkg/txhandler"
)

// Persistence is an autogenerated mock type for the Persistence type
type Persistence struct {
	mock.Mock
}

// AddSubStatusAction provides a mock function with given fields: ctx, txID, subStatus, action, info, err, actionOccurred
func (_m *Persistence) AddSubStatusAction(ctx context.Context, txID string, subStatus apitypes.TxSubStatus, action apitypes.TxAction, info *fftypes.JSONAny, err *fftypes.JSONAny, actionOccurred *fftypes.FFTime) error {
	ret := _m.Called(ctx, txID, subStatus, action, info, err, actionOccurred)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, apitypes.TxSubStatus, apitypes.TxAction, *fftypes.JSONAny, *fftypes.JSONAny, *fftypes.FFTime) error); ok {
		r0 = rf(ctx, txID, subStatus, action, info, err, actionOccurred)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddTransactionConfirmations provides a mock function with given fields: ctx, txID, clearExisting, confirmations
func (_m *Persistence) AddTransactionConfirmations(ctx context.Context, txID string, clearExisting bool, confirmations ...*apitypes.Confirmation) error {
	_va := make([]interface{}, len(confirmations))
	for _i := range confirmations {
		_va[_i] = confirmations[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, txID, clearExisting)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, bool, ...*apitypes.Confirmation) error); ok {
		r0 = rf(ctx, txID, clearExisting, confirmations...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Close provides a mock function with given fields: ctx
func (_m *Persistence) Close(ctx context.Context) {
	_m.Called(ctx)
}

// DeleteCheckpoint provides a mock function with given fields: ctx, streamID
func (_m *Persistence) DeleteCheckpoint(ctx context.Context, streamID *fftypes.UUID) error {
	ret := _m.Called(ctx, streamID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID) error); ok {
		r0 = rf(ctx, streamID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteListener provides a mock function with given fields: ctx, listenerID
func (_m *Persistence) DeleteListener(ctx context.Context, listenerID *fftypes.UUID) error {
	ret := _m.Called(ctx, listenerID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID) error); ok {
		r0 = rf(ctx, listenerID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteStream provides a mock function with given fields: ctx, streamID
func (_m *Persistence) DeleteStream(ctx context.Context, streamID *fftypes.UUID) error {
	ret := _m.Called(ctx, streamID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID) error); ok {
		r0 = rf(ctx, streamID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteTransaction provides a mock function with given fields: ctx, txID
func (_m *Persistence) DeleteTransaction(ctx context.Context, txID string) error {
	ret := _m.Called(ctx, txID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, txID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetCheckpoint provides a mock function with given fields: ctx, streamID
func (_m *Persistence) GetCheckpoint(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStreamCheckpoint, error) {
	ret := _m.Called(ctx, streamID)

	var r0 *apitypes.EventStreamCheckpoint
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID) (*apitypes.EventStreamCheckpoint, error)); ok {
		return rf(ctx, streamID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID) *apitypes.EventStreamCheckpoint); ok {
		r0 = rf(ctx, streamID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apitypes.EventStreamCheckpoint)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *fftypes.UUID) error); ok {
		r1 = rf(ctx, streamID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetListener provides a mock function with given fields: ctx, listenerID
func (_m *Persistence) GetListener(ctx context.Context, listenerID *fftypes.UUID) (*apitypes.Listener, error) {
	ret := _m.Called(ctx, listenerID)

	var r0 *apitypes.Listener
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID) (*apitypes.Listener, error)); ok {
		return rf(ctx, listenerID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID) *apitypes.Listener); ok {
		r0 = rf(ctx, listenerID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apitypes.Listener)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *fftypes.UUID) error); ok {
		r1 = rf(ctx, listenerID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStream provides a mock function with given fields: ctx, streamID
func (_m *Persistence) GetStream(ctx context.Context, streamID *fftypes.UUID) (*apitypes.EventStream, error) {
	ret := _m.Called(ctx, streamID)

	var r0 *apitypes.EventStream
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID) (*apitypes.EventStream, error)); ok {
		return rf(ctx, streamID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID) *apitypes.EventStream); ok {
		r0 = rf(ctx, streamID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apitypes.EventStream)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *fftypes.UUID) error); ok {
		r1 = rf(ctx, streamID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionByID provides a mock function with given fields: ctx, txID
func (_m *Persistence) GetTransactionByID(ctx context.Context, txID string) (*apitypes.ManagedTX, error) {
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

// GetTransactionByIDWithStatus provides a mock function with given fields: ctx, txID, history
func (_m *Persistence) GetTransactionByIDWithStatus(ctx context.Context, txID string, history bool) (*apitypes.TXWithStatus, error) {
	ret := _m.Called(ctx, txID, history)

	var r0 *apitypes.TXWithStatus
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, bool) (*apitypes.TXWithStatus, error)); ok {
		return rf(ctx, txID, history)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, bool) *apitypes.TXWithStatus); ok {
		r0 = rf(ctx, txID, history)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apitypes.TXWithStatus)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, bool) error); ok {
		r1 = rf(ctx, txID, history)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionByNonce provides a mock function with given fields: ctx, signer, nonce
func (_m *Persistence) GetTransactionByNonce(ctx context.Context, signer string, nonce *fftypes.FFBigInt) (*apitypes.ManagedTX, error) {
	ret := _m.Called(ctx, signer, nonce)

	var r0 *apitypes.ManagedTX
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *fftypes.FFBigInt) (*apitypes.ManagedTX, error)); ok {
		return rf(ctx, signer, nonce)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, *fftypes.FFBigInt) *apitypes.ManagedTX); ok {
		r0 = rf(ctx, signer, nonce)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apitypes.ManagedTX)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, *fftypes.FFBigInt) error); ok {
		r1 = rf(ctx, signer, nonce)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionConfirmations provides a mock function with given fields: ctx, txID
func (_m *Persistence) GetTransactionConfirmations(ctx context.Context, txID string) ([]*apitypes.Confirmation, error) {
	ret := _m.Called(ctx, txID)

	var r0 []*apitypes.Confirmation
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]*apitypes.Confirmation, error)); ok {
		return rf(ctx, txID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []*apitypes.Confirmation); ok {
		r0 = rf(ctx, txID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*apitypes.Confirmation)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, txID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionReceipt provides a mock function with given fields: ctx, txID
func (_m *Persistence) GetTransactionReceipt(ctx context.Context, txID string) (*ffcapi.TransactionReceiptResponse, error) {
	ret := _m.Called(ctx, txID)

	var r0 *ffcapi.TransactionReceiptResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*ffcapi.TransactionReceiptResponse, error)); ok {
		return rf(ctx, txID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *ffcapi.TransactionReceiptResponse); ok {
		r0 = rf(ctx, txID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ffcapi.TransactionReceiptResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, txID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// InsertTransactionPreAssignedNonce provides a mock function with given fields: ctx, tx
func (_m *Persistence) InsertTransactionPreAssignedNonce(ctx context.Context, tx *apitypes.ManagedTX) error {
	ret := _m.Called(ctx, tx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.ManagedTX) error); ok {
		r0 = rf(ctx, tx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// InsertTransactionWithNextNonce provides a mock function with given fields: ctx, tx, lookupNextNonce
func (_m *Persistence) InsertTransactionWithNextNonce(ctx context.Context, tx *apitypes.ManagedTX, lookupNextNonce txhandler.NextNonceCallback) error {
	ret := _m.Called(ctx, tx, lookupNextNonce)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.ManagedTX, txhandler.NextNonceCallback) error); ok {
		r0 = rf(ctx, tx, lookupNextNonce)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ListListenersByCreateTime provides a mock function with given fields: ctx, after, limit, dir
func (_m *Persistence) ListListenersByCreateTime(ctx context.Context, after *fftypes.UUID, limit int, dir txhandler.SortDirection) ([]*apitypes.Listener, error) {
	ret := _m.Called(ctx, after, limit, dir)

	var r0 []*apitypes.Listener
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID, int, txhandler.SortDirection) ([]*apitypes.Listener, error)); ok {
		return rf(ctx, after, limit, dir)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID, int, txhandler.SortDirection) []*apitypes.Listener); ok {
		r0 = rf(ctx, after, limit, dir)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*apitypes.Listener)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *fftypes.UUID, int, txhandler.SortDirection) error); ok {
		r1 = rf(ctx, after, limit, dir)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListStreamListenersByCreateTime provides a mock function with given fields: ctx, after, limit, dir, streamID
func (_m *Persistence) ListStreamListenersByCreateTime(ctx context.Context, after *fftypes.UUID, limit int, dir txhandler.SortDirection, streamID *fftypes.UUID) ([]*apitypes.Listener, error) {
	ret := _m.Called(ctx, after, limit, dir, streamID)

	var r0 []*apitypes.Listener
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID, int, txhandler.SortDirection, *fftypes.UUID) ([]*apitypes.Listener, error)); ok {
		return rf(ctx, after, limit, dir, streamID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID, int, txhandler.SortDirection, *fftypes.UUID) []*apitypes.Listener); ok {
		r0 = rf(ctx, after, limit, dir, streamID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*apitypes.Listener)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *fftypes.UUID, int, txhandler.SortDirection, *fftypes.UUID) error); ok {
		r1 = rf(ctx, after, limit, dir, streamID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListStreamsByCreateTime provides a mock function with given fields: ctx, after, limit, dir
func (_m *Persistence) ListStreamsByCreateTime(ctx context.Context, after *fftypes.UUID, limit int, dir txhandler.SortDirection) ([]*apitypes.EventStream, error) {
	ret := _m.Called(ctx, after, limit, dir)

	var r0 []*apitypes.EventStream
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID, int, txhandler.SortDirection) ([]*apitypes.EventStream, error)); ok {
		return rf(ctx, after, limit, dir)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *fftypes.UUID, int, txhandler.SortDirection) []*apitypes.EventStream); ok {
		r0 = rf(ctx, after, limit, dir)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*apitypes.EventStream)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *fftypes.UUID, int, txhandler.SortDirection) error); ok {
		r1 = rf(ctx, after, limit, dir)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListTransactionsByCreateTime provides a mock function with given fields: ctx, after, limit, dir
func (_m *Persistence) ListTransactionsByCreateTime(ctx context.Context, after *apitypes.ManagedTX, limit int, dir txhandler.SortDirection) ([]*apitypes.ManagedTX, error) {
	ret := _m.Called(ctx, after, limit, dir)

	var r0 []*apitypes.ManagedTX
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.ManagedTX, int, txhandler.SortDirection) ([]*apitypes.ManagedTX, error)); ok {
		return rf(ctx, after, limit, dir)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.ManagedTX, int, txhandler.SortDirection) []*apitypes.ManagedTX); ok {
		r0 = rf(ctx, after, limit, dir)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*apitypes.ManagedTX)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *apitypes.ManagedTX, int, txhandler.SortDirection) error); ok {
		r1 = rf(ctx, after, limit, dir)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListTransactionsByNonce provides a mock function with given fields: ctx, signer, after, limit, dir
func (_m *Persistence) ListTransactionsByNonce(ctx context.Context, signer string, after *fftypes.FFBigInt, limit int, dir txhandler.SortDirection) ([]*apitypes.ManagedTX, error) {
	ret := _m.Called(ctx, signer, after, limit, dir)

	var r0 []*apitypes.ManagedTX
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *fftypes.FFBigInt, int, txhandler.SortDirection) ([]*apitypes.ManagedTX, error)); ok {
		return rf(ctx, signer, after, limit, dir)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, *fftypes.FFBigInt, int, txhandler.SortDirection) []*apitypes.ManagedTX); ok {
		r0 = rf(ctx, signer, after, limit, dir)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*apitypes.ManagedTX)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, *fftypes.FFBigInt, int, txhandler.SortDirection) error); ok {
		r1 = rf(ctx, signer, after, limit, dir)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListTransactionsPending provides a mock function with given fields: ctx, afterSequenceID, limit, dir
func (_m *Persistence) ListTransactionsPending(ctx context.Context, afterSequenceID string, limit int, dir txhandler.SortDirection) ([]*apitypes.ManagedTX, error) {
	ret := _m.Called(ctx, afterSequenceID, limit, dir)

	var r0 []*apitypes.ManagedTX
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int, txhandler.SortDirection) ([]*apitypes.ManagedTX, error)); ok {
		return rf(ctx, afterSequenceID, limit, dir)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, int, txhandler.SortDirection) []*apitypes.ManagedTX); ok {
		r0 = rf(ctx, afterSequenceID, limit, dir)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*apitypes.ManagedTX)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, int, txhandler.SortDirection) error); ok {
		r1 = rf(ctx, afterSequenceID, limit, dir)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RichQuery provides a mock function with given fields:
func (_m *Persistence) RichQuery() persistence.RichQuery {
	ret := _m.Called()

	var r0 persistence.RichQuery
	if rf, ok := ret.Get(0).(func() persistence.RichQuery); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(persistence.RichQuery)
		}
	}

	return r0
}

// SetTransactionReceipt provides a mock function with given fields: ctx, txID, receipt
func (_m *Persistence) SetTransactionReceipt(ctx context.Context, txID string, receipt *ffcapi.TransactionReceiptResponse) error {
	ret := _m.Called(ctx, txID, receipt)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *ffcapi.TransactionReceiptResponse) error); ok {
		r0 = rf(ctx, txID, receipt)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateTransaction provides a mock function with given fields: ctx, txID, updates
func (_m *Persistence) UpdateTransaction(ctx context.Context, txID string, updates *apitypes.TXUpdates) error {
	ret := _m.Called(ctx, txID, updates)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *apitypes.TXUpdates) error); ok {
		r0 = rf(ctx, txID, updates)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WriteCheckpoint provides a mock function with given fields: ctx, checkpoint
func (_m *Persistence) WriteCheckpoint(ctx context.Context, checkpoint *apitypes.EventStreamCheckpoint) error {
	ret := _m.Called(ctx, checkpoint)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.EventStreamCheckpoint) error); ok {
		r0 = rf(ctx, checkpoint)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WriteListener provides a mock function with given fields: ctx, spec
func (_m *Persistence) WriteListener(ctx context.Context, spec *apitypes.Listener) error {
	ret := _m.Called(ctx, spec)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.Listener) error); ok {
		r0 = rf(ctx, spec)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WriteStream provides a mock function with given fields: ctx, spec
func (_m *Persistence) WriteStream(ctx context.Context, spec *apitypes.EventStream) error {
	ret := _m.Called(ctx, spec)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *apitypes.EventStream) error); ok {
		r0 = rf(ctx, spec)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewPersistence creates a new instance of Persistence. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewPersistence(t interface {
	mock.TestingT
	Cleanup(func())
}) *Persistence {
	mock := &Persistence{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
