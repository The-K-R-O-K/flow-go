package tests

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rest/websockets"
	dpmock "github.com/onflow/flow-go/engine/access/rest/websockets/data_provider/mock"
	connmock "github.com/onflow/flow-go/engine/access/rest/websockets/mock"

	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	streammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type WsControllerSuite struct {
	suite.Suite

	logger       zerolog.Logger
	wsConfig     websockets.Config
	streamApi    *streammock.API
	streamConfig backend.Config
}

func (s *WsControllerSuite) SetupTest() {
	s.logger = unittest.Logger()
	s.wsConfig = websockets.NewDefaultWebsocketConfig()
	s.streamApi = streammock.NewAPI(s.T())
	s.streamConfig = backend.Config{}
}

func TestWsControllerSuite(t *testing.T) {
	suite.Run(t, new(WsControllerSuite))
}

func (s *WsControllerSuite) TestHappyPath() {
	conn := connmock.NewWebsocketConnectionMock()

	blocks := unittest.BlockFixtures(10)
	expectedBlocks := make([]interface{}, len(blocks))
	for i, block := range blocks {
		expectedBlocks[i] = block
	}

	blocksGenerator := dpmock.NewDataGenerator(expectedBlocks)
	dataProvider := dpmock.NewDataProvider(blocksGenerator)
	dataProviderFactory := dpmock.NewFactory()
	dataProviderFactory.RegisterDataProvider(dataProvider)

	controller := websockets.NewWebSocketController(s.logger, s.wsConfig, dataProviderFactory, conn)

	go func() {
		//TODO: read actual blocks from conn and compare with expected ones
		//block := conn.ReadJSON()
		//conn.Close()
	}()

	controller.HandleConnection(context.TODO())
}
