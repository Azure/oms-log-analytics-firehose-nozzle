package mocks

import (
	"github.com/cloudfoundry/sonde-go/events"
)

type MockFirehoseClient struct {
	MessageChan chan *events.Envelope
	ErrChan     chan error
}

func NewMockFirehoseClient() *MockFirehoseClient {
	return &MockFirehoseClient{
		MessageChan: make(chan *events.Envelope),
		ErrChan:     make(chan error),
	}
}

func (c *MockFirehoseClient) Connect() (<-chan *events.Envelope, <-chan error) {
	return c.MessageChan, c.ErrChan
}

func (c *MockFirehoseClient) CloseConsumer() error {
	return nil
}
