package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// IterateRange defines the range of blocks to iterate over
// the range could be either view based range or height based range.
// when specifying the range, the start and end are inclusive, and the end must be greater than or
// equal to the start
type IterateRange struct {
	Start uint64 // the start of the range
	End   uint64 // the end of the range
}

// IterateProgressReader reads the progress of the iterator, useful for resuming the iteration
// after restart
type IterateProgressReader interface {
	// LoadState reads the next block to iterate
	// caller must ensure the reader is created by the IterateProgressInitializer,
	// otherwise LoadState would return exception.
	LoadState() (uint64, error)
}

// IterateProgressWriter saves the progress of the iterator
type IterateProgressWriter interface {
	// SaveState persists the next block to be iterated
	SaveState(uint64) error
}

// BlockIterator is an interface for iterating over blocks
type BlockIterator interface {
	// Next returns the next block in the iterator
	// Note: this method is not concurrent-safe
	// Note: a block will only be iterated once in a single iteration, however
	// if the iteration is interrupted (e.g. by a restart), the iterator can be
	// resumed from the last checkpoint, which might result in the same block being
	// iterated again.
	// TODO: once upgraded to go 1.23, consider using the Range iterator
	//   Range() iter.Seq2[flow.Identifier, error]
	//   so that the iterator can be used in a for loop:
	//   for blockID, err := range heightIterator.Range()
	Next() (blockID flow.Identifier, hasNext bool, exception error)

	// Checkpoint saves the current state of the iterator
	// so that it can be resumed later
	// when Checkpoint is called, if SaveStateFunc is called with block A,
	// then after restart, the iterator will resume from A.
	// make sure to call this after all the blocks for processing the block IDs returned by
	// Next() are completed.
	Checkpoint() (uint64, error)
}
