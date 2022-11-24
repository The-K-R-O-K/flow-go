// Code generated by mockery v2.13.1. DO NOT EDIT.

package mocks

import (
	irrecoverable "github.com/onflow/flow-go/module/irrecoverable"
	mock "github.com/stretchr/testify/mock"

	model "github.com/onflow/flow-go/consensus/hotstuff/model"
)

// TimeoutAggregator is an autogenerated mock type for the TimeoutAggregator type
type TimeoutAggregator struct {
	mock.Mock
}

// AddTimeout provides a mock function with given fields: timeoutObject
func (_m *TimeoutAggregator) AddTimeout(timeoutObject *model.TimeoutObject) {
	_m.Called(timeoutObject)
}

// Done provides a mock function with given fields:
func (_m *TimeoutAggregator) Done() <-chan struct{} {
	ret := _m.Called()

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func() <-chan struct{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

// PruneUpToView provides a mock function with given fields: lowestRetainedView
func (_m *TimeoutAggregator) PruneUpToView(lowestRetainedView uint64) {
	_m.Called(lowestRetainedView)
}

// Ready provides a mock function with given fields:
func (_m *TimeoutAggregator) Ready() <-chan struct{} {
	ret := _m.Called()

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func() <-chan struct{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

// Start provides a mock function with given fields: _a0
func (_m *TimeoutAggregator) Start(_a0 irrecoverable.SignalerContext) {
	_m.Called(_a0)
}

type mockConstructorTestingTNewTimeoutAggregator interface {
	mock.TestingT
	Cleanup(func())
}

// NewTimeoutAggregator creates a new instance of TimeoutAggregator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewTimeoutAggregator(t mockConstructorTestingTNewTimeoutAggregator) *TimeoutAggregator {
	mock := &TimeoutAggregator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
