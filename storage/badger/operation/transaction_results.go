// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

func InsertTransactionResult(blockID flow.Identifier, transactionResult *flow.TransactionResult) func(*badger.Txn) error {
	return insert(makePrefix(codeTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchInsertTransactionResult(blockID flow.Identifier, transactionResult *flow.TransactionResult) func(batch *badger.WriteBatch) error {
	return batchInsert(makePrefix(codeTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchIndexTransactionResult(blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) func(batch *badger.WriteBatch) error {
	return batchInsert(makePrefix(codeTransactionResultIndex, blockID, txIndex), transactionResult)
}

func RetrieveTransactionResult(blockID flow.Identifier, transactionID flow.Identifier, transactionResult *flow.TransactionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransactionResult, blockID, transactionID), transactionResult)
}
func RetrieveTransactionResultByIndex(blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransactionResultIndex, blockID, txIndex), transactionResult)
}

func LookupTransactionResultsByBlockID(blockID flow.Identifier, txResults *[]flow.TransactionResult) func(*badger.Txn) error {

	txErrIterFunc := func() (checkFunc, createFunc, handleFunc) {
		check := func(_ []byte) bool {
			return true
		}
		var val flow.TransactionResult
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			*txResults = append(*txResults, val)
			return nil
		}
		return check, create, handle
	}

	return traverse(makePrefix(codeTransactionResult, blockID), txErrIterFunc)
}

// LookupTransactionResultsByBlockIDUsingIndex retrieves all tx results for a block, but using
// tx_index index. This correctly handles cases of duplicate transactions within block, and should
// eventually replace uses of LookupTransactionResultsByBlockID
func LookupTransactionResultsByBlockIDUsingIndex(blockID flow.Identifier, txResults *[]flow.TransactionResult) func(*badger.Txn) error {

	txErrIterFunc := func() (checkFunc, createFunc, handleFunc) {
		check := func(_ []byte) bool {
			return true
		}
		var val flow.TransactionResult
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			*txResults = append(*txResults, val)
			return nil
		}
		return check, create, handle
	}

	return traverse(makePrefix(codeTransactionResultIndex, blockID), txErrIterFunc)
}

// RemoveTransactionResultsByBlockID removes the transaction results for the given blockID
func RemoveTransactionResultsByBlockID(blockID flow.Identifier) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		var txResults []flow.TransactionResult

		err := SkipNonExist(LookupTransactionResultsByBlockID(blockID, &txResults))(txn)
		// don't return error if there is no result for the block
		if err != nil {
			return fmt.Errorf("could not find transaction results for block: %v, %w", blockID, err)
		}

		for _, result := range txResults {
			err := RemoveTransactionResult(blockID, result.TransactionID)(txn)
			if err != nil {
				return fmt.Errorf("could not remove transaction result for block %v: %w", blockID, err)
			}
		}

		return nil
	}
}

// RemoveTransactionResult removes the transaction result by blockID and transaction result ID
func RemoveTransactionResult(blockID flow.Identifier, transactionResultID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeTransactionResult, blockID, transactionResultID))
}
