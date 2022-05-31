// Code generated by mockery v2.12.1. DO NOT EDIT.

package mockinsecure

import (
	context "context"

	insecure "github.com/onflow/flow-go/insecure"
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"

	testing "testing"
)

// CorruptedNodeConnector is an autogenerated mock type for the CorruptedNodeConnector type
type CorruptedNodeConnector struct {
	mock.Mock
}

// Connect provides a mock function with given fields: _a0, _a1
func (_m *CorruptedNodeConnector) Connect(_a0 context.Context, _a1 flow.Identifier) (insecure.CorruptedNodeConnection, error) {
	ret := _m.Called(_a0, _a1)

	var r0 insecure.CorruptedNodeConnection
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) insecure.CorruptedNodeConnection); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(insecure.CorruptedNodeConnection)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// WithAttackerAddress provides a mock function with given fields: _a0
func (_m *CorruptedNodeConnector) WithAttackerAddress(_a0 string) {
	_m.Called(_a0)
}

// NewCorruptedNodeConnector creates a new instance of CorruptedNodeConnector. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewCorruptedNodeConnector(t testing.TB) *CorruptedNodeConnector {
	mock := &CorruptedNodeConnector{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
