// Code generated by mockery v2.13.1. DO NOT EDIT.

package mockp2p

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// DisallowListConsumer is an autogenerated mock type for the DisallowListConsumer type
type DisallowListConsumer struct {
	mock.Mock
}

// OnNodeDisallowListUpdate provides a mock function with given fields: list
func (_m *DisallowListConsumer) OnNodeDisallowListUpdate(list flow.IdentifierList) {
	_m.Called(list)
}

type mockConstructorTestingTNewDisallowListConsumer interface {
	mock.TestingT
	Cleanup(func())
}

// NewDisallowListConsumer creates a new instance of DisallowListConsumer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewDisallowListConsumer(t mockConstructorTestingTNewDisallowListConsumer) *DisallowListConsumer {
	mock := &DisallowListConsumer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
