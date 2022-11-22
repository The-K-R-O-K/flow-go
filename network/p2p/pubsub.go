package p2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
)

type ValidationResult int

const (
	ValidationAccept ValidationResult = iota
	ValidationIgnore
	ValidationReject
)

type TopicValidatorFunc func(context.Context, peer.ID, *pubsub.Message) ValidationResult

type PubSubAdapter interface {
	RegisterTopicValidator(topic string, topicValidator TopicValidatorFunc) error
	UnregisterTopicValidator(topic string) error
	Join(topic string) (Topic, error)
	GetTopics() []string
	ListPeers(topic string) []peer.ID
}

type PubSubAdapterConfig interface {
	WithRoutingDiscovery(routing.ContentRouting)
	WithSubscriptionFilter(SubscriptionFilter)
	WithScoreOption(ScoreOption)
	WithMessageIdFunction(f func([]byte) string)
}

type Topic interface {
	String() string
	Close() error
	Publish(context.Context, []byte) error
	Subscribe() (Subscription, error)
}

type ScoreOption interface {
	BuildFlowPubSubScoreOption() pubsub.Option
}

type Subscription interface {
	Cancel()
	Topic() string
	Next(context.Context) (*pubsub.Message, error)
}

type BasePubSubAdapterConfig struct {
	MaxMessageSize int
}

type SubscriptionFilter interface {
	CanSubscribe(string) bool
	FilterIncomingSubscriptions(from peer.ID, opts []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error)
}
