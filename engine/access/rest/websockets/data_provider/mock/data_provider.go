package mock

import (
	"context"

	"github.com/google/uuid"
)

type DataProvider struct {
	id               uuid.UUID
	topicChan        chan<- interface{}
	topic            string
	stopProviderFunc context.CancelFunc
	generator        *DataGenerator
}

func NewDataProvider(generator *DataGenerator) *DataProvider {
	return &DataProvider{
		id:        uuid.New(),
		generator: generator,
	}
}

func (p *DataProvider) SetArgs(ch chan<- interface{}, topic string) {
	p.topicChan = ch
	p.topic = topic
}

func (p *DataProvider) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	p.stopProviderFunc = cancel

	select {
	case <-ctx.Done():
		return
	default:
		p.topicChan <- p.generator.Next()
	}
}

func (p *DataProvider) ID() uuid.UUID {
	return p.id
}

func (p *DataProvider) Topic() string {
	return p.topic
}

func (p *DataProvider) Close() {
	p.stopProviderFunc()
}
