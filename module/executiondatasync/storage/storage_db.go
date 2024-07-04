package storage

import (
	"github.com/ipfs/go-datastore"
)

// TODO: rename
// StorageDB defines the interface for key-value store operations.
type StorageDB interface {
	Datastore() datastore.Batching

	Get(key []byte) (StorageItem, error)
	Set(key, val []byte) error
	Delete(key []byte) error
	Close() error

	Keys(prefix []byte) ([][]byte, error)

	RetryOnConflict(fn func() error) error
	MaxBatchCount() int64
	MaxBatchSize() int64
	RunValueLogGC(discardRatio float64) error
}

type StorageItem struct {
	key []byte
	val []byte
}

func (i *StorageItem) ValueCopy(dst []byte) ([]byte, error) {
	return append(dst, i.val...), nil
}

func (i *StorageItem) Key() []byte {
	return i.key
}