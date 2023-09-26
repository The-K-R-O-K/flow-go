package pebble

import (
	"fmt"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type Bootstrap struct {
	checkpointDir      string
	checkpointFileName string
	log                zerolog.Logger
	db                 *pebble.DB
	leafNodeChan       chan *wal.LeafNode
	rootHeight         uint64
}

func NewBootstrap(db *pebble.DB, checkpointFile string, rootHeight uint64, log zerolog.Logger) (*Bootstrap, error) {
	// check for pre-populated heights, fail if it is populated
	// i.e. the IndexCheckpointFile function has already run for the db in this directory
	checkpointDir, checkpointFileName := filepath.Split(checkpointFile)
	_, _, err := db.Get(latestHeightKey())
	if err == nil {
		// key detected, attempt to run bootstrap on corrupt or already bootstrapped data
		return nil, fmt.Errorf("found latest key set on badger instance, cannot bootstrap populated DB")
	}

	logger := log.With().
		Str("component", "execution_indexer bootstrap").
		Str("checkpoint file", checkpointFile).
		Uint64("root height", rootHeight).
		Logger()

	return &Bootstrap{
		checkpointDir:      checkpointDir,
		checkpointFileName: checkpointFileName,
		log:                logger,
		db:                 db,
		leafNodeChan:       make(chan *wal.LeafNode, checkpointLeafNodeBufSize),
		rootHeight:         rootHeight,
	}, nil
}

func (b *Bootstrap) batchIndexRegisters(leafNodes []*wal.LeafNode) error {
	batch := b.db.NewBatch()
	defer batch.Close()
	for _, register := range leafNodes {
		payload := register.Payload
		key, err := payload.Key()
		if err != nil {
			return fmt.Errorf("could not get key from register payload: %w", err)
		}

		registerID, err := migrations.KeyToRegisterID(key)
		if err != nil {
			return fmt.Errorf("could not get register ID from key: %w", err)
		}

		encoded := newLookupKey(b.rootHeight, registerID).Bytes()
		err = batch.Set(encoded, payload.Value(), nil)
		if err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}

		b.log.Debug().
			Str("register ID", registerID.String()).
			Str("payload", payload.Value().String()).
			Str("encoded key", string(encoded)).
			Msg("batch indexed register")
	}
	err := batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}
	return nil
}

// indexCheckpointFileWorker asynchronously indexes register entries in b.checkpointDir
// with wal.OpenAndReadLeafNodesFromCheckpointV6
func (b *Bootstrap) indexCheckpointFileWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	select {
	case <-ctx.Done():
		return
	default:
	}

	b.log.Debug().Msg("index checkpoint worker")

	// collect leaf nodes to batch index until the channel is closed
	for leafNode := range b.leafNodeChan {
		b.log.Debug().Str("path", leafNode.Path.String()).Msg("index checkpoint worker starting to index batch")
		err := b.batchIndexRegisters([]*wal.LeafNode{leafNode})
		if err != nil {
			ctx.Throw(fmt.Errorf("unable to index registers to pebble: %w", err))
		}
	}
}

// IndexCheckpointFile indexes the checkpoint file in the Dir provided as a component
func (b *Bootstrap) IndexCheckpointFile(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	b.log.Debug().Msg("starting to index checkpoint file")

	ready()
	// index checkpoint file async
	cmb := component.NewComponentManagerBuilder()
	for i := 0; i < pebbleBootstrapWorkerCount; i++ {
		// create workers to read and index registers
		cmb.AddWorker(b.indexCheckpointFileWorker)
	}
	c := cmb.Build()
	c.Start(ctx)
	err := wal.OpenAndReadLeafNodesFromCheckpointV6(b.leafNodeChan, b.checkpointDir, b.checkpointFileName, zerolog.Nop())
	if err != nil {
		// error in reading a leaf node
		ctx.Throw(fmt.Errorf("error reading leaf node: %w", err))
	}
	b.log.Debug().Msg("reading leaf nodes")
	// wait for the indexing to finish before populating heights
	<-c.Done()
	b.log.Debug().Msg("done reading leaf nodes")
	bat := b.db.NewBatch()
	defer bat.Close()
	// update heights atomically to prevent one getting populated without the other
	// leaving it in a corrupted state
	err = bat.Set(firstHeightKey(), EncodedUint64(b.rootHeight), nil)
	if err != nil {
		ctx.Throw(fmt.Errorf("unable to add first height to batch: %w", err))
	}
	err = bat.Set(latestHeightKey(), EncodedUint64(b.rootHeight), nil)
	if err != nil {
		ctx.Throw(fmt.Errorf("unable to add latest height to batch: %w", err))
	}
	err = bat.Commit(pebble.Sync)
	if err != nil {
		ctx.Throw(fmt.Errorf("unable to index first and latest heights: %w", err))
	}
}
