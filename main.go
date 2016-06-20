package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudfoundry/noaa/consumer"
	events "github.com/cloudfoundry/sonde-go/events"
	"github.com/dave-read/pcf-oms-poc/client"
	"github.com/dave-read/pcf-oms-poc/messages"
)

const (
	firehoseSubscriptionID = "oms"
)

// Required parameters
var (
	dopplerAddress = os.Getenv("DOPPLER_ADDR")
	authToken      = os.Getenv("CF_ACCESS_TOKEN")
	omsWorkspace   = os.Getenv("OMS_WORKSPACE")
	omsKey         = os.Getenv("OMS_KEY")
)

func main() {
	// check required parms
	if len(dopplerAddress) == 0 {
		panic("DOPPLER_ADDR env var not provided")
	}
	if len(authToken) == 0 {
		panic("CF_ACCESS_TOKEN env var not provided")
	}
	if len(omsWorkspace) == 0 {
		panic("OMS_WORKSPACE env var not provided")
	}
	if len(omsKey) == 0 {
		panic("OMS_KEY env var not provided")
	}

	// counters
	var msgReceivedCount = 0
	var msgSentCount = 0
	var msgSendErrorCount = 0

	//TODO: should have a ping to make sure connection to OMS is good before subscribing to PCF logs
	client := client.New(omsWorkspace, omsKey)

	// connect to PCF
	consumer := consumer.New(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)
	consumer.SetDebugPrinter(ConsoleDebugPrinter{})
	msgChan, errorChan := consumer.Firehose(firehoseSubscriptionID, authToken)
	go func() {
		for err := range errorChan {
			fmt.Fprintf(os.Stderr, "%v\n", err.Error())
		}
	}()
	// Firehost message processing loop
	for msg := range msgChan {

		msgReceivedCount++
		var omsMessage interface{}
		switch msg.GetEventType() {
		// Metrics
		case events.Envelope_ValueMetric:
			omsMessage = messages.NewValueMetric(msg)
		case events.Envelope_CounterEvent:
			omsMessage = messages.NewCounterEvent(msg)
		case events.Envelope_ContainerMetric:
			omsMessage = messages.NewContainerMetric(msg)
		// Logs Errors
		case events.Envelope_LogMessage:
			omsMessage = messages.NewLogMessage(msg)
		case events.Envelope_Error:
			omsMessage = messages.NewError(msg)
		// HTTP Start/Stop
		case events.Envelope_HttpStart:
			omsMessage = messages.NewHTTPStart(msg)
		case events.Envelope_HttpStartStop:
			omsMessage = messages.NewHTTPStartStop(msg)
		case events.Envelope_HttpStop:
			omsMessage = messages.NewHTTPStop(msg)
		// Unknown
		default:
			fmt.Println("Unexpected message type" + msg.GetEventType().String())
			continue
		}

		// OMS message as JSON
		msgAsJSON, err := json.Marshal(&omsMessage)
		// Version the events during testing
		var msgType = msg.GetEventType().String() + "v1"
		if err != nil {
			fmt.Printf("Error marshalling message type %s to JSON. error: %s", msgType, err)
		} else {
			err = client.PostData(&msgAsJSON, msgType)
			if err != nil {
				msgSendErrorCount++
				fmt.Printf("Error posting message type %s to OMS. error: %s", msgType, err)
			} else {
				msgSentCount++
			}
		}
		fmt.Printf("Current type %s totals ... recieved %d sent %d errors %d\n", msgType, msgReceivedCount, msgSentCount, msgSendErrorCount)
	}
}

type ConsoleDebugPrinter struct{}

func (c ConsoleDebugPrinter) Print(title, dump string) {
	println(title)
	println(dump)
}
