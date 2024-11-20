package mock

import (
	"errors"
	"github.com/onflow/flow-go/engine/access/rest/websockets"
)

type WebsocketConnectionMock struct {
	socket chan interface{}
	echo   chan interface{}
	closed bool
}

var _ websockets.WebsocketConnection = (*WebsocketConnectionMock)(nil)

func NewWebsocketConnectionMock() *WebsocketConnectionMock {
	return &WebsocketConnectionMock{
		socket: make(chan interface{}),
		echo:   make(chan interface{}),
		closed: false,
	}
}

func (m *WebsocketConnectionMock) ReadJSONFromEchoSocket(value interface{}) error {
	val, ok := <-m.echo
	if !ok {
		return errors.New("cannot read from closed connection")
	}

	value = val
	return nil
}

func (m *WebsocketConnectionMock) ReadJSON(value interface{}) error {
	val, ok := <-m.socket
	if !ok {
		return errors.New("cannot read from closed connection")
	}

	value = val
	return nil
}

func (m *WebsocketConnectionMock) WriteJSON(value interface{}) error {
	if m.closed {
		return errors.New("cannot write to closed connection")
	}

	m.socket <- value
	m.echo <- value
	return nil
}

func (m *WebsocketConnectionMock) Close() error {
	close(m.socket)
	close(m.echo)
	m.closed = true

	return nil
}
