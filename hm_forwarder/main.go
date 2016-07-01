package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dave-read/pcf-oms-poc/client"
	"github.com/dave-read/pcf-oms-poc/messages"
)

const (
	firehoseSubscriptionID = "oms"
)

// Required parameters
var (
	//TODO: query info endpoint for URLs
	listenPort   = os.Getenv("LISTEN_PORT")
	omsWorkspace = os.Getenv("OMS_WORKSPACE")
	omsKey       = os.Getenv("OMS_KEY")
	// TODO add parm
	sslSkipVerify = true
)

func main() {
	// check required parms
	if len(listenPort) == 0 {
		panic("LISTEN_PORT env var not provided")
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
	omsClient := client.New(omsWorkspace, omsKey)

	fmt.Printf("Starting with listenPort:%s\n", listenPort)
	// start listening
	l, err := net.Listen("tcp", ":"+listenPort)
	if err != nil {
		panic("Error listening:" + err.Error())
	}
	// Close the listener when the application closes.
	defer l.Close()

	fmt.Println("Listening on port:%s" + listenPort)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		fmt.Println("New Connection")
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go func(conn net.Conn) {
			defer conn.Close()
			reader := bufio.NewReader(conn)
			for {
				line, _, err := reader.ReadLine()
				msgReceivedCount++
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
					//fmt.Printf("#################### Received line %s", s)
					// graphite record is space separated: key val ts
					graphiteParts := strings.Split(s, " ")
					if len(graphiteParts) != 3 {
						fmt.Printf("Incorrect message len.  Expected 3 got:%d", len(graphiteParts))
					}
					// get timestamp
					ts, err := strconv.ParseInt(graphiteParts[2], 10, 64)
					if err != nil {
						fmt.Printf("Error parsing ts:%s", err)
					}
					// get value as float
					val, err := strconv.ParseFloat(graphiteParts[1], 64)
					// parse out the remaining parts from the graphite key
					metricParts := strings.Split(graphiteParts[0], ".")
					if len(metricParts) != 5 {
						fmt.Printf("Incorrect metric len.  Expected 5 got:%d", len(metricParts))
					}

					var metric = messages.ValueMetric{}
					metric.EventType = "PCF_ValueMetric_v1"
					metric.Deployment = metricParts[0]
					metric.Timestamp = time.Unix(ts, 0)
					metric.Job = metricParts[1]
					metric.Index = metricParts[2]
					metric.NozzleInstance = client.NozzleInstance
					var hash = md5.Sum([]byte(s))
					metric.MessageHash = hex.EncodeToString(hash[:])
					metric.Name = metricParts[4]
					metric.Value = val
					metric.Unit = "NA"
					metric.MetricKey = metric.Deployment + "." + metric.Job + "." + metric.Index + "." + metric.Name
					// key plus agent
					metric.SourceInstance = metric.MetricKey + "." + metricParts[3]
					msgAsJSON, _ := json.Marshal(&metric)
					//fmt.Printf("Metric as JSON %s\n", string(msgAsJSON))
					err = omsClient.PostData(&msgAsJSON, "PCF_ValueMetric_v1")
					msgSentCount++
					if err != nil {
						fmt.Printf("Error posting message to OMS %s", err)
						msgSendErrorCount++
					}
				}
			}
		}(conn)
	}
}
