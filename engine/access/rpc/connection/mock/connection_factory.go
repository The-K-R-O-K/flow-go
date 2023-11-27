// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	access "github.com/onflow/flow/protobuf/go/flow/access"

	crypto "github.com/onflow/flow-go/crypto"

	execution "github.com/onflow/flow/protobuf/go/flow/execution"

	io "io"

	mock "github.com/stretchr/testify/mock"
)

// ConnectionFactory is an autogenerated mock type for the ConnectionFactory type
type ConnectionFactory struct {
	mock.Mock
}

// GetAccessAPIClient provides a mock function with given fields: address, networkPubKey
func (_m *ConnectionFactory) GetAccessAPIClient(address string, networkPubKey crypto.PublicKey) (access.AccessAPIClient, io.Closer, error) {
	ret := _m.Called(address, networkPubKey)

	var r0 access.AccessAPIClient
	var r1 io.Closer
	var r2 error
	if rf, ok := ret.Get(0).(func(string, crypto.PublicKey) (access.AccessAPIClient, io.Closer, error)); ok {
		return rf(address, networkPubKey)
	}
	if rf, ok := ret.Get(0).(func(string, crypto.PublicKey) access.AccessAPIClient); ok {
		r0 = rf(address, networkPubKey)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(access.AccessAPIClient)
		}
	}

	if rf, ok := ret.Get(1).(func(string, crypto.PublicKey) io.Closer); ok {
		r1 = rf(address, networkPubKey)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(io.Closer)
		}
	}

	if rf, ok := ret.Get(2).(func(string, crypto.PublicKey) error); ok {
		r2 = rf(address, networkPubKey)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetAccessAPIClientWithPort provides a mock function with given fields: address, networkPubKey
func (_m *ConnectionFactory) GetAccessAPIClientWithPort(address string, networkPubKey crypto.PublicKey) (access.AccessAPIClient, io.Closer, error) {
	ret := _m.Called(address, networkPubKey)

	var r0 access.AccessAPIClient
	var r1 io.Closer
	var r2 error
	if rf, ok := ret.Get(0).(func(string, crypto.PublicKey) (access.AccessAPIClient, io.Closer, error)); ok {
		return rf(address, networkPubKey)
	}
	if rf, ok := ret.Get(0).(func(string, crypto.PublicKey) access.AccessAPIClient); ok {
		r0 = rf(address, networkPubKey)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(access.AccessAPIClient)
		}
	}

	if rf, ok := ret.Get(1).(func(string, crypto.PublicKey) io.Closer); ok {
		r1 = rf(address, networkPubKey)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(io.Closer)
		}
	}

	if rf, ok := ret.Get(2).(func(string, crypto.PublicKey) error); ok {
		r2 = rf(address, networkPubKey)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetExecutionAPIClient provides a mock function with given fields: address
func (_m *ConnectionFactory) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error) {
	ret := _m.Called(address)

	var r0 execution.ExecutionAPIClient
	var r1 io.Closer
	var r2 error
	if rf, ok := ret.Get(0).(func(string) (execution.ExecutionAPIClient, io.Closer, error)); ok {
		return rf(address)
	}
	if rf, ok := ret.Get(0).(func(string) execution.ExecutionAPIClient); ok {
		r0 = rf(address)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(execution.ExecutionAPIClient)
		}
	}

	if rf, ok := ret.Get(1).(func(string) io.Closer); ok {
		r1 = rf(address)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(io.Closer)
		}
	}

	if rf, ok := ret.Get(2).(func(string) error); ok {
		r2 = rf(address)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

type mockConstructorTestingTNewConnectionFactory interface {
	mock.TestingT
	Cleanup(func())
}

// NewConnectionFactory creates a new instance of ConnectionFactory. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewConnectionFactory(t mockConstructorTestingTNewConnectionFactory) *ConnectionFactory {
	mock := &ConnectionFactory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
