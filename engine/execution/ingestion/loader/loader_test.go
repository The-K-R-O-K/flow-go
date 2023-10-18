package loader_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/ingestion/loader"
	stateMock "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

var _ ingestion.BlockLoader = (*loader.Loader)(nil)

func logChain(chain []*flow.Block) {
	log := unittest.Logger()
	for i, block := range chain {
		log.Info().Msgf("block %v, height: %v, ID: %v", i, block.Header.Height, block.ID())
	}
}

func TestLoadingUnexecutedBlocks(t *testing.T) {
	ps := mocks.NewProtocolState()

	// Genesis <- A <- B <- C (finalized)
	chain, result, seal := unittest.ChainFixture(4)
	genesis, blockA, blockB, blockC := chain[0], chain[1], chain[2], chain[3]

	logChain(chain)

	require.NoError(t, ps.Bootstrap(genesis, result, seal))
	require.NoError(t, ps.Extend(blockA))
	require.NoError(t, ps.Extend(blockB))
	require.NoError(t, ps.Extend(blockC))
	require.NoError(t, ps.Finalize(blockC.ID()))

	es := new(stateMock.ExecutionState)
	es.On("GetHighestFinalizedExecuted").Return(genesis.Header.Height)
	headers := new(storage.Headers)
	headers.On("ByHeight", blockA.Header.Height).Return(blockA.Header, nil)
	headers.On("ByHeight", blockB.Header.Height).Return(blockB.Header, nil)
	headers.On("ByHeight", blockC.Header.Height).Return(blockC.Header, nil)

	loader := loader.NewLoader(unittest.Logger(), ps, headers, es)

	unexecuted, err := loader.LoadUnexecuted(context.Background())
	require.NoError(t, err)

	unittest.IDsEqual(t, []flow.Identifier{
		blockA.ID(),
		blockB.ID(),
		blockC.ID(),
	}, unexecuted)
}
