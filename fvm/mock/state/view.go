// Code generated by mockery v2.13.0. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	state "github.com/onflow/flow-go/fvm/state"
)

// View is an autogenerated mock type for the View type
type View struct {
	mock.Mock
}

// AllRegisters provides a mock function with given fields:
func (_m *View) AllRegisters() []flow.RegisterID {
	ret := _m.Called()

	var r0 []flow.RegisterID
	if rf, ok := ret.Get(0).(func() []flow.RegisterID); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.RegisterID)
		}
	}

	return r0
}

// Delete provides a mock function with given fields: owner, controller, key
func (_m *View) Delete(owner string, controller string, key string) error {
	ret := _m.Called(owner, controller, key)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string) error); ok {
		r0 = rf(owner, controller, key)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DropDelta provides a mock function with given fields:
func (_m *View) DropDelta() {
	_m.Called()
}

// Get provides a mock function with given fields: owner, controller, key
func (_m *View) Get(owner string, controller string, key string) ([]byte, error) {
	ret := _m.Called(owner, controller, key)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(string, string, string) []byte); ok {
		r0 = rf(owner, controller, key)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string) error); ok {
		r1 = rf(owner, controller, key)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MergeView provides a mock function with given fields: child
func (_m *View) MergeView(child state.View) error {
	ret := _m.Called(child)

	var r0 error
	if rf, ok := ret.Get(0).(func(state.View) error); ok {
		r0 = rf(child)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewChild provides a mock function with given fields:
func (_m *View) NewChild() state.View {
	ret := _m.Called()

	var r0 state.View
	if rf, ok := ret.Get(0).(func() state.View); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(state.View)
		}
	}

	return r0
}

// RegisterUpdates provides a mock function with given fields:
func (_m *View) RegisterUpdates() ([]flow.RegisterID, [][]byte) {
	ret := _m.Called()

	var r0 []flow.RegisterID
	if rf, ok := ret.Get(0).(func() []flow.RegisterID); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.RegisterID)
		}
	}

	var r1 [][]byte
	if rf, ok := ret.Get(1).(func() [][]byte); ok {
		r1 = rf()
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([][]byte)
		}
	}

	return r0, r1
}

// Set provides a mock function with given fields: owner, controller, key, value
func (_m *View) Set(owner string, controller string, key string, value []byte) error {
	ret := _m.Called(owner, controller, key, value)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string, []byte) error); ok {
		r0 = rf(owner, controller, key, value)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Touch provides a mock function with given fields: owner, controller, key
func (_m *View) Touch(owner string, controller string, key string) error {
	ret := _m.Called(owner, controller, key)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string) error); ok {
		r0 = rf(owner, controller, key)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type NewViewT interface {
	mock.TestingT
	Cleanup(func())
}

// NewView creates a new instance of View. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewView(t NewViewT) *View {
	mock := &View{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
