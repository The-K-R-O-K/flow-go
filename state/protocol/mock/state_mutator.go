// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	protocol "github.com/onflow/flow-go/state/protocol"

	transaction "github.com/onflow/flow-go/storage/badger/transaction"
)

// StateMutator is an autogenerated mock type for the StateMutator type
type StateMutator struct {
	mock.Mock
}

// ApplyServiceEvents provides a mock function with given fields: updater, seals
func (_m *StateMutator) ApplyServiceEvents(updater protocol.StateUpdater, seals []*flow.Seal) ([]func(*transaction.Tx) error, error) {
	ret := _m.Called(updater, seals)

	var r0 []func(*transaction.Tx) error
	var r1 error
	if rf, ok := ret.Get(0).(func(protocol.StateUpdater, []*flow.Seal) ([]func(*transaction.Tx) error, error)); ok {
		return rf(updater, seals)
	}
	if rf, ok := ret.Get(0).(func(protocol.StateUpdater, []*flow.Seal) []func(*transaction.Tx) error); ok {
		r0 = rf(updater, seals)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]func(*transaction.Tx) error)
		}
	}

	if rf, ok := ret.Get(1).(func(protocol.StateUpdater, []*flow.Seal) error); ok {
		r1 = rf(updater, seals)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CommitProtocolState provides a mock function with given fields: blockID, updater
func (_m *StateMutator) CommitProtocolState(blockID flow.Identifier, updater protocol.StateUpdater) (func(*transaction.Tx) error, flow.Identifier) {
	ret := _m.Called(blockID, updater)

	var r0 func(*transaction.Tx) error
	var r1 flow.Identifier
	if rf, ok := ret.Get(0).(func(flow.Identifier, protocol.StateUpdater) (func(*transaction.Tx) error, flow.Identifier)); ok {
		return rf(blockID, updater)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier, protocol.StateUpdater) func(*transaction.Tx) error); ok {
		r0 = rf(blockID, updater)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*transaction.Tx) error)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier, protocol.StateUpdater) flow.Identifier); ok {
		r1 = rf(blockID, updater)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(flow.Identifier)
		}
	}

	return r0, r1
}

// CreateUpdater provides a mock function with given fields: candidateView, parentID
func (_m *StateMutator) CreateUpdater(candidateView uint64, parentID flow.Identifier) (protocol.StateUpdater, error) {
	ret := _m.Called(candidateView, parentID)

	var r0 protocol.StateUpdater
	var r1 error
	if rf, ok := ret.Get(0).(func(uint64, flow.Identifier) (protocol.StateUpdater, error)); ok {
		return rf(candidateView, parentID)
	}
	if rf, ok := ret.Get(0).(func(uint64, flow.Identifier) protocol.StateUpdater); ok {
		r0 = rf(candidateView, parentID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.StateUpdater)
		}
	}

	if rf, ok := ret.Get(1).(func(uint64, flow.Identifier) error); ok {
		r1 = rf(candidateView, parentID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewStateMutator interface {
	mock.TestingT
	Cleanup(func())
}

// NewStateMutator creates a new instance of StateMutator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewStateMutator(t mockConstructorTestingTNewStateMutator) *StateMutator {
	mock := &StateMutator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}