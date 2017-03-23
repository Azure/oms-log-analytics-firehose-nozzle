package omsnozzle

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/Azure/oms-log-analytics-firehose-nozzle/caching"
	"github.com/Azure/oms-log-analytics-firehose-nozzle/client"
	"github.com/Azure/oms-log-analytics-firehose-nozzle/firehose"
	"github.com/Azure/oms-log-analytics-firehose-nozzle/messages"
	events "github.com/cloudfoundry/sonde-go/events"
)

const (
	maxPostGoroutines = 1000
	// Max message size of a sigle post
	maxSizePerBatch = 20000000
)

type OmsNozzle struct {
	logger              lager.Logger
	errChan             <-chan error
	msgChan             <-chan *events.Envelope
	signalChan          chan os.Signal
	omsClient           client.Client
	firehoseClient      firehose.Client
	nozzleConfig        *NozzleConfig
	goroutineSem        chan int // to control the number of active post goroutines
	cachingClient       caching.CachingClient
	totalEventsReceived uint64
	totalEventsSent     uint64
	mutex               *sync.Mutex
}

type NozzleConfig struct {
	OmsTypePrefix         string
	OmsBatchTime          time.Duration
	OmsMaxMsgNumPerBatch  int
	ExcludeMetricEvents   bool
	ExcludeLogEvents      bool
	ExcludeHttpEvents     bool
	LogEventCount         bool
	LogEventCountInterval time.Duration
}

func NewOmsNozzle(logger lager.Logger, firehoseClient firehose.Client, omsClient client.Client, nozzleConfig *NozzleConfig, caching caching.CachingClient) *OmsNozzle {
	return &OmsNozzle{
		logger:              logger,
		errChan:             make(<-chan error),
		msgChan:             make(<-chan *events.Envelope),
		signalChan:          make(chan os.Signal, 2),
		omsClient:           omsClient,
		firehoseClient:      firehoseClient,
		nozzleConfig:        nozzleConfig,
		goroutineSem:        make(chan int, maxPostGoroutines),
		cachingClient:       caching,
		totalEventsReceived: uint64(0),
		totalEventsSent:     uint64(0),
		mutex:               &sync.Mutex{},
	}
}

func (o *OmsNozzle) Start() error {
	o.cachingClient.Initialize()
	o.initialize()
	if o.nozzleConfig.LogEventCount {
		o.logTotalEvents(o.nozzleConfig.LogEventCountInterval)
	}
	err := o.routeEvents()
	return err
}

func (o *OmsNozzle) initialize() {
	// setup for termination signal from CF
	signal.Notify(o.signalChan, syscall.SIGTERM, syscall.SIGINT)

	o.msgChan, o.errChan = o.firehoseClient.Connect()

	//o.consumer.SetDebugPrinter(ConsoleDebugPrinter{})
	// async error channel
	go func() {
		var errorChannelCount = 0
		for err := range o.errChan {
			errorChannelCount++
			o.logger.Error("firehose channel error", err, lager.Data{"error count": errorChannelCount})
		}
	}()
}

func (o *OmsNozzle) logTotalEvents(interval time.Duration) {
	logEventCountTicker := time.NewTicker(interval)
	lastReceivedCount := uint64(0)
	lastSentCount := uint64(0)

	go func() {
		for range logEventCountTicker.C {
			timeStamp := time.Now().UnixNano()
			totalReceivedCount := o.totalEventsReceived
			totalSentCount := o.totalEventsSent
			currentEvents := make(map[string][]interface{})
			var omsMsg OMSMessage
			var msg *events.Envelope

			// Generate CounterEvent
			eventType := events.Envelope_CounterEvent.String()
			msg = o.createEventCountEvent("eventsReceived", totalReceivedCount-lastReceivedCount, totalReceivedCount, &timeStamp)
			omsMsg = messages.NewCounterEvent(msg, o.cachingClient)
			currentEvents[eventType] = append(currentEvents[eventType], omsMsg)
			msg = o.createEventCountEvent("eventsSent", totalSentCount-lastSentCount, totalSentCount, &timeStamp)
			omsMsg = messages.NewCounterEvent(msg, o.cachingClient)
			currentEvents[eventType] = append(currentEvents[eventType], omsMsg)

			o.goroutineSem <- 1
			o.postData(&currentEvents, false)

			lastReceivedCount = totalReceivedCount
			lastSentCount = totalSentCount
		}
	}()
}

func (o *OmsNozzle) createEventCountEvent(name string, deltaCount uint64, count uint64, timeStamp *int64) *events.Envelope {
	counterEvent := &events.CounterEvent{
		Name:  &name,
		Delta: &deltaCount,
		Total: &count,
	}

	eventType := events.Envelope_CounterEvent
	job := "nozzle"
	origin := "stats"
	envelope := &events.Envelope{
		EventType:    &eventType,
		Timestamp:    timeStamp,
		Job:          &job,
		Origin:       &origin,
		CounterEvent: counterEvent,
	}

	return envelope
}

func (o *OmsNozzle) postData(events *map[string][]interface{}, addCount bool) {
	for k, v := range *events {
		if len(v) > 0 {
			if msgAsJson, err := json.Marshal(&v); err != nil {
				o.logger.Error("error marshalling message to JSON", err,
					lager.Data{"event type": k},
					lager.Data{"event count": len(v)})
			} else {
				o.logger.Debug("Posting to OMS",
					lager.Data{"event type": k},
					lager.Data{"event count": len(v)},
					lager.Data{"total size": len(msgAsJson)})
				if len(o.nozzleConfig.OmsTypePrefix) > 0 {
					k = o.nozzleConfig.OmsTypePrefix + k
				}
				nRetries := 4
				for nRetries > 0 {
					requestStartTime := time.Now()
					if err = o.omsClient.PostData(&msgAsJson, k); err != nil {
						nRetries--
						elapsedTime := time.Since(requestStartTime)
						o.logger.Error("error posting message to OMS", err,
							lager.Data{"event type": k},
							lager.Data{"elapse time": elapsedTime.String()},
							lager.Data{"event count": len(v)},
							lager.Data{"total size": len(msgAsJson)},
							lager.Data{"remaining attempts": nRetries})
						time.Sleep(time.Second * 1)
					} else {
						if addCount {
							o.mutex.Lock()
							o.totalEventsSent += uint64(len(v))
							o.mutex.Unlock()
						}
						break
					}
				}
			}
		}
	}
	<-o.goroutineSem
}

func (o *OmsNozzle) routeEvents() error {
	pendingEvents := make(map[string][]interface{})
	// Firehose message processing loop
	ticker := time.NewTicker(o.nozzleConfig.OmsBatchTime)
	for {
		// loop over message and signal channel
		select {
		case s := <-o.signalChan:
			o.logger.Info("exiting", lager.Data{"signal caught": s.String()})
			err := o.firehoseClient.CloseConsumer()
			if err != nil {
				o.logger.Error("error closing consumer", err)
			}
			os.Exit(1)
		case <-ticker.C:
			// get the pending as current
			currentEvents := pendingEvents
			// reset the pending events
			pendingEvents = make(map[string][]interface{})
			o.goroutineSem <- 1
			go o.postData(&currentEvents, true)
		case msg := <-o.msgChan:
			o.totalEventsReceived++
			// process message
			var omsMessage OMSMessage
			var omsMessageType = msg.GetEventType().String()
			switch msg.GetEventType() {
			// Metrics
			case events.Envelope_ValueMetric:
				if !o.nozzleConfig.ExcludeMetricEvents {
					omsMessage = messages.NewValueMetric(msg, o.cachingClient)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}
			case events.Envelope_CounterEvent:
				if !o.nozzleConfig.ExcludeMetricEvents {
					m := messages.NewCounterEvent(msg, o.cachingClient)
					if strings.Contains(m.Name, "TruncatingBuffer.DroppedMessage") {
						o.logger.Error("received TruncatingBuffer alert", nil)
					}
					omsMessage = m
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			case events.Envelope_ContainerMetric:
				if !o.nozzleConfig.ExcludeMetricEvents {
					omsMessage = messages.NewContainerMetric(msg, o.cachingClient)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			// Logs Errors
			case events.Envelope_LogMessage:
				if !o.nozzleConfig.ExcludeLogEvents {
					omsMessage = messages.NewLogMessage(msg, o.cachingClient)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			case events.Envelope_Error:
				if !o.nozzleConfig.ExcludeLogEvents {
					omsMessage = messages.NewError(msg, o.cachingClient)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			// HTTP Start/Stop
			case events.Envelope_HttpStartStop:
				if !o.nozzleConfig.ExcludeHttpEvents {
					omsMessage = messages.NewHTTPStartStop(msg, o.cachingClient)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}
			default:
				o.logger.Info("uncategorized message", lager.Data{"message": msg.String()})
				continue
			}
			// When the size of one type reaches the max per batch, trigger the post immediately
			doPost := false
			for _, v := range pendingEvents {
				if len(v) >= o.nozzleConfig.OmsMaxMsgNumPerBatch {
					doPost = true
					break
				}
			}
			if doPost {
				currentEvents := pendingEvents
				pendingEvents = make(map[string][]interface{})
				o.goroutineSem <- 1
				go o.postData(&currentEvents, true)
			}
		}
	}
}

// OMSMessage is a marker inteface for JSON formatted messages published to OMS
type OMSMessage interface{}

// ConsoleDebugPrinter for debug logging
type ConsoleDebugPrinter struct{}

// Print debug logging
func (c ConsoleDebugPrinter) Print(title, dump string) {
	fmt.Printf("Consumer debug.  title:%s detail:%s", title, dump)
}
