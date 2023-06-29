// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	computer "github.com/onflow/flow-go/engine/execution/computation/computer"
	fvm "github.com/onflow/flow-go/fvm"

	mock "github.com/stretchr/testify/mock"

	snapshot "github.com/onflow/flow-go/fvm/storage/snapshot"

	time "time"
)

// TransactionWriteBehindLogger is an autogenerated mock type for the TransactionWriteBehindLogger type
type TransactionWriteBehindLogger struct {
	mock.Mock
}

// AddTransactionResult provides a mock function with given fields: txn, _a1, output, timeSpent, numTxnConflictRetries
func (_m *TransactionWriteBehindLogger) AddTransactionResult(txn computer.TransactionRequest, _a1 *snapshot.ExecutionSnapshot, output fvm.ProcedureOutput, timeSpent time.Duration, numTxnConflictRetries int) {
	_m.Called(txn, _a1, output, timeSpent, numTxnConflictRetries)
}

type mockConstructorTestingTNewTransactionWriteBehindLogger interface {
	mock.TestingT
	Cleanup(func())
}

// NewTransactionWriteBehindLogger creates a new instance of TransactionWriteBehindLogger. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewTransactionWriteBehindLogger(t mockConstructorTestingTNewTransactionWriteBehindLogger) *TransactionWriteBehindLogger {
	mock := &TransactionWriteBehindLogger{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}