package omsnozzle

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	events "github.com/cloudfoundry/sonde-go/events"
	"github.com/Azure/oms-log-analytics-firehose-nozzle/caching"
	"github.com/Azure/oms-log-analytics-firehose-nozzle/client"
	"github.com/Azure/oms-log-analytics-firehose-nozzle/firehose"
	"github.com/Azure/oms-log-analytics-firehose-nozzle/messages"
)

const (
	maxPostGoroutines = 1000
	// Max message size of a sigle post
	maxSizePerBatch = 20000000
)

type OmsNozzle struct {
	logger         lager.Logger
	errChan        <-chan error
	msgChan        <-chan *events.Envelope
	signalChan     chan os.Signal
	omsClient      client.Client
	firehoseClient firehose.Client
	nozzleConfig   *NozzleConfig
	goroutineSem   chan int // to control the number of active post goroutines
	cachingClient  caching.CachingClient
}

type NozzleConfig struct {
	OmsTypePrefix       string
	OmsBatchTime        time.Duration
	ExcludeMetricEvents bool
	ExcludeLogEvents    bool
	ExcludeHttpEvents   bool
}

func NewOmsNozzle(logger lager.Logger, firehoseClient firehose.Client, omsClient client.Client, nozzleConfig *NozzleConfig, caching caching.CachingClient) *OmsNozzle {
	return &OmsNozzle{
		logger:         logger,
		errChan:        make(<-chan error),
		msgChan:        make(<-chan *events.Envelope),
		signalChan:     make(chan os.Signal, 2),
		omsClient:      omsClient,
		firehoseClient: firehoseClient,
		nozzleConfig:   nozzleConfig,
		goroutineSem:   make(chan int, maxPostGoroutines),
		cachingClient:  caching,
	}
}

func (o *OmsNozzle) Start() error {
	o.cachingClient.Initialize()
	o.initialize()
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

func (o *OmsNozzle) postData(events *map[string][]byte) {
	for k, v := range *events {
		if len(v) > 0 {
			v = append(v, ']')
			o.logger.Debug("Posting to OMS",
				lager.Data{"event type": k},
				lager.Data{"size": len(v)})
			if len(o.nozzleConfig.OmsTypePrefix) > 0 {
				k = o.nozzleConfig.OmsTypePrefix + k
			}
			nRetries := 4
			for nRetries > 0 {
				requestStartTime := time.Now()
				if err := o.omsClient.PostData(&v, k); err != nil {
					nRetries--
					elapsedTime := time.Since(requestStartTime)
					o.logger.Error("error posting message to OMS", err,
						lager.Data{"msg type": k},
						lager.Data{"elapse time": elapsedTime.String()},
						lager.Data{"msg size": len(v)},
						lager.Data{"remaining attempts": nRetries})
					time.Sleep(time.Second * 1)
				} else {
					break
				}
			}
		}
	}
	<-o.goroutineSem
}

func (o *OmsNozzle) marshalAsJson(m *OMSMessage) []byte {
	if j, err := json.Marshal(&m); err != nil {
		o.logger.Error("error marshalling message to JSON", err, lager.Data{"message": *m})
		return nil
	} else {
		return j
	}
}

func (o *OmsNozzle) pushMsgAsJson(eventType string, events *map[string][]byte, msg *[]byte) {
	// Push json messages to json array format
	if len((*events)[eventType]) == 0 {
		(*events)[eventType] = append((*events)[eventType], '[')
	} else {
		(*events)[eventType] = append((*events)[eventType], ',')
	}
	(*events)[eventType] = append((*events)[eventType], *msg...)
}

func (o *OmsNozzle) routeEvents() error {
	pendingEvents := make(map[string][]byte)
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
			pendingEvents = make(map[string][]byte)
			o.goroutineSem <- 1
			go o.postData(&currentEvents)
		case msg := <-o.msgChan:
			// process message
			var omsMessage OMSMessage
			var omsMessageType = msg.GetEventType().String()
			switch msg.GetEventType() {
			// Metrics
			case events.Envelope_ValueMetric:
				if !o.nozzleConfig.ExcludeMetricEvents {
					omsMessage = messages.NewValueMetric(msg, o.cachingClient)
					if m := o.marshalAsJson(&omsMessage); m != nil {
						o.pushMsgAsJson(omsMessageType, &pendingEvents, &m)
					}
				}
			case events.Envelope_CounterEvent:
				if !o.nozzleConfig.ExcludeMetricEvents {
					m := messages.NewCounterEvent(msg, o.cachingClient)
					if strings.Contains(m.Name, "TruncatingBuffer.DroppedMessage") {
						o.logger.Error("received TruncatingBuffer alert", nil)
					}
					omsMessage = m
					if m := o.marshalAsJson(&omsMessage); m != nil {
						o.pushMsgAsJson(omsMessageType, &pendingEvents, &m)
					}
				}

			case events.Envelope_ContainerMetric:
				if !o.nozzleConfig.ExcludeMetricEvents {
					omsMessage = messages.NewContainerMetric(msg, o.cachingClient)
					if m := o.marshalAsJson(&omsMessage); m != nil {
						o.pushMsgAsJson(omsMessageType, &pendingEvents, &m)
					}
				}

			// Logs Errors
			case events.Envelope_LogMessage:
				if !o.nozzleConfig.ExcludeLogEvents {
					omsMessage = messages.NewLogMessage(msg, o.cachingClient)
					if m := o.marshalAsJson(&omsMessage); m != nil {
						o.pushMsgAsJson(omsMessageType, &pendingEvents, &m)
					}
				}

			case events.Envelope_Error:
				if !o.nozzleConfig.ExcludeLogEvents {
					omsMessage = messages.NewError(msg, o.cachingClient)
					if m := o.marshalAsJson(&omsMessage); m != nil {
						o.pushMsgAsJson(omsMessageType, &pendingEvents, &m)
					}
				}

			// HTTP Start/Stop
			case events.Envelope_HttpStartStop:
				if !o.nozzleConfig.ExcludeHttpEvents {
					omsMessage = messages.NewHTTPStartStop(msg, o.cachingClient)
					if m := o.marshalAsJson(&omsMessage); m != nil {
						o.pushMsgAsJson(omsMessageType, &pendingEvents, &m)
					}
				}
			default:
				o.logger.Info("uncategorized message", lager.Data{"message": msg.String()})
				continue
			}
			// When the size of one type reaches the max per batch, trigger the post immediately
			doPost := false
			for _, v := range pendingEvents {
				if len(v) >= maxSizePerBatch {
					doPost = true
					break
				}
			}
			if doPost {
				currentEvents := pendingEvents
				pendingEvents = make(map[string][]byte)
				o.goroutineSem <- 1
				go o.postData(&currentEvents)
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
