// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// EngineEvents is an autogenerated mock type for the EngineEvents type
type EngineEvents struct {
	mock.Mock
}

// ActiveClustersChanged provides a mock function with given fields: _a0
func (_m *EngineEvents) ActiveClustersChanged(_a0 flow.ChainIDList) {
	_m.Called(_a0)
}

type mockConstructorTestingTNewEngineEvents interface {
	mock.TestingT
	Cleanup(func())
}

// NewEngineEvents creates a new instance of EngineEvents. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewEngineEvents(t mockConstructorTestingTNewEngineEvents) *EngineEvents {
	mock := &EngineEvents{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}