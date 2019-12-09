package mocks

import "github.com/Azure/oms-log-analytics-firehose-nozzle/caching"

type MockCaching struct {
	MockGetAppInfo  func(string) caching.AppInfo
	InstanceName    string
	EnvironmentName string
}

func (c *MockCaching) GetAppInfo(appGuid string) caching.AppInfo {
	return c.MockGetAppInfo(appGuid)
}

func (c *MockCaching) GetInstanceName() string {
	return c.InstanceName
}

func (c *MockCaching) GetEnvironmentName() string {
	return c.EnvironmentName
}

func (c *MockCaching) Initialize(loadApps bool) {
	return
}
