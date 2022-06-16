package chunks

import (
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/model/flow"
)

// CFExecutionDataBlockIDMismatch is returned when execution data root's block ID is different from chunk block ID
type CFExecutionDataBlockIDMismatch struct {
	chunkIndex               uint64
	execResID                flow.Identifier
	executionDataRootBlockID flow.Identifier
	chunkBlockID             flow.Identifier
}

func (cf CFExecutionDataBlockIDMismatch) String() string {
	return fmt.Sprintf("execution data root's block ID (%s) is different than chunk's block ID (%s) for chunk %d with result ID %s", cf.executionDataRootBlockID, cf.chunkBlockID, cf.chunkIndex, cf.execResID.String())
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFExecutionDataBlockIDMismatch) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFExecutionDataBlockIDMismatch) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFExecutionDataBlockIDMismatch creates a new instance of Chunk Fault (ExecutionDataBlockIDMismatch)
func NewCFExecutionDataBlockIDMismatch(executionDataRootBlockID flow.Identifier, chunkBlockID flow.Identifier, chInx uint64, execResID flow.Identifier) *CFExecutionDataBlockIDMismatch {
	return &CFExecutionDataBlockIDMismatch{
		chunkIndex:               chInx,
		execResID:                execResID,
		executionDataRootBlockID: executionDataRootBlockID,
		chunkBlockID:             chunkBlockID,
	}
}

// CFExecutionDataChunksLengthMismatch is returned when execution data chunks list has different length than number of chunks for a block
type CFExecutionDataChunksLengthMismatch struct {
	chunkIndex                     uint64
	execResID                      flow.Identifier
	executionDataRootChunkLength   int
	executionResultChunkListLength int
}

func (cf CFExecutionDataChunksLengthMismatch) String() string {
	return fmt.Sprintf("execution data root chunk length (%d) is different then execution result chunk list length (%d) for chunk %d with result ID %s", cf.executionDataRootChunkLength, cf.executionResultChunkListLength, cf.chunkIndex, cf.execResID.String())
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFExecutionDataChunksLengthMismatch) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFExecutionDataChunksLengthMismatch) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFExecutionDataChunksLengthMismatch creates a new instance of Chunk Fault (ExecutionDataBlockIDMismatch)
func NewCFExecutionDataChunksLengthMismatch(executionDataRootChunkLength int, executionResultChunkListLength int, chInx uint64, execResID flow.Identifier) *CFExecutionDataChunksLengthMismatch {
	return &CFExecutionDataChunksLengthMismatch{
		chunkIndex:                     chInx,
		execResID:                      execResID,
		executionDataRootChunkLength:   executionDataRootChunkLength,
		executionResultChunkListLength: executionResultChunkListLength,
	}
}

// CFExecutionDataInvalidChunkCID is returned when execution data chunk's CID is different from computed
type CFExecutionDataInvalidChunkCID struct {
	chunkIndex                uint64
	execResID                 flow.Identifier
	executionDataRootChunkCID cid.Cid
	computedChunkCID          cid.Cid
}

func (cf CFExecutionDataInvalidChunkCID) String() string {
	return fmt.Sprintf("execution data chunk CID (%s) is different then computed (%s) for chunk %d with result ID %s", cf.executionDataRootChunkCID, cf.computedChunkCID, cf.chunkIndex, cf.execResID.String())
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFExecutionDataInvalidChunkCID) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFExecutionDataInvalidChunkCID) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFExecutionDataInvalidChunkCID creates a new instance of Chunk Fault (NewCFExecutionDataInvalidChunkCID)
func NewCFExecutionDataInvalidChunkCID(executionDataRootChunkCID cid.Cid, executionResultChunkListLength cid.Cid, chInx uint64, execResID flow.Identifier) *CFExecutionDataInvalidChunkCID {
	return &CFExecutionDataInvalidChunkCID{
		chunkIndex:                chInx,
		execResID:                 execResID,
		executionDataRootChunkCID: executionDataRootChunkCID,
		computedChunkCID:          executionResultChunkListLength,
	}
}

// CFInvalidExecutionDataID is returned when ExecutionResult's ExecutionDataID is different from computed
type CFInvalidExecutionDataID struct {
	chunkIndex              uint64
	execResID               flow.Identifier
	erExecutionDataID       flow.Identifier
	computedExecutionDataID flow.Identifier
}

func (cf CFInvalidExecutionDataID) String() string {
	return fmt.Sprintf("execution data ID (%s) is different then computed (%s) for chunk %d with result ID %s", cf.erExecutionDataID, cf.computedExecutionDataID, cf.chunkIndex, cf.execResID.String())
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFInvalidExecutionDataID) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFInvalidExecutionDataID) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFInvalidExecutionDataID creates a new instance of Chunk Fault (CFInvalidExecutionDataID)
func NewCFInvalidExecutionDataID(erExecutionDataID flow.Identifier, computedExecutionDataID flow.Identifier, chInx uint64, execResID flow.Identifier) *CFInvalidExecutionDataID {
	return &CFInvalidExecutionDataID{
		chunkIndex:              chInx,
		execResID:               execResID,
		erExecutionDataID:       erExecutionDataID,
		computedExecutionDataID: computedExecutionDataID,
	}
}
