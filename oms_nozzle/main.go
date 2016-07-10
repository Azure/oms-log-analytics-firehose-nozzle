package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/uaago"
	"github.com/cloudfoundry/noaa/consumer"
	events "github.com/cloudfoundry/sonde-go/events"
	"github.com/dave-read/pcf-oms-poc/client"
	"github.com/dave-read/pcf-oms-poc/messages"
)

const (
	firehoseSubscriptionID = "oms-poc"
	// lower limit for override
	minOMSPostTimeoutSeconds = 1
	// upper limit for override
	maxOMSPostTimeoutSeconds = 60
)

// Required parameters
var (
	//TODO: query info endpoint for URLs
	dopplerAddress = os.Getenv("DOPPLER_ADDR")
	uaaAddress     = os.Getenv("UAA_ADDR")
	pcfUser        = os.Getenv("PCF_USER")
	pcfPassword    = os.Getenv("PCF_PASSWORD")
	omsWorkspace   = os.Getenv("OMS_WORKSPACE")
	omsKey         = os.Getenv("OMS_KEY")
	omsPostTimeout = os.Getenv("OMS_POST_TIMEOUT_SEC")
	// TODO add parm
	sslSkipVerify = true
)

func main() {
	// setup for termination signal from CF
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGTERM, syscall.SIGINT)
	// check required parms
	if len(dopplerAddress) == 0 {
		panic("DOPPLER_ADDR env var not provided")
	}
	if len(uaaAddress) == 0 {
		panic("UAA_ADDR env var not provided")
	}
	if len(omsWorkspace) == 0 {
		panic("OMS_WORKSPACE env var not provided")
	}
	if len(omsKey) == 0 {
		panic("OMS_KEY env var not provided")
	}
	if len(pcfUser) == 0 {
		panic("PCF_USER env var not provided")
	}
	if len(pcfPassword) == 0 {
		panic("PCF_PASSWORD env var not provided")
	}
	if len(omsPostTimeout) != 0 {
		i, err := strconv.Atoi(omsPostTimeout)
		if err != nil {
			fmt.Printf("Ignoring OMS_POST_TIMEOUT_SEC value %s. Error converting to int. Error:%s\n", omsPostTimeout, err)
		} else {
			if i > maxOMSPostTimeoutSeconds || i < minOMSPostTimeoutSeconds {
				fmt.Printf("Ignoring OMS_POST_TIMEOUT_SEC value %d. Min value is %d, max value is %d\n", i, minOMSPostTimeoutSeconds, maxOMSPostTimeoutSeconds)
			} else {
				client.HTTPPostTimeout = time.Second * time.Duration(i)
				fmt.Printf("OMS_POST_TIMEOUT_SEC overriden.  New value:%s\n", client.HTTPPostTimeout)
			}
		}
	}

	// counters
	var msgReceivedCount = 0
	var msgSentCount = 0
	var msgSendErrorCount = 0

	//TODO: should have a ping to make sure connection to OMS is good before subscribing to PCF logs
	client := client.New(omsWorkspace, omsKey)
	if client == nil {

	}
	// connect to PCF
	fmt.Printf("Starting with uaaAddress:%s dopplerAddress:%s\n", uaaAddress, dopplerAddress)

	uaaClient, err := uaago.NewClient(uaaAddress)
	if err != nil {
		panic("Error creating uaa client:" + err.Error())
	}

	var authToken string
	authToken, err = uaaClient.GetAuthToken(pcfUser, pcfPassword, true)
	if err != nil {
		panic("Error getting Auth Token" + err.Error())
	}
	consumer := consumer.New(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)

	// TODO: Verify and make configurable.  See https://github.com/cloudfoundry-community/firehose-to-syslog/issues/82
	consumer.SetIdleTimeout(25 * time.Second)
	//consumer.SetDebugPrinter(ConsoleDebugPrinter{})
	// Create firehose connection
	msgChan, errorChan := consumer.Firehose(firehoseSubscriptionID, authToken)
	// async error channel
	go func() {
		var errorChannelCount = 0
		for err := range errorChan {
			errorChannelCount++
			fmt.Fprintf(os.Stderr, "Firehose channel error.  Date:%v errorCount:%d error:%v\n", time.Now(), errorChannelCount, err.Error())
		}
	}()
	pendingEvents := make(map[string][]interface{})
	// Firehose message processing loop
	ticker := time.NewTicker(time.Duration(10) * time.Second)
	for {
		// loop over message and signal channel
		select {
		case s := <-signalChannel:
			fmt.Printf("Signal caught:%s Exiting\n", s.String())
			err := consumer.Close()
			if err != nil {
				fmt.Printf("Error closing consumer:%v\n", err)
			}
			os.Exit(1)
		case <-ticker.C:
			//fmt.Printf("Timer fired ... processing events.  Total events:%d\n", msgReceivedCount)
			for k, v := range pendingEvents {
				// OMS message as JSON
				msgAsJSON, err := json.Marshal(&v)
				if err != nil {
					fmt.Printf("Error marshalling message type %s to JSON. error: %s", k, err)
				} else {
					//fmt.Printf("   EventType:%s\tEventCount:%d\tJSONSize:%d\n", k, len(v), len(msgAsJSON))
					requestStartTime := time.Now()
					err = client.PostData(&msgAsJSON, "PCF_"+k+"_v1")
					elapsedTime := time.Since(requestStartTime)
					if err != nil {
						msgSendErrorCount++
						fmt.Printf("Error posting message type %s to OMS. error: %s elapseTime:%s msgSize:%d\n", k, err, elapsedTime.String(), len(msgAsJSON))
					} else {
						msgSentCount++
					}
				}
			}
			pendingEvents = make(map[string][]interface{})
		case msg := <-msgChan:
			// process message
			msgReceivedCount++
			var omsMessage OMSMessage
			var omsMessageType = msg.GetEventType().String()
			switch msg.GetEventType() {
			// Metrics
			case events.Envelope_ValueMetric:
				omsMessage = messages.NewValueMetric(msg)
				pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)

			case events.Envelope_CounterEvent:
				omsMessage = messages.NewCounterEvent(msg)
				pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)

			case events.Envelope_ContainerMetric:
				omsMessage = messages.NewContainerMetric(msg)
				pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)

			// Logs Errors
			case events.Envelope_LogMessage:
				omsMessage = messages.NewLogMessage(msg)
				pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)

			case events.Envelope_Error:
				omsMessage = messages.NewError(msg)
				pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)

			// HTTP Start/Stop
			case events.Envelope_HttpStart:
				omsMessage = messages.NewHTTPStart(msg)
				pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)

			case events.Envelope_HttpStartStop:
				omsMessage = messages.NewHTTPStartStop(msg)
				pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)

			case events.Envelope_HttpStop:
				omsMessage = messages.NewHTTPStop(msg)
				pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
			// Unknown
			default:
				fmt.Println("Unexpected message type" + msg.GetEventType().String())
				continue
			}
			//Only use this when testing local.  Otherwise you're generate events to yourself
			//fmt.Printf("Current type:%s \ttotal recieved:%d\tsent:%d\terrors:%d\n", msgType, msgReceivedCount, msgSentCount, msgSendErrorCount)
			//fmt.Printf(string(msgAsJSON) + "\n")
		}
	}
}

type OMSMessage interface{}

// ConsoleDebugPrinter for debug logging
type ConsoleDebugPrinter struct{}

// Print debug logging
func (c ConsoleDebugPrinter) Print(title, dump string) {
	fmt.Printf("Consumer debug.  title:%s detail:%s", title, dump)
}
