package websockets

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_provider"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/utils/concurrentmap"
)

type Controller struct {
	logger               zerolog.Logger
	config               Config
	conn                 WebsocketConnection
	communicationChannel chan interface{}
	dataProviders        *concurrentmap.Map[uuid.UUID, dp.DataProvider]
	dataProvidersFactory dp.Factory
	shutdownOnce         sync.Once
}

func NewWebSocketController(
	logger zerolog.Logger,
	config Config,
	factory dp.Factory,
	conn WebsocketConnection,
) *Controller {
	return &Controller{
		logger:               logger.With().Str("component", "websocket-controller").Logger(),
		config:               config,
		conn:                 conn,
		communicationChannel: make(chan interface{}, 10), //TODO: should it be buffered chan?
		dataProviders:        concurrentmap.New[uuid.UUID, dp.DataProvider](),
		dataProvidersFactory: factory,
		shutdownOnce:         sync.Once{},
	}
}

// HandleConnection manages the WebSocket connection, adding context and error handling.
func (c *Controller) HandleConnection(ctx context.Context) {
	//TODO: configure the connection with ping-pong and deadlines
	//TODO: spin up a response limit tracker routine
	go c.readMessagesFromClient(ctx)
	c.writeMessagesToClient(ctx)
}

// writeMessagesToClient reads a messages from communication channel and passes them on to a client WebSocket connection.
// The communication channel is filled by data providers. Besides, the response limit tracker is involved in
// write message regulation
func (c *Controller) writeMessagesToClient(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.communicationChannel:
			if !ok {
				c.shutdownConnection()
				return
			}
			// TODO: handle 'response per second' limits

			err := c.conn.WriteJSON(msg)
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					c.shutdownConnection()
					return
				}

				c.logger.Error().Err(err).Msg("error writing to connection")
			}
		}
	}
}

// readMessagesFromClient continuously reads messages from a client WebSocket connection,
// processes each message, and handles actions based on the message type.
func (c *Controller) readMessagesFromClient(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := c.readMessageFromConn() //TODO: read is blocking, so there's no point in ctx.Done(), right?
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
					c.shutdownConnection()
					return
				}

				if err.Error() == "message is empty" {
					continue
				}

				c.logger.Warn().Err(err).Msg("error reading message from client")
				continue
			}

			baseMsg, validatedMsg, err := c.parseAndValidateMessage(msg)
			if err != nil {
				c.logger.Debug().Err(err).Msg("error parsing and validating client message")
				c.shutdownConnection()
				return
			}

			if err := c.handleAction(ctx, validatedMsg); err != nil {
				c.logger.Warn().Err(err).Str("action", baseMsg.Action).Msg("error handling action")
				c.shutdownConnection()
				return
			}
		}
	}
}

func (c *Controller) readMessageFromConn() (json.RawMessage, error) {
	var message json.RawMessage
	if err := c.conn.ReadJSON(&message); err != nil {
		return nil, fmt.Errorf("error reading JSON from client: %w", err)
	}

	if message == nil {
		return nil, fmt.Errorf("message is empty")
	}

	return message, nil
}

func (c *Controller) parseAndValidateMessage(message json.RawMessage) (models.BaseMessageRequest, interface{}, error) {
	var baseMsg models.BaseMessageRequest
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return models.BaseMessageRequest{}, nil, fmt.Errorf("error unmarshalling base message: %w", err)
	}

	var validatedMsg interface{}
	switch baseMsg.Action {
	case "subscribe":
		var subscribeMsg models.SubscribeMessageRequest
		if err := json.Unmarshal(message, &subscribeMsg); err != nil {
			return baseMsg, nil, fmt.Errorf("error unmarshalling subscribe message: %w", err)
		}
		//TODO: add validation logic for `topic` field
		validatedMsg = subscribeMsg

	case "unsubscribe":
		var unsubscribeMsg models.UnsubscribeMessageRequest
		if err := json.Unmarshal(message, &unsubscribeMsg); err != nil {
			return baseMsg, nil, fmt.Errorf("error unmarshalling unsubscribe message: %w", err)
		}
		validatedMsg = unsubscribeMsg

	case "list_subscriptions":
		var listMsg models.ListSubscriptionsMessageRequest
		if err := json.Unmarshal(message, &listMsg); err != nil {
			return baseMsg, nil, fmt.Errorf("error unmarshalling list subscriptions message: %w", err)
		}
		validatedMsg = listMsg

	default:
		c.logger.Debug().Str("action", baseMsg.Action).Msg("unknown action type")
		return baseMsg, nil, fmt.Errorf("unknown action type: %s", baseMsg.Action)
	}

	return baseMsg, validatedMsg, nil
}

func (c *Controller) handleAction(ctx context.Context, message interface{}) error {
	switch msg := message.(type) {
	case models.SubscribeMessageRequest:
		c.handleSubscribe(ctx, msg)
	case models.UnsubscribeMessageRequest:
		c.handleUnsubscribe(ctx, msg)
	case models.ListSubscriptionsMessageRequest:
		c.handleListSubscriptions(ctx, msg)
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
	return nil
}

func (c *Controller) handleSubscribe(ctx context.Context, msg models.SubscribeMessageRequest) {
	dp := c.dataProvidersFactory.NewDataProvider(c.communicationChannel, msg.Topic)
	c.dataProviders.Add(dp.ID(), dp)

	// firstly, we want to write OK response to client and only after that we can start providing actual data
	response := models.SubscribeMessageResponse{
		BaseMessageResponse: models.BaseMessageResponse{
			Success: true,
		},
		Topic: dp.Topic(),
		ID:    dp.ID().String(),
	}
	c.communicationChannel <- response

	dp.Run(ctx)
}

func (c *Controller) handleUnsubscribe(_ context.Context, msg models.UnsubscribeMessageRequest) {
	id, err := uuid.Parse(msg.ID)
	if err != nil {
		c.logger.Debug().Err(err).Msg("error parsing message ID")
		//TODO: return an error response to client
		c.communicationChannel <- err
		return
	}

	dp, ok := c.dataProviders.Get(id)
	if ok {
		dp.Close()
		c.dataProviders.Remove(id)
	}
}

func (c *Controller) handleListSubscriptions(ctx context.Context, msg models.ListSubscriptionsMessageRequest) {
	//TODO: return a response to client
}

func (c *Controller) shutdownConnection() {
	c.shutdownOnce.Do(func() {
		defer close(c.communicationChannel)
		defer func(conn WebsocketConnection) {
			if err := c.conn.Close(); err != nil {
				c.logger.Error().Err(err).Msg("error closing connection")
			}
		}(c.conn)

		err := c.dataProviders.ForEach(func(_ uuid.UUID, dp dp.DataProvider) error {
			dp.Close()
			return nil
		})
		if err != nil {
			c.logger.Error().Err(err).Msg("error closing data provider")
		}

		c.dataProviders.Clear()
	})
}
