package tests

import (
	"context"
	"github.com/onflow/flow-go/engine/access/rest/websockets"
	"github.com/onflow/flow-go/engine/access/rest/websockets/controller_tests/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	streammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
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

func (s *WsControllerSuite) TestHappyPath(t *testing.T) {
	conn := mock.NewWebsocketConnectionMock()
	controller := websockets.NewWebSocketController(s.logger, s.wsConfig, s.streamApi, s.streamConfig, conn)

	expectedBlocks := unittest.BlockFixtures(10)
	imitateResponses := func() {
		defer conn.Close()
		for _, block := range expectedBlocks {
			conn.WriteJSON(block)
		}
	}

	actualBlocks := make([]*flow.Block, len(expectedBlocks))
	readResponses := func() {
		for i, _ := range actualBlocks {
			conn.ReadJSONFromEchoSocket(&actualBlocks[i])
		}
	}

	requestMessage := models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action: "subscribe",
		},
		Topic:     "blocks",
		Arguments: nil,
	}
	conn.WriteJSON(requestMessage)

	go readResponses()
	go imitateResponses()
	controller.HandleConnection(context.TODO())

	require.Equal(t, expectedBlocks, actualBlocks)
}
