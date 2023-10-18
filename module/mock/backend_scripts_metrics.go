// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// BackendScriptsMetrics is an autogenerated mock type for the BackendScriptsMetrics type
type BackendScriptsMetrics struct {
	mock.Mock
}

// ScriptExecuted provides a mock function with given fields: dur, size
func (_m *BackendScriptsMetrics) ScriptExecuted(dur time.Duration, size int) {
	_m.Called(dur, size)
}

// ScriptExecutionErrorLocal provides a mock function with given fields:
func (_m *BackendScriptsMetrics) ScriptExecutionErrorLocal() {
	_m.Called()
}

// ScriptExecutionErrorMatch provides a mock function with given fields:
func (_m *BackendScriptsMetrics) ScriptExecutionErrorMatch() {
	_m.Called()
}

// ScriptExecutionErrorMismatch provides a mock function with given fields:
func (_m *BackendScriptsMetrics) ScriptExecutionErrorMismatch() {
	_m.Called()
}

// ScriptExecutionErrorOnExecutionNode provides a mock function with given fields:
func (_m *BackendScriptsMetrics) ScriptExecutionErrorOnExecutionNode() {
	_m.Called()
}

// ScriptExecutionResultMatch provides a mock function with given fields:
func (_m *BackendScriptsMetrics) ScriptExecutionResultMatch() {
	_m.Called()
}

// ScriptExecutionResultMismatch provides a mock function with given fields:
func (_m *BackendScriptsMetrics) ScriptExecutionResultMismatch() {
	_m.Called()
}

type mockConstructorTestingTNewBackendScriptsMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewBackendScriptsMetrics creates a new instance of BackendScriptsMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewBackendScriptsMetrics(t mockConstructorTestingTNewBackendScriptsMetrics) *BackendScriptsMetrics {
	mock := &BackendScriptsMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
