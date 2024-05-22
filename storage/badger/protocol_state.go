package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// DefaultProtocolStateCacheSize is the default size for primary protocol state cache.
// Minimally, we have 3 entries per epoch (one on epoch Switchover, one on receiving the Epoch Setup and one when seeing the Epoch Commit event).
// Lets be generous and assume we have 20 different Protocol States per epoch.
var DefaultProtocolStateCacheSize uint = 20

// DefaultProtocolStateByBlockIDCacheSize is the default value for secondary byBlockIdCache.
// We want to be able to cover a broad interval of views without cache misses, so we use a bigger value.
var DefaultProtocolStateByBlockIDCacheSize uint = 1000

// ProtocolState implements persistent storage for storing Protocol States.
// Protocol state uses an embedded cache without storing capabilities(store happens on first retrieval) to avoid unnecessary
// operations and to speed up access to frequently used Protocol State.
type ProtocolState struct {
	db *badger.DB

	// cache is essentially an in-memory map from `EpochProtocolStateEntry.ID()` -> `RichEpochProtocolStateEntry`
	// We do _not_ populate this cache which holds the RichProtocolStateEntrys on store. This is because
	//   (i) we don't have the RichEpochProtocolStateEntry on store readily available and
	//  (ii) new RichEpochProtocolStateEntry are really rare throughout an epoch, so the total cost of populating
	//       the cache becomes negligible over several views.
	// In the future, we might want to populate the cache on store, if we want to maintain frequently-changing
	// information in the protocol state, like the latest sealed block. This should be a smaller amount of work,
	// because the `EpochProtocolStateEntry` is generated by `StateMutator.Build()`. The `StateMutator` should already
	// have the needed Epoch Setup and Commit events, since it starts with a RichEpochProtocolStateEntry for the parent
	// state and consumes Epoch Setup and Epoch Commit events. Though, we leave this optimization for later.
	//
	// `cache` only holds the distinct Protocol States. On the happy path, we expect something like 3 entries per epoch.
	// On the optimal happy path we have 3 entries per epoch: one entry on epoch Switchover, one on receiving the Epoch Setup
	// and one when seeing the Epoch Commit event. Let's be generous and assume we have 20 different Protocol States per epoch.
	// Beyond that, we are certainly leaving the domain of normal operations that we optimize for. Therefore, a cache size of
	// roughly 100 is a reasonable balance between performance and memory consumption.
	cache *Cache[flow.Identifier, *flow.RichEpochProtocolStateEntry]

	// byBlockIdCache is essentially an in-memory map from `Block.ID()` -> `EpochProtocolStateEntry.ID()`. The full
	// Protocol state can be retrieved from the `cache` above.
	// We populate the `byBlockIdCache` on store, because a new entry is added for every block and we probably also
	// query the Protocol state for every block. So argument (ii) from above does not apply here. Furthermore,
	// argument (i) from above also does not apply, because we already have the Protocol State's ID on store,
	// so populating the cache is easy.
	//
	// `byBlockIdCache` will contain an entry for every block. We want to be able to cover a broad interval of views
	// without cache misses, so a cache size of roughly 1000 entries is reasonable.
	byBlockIdCache *Cache[flow.Identifier, flow.Identifier]
}

var _ storage.ProtocolState = (*ProtocolState)(nil)

// NewProtocolState creates a ProtocolState instance, which is a database of Protocol State.
// It supports storing, caching and retrieving by ID or the additionally indexed block ID.
func NewProtocolState(collector module.CacheMetrics,
	epochSetups storage.EpochSetups,
	epochCommits storage.EpochCommits,
	db *badger.DB,
	stateCacheSize uint,
	stateByBlockIDCacheSize uint,
) *ProtocolState {
	retrieveByProtocolStateID := func(protocolStateID flow.Identifier) func(tx *badger.Txn) (*flow.RichEpochProtocolStateEntry, error) {
		var protocolStateEntry flow.EpochProtocolStateEntry
		return func(tx *badger.Txn) (*flow.RichEpochProtocolStateEntry, error) {
			err := operation.RetrieveProtocolState(protocolStateID, &protocolStateEntry)(tx)
			if err != nil {
				return nil, err
			}
			result, err := newRichProtocolStateEntry(&protocolStateEntry, epochSetups, epochCommits)
			if err != nil {
				return nil, fmt.Errorf("could not create rich protocol state entry: %w", err)
			}
			return result, nil
		}
	}

	storeByBlockID := func(blockID flow.Identifier, protocolStateID flow.Identifier) func(*transaction.Tx) error {
		return func(tx *transaction.Tx) error {
			err := transaction.WithTx(operation.IndexProtocolState(blockID, protocolStateID))(tx)
			if err != nil {
				return fmt.Errorf("could not index protocol state for block (%x): %w", blockID[:], err)
			}
			return nil
		}
	}

	retrieveByBlockID := func(blockID flow.Identifier) func(tx *badger.Txn) (flow.Identifier, error) {
		return func(tx *badger.Txn) (flow.Identifier, error) {
			var protocolStateID flow.Identifier
			err := operation.LookupProtocolState(blockID, &protocolStateID)(tx)
			if err != nil {
				return flow.ZeroID, fmt.Errorf("could not lookup protocol state ID for block (%x): %w", blockID[:], err)
			}
			return protocolStateID, nil
		}
	}

	return &ProtocolState{
		db: db,
		cache: newCache[flow.Identifier, *flow.RichEpochProtocolStateEntry](collector, metrics.ResourceProtocolState,
			withLimit[flow.Identifier, *flow.RichEpochProtocolStateEntry](stateCacheSize),
			withStore(noopStore[flow.Identifier, *flow.RichEpochProtocolStateEntry]),
			withRetrieve(retrieveByProtocolStateID)),
		byBlockIdCache: newCache[flow.Identifier, flow.Identifier](collector, metrics.ResourceProtocolStateByBlockID,
			withLimit[flow.Identifier, flow.Identifier](stateByBlockIDCacheSize),
			withStore(storeByBlockID),
			withRetrieve(retrieveByBlockID)),
	}
}

// StoreTx returns an anonymous function (intended to be executed as part of a badger transaction),
// which persists the given protocol state as part of a DB tx. Per convention, the identities in
// the Protocol State must be in canonical order for the current and next epoch (if present),
// otherwise an exception is returned.
// Expected errors of the returned anonymous function:
//   - storage.ErrAlreadyExists if a Protocol State with the given id is already stored
func (s *ProtocolState) StoreTx(protocolStateID flow.Identifier, protocolState *flow.EpochProtocolStateEntry) func(*transaction.Tx) error {
	// front-load sanity checks:
	if !protocolState.CurrentEpoch.ActiveIdentities.Sorted(flow.IdentifierCanonical) {
		return transaction.Fail(fmt.Errorf("sanity check failed: identities are not sorted"))
	}
	if protocolState.NextEpoch != nil && !protocolState.NextEpoch.ActiveIdentities.Sorted(flow.IdentifierCanonical) {
		return transaction.Fail(fmt.Errorf("sanity check failed: next epoch identities are not sorted"))
	}

	// happy path: return anonymous function, whose future execution (as part of a transaction) will store the protocolState
	return transaction.WithTx(operation.InsertProtocolState(protocolStateID, protocolState))
}

// Index returns an anonymous function that is intended to be executed as part of a database transaction.
// In a nutshell, we want to maintain a map from `blockID` to `protocolStateID`, where `blockID` references the
// block that _proposes_ the Protocol State.
// Upon call, the anonymous function persists the specific map entry in the node's database.
// Protocol convention:
//   - Consider block B, whose ingestion might potentially lead to an updated protocol state. For example,
//     the protocol state changes if we seal some execution results emitting service events.
//   - For the key `blockID`, we use the identity of block B which _proposes_ this Protocol State. As value,
//     the hash of the resulting protocol state at the end of processing B is to be used.
//   - CAUTION: The protocol state requires confirmation by a QC and will only become active at the child block,
//     _after_ validating the QC.
//
// Expected errors during normal operations:
//   - storage.ErrAlreadyExists if a Protocol State for the given blockID has already been indexed
func (s *ProtocolState) Index(blockID flow.Identifier, protocolStateID flow.Identifier) func(*transaction.Tx) error {
	return s.byBlockIdCache.PutTx(blockID, protocolStateID)
}

// ByID returns the protocol state by its ID.
// Expected errors during normal operations:
//   - storage.ErrNotFound if no protocol state with the given Identifier is known.
func (s *ProtocolState) ByID(protocolStateID flow.Identifier) (*flow.RichEpochProtocolStateEntry, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	return s.cache.Get(protocolStateID)(tx)
}

// ByBlockID retrieves the Protocol State that the block with the given ID proposes.
// CAUTION: this protocol state requires confirmation by a QC and will only become active at the child block,
// _after_ validating the QC. Protocol convention:
//   - Consider block B, whose ingestion might potentially lead to an updated protocol state. For example,
//     the protocol state changes if we seal some execution results emitting service events.
//   - For the key `blockID`, we use the identity of block B which _proposes_ this Protocol State. As value,
//     the hash of the resulting protocol state at the end of processing B is to be used.
//   - CAUTION: The protocol state requires confirmation by a QC and will only become active at the child block,
//     _after_ validating the QC.
//
// Expected errors during normal operations:
//   - storage.ErrNotFound if no protocol state has been indexed for the given block.
func (s *ProtocolState) ByBlockID(blockID flow.Identifier) (*flow.RichEpochProtocolStateEntry, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	protocolStateID, err := s.byBlockIdCache.Get(blockID)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not lookup protocol state ID for block (%x): %w", blockID[:], err)
	}
	return s.cache.Get(protocolStateID)(tx)
}

// newRichProtocolStateEntry constructs a rich protocol state entry from a protocol state entry.
// It queries and fills in epoch setups and commits for previous and current epochs and possibly next epoch.
// No errors are expected during normal operation.
func newRichProtocolStateEntry(
	protocolState *flow.EpochProtocolStateEntry,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
) (*flow.RichEpochProtocolStateEntry, error) {
	var (
		previousEpochSetup  *flow.EpochSetup
		previousEpochCommit *flow.EpochCommit
		nextEpochSetup      *flow.EpochSetup
		nextEpochCommit     *flow.EpochCommit
		err                 error
	)
	// query and fill in epoch setups and commits for previous and current epochs
	if protocolState.PreviousEpoch != nil {
		previousEpochSetup, err = setups.ByID(protocolState.PreviousEpoch.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve previous epoch setup: %w", err)
		}
		previousEpochCommit, err = commits.ByID(protocolState.PreviousEpoch.CommitID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve previous epoch commit: %w", err)
		}
	}

	currentEpochSetup, err := setups.ByID(protocolState.CurrentEpoch.SetupID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch setup: %w", err)
	}
	currentEpochCommit, err := commits.ByID(protocolState.CurrentEpoch.CommitID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch commit: %w", err)
	}

	// if next epoch has been set up, fill in data for it as well
	nextEpoch := protocolState.NextEpoch
	if nextEpoch != nil {
		nextEpochSetup, err = setups.ByID(nextEpoch.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve next epoch's setup event: %w", err)
		}
		if nextEpoch.CommitID != flow.ZeroID {
			nextEpochCommit, err = commits.ByID(nextEpoch.CommitID)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve next epoch's commit event: %w", err)
			}
		}
	}

	result, err := flow.NewRichProtocolStateEntry(
		protocolState,
		previousEpochSetup,
		previousEpochCommit,
		currentEpochSetup,
		currentEpochCommit,
		nextEpochSetup,
		nextEpochCommit,
	)
	if err != nil {
		// observing an error here would be an indication of severe data corruption or bug in our code since
		// all data should be available and correctly structured at this point.
		return nil, irrecoverable.NewExceptionf("critical failure while instantiating RichEpochProtocolStateEntry: %w", err)
	}
	return result, nil
}
