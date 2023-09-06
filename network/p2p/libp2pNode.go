package p2p

import (
	"context"

	kbucket "github.com/libp2p/go-libp2p-kbucket"
	"github.com/libp2p/go-libp2p/core/host"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"

	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
)

// CoreP2P service management capabilities
type CoreP2P interface {
	// Start the libp2p node.
	Start(ctx irrecoverable.SignalerContext)
	// Stop terminates the libp2p node.
	Stop() error
	// GetIPPort returns the IP and Port the libp2p node is listening on.
	GetIPPort() (string, string, error)
	// Host returns pointer to host object of node.
	Host() host.Host
	// SetComponentManager sets the component manager for the node.
	// SetComponentManager may be called at most once.
	SetComponentManager(cm *component.ComponentManager)
}

// PeerManagement set of node traits related to its lifecycle and metadata retrieval
type PeerManagement interface {
	// AddPeer adds a peer to this node by adding it to this node's peerstore and connecting to it.
	AddPeer(ctx context.Context, peerInfo peer.AddrInfo) error
	// RemovePeer closes the connection with the peer.
	RemovePeer(peerID peer.ID) error
	// ListPeers returns list of peer IDs for peers subscribed to the topic.
	ListPeers(topic string) []peer.ID
	// GetPeersForProtocol returns slice peer IDs for the specified protocol ID.
	GetPeersForProtocol(pid protocol.ID) peer.IDSlice
	// WithPeersProvider sets the PeersProvider for the peer manager.
	// If a peer manager factory is set, this method will set the peer manager's PeersProvider.
	WithPeersProvider(peersProvider PeersProvider)
	// PeerManagerComponent returns the component interface of the peer manager.
	PeerManagerComponent() component.Component
	// RequestPeerUpdate requests an update to the peer connections of this node using the peer manager.
	RequestPeerUpdate()
}

// Routable set of node routing capabilities
type Routable interface {
	// RoutingTable returns the node routing table
	RoutingTable() *kbucket.RoutingTable
	// SetRouting sets the node's routing implementation.
	// SetRouting may be called at most once.
	SetRouting(r routing.Routing)
	// Routing returns node routing object.
	Routing() routing.Routing
}

// StreamManagement peer to peer stream management functions
type UnicastManagement interface {
	// CreateStream returns an existing stream connected to the peer if it exists, or creates a new stream with it.
	CreateStream(ctx context.Context, peerID peer.ID) (libp2pnet.Stream, error)
	// WithDefaultUnicastProtocol overrides the default handler of the unicast manager and registers all preferred protocols.
	WithDefaultUnicastProtocol(defaultHandler libp2pnet.StreamHandler, preferred []protocols.ProtocolName) error
}

// PubSub publish subscribe features for node
type PubSub interface {
	// Subscribe subscribes the node to the given topic and returns the subscription
	Subscribe(topic channels.Topic, topicValidator TopicValidatorFunc) (Subscription, error)
	// UnSubscribe cancels the subscriber and closes the topic.
	Unsubscribe(topic channels.Topic) error
	// Publish publishes the given payload on the topic.
	Publish(ctx context.Context, messageScope flownet.OutgoingMessageScope) error
	// SetPubSub sets the node's pubsub implementation.
	// SetPubSub may be called at most once.
	SetPubSub(ps PubSubAdapter)
}

// LibP2PNode represents a Flow libp2p node. It provides the network layer with the necessary interface to
// control the underlying libp2p node. It is essentially the Flow wrapper around the libp2p node, and allows
// us to define different types of libp2p nodes that can operate in different ways by overriding these methods.
type LibP2PNode interface {
	module.ReadyDoneAware
	Subscriptions
	// PeerConnections connection status information per peer.
	PeerConnections
	// PeerScore exposes the peer score API.
	PeerScore
	// DisallowListNotificationConsumer exposes the disallow list notification consumer API for the node so that
	// it will be notified when a new disallow list update is distributed.
	DisallowListNotificationConsumer
	// CollectionClusterChangesConsumer  is the interface for consuming the events of changes in the collection cluster.
	// This is used to notify the node of changes in the collection cluster.
	// LibP2PNode implements this interface and consumes the events to be notified of changes in the clustering channels.
	// The clustering channels are used by the collection nodes of a cluster to communicate with each other.
	// As the cluster (and hence their cluster channels) of collection nodes changes over time (per epoch) the node needs to be notified of these changes.
	CollectionClusterChangesConsumer
	// DisallowListOracle exposes the disallow list oracle API for external consumers to query about the disallow list.
	DisallowListOracle

	// CoreP2P service management capabilities
	CoreP2P

	// PeerManagement current peer management functions
	PeerManagement

	// Routable routing related features
	Routable

	// PubSub publish subscribe features for node
	PubSub

	// UnicastManagement node stream management
	UnicastManagement
}

// Subscriptions set of funcs related to current subscription info of a node.
type Subscriptions interface {
	// HasSubscription returns true if the node currently has an active subscription to the topic.
	HasSubscription(topic channels.Topic) bool
	// SetUnicastManager sets the unicast manager for the node.
	SetUnicastManager(uniMgr UnicastManager)
}

// CollectionClusterChangesConsumer  is the interface for consuming the events of changes in the collection cluster.
// This is used to notify the node of changes in the collection cluster.
// LibP2PNode implements this interface and consumes the events to be notified of changes in the clustering channels.
// The clustering channels are used by the collection nodes of a cluster to communicate with each other.
// As the cluster (and hence their cluster channels) of collection nodes changes over time (per epoch) the node needs to be notified of these changes.
type CollectionClusterChangesConsumer interface {
	collection.ClusterEvents
}

// PeerScore is the interface for the peer score module. It is used to expose the peer score to other
// components of the node. It is also used to set the peer score exposer implementation.
type PeerScore interface {
	// PeerScoreExposer returns the node's peer score exposer implementation.
	// If the node's peer score exposer has not been set, the second return value will be false.
	PeerScoreExposer() PeerScoreExposer
}

// PeerConnections subset of funcs related to underlying libp2p host connections.
type PeerConnections interface {
	// IsConnected returns true if address is a direct peer of this node else false.
	// Peers are considered not connected if the underlying libp2p host reports the
	// peers as not connected and there are no connections in the connection list.
	// The following error returns indicate a bug in the code:
	//  * network.ErrIllegalConnectionState if the underlying libp2p host reports connectedness as NotConnected but the connections list
	// 	  to the peer is not empty. This indicates a bug within libp2p.
	IsConnected(peerID peer.ID) (bool, error)
}

// DisallowListNotificationConsumer is an interface for consuming disallow/allow list update notifications.
type DisallowListNotificationConsumer interface {
	// OnDisallowListNotification is called when a new disallow list update notification is distributed.
	// Any error on consuming event must handle internally.
	// The implementation must be concurrency safe.
	// Args:
	// 	id: peer ID of the peer being disallow-listed.
	// 	cause: cause of the peer being disallow-listed (only this cause is added to the peer's disallow-listed causes).
	// Returns:
	// 	none
	OnDisallowListNotification(id peer.ID, cause network.DisallowListedCause)

	// OnAllowListNotification is called when a new allow list update notification is distributed.
	// Any error on consuming event must handle internally.
	// The implementation must be concurrency safe.
	// Args:
	// 	id: peer ID of the peer being allow-listed.
	// 	cause: cause of the peer being allow-listed (only this cause is removed from the peer's disallow-listed causes).
	// Returns:
	// 	none
	OnAllowListNotification(id peer.ID, cause network.DisallowListedCause)
}

// DisallowListOracle is an interface for querying disallow-listed peers.
type DisallowListOracle interface {
	// IsDisallowListed determines whether the given peer is disallow-listed for any reason.
	// Args:
	// - peerID: the peer to check.
	// Returns:
	// - []network.DisallowListedCause: the list of causes for which the given peer is disallow-listed. If the peer is not disallow-listed for any reason,
	// a nil slice is returned.
	// - bool: true if the peer is disallow-listed for any reason, false otherwise.
	IsDisallowListed(peerId peer.ID) ([]network.DisallowListedCause, bool)
}
