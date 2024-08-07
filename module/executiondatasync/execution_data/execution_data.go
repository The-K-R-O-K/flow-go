package execution_data

import (
	"errors"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// ExecutionDataDBMode controls which db type to use.
type ExecutionDataDBMode int

const (
	// ExecutionDataDBModeBadger uses badger db
	ExecutionDataDBModeBadger ExecutionDataDBMode = iota + 1

	// ExecutionDataDBModePebble uses pebble db
	ExecutionDataDBModePebble
)

func ParseExecutionDataDBMode(s string) (ExecutionDataDBMode, error) {
	switch s {
	case ExecutionDataDBModeBadger.String():
		return ExecutionDataDBModeBadger, nil
	case ExecutionDataDBModePebble.String():
		return ExecutionDataDBModePebble, nil
	default:
		return 0, errors.New("invalid execution data DB mode")
	}
}

func (m ExecutionDataDBMode) String() string {
	switch m {
	case ExecutionDataDBModeBadger:
		return "badger"
	case ExecutionDataDBModePebble:
		return "pebble"
	default:
		return ""
	}
}

// DefaultMaxBlobSize is the default maximum size of a blob.
// This is calibrated to fit within a libp2p message and not exceed the max size recommended by bitswap.
const DefaultMaxBlobSize = 1 << 20 // 1MiB

// ChunkExecutionData represents the execution data of a chunk
type ChunkExecutionData struct {
	// Collection is the collection for which this chunk was executed
	Collection *flow.Collection

	// Events are the events generated by executing the collection
	Events flow.EventsList

	// TrieUpdate is the trie update generated by executing the collection
	// This includes a list of all registers updated during the execution
	TrieUpdate *ledger.TrieUpdate

	// TransactionResults are the results of executing the transactions in the collection
	// This includes all of the data from flow.TransactionResult, except that it uses a boolean
	// value to indicate if an error occurred instead of a full error message.
	TransactionResults []flow.LightTransactionResult
}

// BlockExecutionData represents the execution data of a block.
type BlockExecutionData struct {
	BlockID             flow.Identifier
	ChunkExecutionDatas []*ChunkExecutionData
}

// ConvertTransactionResults converts a list of flow.TransactionResults into a list of
// flow.LightTransactionResults to be included in a ChunkExecutionData.
func ConvertTransactionResults(results flow.TransactionResults) []flow.LightTransactionResult {
	if len(results) == 0 {
		return nil
	}

	converted := make([]flow.LightTransactionResult, len(results))
	for i, txResult := range results {
		converted[i] = flow.LightTransactionResult{
			TransactionID:   txResult.TransactionID,
			ComputationUsed: txResult.ComputationUsed,
			Failed:          txResult.ErrorMessage != "",
		}
	}
	return converted
}
