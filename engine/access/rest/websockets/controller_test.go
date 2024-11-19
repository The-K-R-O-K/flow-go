package websockets_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/rest/websockets"
	"github.com/onflow/flow-go/engine/access/rest/websockets/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	streammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestWsController(t *testing.T) {
	logger := unittest.Logger()
	config := websockets.NewDefaultWebsocketConfig()
	streamApi := streammock.NewAPI(t)
	streamCfg := backend.Config{}
	conn := mock.NewWebsocketConnectionCustom()
	controller := websockets.NewWebSocketController(logger, config, streamApi, streamCfg, conn)

	// run controller
	controller.HandleConnection(context.TODO())

	// write to conn
	args := map[string]interface{}{
		"start_block_height": 10,
	}
	body := models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{Action: "subscribe"},
		Topic:              "blocks",
		Arguments:          args,
	}
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = conn.WriteJSON(bodyJSON)
		require.NoError(t, err)
	}()

	wg.Wait()
	var actualResult interface{}
	err = conn.ReadTest(actualResult)
	require.NoError(t, err)

	require.Equal(t, "hello", actualResult)
}

type MockWsConn struct {
	*websocket.Conn
}

func NewMockWsConn() *MockWsConn {
	return &MockWsConn{
		Conn: &websocket.Conn{},
	}
}

func (c *MockWsConn) Close() error {
	return c.Conn.Close()
}
