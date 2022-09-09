// Code generated by mockery v1.0.0. DO NOT EDIT.

package wsmocks

import (
	ws "github.com/hyperledger/firefly-transaction-manager/internal/ws"
	mock "github.com/stretchr/testify/mock"
)

// WebSocketChannels is an autogenerated mock type for the WebSocketChannels type
type WebSocketChannels struct {
	mock.Mock
}

// GetChannels provides a mock function with given fields: topic
func (_m *WebSocketChannels) GetChannels(topic string) (chan<- interface{}, chan<- interface{}, <-chan *ws.WebSocketCommandMessageOrError) {
	ret := _m.Called(topic)

	var r0 chan<- interface{}
	if rf, ok := ret.Get(0).(func(string) chan<- interface{}); ok {
		r0 = rf(topic)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan<- interface{})
		}
	}

	var r1 chan<- interface{}
	if rf, ok := ret.Get(1).(func(string) chan<- interface{}); ok {
		r1 = rf(topic)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(chan<- interface{})
		}
	}

	var r2 <-chan *ws.WebSocketCommandMessageOrError
	if rf, ok := ret.Get(2).(func(string) <-chan *ws.WebSocketCommandMessageOrError); ok {
		r2 = rf(topic)
	} else {
		if ret.Get(2) != nil {
			r2 = ret.Get(2).(<-chan *ws.WebSocketCommandMessageOrError)
		}
	}

	return r0, r1, r2
}

// SendReply provides a mock function with given fields: message
func (_m *WebSocketChannels) SendReply(message interface{}) {
	_m.Called(message)
}
