package internal

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
)

type RelayNotifee struct {
	n []network.Notifiee
}

var _ network.Notifiee = (*RelayNotifee)(nil)

func (r *RelayNotifee) Listen(n network.Network, multiaddr multiaddr.Multiaddr) {
	for _, notifiee := range r.n {
		notifiee.Listen(n, multiaddr)
	}
}

func (r *RelayNotifee) ListenClose(n network.Network, multiaddr multiaddr.Multiaddr) {
	for _, notifiee := range r.n {
		notifiee.ListenClose(n, multiaddr)
	}
}

func (r *RelayNotifee) Connected(n network.Network, conn network.Conn) {
	for _, notifiee := range r.n {
		notifiee.Connected(n, conn)
	}
}

func (r *RelayNotifee) Disconnected(n network.Network, conn network.Conn) {
	for _, notifiee := range r.n {
		notifiee.Disconnected(n, conn)
	}
}
