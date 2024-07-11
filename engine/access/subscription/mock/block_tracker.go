// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	context "context"

	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// BlockTracker is an autogenerated mock type for the BlockTracker type
type BlockTracker struct {
	mock.Mock
}

// GetHighestHeight provides a mock function with given fields: _a0
func (_m *BlockTracker) GetHighestHeight(_a0 flow.BlockStatus) (uint64, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for GetHighestHeight")
	}

	var r0 uint64
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.BlockStatus) (uint64, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(flow.BlockStatus) uint64); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func(flow.BlockStatus) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStartHeightFromBlockID provides a mock function with given fields: _a0
func (_m *BlockTracker) GetStartHeightFromBlockID(_a0 flow.Identifier) (uint64, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for GetStartHeightFromBlockID")
	}

	var r0 uint64
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (uint64, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) uint64); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStartHeightFromHeight provides a mock function with given fields: _a0
func (_m *BlockTracker) GetStartHeightFromHeight(_a0 uint64) (uint64, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for GetStartHeightFromHeight")
	}

	var r0 uint64
	var r1 error
	if rf, ok := ret.Get(0).(func(uint64) (uint64, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(uint64) uint64); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func(uint64) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStartHeightFromLatest provides a mock function with given fields: _a0
func (_m *BlockTracker) GetStartHeightFromLatest(_a0 context.Context) (uint64, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for GetStartHeightFromLatest")
	}

	var r0 uint64
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (uint64, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(context.Context) uint64); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ProcessOnFinalizedBlock provides a mock function with given fields:
func (_m *BlockTracker) ProcessOnFinalizedBlock() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ProcessOnFinalizedBlock")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewBlockTracker creates a new instance of BlockTracker. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewBlockTracker(t interface {
	mock.TestingT
	Cleanup(func())
}) *BlockTracker {
	mock := &BlockTracker{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}