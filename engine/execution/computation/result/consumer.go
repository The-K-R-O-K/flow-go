package result

import (
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type ExecutableCollection interface {
	// BlockHeader returns the block header in which collection was included
	BlockHeader() *flow.Header

	// Collection returns the content of the collection
	Collection() *flow.Collection

	// CollectionIndex returns the index of collection in the block
	CollectionIndex() int

	// IsSystemCollection returns true if the collection is the last collection of the block
	IsSystemCollection() bool
}

// ExecutedCollection holds results of a collection execution
type ExecutedCollection interface {
	ExecutableCollection

	// RegisterUpdates returns all registers that were updated during collection execution
	UpdatedRegisters() flow.RegisterEntries

	// ReadRegisterIDs returns all registers that has been read during collection execution
	ReadRegisterIDs() flow.RegisterIDs

	// EmittedEvents returns a list of all the events emitted during collection execution
	EmittedEvents() flow.EventsList

	// ServiceEventList returns a list of only service events emitted during this collection
	ServiceEventList() flow.EventsList

	// ConvertedServiceEvents returns a list of converted service events
	ConvertedServiceEvents() flow.ServiceEventList

	// TransactionResults returns a list of transaction results
	TransactionResults() flow.TransactionResults

	// SpockData returns the spock data that is collected during collection execution
	SpockData() []byte

	// TotalComputationUsed returns the total computation used
	TotalComputationUsed() uint

	// TODO(ramtin): replace me with other methods
	// temp method before changing the committer to be dependent on a different interface
	ExecutionSnapshot() *state.ExecutionSnapshot
}

// ExecutedCollectionConsumer consumes ExecutedCollections
type ExecutedCollectionConsumer interface {
	module.ReadyDoneAware
	OnExecutedCollection(ec ExecutedCollection) error
}

// AttestedCollection holds results of a collection attestation
type AttestedCollection interface {
	ExecutedCollection

	// StartStateCommitment returns a commitment to the state before collection execution
	StartStateCommitment() flow.StateCommitment

	// EndStateCommitment returns a commitment to the state after collection execution
	EndStateCommitment() flow.StateCommitment

	// StateProof returns state proofs that could be used to build a partial trie
	StateProof() flow.StorageProof

	// TODO(ramtin): unlock these
	// // StateDeltaCommitment returns a commitment over the state delta
	// StateDeltaCommitment()  flow.Identifier

	// // TxResultListCommitment returns a commitment over the list of transaction results
	// TxResultListCommitment() flow.Identifier

	// EventCommitment returns commitment over eventList
	EventListCommitment() flow.Identifier
}

// AttestedCollectionConsumer consumes AttestedCollection
type AttestedCollectionConsumer interface {
	module.ReadyDoneAware
	OnAttestedCollection(ac AttestedCollection) error
}

type ExecutedBlock interface {
	// BlockHeader returns the block header in which collection was included
	BlockHeader() *flow.Header

	// Receipt returns the execution receipt
	Receipt() *flow.ExecutionReceipt

	// AttestedCollections returns attested collections
	//
	// TODO(ramtin): this could be reduced, currently we need this
	// to store chunk data packs, trie updates package used by access nodes,
	AttestedCollections() []AttestedCollection
}

// ExecutedBlockConsumer consumes ExecutedBlock
type ExecutedBlockConsumer interface {
	module.ReadyDoneAware
	OnExecutedBlock(eb ExecutedBlock) error
}
