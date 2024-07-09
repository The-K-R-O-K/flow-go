package consensus

import (
	"math/rand"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	mockprot "github.com/onflow/flow-go/state/protocol/mock"
	mockstor "github.com/onflow/flow-go/storage/mock"
	storage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestNewFinalizerPebble(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		headers := &mockstor.Headers{}
		state := &mockprot.FollowerState{}
		tracer := trace.NewNoopTracer()
		fin := NewFinalizerPebble(db, headers, state, tracer)
		assert.Equal(t, fin.db, db)
		assert.Equal(t, fin.headers, headers)
		assert.Equal(t, fin.state, state)
	})
}

// TestMakeFinalValidChain checks whether calling `MakeFinal` with the ID of a valid
// descendant block of the latest finalized header results in the finalization of the
// valid descendant and all of its parents up to the finalized header, but excluding
// the children of the valid descendant.
func TestMakeFinalValidChainPebble(t *testing.T) {

	// create one block that we consider the last finalized
	final := unittest.BlockHeaderFixture()
	final.Height = uint64(rand.Uint32())

	// generate a couple of children that are pending
	parent := final
	var pending []*flow.Header
	total := 8
	for i := 0; i < total; i++ {
		header := unittest.BlockHeaderFixture()
		header.Height = parent.Height + 1
		header.ParentID = parent.ID()
		pending = append(pending, header)
		parent = header
	}

	// create a mock protocol state to check finalize calls
	state := mockprot.NewFollowerState(t)

	// make sure we get a finalize call for the blocks that we want to
	cutoff := total - 3
	var lastID flow.Identifier
	for i := 0; i < cutoff; i++ {
		state.On("Finalize", mock.Anything, pending[i].ID()).Return(nil)
		lastID = pending[i].ID()
	}

	// this will hold the IDs of blocks clean up
	var list []flow.Identifier

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		// insert the latest finalized height
		err := operation.InsertFinalizedHeight(final.Height)(db)
		require.NoError(t, err)

		// map the finalized height to the finalized block ID
		err = operation.IndexBlockHeight(final.Height, final.ID())(db)
		require.NoError(t, err)

		// insert the finalized block header into the DB
		err = operation.InsertHeader(final.ID(), final)(db)
		require.NoError(t, err)

		// insert all of the pending blocks into the DB
		for _, header := range pending {
			err = operation.InsertHeader(header.ID(), header)(db)
			require.NoError(t, err)
		}

		// initialize the finalizer with the dependencies and make the call
		metrics := metrics.NewNoopCollector()
		fin := FinalizerPebble{
			db:      db,
			headers: storage.NewHeaders(metrics, db),
			state:   state,
			tracer:  trace.NewNoopTracer(),
			cleanup: LogCleanup(&list),
		}
		err = fin.MakeFinal(lastID)
		require.NoError(t, err)
	})

	// make sure that finalize was called on protocol state for all desired blocks
	state.AssertExpectations(t)

	// make sure that cleanup was called for all of them too
	assert.ElementsMatch(t, list, flow.GetIDs(pending[:cutoff]))
}

// TestMakeFinalInvalidHeight checks whether we receive an error when calling `MakeFinal`
// with a header that is at the same height as the already highest finalized header.
func TestMakeFinalInvalidHeightPebble(t *testing.T) {

	// create one block that we consider the last finalized
	final := unittest.BlockHeaderFixture()
	final.Height = uint64(rand.Uint32())

	// generate an alternative block at same height
	pending := unittest.BlockHeaderFixture()
	pending.Height = final.Height

	// create a mock protocol state to check finalize calls
	state := mockprot.NewFollowerState(t)

	// this will hold the IDs of blocks clean up
	var list []flow.Identifier

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		// insert the latest finalized height
		err := operation.InsertFinalizedHeight(final.Height)(db)
		require.NoError(t, err)

		// map the finalized height to the finalized block ID
		err = operation.IndexBlockHeight(final.Height, final.ID())(db)
		require.NoError(t, err)

		// insert the finalized block header into the DB
		err = operation.InsertHeader(final.ID(), final)(db)
		require.NoError(t, err)

		// insert all of the pending header into DB
		err = operation.InsertHeader(pending.ID(), pending)(db)
		require.NoError(t, err)

		// initialize the finalizer with the dependencies and make the call
		metrics := metrics.NewNoopCollector()
		fin := FinalizerPebble{
			db:      db,
			headers: storage.NewHeaders(metrics, db),
			state:   state,
			tracer:  trace.NewNoopTracer(),
			cleanup: LogCleanup(&list),
		}
		err = fin.MakeFinal(pending.ID())
		require.Error(t, err)
	})

	// make sure that nothing was finalized
	state.AssertExpectations(t)

	// make sure no cleanup was done
	assert.Empty(t, list)
}

// TestMakeFinalDuplicate checks whether calling `MakeFinal` with the ID of the currently
// highest finalized header is a no-op and does not result in an error.
func TestMakeFinalDuplicatePebble(t *testing.T) {

	// create one block that we consider the last finalized
	final := unittest.BlockHeaderFixture()
	final.Height = uint64(rand.Uint32())

	// create a mock protocol state to check finalize calls
	state := mockprot.NewFollowerState(t)

	// this will hold the IDs of blocks clean up
	var list []flow.Identifier

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		// insert the latest finalized height
		err := operation.InsertFinalizedHeight(final.Height)(db)
		require.NoError(t, err)

		// map the finalized height to the finalized block ID
		err = operation.IndexBlockHeight(final.Height, final.ID())(db)
		require.NoError(t, err)

		// insert the finalized block header into the DB
		err = operation.InsertHeader(final.ID(), final)(db)
		require.NoError(t, err)

		// initialize the finalizer with the dependencies and make the call
		metrics := metrics.NewNoopCollector()
		fin := FinalizerPebble{
			db:      db,
			headers: storage.NewHeaders(metrics, db),
			state:   state,
			tracer:  trace.NewNoopTracer(),
			cleanup: LogCleanup(&list),
		}
		err = fin.MakeFinal(final.ID())
		require.NoError(t, err)
	})

	// make sure that nothing was finalized
	state.AssertExpectations(t)

	// make sure no cleanup was done
	assert.Empty(t, list)
}
