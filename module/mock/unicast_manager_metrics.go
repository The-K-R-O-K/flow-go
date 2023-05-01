// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// UnicastManagerMetrics is an autogenerated mock type for the UnicastManagerMetrics type
type UnicastManagerMetrics struct {
	mock.Mock
}

// OnEstablishStreamFailure provides a mock function with given fields: duration, attempts
func (_m *UnicastManagerMetrics) OnEstablishStreamFailure(duration time.Duration, attempts int) {
	_m.Called(duration, attempts)
}

// OnPeerDialFailure provides a mock function with given fields: duration, attempts
func (_m *UnicastManagerMetrics) OnPeerDialFailure(duration time.Duration, attempts int) {
	_m.Called(duration, attempts)
}

// OnPeerDialed provides a mock function with given fields: duration, attempts
func (_m *UnicastManagerMetrics) OnPeerDialed(duration time.Duration, attempts int) {
	_m.Called(duration, attempts)
}

// OnStreamCreated provides a mock function with given fields: duration, attempts
func (_m *UnicastManagerMetrics) OnStreamCreated(duration time.Duration, attempts int) {
	_m.Called(duration, attempts)
}

// OnStreamCreationFailure provides a mock function with given fields: duration, attempts
func (_m *UnicastManagerMetrics) OnStreamCreationFailure(duration time.Duration, attempts int) {
	_m.Called(duration, attempts)
}

// OnStreamEstablished provides a mock function with given fields: duration, attempts
func (_m *UnicastManagerMetrics) OnStreamEstablished(duration time.Duration, attempts int) {
	_m.Called(duration, attempts)
}

type mockConstructorTestingTNewUnicastManagerMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewUnicastManagerMetrics creates a new instance of UnicastManagerMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewUnicastManagerMetrics(t mockConstructorTestingTNewUnicastManagerMetrics) *UnicastManagerMetrics {
	mock := &UnicastManagerMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
