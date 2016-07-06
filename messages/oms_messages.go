package messages

import (
	"crypto/md5"
	hex "encoding/hex"
	"fmt"
	"strings"
	"time"

	events "github.com/cloudfoundry/sonde-go/events"
	"github.com/dave-read/pcf-oms-poc/client"
)

// BaseMessage contains common data elements
type BaseMessage struct {
	EventType      string
	Deployment     string
	Timestamp      time.Time
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
func NewBaseMessage(e *events.Envelope) *BaseMessage {
	var b = BaseMessage{
		EventType:      e.GetEventType().String(),
		Deployment:     e.GetDeployment(),
		Job:            e.GetJob(),
		Index:          e.GetIndex(),
		IP:             e.GetIp(),
		NozzleInstance: client.NozzleInstance,
	}
	if e.Timestamp != nil {
		b.Timestamp = time.Unix(0, *e.Timestamp)
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

// An HTTPStart event is emitted when a client sends a request (or immediately when a server receives the request).
type HTTPStart struct {
	BaseMessage
	Timestamp       time.Time
	RequestID       string
	PeerType        string // Client/Server
	Method          string // HTTP Method
	URI             string
	RemoteAddress   string
	UserAgent       string
	ParentRequestID string
	ApplicationID   string
	InstanceIndex   int32
	InstanceID      string
}

// NewHTTPStart creates a new NewHTTPStart
func NewHTTPStart(e *events.Envelope) *HTTPStart {
	var m = e.GetHttpStart()
	var r = HTTPStart{
		Timestamp:     time.Unix(0, m.GetTimestamp()),
		BaseMessage:   *NewBaseMessage(e),
		URI:           m.GetUri(),
		RemoteAddress: m.GetRemoteAddress(),
		UserAgent:     m.GetUserAgent(),
		InstanceIndex: m.GetInstanceIndex(),
		InstanceID:    m.GetInstanceId(),
	}
	if m.RequestId != nil {
		r.RequestID = m.GetRequestId().String()
	}
	if m.PeerType != nil {
		r.PeerType = m.GetPeerType().String() // Client/Server
	}
	if m.Method != nil {
		r.Method = m.GetMethod().String() // HTTP method
	}
	if m.ParentRequestId != nil {
		r.ParentRequestID = m.GetParentRequestId().String()
	}
	if m.ApplicationId != nil {
		r.ApplicationID = m.GetApplicationId().String()
	}
	return &r
}

// An HTTPStop event is emitted when a client receives a response to its request (or when a server completes its handling and returns a response).
type HTTPStop struct {
	BaseMessage
	Timestamp     time.Time
	URI           string
	RequestID     string
	PeerType      string // Client/Server
	StatusCode    int32  // HTTP Status
	ContentLength int64
	ApplicationID string
}

// NewHTTPStop creates a new NewHTTPStop
func NewHTTPStop(e *events.Envelope) *HTTPStop {
	var m = e.GetHttpStop()
	var r = HTTPStop{
		BaseMessage:   *NewBaseMessage(e),
		Timestamp:     time.Unix(0, m.GetTimestamp()),
		URI:           m.GetUri(),
		StatusCode:    m.GetStatusCode(),
		ContentLength: m.GetContentLength(),
	}
	if m.RequestId != nil {
		r.RequestID = m.GetRequestId().String()
	}
	if m.PeerType != nil {
		r.PeerType = m.GetPeerType().String() // Client/Server
	}
	if m.ApplicationId != nil {
		r.ApplicationID = m.GetApplicationId().String()
	}
	return &r
}

// An HTTPStartStop event represents the whole lifecycle of an HTTP request.
type HTTPStartStop struct {
	BaseMessage
	StartTimestamp time.Time
	StopTimestamp  time.Time
	RequestID      string
	PeerType       string // Client/Server
	Method         string // HTTP method
	URI            string
	RemoteAddress  string
	UserAgent      string
	StatusCode     int32
	ContentLength  int64
	ApplicationID  string
	InstanceIndex  int32
	InstanceID     string
	Forwarded      string
}

// NewHTTPStartStop creates a new NewHTTPStartStop
func NewHTTPStartStop(e *events.Envelope) *HTTPStartStop {

	var m = e.GetHttpStartStop()
	var r = HTTPStartStop{
		BaseMessage:    *NewBaseMessage(e),
		StartTimestamp: time.Unix(0, m.GetStartTimestamp()),
		StopTimestamp:  time.Unix(0, m.GetStopTimestamp()),
		URI:            m.GetUri(),
		RemoteAddress:  m.GetRemoteAddress(),
		UserAgent:      m.GetUserAgent(),
		StatusCode:     m.GetStatusCode(),
		ContentLength:  m.GetContentLength(),
		InstanceIndex:  m.GetInstanceIndex(),
		InstanceID:     m.GetInstanceId(),
	}
	if m.RequestId != nil {
		r.RequestID = m.GetRequestId().String()
	}
	if m.PeerType != nil {
		r.PeerType = m.GetPeerType().String() // Client/Server
	}
	if m.Method != nil {
		r.Method = m.GetMethod().String() // HTTP method
	}
	if m.ApplicationId != nil {
		r.ApplicationID = m.GetApplicationId().String()
	}

	if e.HttpStartStop.GetForwarded() != nil {
		r.Forwarded = strings.Join(e.GetHttpStartStop().GetForwarded(), ",")
	}
	return &r
}

//A LogMessage contains a "log line" and associated metadata.
type LogMessage struct {
	BaseMessage
	Message        string
	MessageType    string // OUT or ERROR
	Timestamp      time.Time
	AppID          string
	SourceType     string // APP,RTR,DEA,STG,etc
	SourceInstance string
	SourceTypeKey  string // Key for aggregation until multiple levels of grouping supported
}

// NewLogMessage creates a new NewLogMessage
func NewLogMessage(e *events.Envelope) *LogMessage {
	var m = e.GetLogMessage()
	var r = LogMessage{
		BaseMessage:    *NewBaseMessage(e),
		Timestamp:      time.Unix(0, *e.LogMessage.Timestamp),
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
func NewError(e *events.Envelope) *Error {
	return &Error{
		BaseMessage: *NewBaseMessage(e),
		Source:      *e.Error.Source,
		Code:        *e.Error.Code,
		Message:     *e.Error.Message,
	}
}

// A ContainerMetric records resource usage of an app in a container.
type ContainerMetric struct {
	BaseMessage
	ApplicationID string
	InstanceIndex int32
	CPUPercentage float64 `json:",omitempty"`
	MemoryBytes   uint64  `json:",omitempty"`
	DiskBytes     uint64  `json:",omitempty"`
}

// NewContainerMetric creates a new Container Metric
func NewContainerMetric(e *events.Envelope) *ContainerMetric {
	return &ContainerMetric{
		BaseMessage:   *NewBaseMessage(e),
		ApplicationID: *e.ContainerMetric.ApplicationId,
		InstanceIndex: *e.ContainerMetric.InstanceIndex,
		CPUPercentage: *e.ContainerMetric.CpuPercentage,
		MemoryBytes:   *e.ContainerMetric.MemoryBytes,
		DiskBytes:     *e.ContainerMetric.DiskBytes,
	}
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
func NewCounterEvent(e *events.Envelope) *CounterEvent {
	var r = CounterEvent{
		BaseMessage: *NewBaseMessage(e),
		Delta:       *e.CounterEvent.Delta,
		Total:       *e.CounterEvent.Total,
	}
	r.CounterKey = fmt.Sprintf("%s.%s", r.Job, r.Name)
	r.Name = e.GetOrigin() + "." + e.GetCounterEvent().GetName()
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
func NewValueMetric(e *events.Envelope) *ValueMetric {
	var r = ValueMetric{
		BaseMessage: *NewBaseMessage(e),
		Value:       *e.ValueMetric.Value,
		Unit:        *e.ValueMetric.Unit,
	}
	r.Name = e.GetOrigin() + "." + e.GetValueMetric().GetName()
	r.MetricKey = fmt.Sprintf("%s.%s", r.Job, r.Name)
	return &r
}
