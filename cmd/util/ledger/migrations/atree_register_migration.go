package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	util2 "github.com/onflow/flow-go/module/util"
)

// AtreeRegisterMigrator is a migrator that converts the storage of an account from the
// old atree format to the new atree format.
// Account "storage used" should be correctly updated after the migration.
type AtreeRegisterMigrator struct {
	log zerolog.Logger

	sampler zerolog.Sampler
	rw      reporters.ReportWriter
	rwf     reporters.ReportWriterFactory

	metrics *metrics

	nWorkers int
}

var _ AccountBasedMigration = (*AtreeRegisterMigrator)(nil)
var _ io.Closer = (*AtreeRegisterMigrator)(nil)

func NewAtreeRegisterMigrator(
	rwf reporters.ReportWriterFactory,
) *AtreeRegisterMigrator {

	sampler := util2.NewTimedSampler(30 * time.Second)

	migrator := &AtreeRegisterMigrator{
		sampler: sampler,

		rwf: rwf,
		rw:  rwf.ReportWriter("atree-register-migrator"),

		metrics: &metrics{},
	}

	return migrator
}

func (m *AtreeRegisterMigrator) Close() error {
	m.rw.Close()
	m.log.Info().
		Str("average_non_zero_clone_time", m.metrics.AverageNonZeroCloneTime().String()).
		Str("average_non_zero_save_time", m.metrics.AverageNonZeroSaveTime().String()).
		Int("non_zero_time_clones", m.metrics.cloned).
		Int("non_zero_time_saves", m.metrics.saved).
		Msg("metrics")

	return nil
}

func (m *AtreeRegisterMigrator) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	nWorkers int,
) error {
	m.log = log.With().Str("migration", "atree-register-migration").Logger()
	m.nWorkers = nWorkers

	return nil
}

func (m *AtreeRegisterMigrator) MigrateAccount(
	_ context.Context,
	address common.Address,
	oldPayloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	if address == common.ZeroAddress {
		return oldPayloads, nil
	}

	if reason, ok := knownProblematicAccounts[address]; ok {
		m.log.Info().
			Str("account", address.Hex()).
			Str("reason", reason).
			Msg("Account is known to have issues. Skipping it.")
		return oldPayloads, nil
	}

	if address != mustHexToAddress("4eded0de73020ca5") {
		// for testing purposes migrate only one account
		return oldPayloads, nil
	}

	// create all the runtime components we need for the migration
	mr, err := newMigratorRuntime(address, oldPayloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
	}

	//// check the storage health
	//healthOk, err := m.checkStorageHealth(mr)
	//if err != nil {
	//	return nil, fmt.Errorf("storage health issues for address %s: %w", address.Hex(), err)
	//}

	// keep track of all storage maps that were accessed
	// if they are empty the wont be changed, but we sill need to copy them over
	storageMapIds := make(map[string]struct{})

	// Do the storage conversion
	changes, err := m.migrateAccountStorage(mr, storageMapIds)
	if err != nil {
		if errors.Is(err, skippableAccountError) {
			return oldPayloads, nil
		}
		return nil, fmt.Errorf("failed to convert storage for address %s: %w", address.Hex(), err)
	}

	originalLen := len(oldPayloads)

	newPayloads, err := m.validateChangesAndCreateNewRegisters(mr, changes, storageMapIds)
	if err != nil {
		if errors.Is(err, skippableAccountError) {
			return oldPayloads, nil
		}
		return nil, err
	}

	newLen := len(newPayloads)

	if newLen > originalLen {
		m.rw.Write(migrationProblem{
			Address: address.Hex(),
			Key:     "",
			Size:    len(mr.Snapshot.Payloads),
			Kind:    "more_registers_after_migration",
			Msg:     fmt.Sprintf("original: %d, new: %d", originalLen, newLen),
		})
	}

	if _, ok := accountsToLog[address]; ok {
		m.dumpAccount(mr.Address, oldPayloads, newPayloads)
	}

	return newPayloads, nil
}

func (m *AtreeRegisterMigrator) migrateAccountStorage(
	mr *migratorRuntime,
	storageMapIds map[string]struct{},
) (map[flow.RegisterID]flow.RegisterValue, error) {

	// iterate through all domains and migrate them
	for _, domain := range domains {
		err := m.convertStorageDomain(mr, storageMapIds, domain)
		if err != nil {
			return nil, fmt.Errorf("failed to convert storage domain %s : %w", domain, err)
		}
	}

	// commit the storage changes
	err := mr.Storage.Commit(mr.Interpreter, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit storage: %w", err)
	}

	// finalize the transaction
	result, err := mr.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	return result.WriteSet, nil
}

func (m *AtreeRegisterMigrator) convertStorageDomain(
	mr *migratorRuntime,
	storageMapIds map[string]struct{},
	domain string,
) error {

	storageMap := mr.Storage.GetStorageMap(mr.Address, domain, false)
	if storageMap == nil {
		// no storage for this domain
		return nil
	}
	storageMapIds[string(atree.SlabIndexToLedgerKey(storageMap.StorageID().Index))] = struct{}{}

	iterator := storageMap.Iterator(util.NopMemoryGauge{})
	keys := make([]interpreter.StringStorageMapKey, 0)
	// to be safe avoid modifying the map while iterating
	for {
		key := iterator.NextKey()
		if key == nil {
			break
		}

		stringKey, ok := key.(interpreter.StringAtreeValue)
		if !ok {
			return fmt.Errorf("invalid key type %T, expected interpreter.StringAtreeValue", key)
		}

		keys = append(keys, interpreter.StringStorageMapKey(stringKey))
	}

	for _, key := range keys {
		err := func() error {
			var value interpreter.Value

			err := capturePanic(func() {
				value = storageMap.ReadValue(util.NopMemoryGauge{}, key)
			})
			if err != nil {
				return fmt.Errorf("failed to read value for key %s: %w", key, err)
			}

			m.metrics.trackCloneTime(
				func() {
					value, err = m.cloneValue(mr, value)
				},
			)
			if err != nil {
				return fmt.Errorf("failed to clone value for key %s: %w", key, err)
			}

			m.metrics.trackSaveTime(
				func() {
					err = capturePanic(func() {
						// set value will first purge the old value
						storageMap.SetValue(mr.Interpreter, key, value)
					})
				},
			)
			if err != nil {
				return fmt.Errorf("failed to set value for key %s: %w", key, err)
			}

			return nil
		}()
		if err != nil {

			m.rw.Write(migrationProblem{
				Address: mr.Address.Hex(),
				Size:    len(mr.Snapshot.Payloads),
				Key:     string(key),
				Kind:    "migration_failure",
				Msg:     err.Error(),
			})
			return skippableAccountError
		}
	}

	return nil
}

func newMigratorRuntime(
	address common.Address,
	payloads []*ledger.Payload,
) (
	*migratorRuntime,
	error,
) {
	snapshot, err := util.NewPayloadSnapshot(payloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload snapshot: %w", err)
	}
	transactionState := state.NewTransactionState(snapshot, state.DefaultParameters())
	accounts := environment.NewAccounts(transactionState)

	accountsAtreeLedger := util.NewAccountsAtreeLedger(accounts)
	storage := runtime.NewStorage(accountsAtreeLedger, util.NopMemoryGauge{})

	ri := &util.MigrationRuntimeInterface{
		Accounts: accounts,
	}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{
		AccountLinkingEnabled: true,
		// Attachments are enabled everywhere except for Mainnet
		AttachmentsEnabled: true,
		// Capability Controllers are enabled everywhere except for Mainnet
		CapabilityControllersEnabled: true,
	})

	env.Configure(
		ri,
		runtime.NewCodesAndPrograms(),
		storage,
		runtime.NewCoverageReport(),
	)

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		env.InterpreterConfig)
	if err != nil {
		return nil, err
	}

	return &migratorRuntime{
		Address:          address,
		Payloads:         payloads,
		Snapshot:         snapshot,
		TransactionState: transactionState,
		Interpreter:      inter,
		Storage:          storage,
	}, nil
}

type migratorRuntime struct {
	Snapshot         *util.PayloadSnapshot
	TransactionState state.NestedTransactionPreparer
	Interpreter      *interpreter.Interpreter
	Storage          *runtime.Storage
	Payloads         []*ledger.Payload
	Address          common.Address
}

func (m *AtreeRegisterMigrator) validateChangesAndCreateNewRegisters(
	mr *migratorRuntime,
	changes map[flow.RegisterID]flow.RegisterValue,
	storageMapIds map[string]struct{},
) ([]*ledger.Payload, error) {
	originalPayloadsSnapshot := mr.Snapshot
	originalPayloads := originalPayloadsSnapshot.Payloads
	newPayloads := make([]*ledger.Payload, 0, len(originalPayloads))

	// store state payload so that it can be updated
	var statePayload *ledger.Payload

	for id, value := range changes {
		// delete all values that were changed from the original payloads so that we can
		// check what remains
		delete(originalPayloads, id)

		if len(value) == 0 {
			// value was deleted
			continue
		}

		ownerAddress, err := common.BytesToAddress([]byte(id.Owner))
		if err != nil {
			return nil, fmt.Errorf("failed to convert owner address: %w", err)
		}

		if ownerAddress.Hex() != mr.Address.Hex() {
			// something was changed that does not belong to this account. Log it.
			m.log.Error().
				Str("key", id.String()).
				Str("owner_address", ownerAddress.Hex()).
				Str("account", mr.Address.Hex()).
				Msg("key is part of the change set, but is for a different account")

			return nil, fmt.Errorf("register for a different account was produced during migration")
		}

		if isAccountKey(util.RegisterIDToKey(id)) {
			statePayload = ledger.NewPayload(util.RegisterIDToKey(id), value)
			// we will append this later
			continue
		}

		newPayloads = append(newPayloads, ledger.NewPayload(util.RegisterIDToKey(id), value))
	}

	removedSize := uint64(0)

	// add all values that were not changed
	for id, value := range originalPayloads {
		if len(value.Value()) == 0 {
			// this is strange, but we don't want to add empty values. Log it.
			m.log.Warn().Msgf("empty value for key %s", id)
			continue
		}

		key := util.RegisterIDToKey(id)
		if isAccountKey(key) {
			statePayload = value
			// we will append this later
			continue
		}

		if id.IsInternalState() {
			// this is expected. Move it to the new payloads
			newPayloads = append(newPayloads, value)
			continue
		}

		isADomainKey := false
		for _, domain := range domains {
			if id.Key == domain {
				isADomainKey = true
				break
			}
		}
		if isADomainKey {
			// TODO: check if this is really expected
			// this is expected. Move it to the new payloads
			newPayloads = append(newPayloads, value)
			continue
		}

		if _, ok := storageMapIds[id.Key]; ok {
			newPayloads = append(newPayloads, value)
			continue
		}

		m.rw.Write(migrationProblem{
			Address: mr.Address.Hex(),
			Key:     id.String(),
			Size:    len(mr.Snapshot.Payloads),
			Kind:    "not_migrated",
			Msg:     fmt.Sprintf("%x", value),
		})

		size, err := payloadSize(key, value)
		if err != nil {
			return nil, fmt.Errorf("failed to get payload size: %w", err)
		}

		removedSize += size

		// this is ok
		// return nil, skippableAccountError
	}

	if statePayload == nil {
		return nil, fmt.Errorf("state payload was not found")
	}

	if removedSize > 0 {
		status, err := environment.AccountStatusFromBytes(statePayload.Value())
		if err != nil {
			return nil, fmt.Errorf("could not parse account status: %w", err)
		}

		status.SetStorageUsed(status.StorageUsed() - removedSize)

		newPayload, err := newPayloadWithValue(statePayload, status.ToBytes())
		if err != nil {
			return nil, fmt.Errorf("cannot create new payload with value: %w", err)
		}

		statePayload = &newPayload

	}

	newPayloads = append(newPayloads, statePayload)

	//if hasMissingKeys && m.sampler.Sample(zerolog.InfoLevel) {
	//	m.dumpAccount(mr.Address, mr.Payloads, newPayloads)
	//}
	return newPayloads, nil
}

func (m *AtreeRegisterMigrator) dumpAccount(address common.Address, before, after []*ledger.Payload) {
	beforeWriter := m.rwf.ReportWriter(fmt.Sprintf("account-before-%s", address.Hex()))
	for _, p := range before {
		beforeWriter.Write(p)
	}
	beforeWriter.Close()

	afterWriter := m.rwf.ReportWriter(fmt.Sprintf("account-after-%s", address.Hex()))
	for _, p := range after {
		afterWriter.Write(p)
	}
	afterWriter.Close()
}

func (m *AtreeRegisterMigrator) cloneValue(
	mr *migratorRuntime,
	value interpreter.Value,
) (interpreter.Value, error) {

	yes, err := m.isCricketMomentsShardedCollection(mr, value)
	if err != nil {
		return nil, err
	}

	if yes {
		m.log.Info().Msg("migrating CricketMomentsShardedCollection")
		return m.cloneCricketMomentsShardedCollection(mr, value)
	}

	// if this very special register on this very special account then handle
	// it differently

	err = capturePanic(func() {
		// force the value to be read entirely
		value = value.Clone(mr.Interpreter)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (m *AtreeRegisterMigrator) isCricketMomentsShardedCollection(
	mr *migratorRuntime,
	value interpreter.Value,
) (bool, error) {
	if mr.Address != mustHexToAddress("0x4eded0de73020ca5") {
		return false, nil
	}

	compositeValue, ok := value.(*interpreter.CompositeValue)
	if !ok {
		return false, nil
	}

	return string(compositeValue.TypeID()) == "A.4eded0de73020ca5.CricketMomentsShardedCollection.ShardedCollection", nil
}

func (m *AtreeRegisterMigrator) cloneCricketMomentsShardedCollection(
	mr *migratorRuntime,
	value interpreter.Value,
) (interpreter.Value, error) {
	// the CricketMomentsShardedCollection is a resource wit two fields
	//
	// pub var collections: @{UInt64: CricketMoments.Collection}
	// pub let numBuckets: UInt64
	//
	// each collection has one field
	//
	// pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
	//
	// the goal is to copy the ownedNFTs field of each collection with multiple go-rutines
	// and then reassemble the CricketMomentsShardedCollection

	shardedCollectionResource, ok := value.(*interpreter.CompositeValue)
	if !ok {
		return nil, fmt.Errorf("expected *interpreter.CompositeValue, got %T", value)
	}
	shardedCollectionMapField := shardedCollectionResource.GetField(
		mr.Interpreter,
		interpreter.EmptyLocationRange,
		"collections",
	)
	if shardedCollectionMapField == nil {
		return nil, fmt.Errorf("expected collections field")
	}
	shardedCollectionMap, ok := shardedCollectionMapField.(*interpreter.DictionaryValue)
	if !ok {
		return nil, fmt.Errorf("expected collections to be *interpreter.DictionaryValue, got %T", shardedCollectionMapField)
	}

	type valueWithKeys struct {
		outerKey interpreter.Value
		innerKey interpreter.Value
		value    interpreter.Value
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	cloneChan := make(chan valueWithKeys, m.nWorkers)
	clonedChan := make(chan valueWithKeys, m.nWorkers)
	wg := sync.WaitGroup{}
	wg.Add(m.nWorkers)

	for i := 0; i < m.nWorkers; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case clone, ok := <-cloneChan:
					if !ok {
						return
					}
					err := capturePanic(func() {
						clone.value = clone.value.Clone(mr.Interpreter)
					})
					if err != nil {
						cancel(err)
						return
					}
					clonedChan <- clone
				}
			}
		}()
	}

	go func() {
		shardedCollectionMapIterator := shardedCollectionMap.Iterator()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			outerKey := shardedCollectionMapIterator.NextKey(nil)
			if outerKey == nil {
				break
			}
			value := shardedCollectionMap.GetKey(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				outerKey,
			)

			collection, ok := value.(*interpreter.CompositeValue)
			if !ok {
				cancel(fmt.Errorf("expected collection to be *interpreter.CompositeValue, got %T", value))
				return
			}

			ownedNFTsRaw := collection.GetField(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				"ownedNFTs",
			)
			if ownedNFTsRaw == nil {
				cancel(fmt.Errorf("expected ownedNFTs field"))
				return
			}
			ownedNFTs, ok := ownedNFTsRaw.(*interpreter.DictionaryValue)
			if !ok {
				cancel(fmt.Errorf("expected ownedNFTs to be *interpreter.DictionaryValue, got %T", ownedNFTsRaw))
				return
			}

			ownedNFTsIterator := ownedNFTs.Iterator()
			keys := make([]interpreter.Value, 0, ownedNFTs.Count())
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				innerKey := ownedNFTsIterator.NextKey(nil)
				if innerKey == nil {
					break
				}
				value := ownedNFTs.GetKey(
					mr.Interpreter,
					interpreter.EmptyLocationRange,
					innerKey,
				)

				cloneChan <- valueWithKeys{
					innerKey: innerKey,
					outerKey: outerKey,
					value:    value,
				}

				keys = append(keys, innerKey)
			}

			for _, key := range keys {
				ownedNFTs.Remove(
					mr.Interpreter,
					interpreter.EmptyLocationRange,
					key,
				)
			}
		}
		close(cloneChan)
	}()

	done := make(chan struct{})
	cloned := make([]valueWithKeys, 0, 10_0000_000)

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case clone, ok := <-clonedChan:
				if !ok {
					return
				}
				cloned = append(cloned, clone)
			}
		}
	}()

	wg.Wait()
	close(clonedChan)
	<-done

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	m.log.Info().
		Int("cloned", len(cloned)).
		Msg("CricketMomentsShardedCollection: all values cloned")

	// clone a now empty sharded collection
	shardedCollectionResource = shardedCollectionResource.Clone(mr.Interpreter).(*interpreter.CompositeValue)
	shardedCollectionMapField = shardedCollectionResource.GetField(
		mr.Interpreter,
		interpreter.EmptyLocationRange,
		"collections",
	)
	if shardedCollectionMapField == nil {
		return nil, fmt.Errorf("expected collections field")
	}
	shardedCollectionMap, ok = shardedCollectionMapField.(*interpreter.DictionaryValue)
	if !ok {
		return nil, fmt.Errorf("expected collections to be *interpreter.DictionaryValue, got %T", shardedCollectionMapField)
	}

	shards := make(map[interpreter.Value]*interpreter.DictionaryValue)

	shardedCollectionMapIterator := shardedCollectionMap.Iterator()
	for {
		outerKey := shardedCollectionMapIterator.NextKey(nil)
		if outerKey == nil {
			break
		}
		value := shardedCollectionMap.GetKey(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			outerKey,
		)

		collection, ok := value.(*interpreter.CompositeValue)
		if !ok {
			return nil, fmt.Errorf("expected collection to be *interpreter.CompositeValue, got %T", value)
		}

		ownedNFTsRaw := collection.GetField(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			"ownedNFTs",
		)
		if ownedNFTsRaw == nil {
			return nil, fmt.Errorf("expected ownedNFTs field")
		}
		ownedNFTs, ok := ownedNFTsRaw.(*interpreter.DictionaryValue)
		if !ok {
			return nil, fmt.Errorf("expected ownedNFTs to be *interpreter.DictionaryValue, got %T", ownedNFTsRaw)
		}

		shards[outerKey] = ownedNFTs
	}

	for _, clone := range cloned {
		ownedNFTs, ok := shards[clone.outerKey]
		if !ok {
			return nil, fmt.Errorf("expected shard for key %s", clone.outerKey)
		}

		ownedNFTs.Insert(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			clone.innerKey,
			clone.value,
		)
	}

	m.log.Info().
		Int("cloned", len(cloned)).
		Msg("CricketMomentsShardedCollection: cloned")

	// everything is cloned which means we forced the entire value to be read from storage
	return shardedCollectionResource, nil
}

// capturePanic captures panics and converts them to errors
// this is needed for some cadence functions that panic on error
func capturePanic(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	f()

	return
}

// convert all domains
var domains = []string{
	common.PathDomainStorage.Identifier(),
	common.PathDomainPrivate.Identifier(),
	common.PathDomainPublic.Identifier(),
	runtime.StorageDomainContract,
}

// migrationProblem is a struct for reporting errors
type migrationProblem struct {
	Address string
	// Size is the account size in register count
	Size int
	Key  string
	Kind string
	Msg  string
}

var knownProblematicAccounts = map[common.Address]string{
	// Testnet accounts with broken contracts
	mustHexToAddress("434a1f199a7ae3ba"): "Broken contract FanTopPermission",
	mustHexToAddress("454c9991c2b8d947"): "Broken contract Test",
	mustHexToAddress("48602d8056ff9d93"): "Broken contract FanTopPermission",
	mustHexToAddress("5d63c34d7f05e5a4"): "Broken contract FanTopPermission",
	mustHexToAddress("5e3448b3cffb97f2"): "Broken contract FanTopPermission",
	mustHexToAddress("7d8c7e050c694eaa"): "Broken contract Test",
	mustHexToAddress("ba53f16ede01972d"): "Broken contract FanTopPermission",
	mustHexToAddress("c843c1f5a4805c3a"): "Broken contract FanTopPermission",
	mustHexToAddress("48d3be92e6e4a973"): "Broken contract FanTopPermission",
	// Mainnet account
	// mustHexToAddress("4eded0de73020ca5"): "Account to big to migrate",
}

var accountsToLog = map[common.Address]string{}

func mustHexToAddress(hex string) common.Address {
	address, err := common.HexToAddress(hex)
	if err != nil {
		panic(err)
	}
	return address
}

var skippableAccountError = errors.New("account can be skipped")

type metrics struct {
	timeToClone      time.Duration
	clonedWithZeroMS int
	cloned           int
	timeToSave       time.Duration
	savedWithZeroMS  int
	saved            int

	mu sync.Mutex
}

func (m *metrics) trackCloneTime(clone func()) {
	start := time.Now()
	clone()

	time := time.Since(start)
	if time == 0 {
		// only count non-zero times
		m.clonedWithZeroMS += 1
		return
	}
	m.mu.Lock()
	m.timeToClone += time
	m.cloned += 1
	m.mu.Unlock()
}

func (m *metrics) trackSaveTime(save func()) {
	start := time.Now()
	save()

	time := time.Since(start)
	if time == 0 {
		// only count non-zero times
		m.savedWithZeroMS += 1
		return
	}
	m.mu.Lock()
	m.timeToSave += time
	m.saved += 1
	m.mu.Unlock()
}

func (m *metrics) AverageNonZeroCloneTime() time.Duration {
	avg := m.timeToClone / time.Duration(m.cloned)
	return avg
}

func (m *metrics) AverageNonZeroSaveTime() time.Duration {
	avg := m.timeToSave / time.Duration(m.saved)
	return avg
}