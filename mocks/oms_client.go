package mocks

import "sync"

type MockOmsClient struct {
	postedMessages map[string]string
	mutex          sync.Mutex
}

func NewMockOmsClient() *MockOmsClient {
	return &MockOmsClient{
		postedMessages: make(map[string]string),
	}
}

func (c *MockOmsClient) PostData(msg *[]byte, logType string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.postedMessages[logType] = string(*msg)
	return nil
}

func (c *MockOmsClient) GetPostedMessages() map[string]string {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.postedMessages
}
