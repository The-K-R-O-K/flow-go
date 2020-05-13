// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type State struct {
	metrics    module.ComplianceMetrics
	db         *badger.DB
	clusters   uint
	headers    storage.Headers
	identities storage.Identities
	seals      storage.Seals
	payloads   storage.Payloads
	blocks     storage.Blocks
	expiry     uint
}

// NewState initializes a new state backed by a badger database, applying the
// optional configuration parameters.
func NewState(metrics module.ComplianceMetrics, db *badger.DB, headers storage.Headers, identities storage.Identities, seals storage.Seals, payloads storage.Payloads, blocks storage.Blocks, options ...func(*State)) (*State, error) {
	s := &State{
		metrics:    metrics,
		db:         db,
		clusters:   1,
		headers:    headers,
		identities: identities,
		seals:      seals,
		payloads:   payloads,
		blocks:     blocks,
		expiry:     flow.DefaultTransactionExpiry,
	}
	for _, option := range options {
		option(s)
	}

	if s.clusters < 1 {
		return nil, fmt.Errorf("must have at least one cluster)")
	}

	return s, nil
}

func (s *State) Sealed() protocol.Snapshot {

	// retrieve the latest sealed height
	var sealed uint64
	err := s.db.View(operation.RetrieveSealedHeight(&sealed))
	if err != nil {
		return &Snapshot{err: fmt.Errorf("could not retrieve sealed height: %w", err)}
	}

	return s.AtHeight(sealed)
}

func (s *State) Final() protocol.Snapshot {

	// retrieve the latest finalized height
	var finalized uint64
	err := s.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return &Snapshot{err: fmt.Errorf("could not retrieve finalized height: %w", err)}
	}

	return s.AtHeight(finalized)
}

func (s *State) AtHeight(height uint64) protocol.Snapshot {

	// retrieve the block ID for the finalized height
	var blockID flow.Identifier
	err := s.db.View(operation.LookupBlockHeight(height, &blockID))
	if err != nil {
		return &Snapshot{err: fmt.Errorf("could not look up block by height: %w", err)}
	}

	return s.AtBlockID(blockID)
}

func (s *State) AtBlockID(blockID flow.Identifier) protocol.Snapshot {
	snapshot := &Snapshot{
		state:   s,
		blockID: blockID,
	}
	return snapshot
}

func (s *State) Mutate() protocol.Mutator {
	m := &Mutator{
		state: s,
	}
	return m
}
