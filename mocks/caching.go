package mocks

type MockCaching struct {
	MockGetAppName func(string) string
	InstanceName   string
}

func (c *MockCaching) GetAppName(appGuid string) string {
	return c.MockGetAppName(appGuid)
}

func (c *MockCaching) GetInstanceName() string {
	return c.InstanceName
}

func (c *MockCaching) Initialize() {
	return
}
