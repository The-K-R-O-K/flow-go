package mock

import (
	"errors"

	"github.com/stretchr/testify/mock"
)

type WebsocketConnectionMock struct {
	mock.Mock
	socket     chan interface{}
	testSocket chan interface{}
	//TODO: need 2 channels or 2 routines
	closed bool
}

func NewWebsocketConnectionCustom() *WebsocketConnectionMock {
	return &WebsocketConnectionMock{
		socket:     make(chan interface{}, 2),
		testSocket: make(chan interface{}, 2),
	}
}

func (c *WebsocketConnectionMock) ReadJSON(v interface{}) error {
	v, ok := <-c.socket
	if !ok {
		c.closed = true
		return errors.New("channel closed")
	}

	return nil
}

func (c *WebsocketConnectionMock) ReadTest(v interface{}) error {
	v, ok := <-c.testSocket
	if !ok {
		c.closed = true
		return errors.New("channel closed")
	}

	return nil
}

func (c *WebsocketConnectionMock) WriteJSON(v interface{}) error {
	if c.closed {
		return errors.New("channel closed")
	}

	c.socket <- v
	c.testSocket <- v
	return nil
}

func (c *WebsocketConnectionMock) Close() error {
	c.closed = true
	close(c.socket)
	return nil
}
