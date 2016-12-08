package omsnozzle

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry-incubator/uaago"
	"github.com/cloudfoundry/noaa/consumer"
	events "github.com/cloudfoundry/sonde-go/events"
	"github.com/dave-read/pcf-oms-poc/client"
	"github.com/dave-read/pcf-oms-poc/messages"
)

type OmsNozzle struct {
	errChan            <-chan error
	msgChan            <-chan *events.Envelope
	signalChan         chan os.Signal
	consumer           *consumer.Consumer
	omsClient          *client.Client
	cfClientConfig     *cfclient.Config
	nozzleConfig       *NozzleConfig
	nozzleInstanceName string
}

type NozzleConfig struct {
	UaaAddress             string
	UaaClientName          string
	UaaClientSecret        string
	TrafficControllerUrl   string
	SkipSslValidation      bool
	IdleTimeout            time.Duration
	FirehoseSubscriptionId string
	OmsTypePrefix          string
	OmsBatchTime           time.Duration
	ExcludeMetricEvents    bool
	ExcludeLogEvents       bool
	ExcludeHttpEvents      bool
}

type CfClientTokenRefresh struct {
	cfClient *cfclient.Client
}

func NewOmsNozzle(cfClientConfig *cfclient.Config, omsClient *client.Client, nozzleConfig *NozzleConfig) *OmsNozzle {
	return &OmsNozzle{
		errChan:        make(<-chan error),
		msgChan:        make(<-chan *events.Envelope),
		signalChan:     make(chan os.Signal, 1),
		omsClient:      omsClient,
		cfClientConfig: cfClientConfig,
		nozzleConfig:   nozzleConfig,
	}
}

func (o *OmsNozzle) Start() error {
	o.initialize()
	err := o.routeEvents()
	return err
}

func (o *OmsNozzle) setInstanceName() error {
	// instance id to track multiple nozzles, used for logging
	hostName, err := os.Hostname()
	if err != nil {
		fmt.Printf("Error getting hostname for NozzleInstance: %s\n", err)
		o.nozzleInstanceName = fmt.Sprintf("pid-%d", os.Getpid())
	} else {
		o.nozzleInstanceName = fmt.Sprintf("pid-%d@%s", os.Getpid(), hostName)
	}
	fmt.Printf("Nozzle instance name: %s\n", o.nozzleInstanceName)
	return err
}

func (o *OmsNozzle) initialize() {
	// setup for termination signal from CF
	signal.Notify(o.signalChan, syscall.SIGTERM, syscall.SIGINT)
	o.setInstanceName()

	fmt.Printf("Starting with uaaAddress:%s, dopplerAddress:%s\n", o.nozzleConfig.UaaAddress, o.nozzleConfig.TrafficControllerUrl)
	uaaClient, err := uaago.NewClient(o.nozzleConfig.UaaAddress)
	if err != nil {
		panic("Error creating uaa client:" + err.Error())
	}
	authToken, err := uaaClient.GetAuthToken(o.nozzleConfig.UaaClientName, o.nozzleConfig.UaaClientSecret, o.nozzleConfig.SkipSslValidation)
	if err != nil {
		panic("Error getting auth token:" + err.Error())
	}

	o.consumer = consumer.New(
		o.nozzleConfig.TrafficControllerUrl,
		&tls.Config{InsecureSkipVerify: o.nozzleConfig.SkipSslValidation},
		nil)

	o.consumer.SetIdleTimeout(o.nozzleConfig.IdleTimeout)
	o.msgChan, o.errChan = o.consumer.Firehose(o.nozzleConfig.FirehoseSubscriptionId, authToken)

	//o.consumer.SetDebugPrinter(ConsoleDebugPrinter{})
	// async error channel
	go func() {
		var errorChannelCount = 0
		for err := range o.errChan {
			errorChannelCount++
			fmt.Fprintf(os.Stderr, "Firehose channel error.  Date:%v errorCount:%d error:%v\n", time.Now(), errorChannelCount, err.Error())
		}
	}()
}

func (o *OmsNozzle) routeEvents() error {
	// counters
	var msgReceivedCount = 0
	var msgSentCount = 0
	var msgSendErrorCount = 0

	pendingEvents := make(map[string][]interface{})
	// Firehose message processing loop
	ticker := time.NewTicker(o.nozzleConfig.OmsBatchTime)
	for {
		// loop over message and signal channel
		select {
		case s := <-o.signalChan:
			fmt.Printf("Signal caught:%s Exiting\n", s.String())
			err := o.consumer.Close()
			if err != nil {
				fmt.Printf("Error closing consumer:%v\n", err)
			}
			os.Exit(1)
		case <-ticker.C:
			// get the pending as current
			currentEvents := pendingEvents
			// reset the pending events
			pendingEvents = make(map[string][]interface{})
			go func() {
				//fmt.Printf("Timer fired ... processing events.  Total events:%d\n", msgReceivedCount)
				for k, v := range currentEvents {
					// OMS message as JSON
					msgAsJSON, err := json.Marshal(&v)
					if err != nil {
						fmt.Printf("Error marshalling message type %s to JSON. error: %s", k, err)
					} else {
						//fmt.Printf(string(msgAsJSON) + "\n")
						//fmt.Printf("   EventType:%s\tEventCount:%d\tJSONSize:%d\n", k, len(v), len(msgAsJSON))
						requestStartTime := time.Now()
						if len(o.nozzleConfig.OmsTypePrefix) > 0 {
							k = o.nozzleConfig.OmsTypePrefix + k
						}
						err = o.omsClient.PostData(&msgAsJSON, k)
						elapsedTime := time.Since(requestStartTime)
						if err != nil {
							msgSendErrorCount++
							fmt.Printf("Error posting message type %s to OMS. error: %s elapseTime:%s msgSize:%d\n", k, err, elapsedTime.String(), len(msgAsJSON))
						} else {
							msgSentCount++
						}
					}
				}
				//fmt.Print("Finished processing events.\n")
			}()
		case msg := <-o.msgChan:
			// process message
			msgReceivedCount++
			var omsMessage OMSMessage
			var omsMessageType = msg.GetEventType().String()
			switch msg.GetEventType() {
			// Metrics
			case events.Envelope_ValueMetric:
				if !o.nozzleConfig.ExcludeMetricEvents {
					omsMessage = messages.NewValueMetric(msg, o.nozzleInstanceName)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}
			case events.Envelope_CounterEvent:
				if !o.nozzleConfig.ExcludeMetricEvents {
					omsMessage = messages.NewCounterEvent(msg, o.nozzleInstanceName)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			case events.Envelope_ContainerMetric:
				if !o.nozzleConfig.ExcludeMetricEvents {
					omsMessage = messages.NewContainerMetric(msg, o.nozzleInstanceName)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			// Logs Errors
			case events.Envelope_LogMessage:
				if !o.nozzleConfig.ExcludeLogEvents {
					omsMessage = messages.NewLogMessage(msg, o.nozzleInstanceName)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			case events.Envelope_Error:
				if !o.nozzleConfig.ExcludeLogEvents {
					omsMessage = messages.NewError(msg, o.nozzleInstanceName)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			// HTTP Start/Stop
			case events.Envelope_HttpStartStop:
				if !o.nozzleConfig.ExcludeHttpEvents {
					omsMessage = messages.NewHTTPStartStop(msg, o.nozzleInstanceName)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}
			default:
				fmt.Println("Unexpected message type" + msg.GetEventType().String())
				continue
			}
			//Only use this when testing local.  Otherwise you're generate events to yourself
			//fmt.Printf("Current type:%s \ttotal recieved:%d\tsent:%d\terrors:%d\n", omsMessageType, msgReceivedCount, msgSentCount, msgSendErrorCount)
		default:
		}
	}
}

func (ct *CfClientTokenRefresh) RefreshAuthToken() (string, error) {
	return ct.cfClient.GetToken()
}

// OMSMessage is a marker inteface for JSON formatted messages published to OMS
type OMSMessage interface{}

// ConsoleDebugPrinter for debug logging
type ConsoleDebugPrinter struct{}

// Print debug logging
func (c ConsoleDebugPrinter) Print(title, dump string) {
	fmt.Printf("Consumer debug.  title:%s detail:%s", title, dump)
}
