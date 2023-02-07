package unittest

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/storage"
)

func CreateMockStore() *PayloadStore {
	return &PayloadStore{
		stored: make(map[hash.Hash][]byte),
	}
}

func CreateMockPayloadStore() *storage.PayloadStorage {
	store := CreateMockStore()
	return storage.NewPayloadStorage(store)
}

// a mock key-value storage
type PayloadStore struct {
	sync.RWMutex
	stored map[hash.Hash][]byte
}

func (s *PayloadStore) Get(hash hash.Hash) ([]byte, error) {
	s.RLock()
	defer s.RUnlock()
	node, found := s.stored[hash]
	if !found {
		return nil, fmt.Errorf("key not found: %v", hash)
	}

	// return the copied data
	buf := make([]byte, len(node))
	copy(buf[:], node)
	return buf, nil
}

func (s *PayloadStore) GetMul(hashs []hash.Hash) ([][]byte, error) {
	s.RLock()
	defer s.RUnlock()

	missingHashs := make([]hash.Hash, 0, len(hashs))
	values := make([][]byte, len(hashs))
	for i, hash := range hashs {
		node, found := s.stored[hash]
		if !found {
			missingHashs = append(missingHashs, hash)
			continue
		}

		values[i] = node
	}

	if len(missingHashs) > 0 {
		return nil, fmt.Errorf("keys not found %v", missingHashs)
	}
	return values, nil
}

func (s *PayloadStore) SetMul(keys []hash.Hash, values [][]byte) error {
	s.Lock()
	defer s.Unlock()

	for i, key := range keys {
		value := values[i]
		s.stored[key] = value
	}
	return nil
}

func (s *PayloadStore) Count() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.stored)
}
