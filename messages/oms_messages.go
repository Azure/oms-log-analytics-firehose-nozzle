package messages

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	hex "encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	events "github.com/cloudfoundry/sonde-go/events"
	"github.com/lizzha/pcf-oms-poc/caching"
)

var Caching *caching.Caching

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
func NewBaseMessage(e *events.Envelope, nozzleInstanceName string) *BaseMessage {
	var b = BaseMessage{
		EventType:      e.GetEventType().String(),
		Deployment:     e.GetDeployment(),
		Job:            e.GetJob(),
		Index:          e.GetIndex(),
		IP:             e.GetIp(),
		NozzleInstance: nozzleInstanceName,
	}
	if e.Timestamp != nil {
		b.EventTime = time.Unix(0, *e.Timestamp)
	} else {
		fmt.Printf("Message did not have timestamp. EventType:%s Event:%s\n", b.EventType, e.String())
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
func NewHTTPStartStop(e *events.Envelope, nozzleInstanceName string) *HTTPStartStop {
	var m = e.GetHttpStartStop()
	var r = HTTPStartStop{
		BaseMessage:    *NewBaseMessage(e, nozzleInstanceName),
		StartTimestamp: m.GetStartTimestamp(),
		StopTimestamp:  m.GetStopTimestamp(),
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
		id := cfUUIDToString(m.ApplicationId)
		r.ApplicationID = id
		r.ApplicationName, _ = Caching.GetAppName(id)
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
func NewLogMessage(e *events.Envelope, nozzleInstanceName string) *LogMessage {
	var m = e.GetLogMessage()
	var r = LogMessage{
		BaseMessage:    *NewBaseMessage(e, nozzleInstanceName),
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
	r.ApplicationName, _ = Caching.GetAppName(r.AppID)
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
func NewError(e *events.Envelope, nozzleInstanceName string) *Error {
	return &Error{
		BaseMessage: *NewBaseMessage(e, nozzleInstanceName),
		Source:      *e.Error.Source,
		Code:        *e.Error.Code,
		Message:     *e.Error.Message,
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
func NewContainerMetric(e *events.Envelope, nozzleInstanceName string) *ContainerMetric {
	var r = ContainerMetric{
		BaseMessage:      *NewBaseMessage(e, nozzleInstanceName),
		ApplicationID:    *e.ContainerMetric.ApplicationId,
		InstanceIndex:    *e.ContainerMetric.InstanceIndex,
		CPUPercentage:    *e.ContainerMetric.CpuPercentage,
		MemoryBytes:      *e.ContainerMetric.MemoryBytes,
		DiskBytes:        *e.ContainerMetric.DiskBytes,
		MemoryBytesQuota: *e.ContainerMetric.MemoryBytesQuota,
		DiskBytesQuota:   *e.ContainerMetric.DiskBytesQuota,
	}
	r.ApplicationName, _ = Caching.GetAppName(r.ApplicationID)
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
func NewCounterEvent(e *events.Envelope, nozzleInstanceName string) *CounterEvent {
	var r = CounterEvent{
		BaseMessage: *NewBaseMessage(e, nozzleInstanceName),
		Delta:       *e.CounterEvent.Delta,
		Total:       *e.CounterEvent.Total,
	}
	r.CounterKey = fmt.Sprintf("%s.%s", r.Job, r.Name)
	r.Name = e.GetOrigin() + "." + e.GetCounterEvent().GetName()
	if strings.Contains(r.Name, "TruncatingBuffer.DroppedMessage") {
		fmt.Fprintln(os.Stderr, "Received Dropped Message Event")
	}
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
func NewValueMetric(e *events.Envelope, nozzleInstanceName string) *ValueMetric {
	var r = ValueMetric{
		BaseMessage: *NewBaseMessage(e, nozzleInstanceName),
		Value:       *e.ValueMetric.Value,
		Unit:        *e.ValueMetric.Unit,
	}
	r.Name = e.GetOrigin() + "." + e.GetValueMetric().GetName()
	r.MetricKey = fmt.Sprintf("%s.%s", r.Job, r.Name)
	return &r
}

func cfUUIDToString(uuid *events.UUID) string {
	lowBytes := new(bytes.Buffer)
	binary.Write(lowBytes, binary.LittleEndian, uuid.Low)
	highBytes := new(bytes.Buffer)
	binary.Write(highBytes, binary.LittleEndian, uuid.High)
	return fmt.Sprintf("%x-%x-%x-%x-%x", lowBytes.Bytes()[0:4], lowBytes.Bytes()[4:6], lowBytes.Bytes()[6:8], highBytes.Bytes()[0:2], highBytes.Bytes()[2:])
}
