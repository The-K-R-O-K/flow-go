package hand_mock

import (
	"errors"

	"github.com/onflow/flow-go/engine/access/rest/websockets"
)

//TODO: we have to use read and write socket for sure

type WebsocketConnectionMock struct {
	readSocket  chan interface{}
	writeSocket chan interface{}
	closed      bool
}

var _ websockets.WebsocketConnection = (*WebsocketConnectionMock)(nil)

func NewWebsocketConnectionMock() *WebsocketConnectionMock {
	return &WebsocketConnectionMock{
		readSocket:  make(chan interface{}, 20),
		writeSocket: make(chan interface{}, 20),
		closed:      false,
	}
}

func (m *WebsocketConnectionMock) ReadJSON(value interface{}) error {
	val, ok := <-m.readSocket
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

	m.readSocket <- value
	return nil
}

func (m *WebsocketConnectionMock) Close() error {
	close(m.readSocket)
	m.closed = true
	return nil
}
