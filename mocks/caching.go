package mocks

type MockCaching struct {
	MockGetAppName  func(string) string
	InstanceName    string
	EnvironmentName string
}

func (c *MockCaching) GetAppName(appGuid string) string {
	return c.MockGetAppName(appGuid)
}

func (c *MockCaching) GetInstanceName() string {
	return c.InstanceName
}

func (c *MockCaching) GetEnvironmentName() string {
	return c.EnvironmentName
}

func (c *MockCaching) Initialize() {
	return
}
