package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// TransactionStatusesDataProvider is responsible for providing tx statuses
type TransactionStatusesDataProvider struct {
	*baseDataProvider

	logger        zerolog.Logger
	api           access.API
	linkGenerator commonmodels.LinkGenerator
}

var _ DataProvider = (*TransactionStatusesDataProvider)(nil)

func NewTransactionStatusesDataProvider(
	ctx context.Context,
	logger zerolog.Logger,
	api access.API,
	subscriptionID string,
	linkGenerator commonmodels.LinkGenerator,
	topic string,
	arguments wsmodels.Arguments,
	send chan<- interface{},
) (*TransactionStatusesDataProvider, error) {
	p := &TransactionStatusesDataProvider{
		logger:        logger.With().Str("component", "transaction-statuses-data-provider").Logger(),
		api:           api,
		linkGenerator: linkGenerator,
	}

	// Initialize txID passed to the provider.
	txID, err := parseTransactionID(arguments)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments for tx statuses data provider: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	p.baseDataProvider = newBaseDataProvider(
		subscriptionID,
		topic,
		arguments,
		cancel,
		send,
		p.createSubscription(subCtx, txID), // Set up a subscription to tx statuses based on arguments.
	)

	return p, nil
}

// Run starts processing the subscription for events and handles responses.
//
// Expected errors during normal operations:
//   - context.Canceled: if the operation is canceled, during an unsubscribe action.
func (p *TransactionStatusesDataProvider) Run() error {
	messageIndex := counters.NewMonotonicCounter(0)

	return run(
		p.closedChan,
		p.subscription,
		func(response []*access.TransactionResult) error {
			return p.sendResponse(response, &messageIndex)
		},
	)
}

func (p *TransactionStatusesDataProvider) sendResponse(
	txResults []*access.TransactionResult,
	messageIndex *counters.StrictMonotonicCounter,
) error {
	for i := range txResults {
		txStatusesPayload := models.NewTransactionStatusesResponse(p.linkGenerator, txResults[i], messageIndex.Value())
		messageIndex.Increment()

		response := models.BaseDataProvidersResponse{
			SubscriptionID: p.ID(),
			Topic:          p.Topic(),
			Payload:        &txStatusesPayload,
		}
		p.send <- &response
	}

	return nil
}

// createSubscription creates a new subscription using the specified input arguments.
func (p *TransactionStatusesDataProvider) createSubscription(
	ctx context.Context,
	txID flow.Identifier,
) subscription.Subscription {
	return p.api.SubscribeTransactionStatuses(ctx, txID, entities.EventEncodingVersion_JSON_CDC_V0)
}

// parseTransactionID validates and initializes the transaction ID argument.
func parseTransactionID(
	arguments wsmodels.Arguments,
) (flow.Identifier, error) {
	var txID flow.Identifier
	allowedFields := []string{
		"tx_id",
	}
	err := ensureAllowedFields(arguments, allowedFields)
	if err != nil {
		return flow.ZeroID, err
	}

	if txIDIn, ok := arguments["tx_id"]; ok && txIDIn != "" {
		result, ok := txIDIn.(string)
		if !ok {
			return txID, fmt.Errorf("'tx_id' must be a string")
		}
		var txIDParsed parser.ID
		err := txIDParsed.Parse(result)
		if err != nil {
			return txID, fmt.Errorf("invalid 'tx_id': %w", err)
		}
		txID = txIDParsed.Flow()
	}

	return txID, nil
}
