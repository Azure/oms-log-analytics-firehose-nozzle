package messages

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	hex "encoding/hex"
	"fmt"
	"strings"
	"time"

	events "github.com/cloudfoundry/sonde-go/events"
	"github.com/lizzha/pcf-oms-poc/caching"
)

// BaseMessage contains common data elements
type BaseMessage struct {
	EventType      string
	Deployment     string
	EventTime      time.Time
	Job            string
	Index          string
	IP             string
	Tags           string
	NozzleInstance string
	MessageHash    string
	// for grouping in OMS until multi-field grouping is supported
	SourceInstance string
	Origin         string
}

// NewBaseMessage Creates the common attributes of messages
func NewBaseMessage(e *events.Envelope, c caching.CachingClient) *BaseMessage {
	var b = BaseMessage{
		EventType:      e.GetEventType().String(),
		Deployment:     e.GetDeployment(),
		Job:            e.GetJob(),
		Index:          e.GetIndex(),
		IP:             e.GetIp(),
		NozzleInstance: c.GetInstanceName(),
	}
	if e.Timestamp != nil {
		b.EventTime = time.Unix(0, *e.Timestamp)
	}
	if e.Origin != nil {
		b.Origin = e.GetOrigin()
	}
	if e.Deployment != nil && e.Job != nil && e.Index != nil {
		b.SourceInstance = fmt.Sprintf("%s.%s.%s", e.GetDeployment(), e.GetJob(), e.GetIndex())
	} else {
		b.SourceInstance = "MISSING"
	}

	if e.GetTags() != nil {
		b.Tags = fmt.Sprintf("%v", e.GetTags())
	}
	// String() returns string from underlying protobuf message
	var hash = md5.Sum([]byte(e.String()))
	b.MessageHash = hex.EncodeToString(hash[:])

	return &b
}

// An HTTPStartStop event represents the whole lifecycle of an HTTP request.
type HTTPStartStop struct {
	BaseMessage
	StartTimestamp  int64
	StopTimestamp   int64
	RequestID       string
	PeerType        string // Client/Server
	Method          string // HTTP method
	URI             string
	RemoteAddress   string
	UserAgent       string
	StatusCode      int32
	ContentLength   int64
	ApplicationID   string
	ApplicationName string
	InstanceIndex   int32
	InstanceID      string
	Forwarded       string
}

// NewHTTPStartStop creates a new NewHTTPStartStop
func NewHTTPStartStop(e *events.Envelope, c caching.CachingClient) *HTTPStartStop {
	var m = e.GetHttpStartStop()
	var r = HTTPStartStop{
		BaseMessage:    *NewBaseMessage(e, c),
		StartTimestamp: m.GetStartTimestamp(),
		StopTimestamp:  m.GetStopTimestamp(),
		PeerType:       m.GetPeerType().String(), // Client/Server
		Method:         m.GetMethod().String(),   // HTTP method
		URI:            m.GetUri(),
		RemoteAddress:  m.GetRemoteAddress(),
		UserAgent:      m.GetUserAgent(),
		StatusCode:     m.GetStatusCode(),
		ContentLength:  m.GetContentLength(),
		InstanceIndex:  m.GetInstanceIndex(),
		InstanceID:     m.GetInstanceId(),
	}
	if m.RequestId != nil {
		r.RequestID = cfUUIDToString(m.RequestId)
	}
	if m.ApplicationId != nil {
		id := cfUUIDToString(m.ApplicationId)
		r.ApplicationID = id
		r.ApplicationName = c.GetAppName(id)
	}
	if e.HttpStartStop.GetForwarded() != nil {
		r.Forwarded = strings.Join(e.GetHttpStartStop().GetForwarded(), ",")
	}
	return &r
}

//A LogMessage contains a "log line" and associated metadata.
type LogMessage struct {
	BaseMessage
	Message         string
	MessageType     string // OUT or ERROR
	Timestamp       int64
	AppID           string
	ApplicationName string
	SourceType      string // APP,RTR,DEA,STG,etc
	SourceInstance  string
	SourceTypeKey   string // Key for aggregation until multiple levels of grouping supported
}

// NewLogMessage creates a new NewLogMessage
func NewLogMessage(e *events.Envelope, c caching.CachingClient) *LogMessage {
	var m = e.GetLogMessage()
	var r = LogMessage{
		BaseMessage:    *NewBaseMessage(e, c),
		Timestamp:      m.GetTimestamp(),
		AppID:          m.GetAppId(),
		SourceType:     m.GetSourceType(),
		SourceInstance: m.GetSourceInstance(),
	}
	if m.Message != nil {
		r.Message = string(m.GetMessage())
	}
	if m.MessageType != nil {
		r.MessageType = m.MessageType.String()
		r.SourceTypeKey = r.SourceType + "-" + r.MessageType
	}
	if m.AppId != nil {
		r.ApplicationName = c.GetAppName(*m.AppId)
	}
	return &r
}

// An Error event represents an error in the originating process.
type Error struct {
	BaseMessage
	Source  string
	Code    int32
	Message string
}

// NewError creates a new NewError
func NewError(e *events.Envelope, c caching.CachingClient) *Error {
	return &Error{
		BaseMessage: *NewBaseMessage(e, c),
		Source:      e.Error.GetSource(),
		Code:        e.Error.GetCode(),
		Message:     e.Error.GetMessage(),
	}
}

// A ContainerMetric records resource usage of an app in a container.
type ContainerMetric struct {
	BaseMessage
	ApplicationID    string
	ApplicationName  string
	InstanceIndex    int32
	CPUPercentage    float64 `json:",omitempty"`
	MemoryBytes      uint64  `json:",omitempty"`
	DiskBytes        uint64  `json:",omitempty"`
	MemoryBytesQuota uint64  `json:",omitempty"`
	DiskBytesQuota   uint64  `json:",omitempty"`
}

// NewContainerMetric creates a new Container Metric
func NewContainerMetric(e *events.Envelope, c caching.CachingClient) *ContainerMetric {
	var m = e.GetContainerMetric()
	var r = ContainerMetric{
		BaseMessage:      *NewBaseMessage(e, c),
		ApplicationID:    m.GetApplicationId(),
		InstanceIndex:    m.GetInstanceIndex(),
		CPUPercentage:    m.GetCpuPercentage(),
		MemoryBytes:      m.GetMemoryBytes(),
		DiskBytes:        m.GetDiskBytes(),
		MemoryBytesQuota: m.GetMemoryBytesQuota(),
		DiskBytesQuota:   m.GetDiskBytesQuota(),
	}
	if m.ApplicationId != nil {
		r.ApplicationName = c.GetAppName(*m.ApplicationId)
	}
	return &r
}

// A CounterEvent represents the increment of a counter. It contains only the change in the value; it is the responsibility of downstream consumers to maintain the value of the counter.
type CounterEvent struct {
	BaseMessage
	Name       string
	Delta      uint64
	Total      uint64
	CounterKey string
}

// NewCounterEvent creates a new CounterEvent
func NewCounterEvent(e *events.Envelope, c caching.CachingClient) *CounterEvent {
	var r = CounterEvent{
		BaseMessage: *NewBaseMessage(e, c),
		Name:        e.CounterEvent.GetName(),
		Delta:       e.CounterEvent.GetDelta(),
		Total:       e.CounterEvent.GetTotal(),
	}
	r.CounterKey = fmt.Sprintf("%s.%s.%s", r.Job, e.GetOrigin(), r.Name)
	return &r
}

// A ValueMetric indicates the value of a metric at an instant in time.
type ValueMetric struct {
	BaseMessage
	Name      string
	Value     float64
	Unit      string
	MetricKey string
}

// NewValueMetric creates a new ValueMetric
func NewValueMetric(e *events.Envelope, c caching.CachingClient) *ValueMetric {
	var r = ValueMetric{
		BaseMessage: *NewBaseMessage(e, c),
		Name:        e.ValueMetric.GetName(),
		Value:       e.ValueMetric.GetValue(),
		Unit:        e.ValueMetric.GetUnit(),
	}
	r.MetricKey = fmt.Sprintf("%s.%s.%s", r.Job, e.GetOrigin(), r.Name)
	return &r
}

func cfUUIDToString(uuid *events.UUID) string {
	lowBytes := new(bytes.Buffer)
	binary.Write(lowBytes, binary.LittleEndian, uuid.Low)
	highBytes := new(bytes.Buffer)
	binary.Write(highBytes, binary.LittleEndian, uuid.High)
	return fmt.Sprintf("%x-%x-%x-%x-%x", lowBytes.Bytes()[0:4], lowBytes.Bytes()[4:6], lowBytes.Bytes()[6:8], highBytes.Bytes()[0:2], highBytes.Bytes()[2:])
}
