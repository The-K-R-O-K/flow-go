package badger_test

import (
	"errors"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestIndexStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewIndex(metrics, db)

		blockID := unittest.IdentifierFixture()
		expected := unittest.IndexFixture()

		// retreive without store
		_, err := store.ByBlockID(blockID)
		require.True(t, errors.Is(err, storage.ErrNotFound))

		// store index
		err = store.Store(blockID, expected)
		require.NoError(t, err)

		// retreive index
		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

// Test that we can store and retrieve indexes from a compressed database
func TestIndexStoreRetrieveCompaction(t *testing.T) {
	dbDir := unittest.TempDir(t)
	defer func() {
		require.NoError(t, os.RemoveAll(dbDir))
	}()

	opts := badger.
		DefaultOptions(dbDir).
		WithKeepL0InMemory(true).
		WithLogger(nil)
	db, err := badger.Open(opts)
	require.NoError(t, err)

	metrics := metrics.NewNoopCollector()
	store := badgerstorage.NewIndex(metrics, db)

	blockID := unittest.IdentifierFixture()
	expected := unittest.IndexFixture()

	// retreive without store
	_, err = store.ByBlockID(blockID)
	require.True(t, errors.Is(err, storage.ErrNotFound))

	// store index
	err = store.Store(blockID, expected)
	require.NoError(t, err)

	// retreive index
	actual, err := store.ByBlockID(blockID)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.NoError(t, db.Close())

	// reopen the database with compression
	compressed := badger.
		DefaultOptions(dbDir).
		WithCompression(options.Snappy).
		WithKeepL0InMemory(true).
		WithLogger(nil)
	cdb, err := badger.Open(compressed)
	require.NoError(t, err)

	cstore := badgerstorage.NewIndex(metrics, cdb)

	// retreive index using the compressed database, and
	// ensure that it is able to read the data saved by un-compressed database
	actual, err = cstore.ByBlockID(blockID)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// store a different index using the compressed database,
	// ensure that the compressed index can be retrieved.
	blockID2 := unittest.IdentifierFixture()
	expected2 := unittest.IndexFixture()

	err = cstore.Store(blockID2, expected2)
	require.NoError(t, err)

	actual2, err := cstore.ByBlockID(blockID2)
	require.NoError(t, err)
	require.Equal(t, expected2, actual2)
	require.NoError(t, cdb.Close())
}
