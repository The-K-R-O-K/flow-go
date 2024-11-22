package hand_mock

import (
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_provider"
)

type Factory struct {
	cachedDataProvider *DataProvider
}

func (f *Factory) NewDataProvider(ch chan<- interface{}, topic string) data_provider.DataProvider {
	f.cachedDataProvider.SetArgs(ch, topic)
	return f.cachedDataProvider
}

func NewFactory() *Factory {
	return &Factory{
		cachedDataProvider: nil,
	}
}

func (f *Factory) RegisterDataProvider(dataProvider *DataProvider) *Factory {
	f.cachedDataProvider = dataProvider
	return f
}
