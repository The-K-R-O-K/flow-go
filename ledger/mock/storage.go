// Code generated by mockery v2.13.1. DO NOT EDIT.

package mock

import (
	hash "github.com/onflow/flow-go/ledger/common/hash"

	mock "github.com/stretchr/testify/mock"
)

// Storage is an autogenerated mock type for the Storage type
type Storage struct {
	mock.Mock
}

// Get provides a mock function with given fields: _a0
func (_m *Storage) Get(_a0 hash.Hash) ([]byte, error) {
	ret := _m.Called(_a0)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(hash.Hash) []byte); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(hash.Hash) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMul provides a mock function with given fields: _a0
func (_m *Storage) GetMul(_a0 []hash.Hash) ([][]byte, error) {
	ret := _m.Called(_a0)

	var r0 [][]byte
	if rf, ok := ret.Get(0).(func([]hash.Hash) [][]byte); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([][]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func([]hash.Hash) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SetMul provides a mock function with given fields: keys, values
func (_m *Storage) SetMul(keys []hash.Hash, values [][]byte) error {
	ret := _m.Called(keys, values)

	var r0 error
	if rf, ok := ret.Get(0).(func([]hash.Hash, [][]byte) error); ok {
		r0 = rf(keys, values)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewStorage interface {
	mock.TestingT
	Cleanup(func())
}

// NewStorage creates a new instance of Storage. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewStorage(t mockConstructorTestingTNewStorage) *Storage {
	mock := &Storage{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
