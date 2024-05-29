// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	protocol "github.com/onflow/flow-go/state/protocol"

	transaction "github.com/onflow/flow-go/storage/badger/transaction"
)

// MutableProtocolState is an autogenerated mock type for the MutableProtocolState type
type MutableProtocolState struct {
	mock.Mock
}

// AtBlockID provides a mock function with given fields: blockID
func (_m *MutableProtocolState) AtBlockID(blockID flow.Identifier) (protocol.DynamicProtocolState, error) {
	ret := _m.Called(blockID)

	var r0 protocol.DynamicProtocolState
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (protocol.DynamicProtocolState, error)); ok {
		return rf(blockID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) protocol.DynamicProtocolState); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.DynamicProtocolState)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// EvolveState provides a mock function with given fields: parentBlockID, candidateView, candidateSeals
func (_m *MutableProtocolState) EvolveState(parentBlockID flow.Identifier, candidateView uint64, candidateSeals []*flow.Seal) (flow.Identifier, *transaction.DeferredBlockPersist, error) {
	ret := _m.Called(parentBlockID, candidateView, candidateSeals)

	var r0 flow.Identifier
	var r1 *transaction.DeferredBlockPersist
	var r2 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, uint64, []*flow.Seal) (flow.Identifier, *transaction.DeferredBlockPersist, error)); ok {
		return rf(parentBlockID, candidateView, candidateSeals)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier, uint64, []*flow.Seal) flow.Identifier); ok {
		r0 = rf(parentBlockID, candidateView, candidateSeals)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier, uint64, []*flow.Seal) *transaction.DeferredBlockPersist); ok {
		r1 = rf(parentBlockID, candidateView, candidateSeals)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*transaction.DeferredBlockPersist)
		}
	}

	if rf, ok := ret.Get(2).(func(flow.Identifier, uint64, []*flow.Seal) error); ok {
		r2 = rf(parentBlockID, candidateView, candidateSeals)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GlobalParams provides a mock function with given fields:
func (_m *MutableProtocolState) GlobalParams() protocol.GlobalParams {
	ret := _m.Called()

	var r0 protocol.GlobalParams
	if rf, ok := ret.Get(0).(func() protocol.GlobalParams); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.GlobalParams)
		}
	}

	return r0
}

// KVStoreAtBlockID provides a mock function with given fields: blockID
func (_m *MutableProtocolState) KVStoreAtBlockID(blockID flow.Identifier) (protocol.KVStoreReader, error) {
	ret := _m.Called(blockID)

	var r0 protocol.KVStoreReader
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (protocol.KVStoreReader, error)); ok {
		return rf(blockID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) protocol.KVStoreReader); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.KVStoreReader)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMutableProtocolState interface {
	mock.TestingT
	Cleanup(func())
}

// NewMutableProtocolState creates a new instance of MutableProtocolState. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMutableProtocolState(t mockConstructorTestingTNewMutableProtocolState) *MutableProtocolState {
	mock := &MutableProtocolState{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}