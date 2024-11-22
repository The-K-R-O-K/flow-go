package websockets

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	dpmock "github.com/onflow/flow-go/engine/access/rest/websockets/data_provider/mock"
	connmock "github.com/onflow/flow-go/engine/access/rest/websockets/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	streammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type WsControllerSuite struct {
	suite.Suite

	logger       zerolog.Logger
	wsConfig     Config
	streamApi    *streammock.API
	streamConfig backend.Config
}

func (s *WsControllerSuite) SetupTest() {
	s.logger = unittest.Logger()
	s.wsConfig = NewDefaultWebsocketConfig()
	s.streamApi = streammock.NewAPI(s.T())
	s.streamConfig = backend.Config{}
}

func TestWsControllerSuite(t *testing.T) {
	suite.Run(t, new(WsControllerSuite))
}

func (s *WsControllerSuite) AttachSubscribedConnection(conn *connmock.WebsocketConnection, topic string) *connmock.WebsocketConnection {
	requestMessage := models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{Action: "subscribe"},
		Topic:              topic,
		Arguments:          nil,
	}

	// The very first message from a client is a request to subscribe to some topic
	conn.
		On("ReadJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			reqMsg := args.Get(0).(*json.RawMessage)
			msg, err := json.Marshal(requestMessage)
			require.NoError(s.T(), err)
			*reqMsg = msg
		}).
		Return(nil).
		Once()

	// The very first message from a controller to a client is a response to subscribe request
	conn.
		On("WriteJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			response := args.Get(0).(models.SubscribeMessageResponse)
			require.True(s.T(), response.Success)
		}).
		Return(nil).
		Once()

	return conn
}

func (s *WsControllerSuite) AttachEmptyConnection(conn *connmock.WebsocketConnection) *connmock.WebsocketConnection {
	conn.
		On("ReadJSON", mock.Anything).
		Return(nil)

	conn.
		On("WriteJSON", mock.Anything).
		Return(nil)

	return conn
}

func (s *WsControllerSuite) TestSubscribeBlocks() {
	conn := connmock.NewWebsocketConnection(s.T())
	conn.On("Close").Return(nil).Once()

	dataProvider := dpmock.NewDataProvider(s.T())
	dataProvider.On("ID").Return(uuid.New())
	dataProvider.On("Close").Run(func(args mock.Arguments) {})
	dataProvider.On("Topic").Return("blocks").Once()

	dataProviderFactory := dpmock.NewFactory(s.T())
	dataProviderFactory.
		On("NewDataProvider", mock.Anything, mock.Anything).
		Return(dataProvider).
		Once()

	controller := NewWebSocketController(s.logger, s.wsConfig, dataProviderFactory, conn)

	// we want data provider to write some block to controller
	expectedBlock := unittest.BlockFixture()
	dataProvider.
		On("Run", mock.Anything).
		Run(func(args mock.Arguments) {
			controller.communicationChannel <- expectedBlock
		}).
		Once()

	s.AttachSubscribedConnection(conn, "blocks")

	ctx, cancel := context.WithCancel(context.Background())
	var actualBlock flow.Block

	// controller reads a block from data provider and pass it on to a client
	conn.
		On("WriteJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			block := args.Get(0).(flow.Block)
			actualBlock = block
			cancel() // stop provider after this func
		}).
		Return(nil).
		Once()

	s.AttachEmptyConnection(conn)

	// blocking call until the connection is closed by reader or writer
	controller.HandleConnection(ctx)

	require.Equal(s.T(), expectedBlock, actualBlock)
}
