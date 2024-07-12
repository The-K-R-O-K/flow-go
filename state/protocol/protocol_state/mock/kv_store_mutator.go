// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	protocol "github.com/onflow/flow-go/state/protocol"
)

// KVStoreMutator is an autogenerated mock type for the KVStoreMutator type
type KVStoreMutator struct {
	mock.Mock
}

// GetEpochExtensionViewCount provides a mock function with given fields:
func (_m *KVStoreMutator) GetEpochExtensionViewCount() uint64 {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetEpochExtensionViewCount")
	}

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// GetEpochStateID provides a mock function with given fields:
func (_m *KVStoreMutator) GetEpochStateID() flow.Identifier {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetEpochStateID")
	}

	var r0 flow.Identifier
	if rf, ok := ret.Get(0).(func() flow.Identifier); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	return r0
}

// GetProtocolStateVersion provides a mock function with given fields:
func (_m *KVStoreMutator) GetProtocolStateVersion() uint64 {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetProtocolStateVersion")
	}

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// GetVersionUpgrade provides a mock function with given fields:
func (_m *KVStoreMutator) GetVersionUpgrade() *protocol.ViewBasedActivator[uint64] {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetVersionUpgrade")
	}

	var r0 *protocol.ViewBasedActivator[uint64]
	if rf, ok := ret.Get(0).(func() *protocol.ViewBasedActivator[uint64]); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*protocol.ViewBasedActivator[uint64])
		}
	}

	return r0
}

// ID provides a mock function with given fields:
func (_m *KVStoreMutator) ID() flow.Identifier {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ID")
	}

	var r0 flow.Identifier
	if rf, ok := ret.Get(0).(func() flow.Identifier); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	return r0
}

// SetEpochStateID provides a mock function with given fields: stateID
func (_m *KVStoreMutator) SetEpochStateID(stateID flow.Identifier) {
	_m.Called(stateID)
}

// SetVersionUpgrade provides a mock function with given fields: version
func (_m *KVStoreMutator) SetVersionUpgrade(version *protocol.ViewBasedActivator[uint64]) {
	_m.Called(version)
}

// VersionedEncode provides a mock function with given fields:
func (_m *KVStoreMutator) VersionedEncode() (uint64, []byte, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for VersionedEncode")
	}

	var r0 uint64
	var r1 []byte
	var r2 error
	if rf, ok := ret.Get(0).(func() (uint64, []byte, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func() []byte); ok {
		r1 = rf()
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]byte)
		}
	}

	if rf, ok := ret.Get(2).(func() error); ok {
		r2 = rf()
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// NewKVStoreMutator creates a new instance of KVStoreMutator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewKVStoreMutator(t interface {
	mock.TestingT
	Cleanup(func())
}) *KVStoreMutator {
	mock := &KVStoreMutator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
