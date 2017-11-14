package mocks

type AppInfo struct {
	Name    string
	Org     string
	OrgId   string
	Space   string
	SpaceId string
	Monitored bool
}

type MockCaching struct {
	MockGetAppInfo  func(string) AppInfo
	InstanceName    string
	EnvironmentName string
}

func (c *MockCaching) GetAppInfo(appGuid string) AppInfo {
	return c.MockGetAppInfo(appGuid)
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
