// Code generated by mockery v2.21.4. DO NOT EDIT.

package mocks

import (
	model "github.com/onflow/flow-go/consensus/hotstuff/model"
	mock "github.com/stretchr/testify/mock"
)

// TimeoutProcessor is an autogenerated mock type for the TimeoutProcessor type
type TimeoutProcessor struct {
	mock.Mock
}

// Process provides a mock function with given fields: timeout
func (_m *TimeoutProcessor) Process(timeout *model.TimeoutObject) error {
	ret := _m.Called(timeout)

	var r0 error
	if rf, ok := ret.Get(0).(func(*model.TimeoutObject) error); ok {
		r0 = rf(timeout)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewTimeoutProcessor interface {
	mock.TestingT
	Cleanup(func())
}

// NewTimeoutProcessor creates a new instance of TimeoutProcessor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewTimeoutProcessor(t mockConstructorTestingTNewTimeoutProcessor) *TimeoutProcessor {
	mock := &TimeoutProcessor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
