package indexer

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// ExecutionState indexes the execution state.
type ExecutionState struct {
	registers  storage.RegisterIndex
	headers    storage.Headers
	events     storage.Events
	indexRange *SequentialIndexRange
	log        zerolog.Logger
}

// New execution state indexer with provided storage access for registers and headers as well as initial height.
// This method will initialize the index starting height and end height to that found in the register storage,
// if no height was previously persisted it will use the provided initHeight.
func New(
	log zerolog.Logger,
	registers storage.RegisterIndex,
	headers storage.Headers,
	events storage.Events,
	initHeight uint64,
) (*ExecutionState, error) {
	// get the first indexed height from the register storage, if not found use the default start index height provided
	first, err := registers.FirstHeight()
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			first = initHeight
		} else {
			return nil, err
		}
	}

	last, err := registers.LatestHeight()
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			last = first // if last was not found we are just starting to index
		} else {
			return nil, err
		}
	}

	log.Debug().
		Uint64("first_height", first).
		Uint64("latest_height", last).
		Msgf("initialized indexer range")

	indexRange, err := NewSequentialIndexRange(first, last)
	if err != nil {
		return nil, err
	}

	return &ExecutionState{
		registers:  registers,
		headers:    headers,
		events:     events,
		indexRange: indexRange,
		log:        log.With().Str("component", "execution_indexer").Logger(),
	}, nil
}

// HeightByBlockID retrieves the height for block ID.
// Expected errors:
// - storage.ErrNotFound if a block is not found in the storage.
func (i *ExecutionState) HeightByBlockID(ID flow.Identifier) (uint64, error) {
	header, err := i.headers.ByBlockID(ID)
	if err != nil {
		return 0, fmt.Errorf("could not find block by ID %: %w", ID, err)
	}

	return header.Height, nil
}

// RegisterValues retrieves register values by the register IDs at the provided block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// Expected errors:
// - storage.ErrNotFound if the register by the ID was never indexed
// - ErrIndexBoundary if the height is out of indexed height boundary
func (i *ExecutionState) RegisterValues(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	_, err := i.indexRange.Contained(height)
	if err != nil {
		return nil, err
	}

	values := make([]flow.RegisterValue, len(IDs))

	for j, id := range IDs {
		value, err := i.registers.Get(id, height)
		if err != nil {
			return nil, err
		}

		values[j] = value
	}

	return values, nil
}

// IndexBlockData indexes all execution block data by height.
// This method shouldn't be used concurrently.
// Expected errors:
// - ErrIndexValue if height was not incremented in sequence
// - storage.ErrNotFound if the block for execution data was not found
func (i *ExecutionState) IndexBlockData(ctx context.Context, data *execution_data.BlockExecutionDataEntity) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// todo can we use the headers index from badger to get the height of a block provided from exec sync API?
	block, err := i.headers.ByBlockID(data.BlockID)
	if err != nil {
		// todo should we wrap sentinel error or should we use irrecoverable.exception as per design guidelines https://github.com/onflow/flow-go/blob/master/CodingConventions.md
		return fmt.Errorf("could not get the block by ID %s: %w", data.BlockID, err)
	}

	lg := i.log.With().
		Hex("block_id", logging.ID(data.BlockID)).
		Uint64("height", block.Height).
		Logger()

	lg.Debug().Msgf("indexing new block")

	if _, err := i.indexRange.CanIncrease(block.Height); err != nil {
		return err
	}

	// concurrently process indexing of block data
	g, _ := errgroup.WithContext(ctx)

	updates := make([]*ledger.TrieUpdate, 0)
	events := make([]flow.Event, 0)
	collections := make([]*flow.Collection, 0)

	for _, chunk := range data.ChunkExecutionDatas {
		if chunk.TrieUpdate != nil {
			updates = append(updates, chunk.TrieUpdate)
		}

		events = append(events, chunk.Events...)
		collections = append(collections, chunk.Collection)
	}

	if len(events) > 0 {
		g.Go(func() error {
			err := i.indexEvents(data.BlockID, events)
			if err != nil {
				return fmt.Errorf("could not index events at height %d: %w", block.Height, err)
			}
			return nil
		})
	}

	registersUpdated := 0
	g.Go(func() error {
		// we are iterating all the registers and overwrite any existing register at the same path
		// this will make sure if we have multiple register changes only the last change will get persisted
		// if block has two chucks:
		// first chunk updates: { X: 1, Y: 2 }
		// second chunk updates: { X: 2 }
		// then we should persist only {X: 2: Y: 2}

		payloads := make(map[ledger.Path]*ledger.Payload)
		for _, update := range updates {
			for i, path := range update.Paths {
				payloads[path] = update.Payloads[i] // todo should we use TrieUpdate.Paths or TrieUpdate.Payload.Key?
			}
		}

		registersUpdated = len(payloads)

		err = i.indexRegisterPayloads(maps.Values(payloads), block.Height)
		if err != nil {
			return fmt.Errorf("could not index register payloads at height %d: %w", block.Height, err)
		}
		return nil
	})

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("failed to index block data at height %d: %w", block.Height, err)
	}

	lg.Debug().
		Int("register_count", registersUpdated).
		Msg("indexed block data")

	return i.indexRange.Increase(block.Height)
}

func (i *ExecutionState) indexEvents(blockID flow.Identifier, events flow.EventsList) error {
	// Note: service events are currently not included in execution data: https://github.com/onflow/flow-go/issues/4624
	return i.events.Store(blockID, []flow.EventsList{events})
}

func (i *ExecutionState) indexRegisterPayloads(payloads []*ledger.Payload, height uint64) error {
	regEntries := make(flow.RegisterEntries, len(payloads))

	for j, payload := range payloads {
		k, err := payload.Key()
		if err != nil {
			return err
		}

		id, err := migrations.KeyToRegisterID(k)
		if err != nil {
			return err
		}

		regEntries[j] = flow.RegisterEntry{
			Key:   id,
			Value: payload.Value(),
		}
	}

	return i.registers.Store(regEntries, height)
}
