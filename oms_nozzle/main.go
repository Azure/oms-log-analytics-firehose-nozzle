package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudfoundry-incubator/uaago"
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
	//TODO: query info endpoint for URLs
	dopplerAddress = os.Getenv("DOPPLER_ADDR")
	uaaAddress     = os.Getenv("UAA_ADDR")
	pcfUser        = os.Getenv("PCF_USER")
	pcfPassword    = os.Getenv("PCF_PASSWORD")
	omsWorkspace   = os.Getenv("OMS_WORKSPACE")
	omsKey         = os.Getenv("OMS_KEY")
	// TODO add parm
	sslSkipVerify = true
)

func main() {
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

	// counters
	var msgReceivedCount = 0
	var msgSentCount = 0
	var msgSendErrorCount = 0

	//TODO: should have a ping to make sure connection to OMS is good before subscribing to PCF logs
	client := client.New(omsWorkspace, omsKey)

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
	// Create firehose connection
	msgChan, errorChan := consumer.Firehose(firehoseSubscriptionID, authToken)
	go func() {
		for err := range errorChan {
			fmt.Fprintf(os.Stderr, "%v\n", err.Error())
		}
	}()
	// Firehose message processing loop

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
		var msgType = "PCF_" + msg.GetEventType().String() + "_v1"
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
		fmt.Printf("Current type:%s \ttotal recieved:%d\tsent:%d\terrors:%d\n", msgType, msgReceivedCount, msgSentCount, msgSendErrorCount)
	}
}
