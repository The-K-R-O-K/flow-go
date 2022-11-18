package corruptlibp2p

import (
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/routing"
	discoveryRouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

type CorruptPubSubAdapterConfig struct {
	options []corrupt.Option
}

var _ p2p.PubSubAdapterConfig = (*CorruptPubSubAdapterConfig)(nil)

func NewCorruptPubSubAdapterConfig(base *p2p.BasePubSubAdapterConfig) *CorruptPubSubAdapterConfig {
	return &CorruptPubSubAdapterConfig{
		options: defaultCorruptPubsubOptions(base),
	}
}

func (c *CorruptPubSubAdapterConfig) WithRoutingDiscovery(routing routing.ContentRouting) {
	c.options = append(c.options, corrupt.WithDiscovery(discoveryRouting.NewRoutingDiscovery(routing)))
}

func (c *CorruptPubSubAdapterConfig) WithSubscriptionFilter(filter p2p.SubscriptionFilter) {
	c.options = append(c.options, corrupt.WithSubscriptionFilter(filter))
}

func (c *CorruptPubSubAdapterConfig) WithScoreOption(_ p2p.ScoreOption) {
	// Corrupt does not support score options. This is a no-op.
}

func (c *CorruptPubSubAdapterConfig) WithMessageIdFunction(f func([]byte) string) {
	c.options = append(c.options, corrupt.WithMessageIdFn(func(pmsg *pb.Message) string {
		return f(pmsg.Data)
	}))
}

func (c *CorruptPubSubAdapterConfig) Build() []corrupt.Option {
	return c.options
}

func defaultCorruptPubsubOptions(base *p2p.BasePubSubAdapterConfig) []corrupt.Option {
	return []corrupt.Option{
		corrupt.WithMessageSigning(true),
		corrupt.WithStrictSignatureVerification(true),
		corrupt.WithMaxMessageSize(base.MaxMessageSize),
	}
}
