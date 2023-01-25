package assigner

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// The Assigner engine reads the receipts from each finalized block.
// For each receipt, it reads its result and find the chunks the assigned
// to me to verify, and then save it to the chunks job queue for the
// fetcher engine to process.
type Engine struct {
	unit                  *engine.Unit
	log                   zerolog.Logger
	metrics               module.VerificationMetrics
	tracer                module.Tracer
	me                    module.Local
	state                 protocol.State
	assigner              module.ChunkAssigner      // to determine chunks this node should verify.
	chunksQueue           storage.ChunksQueue       // to store chunks to be verified.
	newChunkListener      module.NewJobListener     // to notify chunk queue consumer about a new chunk.
	blockConsumerNotifier module.ProcessingNotifier // to report a block has been processed.
	stopAtHeight          uint64
	stopAtBlockID         atomic.Value
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	me module.Local,
	state protocol.State,
	assigner module.ChunkAssigner,
	chunksQueue storage.ChunksQueue,
	newChunkListener module.NewJobListener,
	stopAtHeight uint64,
) *Engine {
	e := &Engine{
		unit:             engine.NewUnit(),
		log:              log.With().Str("engine", "assigner").Logger(),
		metrics:          metrics,
		tracer:           tracer,
		me:               me,
		state:            state,
		assigner:         assigner,
		chunksQueue:      chunksQueue,
		newChunkListener: newChunkListener,
		stopAtHeight:     stopAtHeight,
	}
	e.stopAtBlockID.Store(flow.ZeroID)
	return e
}

func (e *Engine) WithBlockConsumerNotifier(notifier module.ProcessingNotifier) {
	e.blockConsumerNotifier = notifier
}

func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// resultChunkAssignment receives an execution result that appears in a finalized incorporating block.
// In case this verification node is authorized at the reference block of this execution receipt's result,
// chunk assignment is computed for the result, and the list of assigned chunks returned.
func (e *Engine) resultChunkAssignment(ctx context.Context,
	result *flow.ExecutionResult,
	incorporatingBlock flow.Identifier,
) (flow.ChunkList, error) {
	resultID := result.ID()
	log := log.With().
		Hex("result_id", logging.ID(resultID)).
		Hex("executed_block_id", logging.ID(result.BlockID)).
		Hex("incorporating_block_id", logging.ID(incorporatingBlock)).
		Logger()
	e.metrics.OnExecutionResultReceivedAtAssignerEngine()

	// verification node should be authorized at the reference block id.
	ok, err := authorizedAsVerification(e.state, result.BlockID, e.me.NodeID())
	if err != nil {
		return nil, fmt.Errorf("could not verify weight of verification node for result at reference block id: %w", err)
	}
	if !ok {
		log.Warn().Msg("node is not authorized at reference block id, receipt is discarded")
		return nil, nil
	}

	// chunk assignment
	chunkList, err := e.chunkAssignments(ctx, result, incorporatingBlock)
	if err != nil {
		return nil, fmt.Errorf("could not determine chunk assignment: %w", err)
	}
	e.metrics.OnChunksAssignmentDoneAtAssigner(len(chunkList))

	// TODO: de-escalate to debug level on stable version.
	log.Info().
		Int("total_chunks", len(result.Chunks)).
		Int("total_assigned_chunks", len(chunkList)).
		Msg("chunk assignment done")

	return chunkList, nil
}

// processChunk receives a chunk that belongs to execution result id. It creates a chunk locator
// for the chunk and stores the chunk locator in the chunks queue.
//
// Note that the chunk is assume to be legitimately assigned to this verification node
// (through the chunk assigner), and belong to the execution result.
//
// Deduplication of chunk locators is delegated to the chunks queue.
func (e *Engine) processChunk(chunk *flow.Chunk, resultID flow.Identifier, blockHeight uint64) (bool, error) {
	lg := e.log.With().
		Hex("result_id", logging.ID(resultID)).
		Hex("chunk_id", logging.ID(chunk.ID())).
		Uint64("chunk_index", chunk.Index).
		Uint64("block_height", blockHeight).
		Logger()

	locator := &chunks.Locator{
		ResultID: resultID,
		Index:    chunk.Index,
	}

	// pushes chunk locator to the chunks queue
	ok, err := e.chunksQueue.StoreChunkLocator(locator)
	if err != nil {
		return false, fmt.Errorf("could not push chunk locator to chunks queue: %w", err)
	}
	if !ok {
		lg.Debug().Msg("could not push duplicate chunk locator to chunks queue")
		return false, nil
	}

	e.metrics.OnAssignedChunkProcessedAtAssigner()

	// notifies chunk queue consumer of a new chunk
	e.newChunkListener.Check()
	lg.Info().Msg("chunk locator successfully pushed to chunks queue")

	return true, nil
}

// ProcessFinalizedBlock is the entry point of assigner engine. It pushes the block down the pipeline with tracing on it enabled.
// Through the pipeline the execution receipts included in the block are indexed, and their chunk assignments are done, and
// the assigned chunks are pushed to the chunks queue, which is the output stream of this engine.
// Once the assigner engine is done handling all the receipts in the block, it notifies the block consumer.
func (e *Engine) ProcessFinalizedBlock(block *flow.Block) {
	blockID := block.ID()

	span, ctx := e.tracer.StartBlockSpan(e.unit.Ctx(), blockID, trace.VERProcessFinalizedBlock)
	defer span.End()

	e.processFinalizedBlock(ctx, block)
}

// processFinalizedBlock indexes the execution receipts included in the block, performs chunk assignment on its result, and
// processes the chunks assigned to this verification node by pushing them to the chunks consumer.
func (e *Engine) processFinalizedBlock(ctx context.Context, block *flow.Block) {

	blockID := block.ID()
	// we should always notify block consumer before returning.
	defer e.blockConsumerNotifier.Notify(blockID)

	// if max sealed is at or above stop height-1, we can safely crash, knowing
	// that there are no more receipts to verify.
	// We must use equal or greater since we cannot assume all VNs are stopping at the same height
	// or stopping at all - blocks can still be verified in a system, or emergency sealing can be on.
	// This is also safe even if this function runs concurrently and/or on blocks out of order -
	// once certain height is sealed there is no point in verifying at and below it anyway.
	if e.stopAtHeight > 0 {
		highestSealed, err := e.highestSealed()
		if err != nil {
			e.log.Fatal().Err(err).Msg("cannot query highest sealed height")
			return
		}
		if highestSealed >= e.stopAtHeight-1 {
			// start crash sequence
			// TODO put restart sentinel in a DB with restart height of e.stopHeight-1, after restart VN star processing at e.stopHeight

			e.log.Fatal().Msgf("block sealed at height %d - stopping node, since stop at %d requested", highestSealed, e.stopAtHeight)
		}
	}

	// keeps track of total assigned, processed and skipped chunks in
	// this block for logging.
	assignedChunksCount := uint64(0)
	processedChunksCount := uint64(0)
	heightSkippedChunksCount := uint64(0)

	lg := e.log.With().
		Hex("block_id", logging.ID(blockID)).
		Uint64("block_height", block.Header.Height).
		Int("result_num", len(block.Payload.Results)).Logger()
	lg.Debug().Msg("new finalized block arrived")

	// determine chunk assigment on each result and pushes the assigned chunks to the chunks queue.
	receiptsGroupedByResultID := block.Payload.Receipts.GroupByResultID() // for logging purposes
	for _, result := range block.Payload.Results {
		resultID := result.ID()

		// log receipts committing to result
		resultLog := lg.With().Hex("result_id", logging.ID(resultID)).Logger()
		for _, receipt := range receiptsGroupedByResultID.GetGroup(resultID) {
			resultLog = resultLog.With().Hex("receipts_for_result", logging.ID(receipt.ID())).Logger()
		}
		resultLog.Debug().Msg("determining chunk assignment for incorporated result")

		// compute chunk assignment
		chunkList, err := e.resultChunkAssignmentWithTracing(ctx, result, blockID)
		if err != nil {
			resultLog.Fatal().Err(err).Msg("could not determine assigned chunks for result")
		}

		assignedChunksCount += uint64(len(chunkList))

		for _, chunk := range chunkList {

			// is chunk's block at or above stop height, skip completely
			if e.stopAtHeight > 0 {
				heightForBlock, err := e.heightForBlock(chunk.BlockID)
				if err != nil {
					resultLog.Fatal().Err(err).Msg("cannot query height for a block")
					return
				}
				if heightForBlock >= e.stopAtHeight {
					heightSkippedChunksCount++
					continue
				}
			}

			processed, err := e.processChunkWithTracing(ctx, chunk, resultID, block.Header.Height)
			if err != nil {
				resultLog.Fatal().
					Err(err).
					Hex("chunk_id", logging.ID(chunk.ID())).
					Uint64("chunk_index", chunk.Index).
					Msg("could not process chunk")
			}

			if processed {
				processedChunksCount++
			}
		}
	}

	e.metrics.OnFinalizedBlockArrivedAtAssigner(block.Header.Height)
	lg.Info().
		Uint64("total_assigned_chunks", assignedChunksCount).
		Uint64("total_processed_chunks", processedChunksCount).
		Uint64("total_processed_chunks", processedChunksCount).
		Uint64("total_skipped_chunks", heightSkippedChunksCount).
		Msg("finished processing finalized block")
}

// chunkAssignments returns the list of chunks in the chunk list assigned to this verification node.
func (e *Engine) chunkAssignments(ctx context.Context, result *flow.ExecutionResult, incorporatingBlock flow.Identifier) (flow.ChunkList, error) {
	span, _ := e.tracer.StartSpanFromContext(ctx, trace.VERMatchMyChunkAssignments)
	defer span.End()

	assignment, err := e.assigner.Assign(result, incorporatingBlock)
	if err != nil {
		return nil, err
	}

	mine, err := assignedChunks(e.me.NodeID(), assignment, result.Chunks)
	if err != nil {
		return nil, fmt.Errorf("could not determine my assignments: %w", err)
	}

	return mine, nil
}

// authorizedAsVerification checks whether this instance of verification node is authorized at specified block ID.
// It returns true and nil if verification node has positive weight at referenced block ID, and returns false and nil otherwise.
// It returns false and error if it could not extract the weight of node as a verification node at the specified block.
func authorizedAsVerification(state protocol.State, blockID flow.Identifier, identifier flow.Identifier) (bool, error) {
	// TODO define specific error for handling cases
	identity, err := state.AtBlockID(blockID).Identity(identifier)
	if err != nil {
		return false, nil
	}

	// checks role of node is verification
	if identity.Role != flow.RoleVerification {
		return false, fmt.Errorf("node has an invalid role. expected: %s, got: %s", flow.RoleVerification, identity.Role)
	}

	// checks identity has not been ejected
	if identity.Ejected {
		return false, nil
	}

	// checks identity has weight
	if identity.Weight == 0 {
		return false, nil
	}

	return true, nil
}

// resultChunkAssignmentWithTracing computes the chunk assignment for the provided receipt with tracing enabled.
func (e *Engine) resultChunkAssignmentWithTracing(
	ctx context.Context,
	result *flow.ExecutionResult,
	incorporatingBlock flow.Identifier,
) (flow.ChunkList, error) {
	var err error
	var chunkList flow.ChunkList
	e.tracer.WithSpanFromContext(ctx, trace.VERAssignerHandleExecutionReceipt, func() {
		chunkList, err = e.resultChunkAssignment(ctx, result, incorporatingBlock)
	})
	return chunkList, err
}

// processChunkWithTracing receives a chunks belong to the same execution result and processes it with tracing enabled.
//
// Note that the chunk in the input should be legitimately assigned to this verification node
// (through the chunk assigner), and belong to the same execution result.
func (e *Engine) processChunkWithTracing(ctx context.Context, chunk *flow.Chunk, resultID flow.Identifier, blockHeight uint64) (bool, error) {
	var err error
	var processed bool
	e.tracer.WithSpanFromContext(ctx, trace.VERAssignerProcessChunk, func() {
		processed, err = e.processChunk(chunk, resultID, blockHeight)
	})
	return processed, err
}

func (e *Engine) highestSealed() (uint64, error) {
	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return 0, fmt.Errorf("cannot query head of sealed state: %w", err)
	}
	return sealed.Height, nil
}

func (e *Engine) heightForBlock(id flow.Identifier) (uint64, error) {
	header, err := e.state.AtBlockID(id).Head()
	if err != nil {
		return 0, fmt.Errorf("cannot query state at block %s: %w", id, err)
	}
	return header.Height, nil
}

// assignedChunks returns the chunks assigned to a specific assignee based on the input chunk assignment.
func assignedChunks(assignee flow.Identifier, assignment *chunks.Assignment, chunks flow.ChunkList) (flow.ChunkList, error) {
	// indices of chunks assigned to verifier
	chunkIndices := assignment.ByNodeID(assignee)

	// chunks keeps the list of chunks assigned to the verifier
	myChunks := make(flow.ChunkList, 0, len(chunkIndices))
	for _, index := range chunkIndices {
		chunk, ok := chunks.ByIndex(index)
		if !ok {
			return nil, fmt.Errorf("chunk out of range requested: %v", index)
		}

		myChunks = append(myChunks, chunk)
	}

	return myChunks, nil
}
