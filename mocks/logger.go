package mocks

import (
	"code.cloudfoundry.org/lager"
	"sync"
)

type MockLogger struct {
	logs  map[lager.LogLevel][]Log
	mutex sync.Mutex
}

type Log struct {
	Action string
	Err    error
	Data   []lager.Data
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		logs: make(map[lager.LogLevel][]Log),
	}
}

func (m *MockLogger) RegisterSink(lager.Sink) {
	panic("Not implemented")
}

func (m *MockLogger) Session(task string, data ...lager.Data) lager.Logger {
	panic("Not implemented")
}

func (m *MockLogger) SessionName() string {
	panic("Not implemented")
}

func (m *MockLogger) Debug(action string, data ...lager.Data) {
	m.mutex.Lock()
	m.logs[lager.DEBUG] = append(m.logs[lager.DEBUG], Log{
		Action: action,
		Data:   data,
	})
	m.mutex.Unlock()
}

func (m *MockLogger) Info(action string, data ...lager.Data) {
	m.mutex.Lock()
	m.logs[lager.INFO] = append(m.logs[lager.INFO], Log{
		Action: action,
		Data:   data,
	})
	m.mutex.Unlock()
}

func (m *MockLogger) Error(action string, err error, data ...lager.Data) {
	m.mutex.Lock()
	m.logs[lager.ERROR] = append(m.logs[lager.ERROR], Log{
		Action: action,
		Err:    err,
		Data:   data,
	})
	m.mutex.Unlock()
}

func (m *MockLogger) Fatal(action string, err error, data ...lager.Data) {
	panic("Not implemented")
}

func (m *MockLogger) WithData(lager.Data) lager.Logger {
	panic("Not implemented")
}

func (m *MockLogger) GetLogs(level lager.LogLevel) []Log {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.logs[level]
}
