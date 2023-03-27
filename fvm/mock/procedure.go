// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	fvm "github.com/onflow/flow-go/fvm"
	derived "github.com/onflow/flow-go/fvm/derived"

	mock "github.com/stretchr/testify/mock"

	storage "github.com/onflow/flow-go/fvm/storage"
)

// Procedure is an autogenerated mock type for the Procedure type
type Procedure struct {
	mock.Mock
}

// ComputationLimit provides a mock function with given fields: ctx
func (_m *Procedure) ComputationLimit(ctx fvm.Context) uint64 {
	ret := _m.Called(ctx)

	var r0 uint64
	if rf, ok := ret.Get(0).(func(fvm.Context) uint64); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// ExecutionTime provides a mock function with given fields:
func (_m *Procedure) ExecutionTime() derived.LogicalTime {
	ret := _m.Called()

	var r0 derived.LogicalTime
	if rf, ok := ret.Get(0).(func() derived.LogicalTime); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(derived.LogicalTime)
	}

	return r0
}

// MemoryLimit provides a mock function with given fields: ctx
func (_m *Procedure) MemoryLimit(ctx fvm.Context) uint64 {
	ret := _m.Called(ctx)

	var r0 uint64
	if rf, ok := ret.Get(0).(func(fvm.Context) uint64); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// NewExecutor provides a mock function with given fields: ctx, txnState
func (_m *Procedure) NewExecutor(ctx fvm.Context, txnState storage.Transaction) fvm.ProcedureExecutor {
	ret := _m.Called(ctx, txnState)

	var r0 fvm.ProcedureExecutor
	if rf, ok := ret.Get(0).(func(fvm.Context, storage.Transaction) fvm.ProcedureExecutor); ok {
		r0 = rf(ctx, txnState)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(fvm.ProcedureExecutor)
		}
	}

	return r0
}

// SetOutput provides a mock function with given fields: output
func (_m *Procedure) SetOutput(output fvm.ProcedureOutput) {
	_m.Called(output)
}

// ShouldDisableMemoryAndInteractionLimits provides a mock function with given fields: ctx
func (_m *Procedure) ShouldDisableMemoryAndInteractionLimits(ctx fvm.Context) bool {
	ret := _m.Called(ctx)

	var r0 bool
	if rf, ok := ret.Get(0).(func(fvm.Context) bool); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Type provides a mock function with given fields:
func (_m *Procedure) Type() fvm.ProcedureType {
	ret := _m.Called()

	var r0 fvm.ProcedureType
	if rf, ok := ret.Get(0).(func() fvm.ProcedureType); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(fvm.ProcedureType)
	}

	return r0
}

type mockConstructorTestingTNewProcedure interface {
	mock.TestingT
	Cleanup(func())
}

// NewProcedure creates a new instance of Procedure. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewProcedure(t mockConstructorTestingTNewProcedure) *Procedure {
	mock := &Procedure{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}