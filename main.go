package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
	/*
		if len(authToken) == 0 {
			panic("CF_ACCESS_TOKEN env var not provided")
		}
	*/
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
	fmt.Printf("UAA %s\n", uaaAddress)
	fmt.Printf("PCF USER %s\n", pcfUser)
	fmt.Printf("PCF PWD %s\n", pcfPassword)

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
	consumer.SetDebugPrinter(ConsoleDebugPrinter{})
	var waitGroup sync.WaitGroup
	//TODO track completions
	waitGroup.Add(1)
	msgChan, errorChan := consumer.Firehose(firehoseSubscriptionID, authToken)
	go func() {
		for err := range errorChan {
			fmt.Fprintf(os.Stderr, "%v\n", err.Error())
		}
	}()
	// Firehose message processing loop
	go func() {
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
			fmt.Printf("Current type %s totals ... recieved %d sent %d errors %d\n", msgType, msgReceivedCount, msgSentCount, msgSendErrorCount)
		}
	}()

	go func() {

		l, err := net.Listen("tcp", ":8888")
		if err != nil {
			fmt.Println("Error listening:", err.Error())
			os.Exit(1)
		}
		// Close the listener when the application closes.
		defer l.Close()

		fmt.Println("Listening on localhost:8888")
		for {
			// Listen for an incoming connection.
			conn, err := l.Accept()
			if err != nil {
				fmt.Println("Error accepting: ", err.Error())
				os.Exit(1)
			}
			// Handle connections in a new goroutine.
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
					line, _, err := reader.ReadLine()
					if err != nil {
						if err == io.EOF {
							if len(line) > 0 {
								fmt.Printf("[tcp] Unfinished line: %#v", line)
							}
						} else {
							panic(err)
						}
						break
					}
					if len(line) > 0 { // skip empty lines
						var s = string(line)
						fmt.Printf("#################### Received line %s", s)
						parts := strings.Split(s, " ")
						ts, err := strconv.ParseInt(parts[2], 10, 64)
						if err != nil {
							panic(err)
						}
						fmt.Printf("TS as int %d", ts)
						metric := messages.HealthMonitorMetric{
						//Timestamp: time.Unix(ts, 0),
						//Key:       parts[0],
						//Value:     parts[1],
						}
						msgAsJSON, _ := json.Marshal(&metric)
						fmt.Printf("Metric as JSON %s\n", string(msgAsJSON))
						client.PostData(&msgAsJSON, "PCF_HealthMonitor")
					}
				}
			}(conn)
		}
	}()

	waitGroup.Wait()
}

type ConsoleDebugPrinter struct{}

func (c ConsoleDebugPrinter) Print(title, dump string) {
	println(title)
	println(dump)
}
