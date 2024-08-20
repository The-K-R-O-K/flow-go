package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// RegisterIndex defines methods for the register index.
type RegisterIndex interface {
	// Get register by the register ID at a given block height.
	//
	// If the register at the given height was not indexed, returns the highest
	// height the register was indexed at.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the given height was not indexed yet or lower than the first indexed height.
	// - storage.ErrNotFound if the given height is indexed, but the register does not exist.
	Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)

	// LatestHeight returns the latest indexed height. Returns the latest indexed height found in the cache.
	LatestHeight() uint64

	// FirstHeight at which we started to index. Returns the first indexed height found in the cache.
	FirstHeight() uint64

	// Store batch of register entries at the provided block height.
	//
	// The provided height must either be one higher than the current height or the same to ensure idempotency,
	// otherwise and error is returned. If the height is not within those bounds there is either a bug
	// or state corruption.
	//
	// No errors are expected during normal operation.
	Store(entries flow.RegisterEntries, height uint64) error

	// FirstStoredHeight at which we started to index. Returns the first indexed height found in the database.
	//
	// No errors are expected during normal operation.
	FirstStoredHeight() (uint64, error)

	// LatestStoredHeight returns the latest indexed height. Returns the latest indexed height found in the database.
	//
	// No errors are expected during normal operation.
	LatestStoredHeight() (uint64, error)

	// UpdateFirstStoredHeight updates the first height value,
	// It is up to the caller to ensure that this is never
	// called with a value lower than the pruned height.
	//
	// No errors are expected during normal operation.
	UpdateFirstStoredHeight(uint64) error

	// PruneUpToHeight removes all data from storage corresponding
	// to block heights up to and including the given height.
	//
	// No errors are expected during normal operation.
	PruneUpToHeight(uint64) error
}
