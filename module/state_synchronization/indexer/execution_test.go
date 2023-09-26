package indexer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/pebble/registers"
	"github.com/onflow/flow-go/utils/unittest"
)

type indexTest struct {
	t                *testing.T
	indexer          *ExecutionState
	registers        *storagemock.Registers
	events           *storagemock.Events
	headers          *storagemock.Headers
	ctx              context.Context
	blocks           []*flow.Block
	data             *execution_data.BlockExecutionDataEntity
	lastHeightStore  func(t *testing.T) (uint64, error)
	firstHeightStore func(t *testing.T) (uint64, error)
	registersStore   func(t *testing.T, entries flow.RegisterEntries, height uint64) error
	eventsStore      func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error
	registersGet     func(t *testing.T, IDs flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

func newIndexTest(
	t *testing.T,
	blocks []*flow.Block,
	exeData *execution_data.BlockExecutionDataEntity,
) *indexTest {
	registers := storagemock.NewRegisters(t)
	events := storagemock.NewEvents(t)
	headers := newBlockHeadersStorage(blocks)

	return &indexTest{
		t:         t,
		registers: registers,
		events:    events,
		blocks:    blocks,
		ctx:       context.Background(),
		data:      exeData,
		headers:   headers.(*storagemock.Headers), // convert it back to mock type for tests
	}
}

func (i *indexTest) useDefaultBlockByHeight() *indexTest {
	i.headers.
		On("BlockIDByHeight", mocks.AnythingOfType("uint64")).
		Return(func(height uint64) (flow.Identifier, error) {
			for _, b := range i.blocks {
				if b.Header.Height == height {
					return b.ID(), nil
				}
			}
			return flow.ZeroID, fmt.Errorf("not found")
		})

	return i
}

func (i *indexTest) setLastHeight(f func(t *testing.T) (uint64, error)) *indexTest {
	i.registers.
		On("LatestHeight").
		Return(func() (uint64, error) {
			return f(i.t)
		})
	return i
}

func (i *indexTest) useDefaultLastHeight() *indexTest {
	i.registers.
		On("LatestHeight").
		Return(func() (uint64, error) {
			return i.blocks[len(i.blocks)-1].Header.Height, nil
		})
	return i
}

func (i *indexTest) useDefaultFirstHeight() *indexTest {
	i.registers.
		On("FirstHeight").
		Return(func() (uint64, error) {
			return i.blocks[0].Header.Height, nil
		})
	return i
}

func (i *indexTest) useDefaultHeights() *indexTest {
	i.useDefaultFirstHeight()
	return i.useDefaultLastHeight()
}

func (i *indexTest) setFirstHeight(f func(t *testing.T) (uint64, error)) *indexTest {
	i.registers.
		On("FirstHeight").
		Return(func() (uint64, error) {
			return f(i.t)
		})
	return i
}

func (i *indexTest) setStoreRegisters(f func(t *testing.T, entries flow.RegisterEntries, height uint64) error) *indexTest {
	i.registers.
		On("Store", mock.AnythingOfType("flow.RegisterEntries"), mock.AnythingOfType("uint64")).
		Return(func(entries flow.RegisterEntries, height uint64) error {
			return f(i.t, entries, height)
		}).Once()
	return i
}

func (i *indexTest) setStoreEvents(f func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error) *indexTest {
	i.events.
		On("Store", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList")).
		Return(func(ID flow.Identifier, events []flow.EventsList) error {
			return f(i.t, ID, events)
		})
	return i
}

func (i *indexTest) setGetRegisters(f func(t *testing.T, ID flow.RegisterID, height uint64) (flow.RegisterValue, error)) *indexTest {
	i.registers.
		On("Get", mock.AnythingOfType("flow.RegisterID"), mock.AnythingOfType("uint64")).
		Return(func(IDs flow.RegisterID, height uint64) (flow.RegisterValue, error) {
			return f(i.t, IDs, height)
		})
	return i
}

func (i *indexTest) initIndexer() *indexTest {
	if len(i.registers.ExpectedCalls) == 0 {
		i.useDefaultHeights() // only set when no other were set
	}
	indexer, err := New(i.registers, i.headers, nil, zerolog.Nop())
	require.NoError(i.t, err)
	i.indexer = indexer
	return i
}

func (i *indexTest) runIndexBlockData() error {
	i.initIndexer()
	return i.indexer.IndexBlockData(i.ctx, i.data)
}

func (i *indexTest) runGetRegisters(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	i.initIndexer()
	return i.indexer.RegisterValues(IDs, height)
}

func TestExecutionState_IndexBlockData(t *testing.T) {
	blocks := blocksFixture(5)
	block := blocks[len(blocks)-1]

	// this test makes sure the index block data is correctly calling store register with the
	// same entries we create as a block execution data test, and correctly converts the registers
	t.Run("Index Single Chunk and Single Register", func(t *testing.T) {
		trie := trieUpdateFixture()
		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{TrieUpdate: trie},
			},
		}
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		err := newIndexTest(t, blocks, execData).
			initIndexer().
			// make sure update registers match in length and are same as block data ledger payloads
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				assert.Equal(t, height, block.Header.Height)
				assert.Len(t, trie.Payloads, entries.Len())

				// make sure all the registers from the execution data have been stored as well the value matches
				trieRegistersPayloadComparer(t, trie.Payloads, entries)
				return nil
			}).
			runIndexBlockData()

		assert.NoError(t, err)
	})

	// this test makes sure that if we have multiple trie updates in a single block data
	// and some of those trie updates are for same register but have different values,
	// we only update that register once with the latest value, so this makes sure merging of
	// registers is done correctly.
	t.Run("Index Multiple Chunks and Merge Same Register Updates", func(t *testing.T) {
		tries := []*ledger.TrieUpdate{trieUpdateFixture(), trieUpdateFixture()}
		// make sure we have two register updates that are updating the same value, so we can check
		// if the value from the second update is being persisted instead of first
		tries[1].Paths[0] = tries[0].Paths[0]
		testValue := tries[1].Payloads[0]
		key, err := testValue.Key()
		require.NoError(t, err)
		testRegisterID, err := migrations.KeyToRegisterID(key)
		require.NoError(t, err)

		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{TrieUpdate: tries[0]},
				{TrieUpdate: tries[1]},
			},
		}
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		testRegisterFound := false
		err = newIndexTest(t, blocks, execData).
			initIndexer().
			// make sure update registers match in length and are same as block data ledger payloads
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				for _, entry := range entries {
					if entry.Key.String() == testRegisterID.String() {
						testRegisterFound = true
						assert.True(t, testValue.Value().Equals(entry.Value))
					}
				}
				// we should make sure the register updates are equal to both payloads' length -1 since we don't
				// duplicate the same register
				assert.Equal(t, len(tries[0].Payloads)+len(tries[1].Payloads)-1, len(entries))
				return nil
			}).
			runIndexBlockData()

		assert.NoError(t, err)
		assert.True(t, testRegisterFound)
	})

	// this test makes sure we get correct error when we try to index block that is not
	// within the range of indexed heights.
	t.Run("Invalid Heights", func(t *testing.T) {
		last := blocks[len(blocks)-1]
		ed := &execution_data.BlockExecutionData{
			BlockID: last.Header.ID(),
		}
		execData := execution_data.NewBlockExecutionDataEntity(last.ID(), ed)

		err := newIndexTest(t, blocks, execData).
			// return a height one smaller than the latest block in storage
			setLastHeight(func(t *testing.T) (uint64, error) {
				return blocks[len(blocks)-3].Header.Height, nil
			}).
			useDefaultFirstHeight().
			runIndexBlockData()

		assert.True(t, errors.Is(err, ErrIndexValue))
	})

	// this test makes sure that if a block we try to index is not found in block storage
	// we get correct error.
	t.Run("Unknown block ID", func(t *testing.T) {
		unknownBlock := blocksFixture(1)[0]
		ed := &execution_data.BlockExecutionData{
			BlockID: unknownBlock.Header.ID(),
		}
		execData := execution_data.NewBlockExecutionDataEntity(unknownBlock.Header.ID(), ed)

		err := newIndexTest(t, blocks, execData).runIndexBlockData()

		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})

}

func TestExecutionState_RegisterValues(t *testing.T) {
	t.Run("Get value for single register", func(t *testing.T) {
		blocks := blocksFixture(5)
		height := blocks[1].Header.Height
		ids := []flow.RegisterID{{
			Owner: "1",
			Key:   "2",
		}}
		val := flow.RegisterValue("0x1")

		values, err := newIndexTest(t, blocks, nil).
			initIndexer().
			setGetRegisters(func(t *testing.T, ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
				return val, nil
			}).
			runGetRegisters(ids, height)

		assert.NoError(t, err)
		assert.Equal(t, values, []flow.RegisterValue{val})
	})
}

// helper to store register at height and increment index range
func storeRegisterWithValue(indexer *ExecutionState, height uint64, owner string, key string, value []byte) error {
	payload := ledgerPayloadWithValuesFixture(owner, key, value)
	err := indexer.indexRegisterPayloads([]*ledger.Payload{payload}, height)
	if err != nil {
		return err
	}

	err = indexer.indexRange.Increase(height)
	if err != nil {
		return err
	}

	return nil
}

func TestIntegration_StoreAndGet(t *testing.T) {
	regOwner := "f8d6e0586b0a20c7"
	regKey := "code"
	registerID := flow.NewRegisterID(regOwner, regKey)
	logger := zerolog.New(os.Stdout)

	// this test makes sure index values for a single register are correctly updated and always last value is returned
	t.Run("Single Index Value Changes", func(t *testing.T) {
		RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			indexer, err := New(registers, nil, nil, logger)
			require.NoError(t, err)

			values := [][]byte{[]byte("1"), []byte("1"), []byte("2"), []byte("3") /*nil,*/, []byte("4")}

			value, err := indexer.RegisterValues(flow.RegisterIDs{registerID}, 0)
			require.Nil(t, value)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			for i, val := range values {
				testDesc := fmt.Sprintf("test itteration number %d failed with test value %s", i, val)
				height := uint64(i + 1)
				err := storeRegisterWithValue(indexer, height, regOwner, regKey, val)
				assert.NoError(t, err)

				results, err := indexer.RegisterValues(flow.RegisterIDs{registerID}, height)
				require.Nil(t, err, testDesc)
				assert.Equal(t, val, results[0])
			}
		})
	})

	// this test makes sure that even if indexed values for a specific register are requested with higher height
	// the correct highest height indexed value is returned.
	// e.g. we index A{h(1) -> X}, A{h(2) -> Y}, when we request h(5) we get value Y
	t.Run("Single Index Value At Later Heights", func(t *testing.T) {
		RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			indexer, err := New(registers, nil, nil, zerolog.Nop())
			require.NoError(t, err)

			value := []byte("1")

			require.NoError(t, storeRegisterWithValue(indexer, 1, regOwner, regKey, value))

			require.NoError(t, indexer.indexRegisterPayloads(nil, 2))
			assert.NoError(t, indexer.indexRange.Increase(2))

			require.NoError(t, indexer.indexRegisterPayloads(nil, 3))
			assert.NoError(t, indexer.indexRange.Increase(3))

			val, err := indexer.RegisterValues(flow.RegisterIDs{registerID}, uint64(3))
			require.Nil(t, err)
			assert.Equal(t, value, val[0])

			value = []byte("2")
			err = storeRegisterWithValue(indexer, 4, regOwner, regKey, value)
			require.NoError(t, err)

			require.NoError(t, indexer.indexRegisterPayloads(nil, 5))
			assert.NoError(t, indexer.indexRange.Increase(5))

			val, err = indexer.RegisterValues(flow.RegisterIDs{registerID}, uint64(5))
			require.Nil(t, err)
			assert.Equal(t, value, val[0])
		})
	})

	// this test makes sure we correctly handle weird payloads
	t.Run("Empty and Nil Payloads", func(t *testing.T) {
		RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			indexer, err := New(registers, nil, nil, zerolog.Nop())
			require.NoError(t, err)

			require.NoError(t, indexer.indexRegisterPayloads([]*ledger.Payload{}, 1))
			require.NoError(t, indexer.indexRegisterPayloads([]*ledger.Payload{}, 1))
			require.NoError(t, indexer.indexRange.Increase(1))
			require.NoError(t, indexer.indexRegisterPayloads(nil, 2))
		})
	})
}

func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByID := make(map[flow.Identifier]*flow.Block, 0)
	for _, b := range blocks {
		blocksByID[b.ID()] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByID(blocksByID))
}

func blocksFixture(n int) []*flow.Block {
	blocks := make([]*flow.Block, n)

	genesis := unittest.BlockFixture()
	blocks[0] = &genesis
	for i := 1; i < n; i++ {
		blocks[i] = unittest.BlockWithParentFixture(blocks[i-1].Header)
	}

	return blocks
}

func bootstrapTrieUpdates() *ledger.TrieUpdate {
	opts := []fvm.Option{
		fvm.WithChain(flow.Testnet.Chain()),
	}
	ctx := fvm.NewContext(opts...)
	vm := fvm.NewVirtualMachine()

	snapshotTree := snapshot.NewSnapshotTree(nil)

	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	executionSnapshot, _, _ := vm.Run(
		ctx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		snapshotTree)

	payloads := make([]*ledger.Payload, 0)
	for regID, regVal := range executionSnapshot.WriteSet {
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				{
					Type:  state.KeyPartOwner,
					Value: []byte(regID.Owner),
				},
				{
					Type:  state.KeyPartKey,
					Value: []byte(regID.Key),
				},
			},
		}

		payloads = append(payloads, ledger.NewPayload(key, regVal))
	}

	return trieUpdateWithPayloadsFixture(payloads)
}

func trieUpdateWithPayloadsFixture(payloads []*ledger.Payload) *ledger.TrieUpdate {
	keys := make([]ledger.Key, 0)
	values := make([]ledger.Value, 0)
	for _, payload := range payloads {
		key, _ := payload.Key()
		keys = append(keys, key)
		values = append(values, payload.Value())
	}

	update, _ := ledger.NewUpdate(ledger.DummyState, keys, values)
	trie, _ := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
	return trie
}

func trieUpdateFixture() *ledger.TrieUpdate {
	return trieUpdateWithPayloadsFixture(
		[]*ledger.Payload{
			ledgerPayloadFixture(),
			ledgerPayloadFixture(),
			ledgerPayloadFixture(),
			ledgerPayloadFixture(),
		})
}

func ledgerPayloadFixture() *ledger.Payload {
	owner := unittest.RandomAddressFixture()
	key := make([]byte, 8)
	rand.Read(key)
	val := make([]byte, 8)
	rand.Read(val)
	return ledgerPayloadWithValuesFixture(owner.String(), fmt.Sprintf("%x", key), val)
}

func ledgerPayloadWithValuesFixture(owner string, key string, value []byte) *ledger.Payload {
	k := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  state.KeyPartOwner,
				Value: []byte(owner),
			},
			{
				Type:  state.KeyPartKey,
				Value: []byte(key),
			},
		},
	}

	return ledger.NewPayload(k, value)
}

// trieRegistersPayloadComparer checks that trie payloads and register payloads are same, used for testing.
func trieRegistersPayloadComparer(t *testing.T, triePayloads []*ledger.Payload, registerPayloads flow.RegisterEntries) {
	assert.Equal(t, len(triePayloads), len(registerPayloads.Values()), "registers length should equal")

	// crate a lookup map that matches flow register ID to index in the payloads slice
	payloadRegID := make(map[flow.RegisterID]int)
	for i, p := range triePayloads {
		k, _ := p.Key()
		regKey, _ := migrations.KeyToRegisterID(k)
		payloadRegID[regKey] = i
	}

	for _, entry := range registerPayloads {
		index, ok := payloadRegID[entry.Key]
		assert.True(t, ok, fmt.Sprintf("register entry not found for key %s", entry.Key.String()))
		val := triePayloads[index].Value()
		assert.True(t, val.Equals(entry.Value), fmt.Sprintf("payload values not same %s - %s", val, entry.Value))
	}
}

// duplicated from register tests https://github.com/onflow/flow-go/blob/aa41e76c824260f8f08aacbe46471619ecf3fe6e/storage/pebble/registers_test.go#L291
const (
	placeHolderHeight          = uint64(0)
	MinLookupKeyLen            = 3 + registers.HeightSuffixLen
	codeFirstBlockHeight  byte = 3
	codeLatestBlockHeight byte = 4
)

var latestHeightKeyLiteral = binary.BigEndian.AppendUint64(
	[]byte{codeLatestBlockHeight, byte('/'), byte('/')}, placeHolderHeight)

var firstHeightKeyLiteral = binary.BigEndian.AppendUint64(
	[]byte{codeFirstBlockHeight, byte('/'), byte('/')}, placeHolderHeight)

func RunWithRegistersStorageAtInitialHeights(tb testing.TB, first uint64, latest uint64, f func(r *pebbleStorage.Registers)) {
	cache := pebble.NewCache(1 << 20)
	opts := pebbleStorage.DefaultPebbleOptions(cache, registers.NewMVCCComparer())
	unittest.RunWithConfiguredPebbleInstance(tb, opts, func(p *pebble.DB) {
		// insert initial heights to pebble
		require.NoError(tb, p.Set(firstHeightKeyLiteral, pebbleStorage.EncodedUint64(first), nil))
		require.NoError(tb, p.Set(latestHeightKeyLiteral, pebbleStorage.EncodedUint64(latest), nil))
		r, err := pebbleStorage.NewRegisters(p, zerolog.Nop())
		require.NoError(tb, err)
		f(r)
	})
}
