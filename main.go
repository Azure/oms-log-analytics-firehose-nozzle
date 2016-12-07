package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry-incubator/uaago"
	"github.com/cloudfoundry/noaa/consumer"
	events "github.com/cloudfoundry/sonde-go/events"
	"github.com/dave-read/pcf-oms-poc/client"
	"github.com/dave-read/pcf-oms-poc/messages"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	firehoseSubscriptionID = "oms-poc"
	// lower limit for override
	minOMSPostTimeoutSeconds = 1
	// upper limit for override
	maxOMSPostTimeoutSeconds = 60
	// filter metrics
	metricEventType = "METRIC"
	// filter stdout/stderr events
	logEventType = "LOG"
	// filter http start/stop events
	httpEventType = "HTTP"

	version = "0.1.0"
)

// Required parameters
var (
	//TODO: query info endpoint for URLs
	apiAddress        = kingpin.Flag("api-addr", "Api URL").OverrideDefaultFromEnvar("API_ADDR").String()
	dopplerAddress    = kingpin.Flag("doppler-addr", "Traffic controller URL").OverrideDefaultFromEnvar("DOPPLER_ADDR").String()
	uaaAddress        = kingpin.Flag("uaa-addr", "UAA URL").OverrideDefaultFromEnvar("UAA_ADDR").String()
	uaaClientName     = kingpin.Flag("uaa-client-name", "UAA client name").OverrideDefaultFromEnvar("UAA_CLIENT_NAME").String()
	uaaClientSecret   = kingpin.Flag("uaa-client-secret", "UAA client secret").OverrideDefaultFromEnvar("UAA_CLIENT_SECRET").String()
	cfUser            = kingpin.Flag("cf-user", "CF user name").OverrideDefaultFromEnvar("CF_USER").String()
	cfPassword        = kingpin.Flag("cf-password", "Password of the CF user").OverrideDefaultFromEnvar("CF_PASSWORD").String()
	omsWorkspace      = kingpin.Flag("oms-workspace", "OMS workspace ID").OverrideDefaultFromEnvar("OMS_WORKSPACE").String()
	omsKey            = kingpin.Flag("oms-key", "OMS workspace key").OverrideDefaultFromEnvar("OMS_KEY").String()
	omsPostTimeout    = kingpin.Flag("oms-post-timeout", "HTTP timeout for posting events to OMS Log Analytics").Default("5s").OverrideDefaultFromEnvar("OMS_POST_TIMEOUT").Duration()
	omsTypePrefix     = kingpin.Flag("oms-type-prefix", "Prefix to identify the CF related messags in OMS Log Analytics").Default("CF_").OverrideDefaultFromEnvar("OMS_TYPE_PREFIX").String()
	// comma separated list of types to exclude.  For now use metric,log,http and revisit later
	eventFilter       = kingpin.Flag("eventFilter", "Comma separated list of types to exclude").Default("").OverrideDefaultFromEnvar("EVENT_FILTER").String()
	skipSslValidation = kingpin.Flag("skip-ssl-validation", "Skip SSL validation").Default("false").OverrideDefaultFromEnvar("SKIP_SSL_VALIDATION").Bool()
	idleTimeout       = kingpin.Flag("idle-timeout", "Keep Alive duration for the firehose consumer").Default("25s").OverrideDefaultFromEnvar("IDLE_TIMEOUT").Duration()

	excludeMetricEvents = false
	excludeLogEvents    = false
	excludeHttpEvents   = false
)

func main() {
	kingpin.Version(version)
	kingpin.Parse()

	// setup for termination signal from CF
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGTERM, syscall.SIGINT)

	// enable thread dump
	threadDumpChan := registerGoRoutineDumpSignalChannel()
	defer close(threadDumpChan)
	go dumpGoRoutine(threadDumpChan)

	// check required parms
	switch {
	case len(*apiAddress) == 0:
		panic("API_ADDR env var not provided")
	case len(*dopplerAddress) == 0:
		panic("DOPPLER_ADDR env var not provided")
	case len(*uaaAddress) == 0:
		panic("UAA_ADDR env var not provided")
	case len(*uaaClientName) == 0:
		panic("UAA_CLIENT_NAME env var not provided")
	case len(*uaaClientSecret) == 0:
		panic("UAA_CLIENT_SECRET env var not provided")
	case len(*omsWorkspace) == 0:
		panic("OMS_WORKSPACE env var not provided")
	case len(*omsKey) == 0:
		panic("OMS_KEY env var not provided")
	case len(*cfUser) == 0:
		panic("CF_USER env var not provided")
	case len(*cfPassword) == 0:
		panic("CF_PASSWORD env var not provided")
	}

	if maxOMSPostTimeoutSeconds >= omsPostTimeout.Seconds() && minOMSPostTimeoutSeconds <= omsPostTimeout.Seconds() {
		fmt.Printf("OMS_POST_TIMEOUT:%s\n", *omsPostTimeout)
		client.HTTPPostTimeout = *omsPostTimeout
	} else {
		fmt.Printf("Ignoring OMS_POST_TIMEOUT value %s. Min value is %d, max value is %d\n. Set to default 5s.", *omsPostTimeout, minOMSPostTimeoutSeconds, maxOMSPostTimeoutSeconds)
	}

	fmt.Printf("OMS_TYPE_PREFIX:%s\n", *omsTypePrefix)
	fmt.Printf("SKIP_SSL_VALIDATION:%v\n", *skipSslValidation)
	if len(*eventFilter) > 0 {
		*eventFilter = strings.ToUpper(*eventFilter)
		// by default we don't filter any events
		if strings.Contains(*eventFilter, metricEventType) {
			excludeMetricEvents = true
		}
		if strings.Contains(*eventFilter, logEventType) {
			excludeLogEvents = true
		}
		if strings.Contains(*eventFilter, httpEventType) {
			excludeHttpEvents = true
		}
		fmt.Printf("EVENT_FILTER is:%s filter values are excludeMetricEvents:%t excludeLogEvents:%t excludeHTTPEvents:%t\n", eventFilter, excludeMetricEvents, excludeLogEvents, excludeHttpEvents)
	} else {
		fmt.Print("No value for EVENT_FILTER evironment variable.  All events will be published\n")
	}

	// counters
	var msgReceivedCount = 0
	var msgSentCount = 0
	var msgSendErrorCount = 0

	messages.CfClientConfig = &cfclient.Config{
		ApiAddress:        *apiAddress,
		Username:          *cfUser,
		Password:          *cfPassword,
		SkipSslValidation: *skipSslValidation,
	}

	cfClient, err := cfclient.NewClient(messages.CfClientConfig)
	if err != nil {
		panic("Error creating cfclient:" + err.Error())
	}

	apps, err := cfClient.ListApps()
	if err != nil {
		panic("Error getting app list:" + err.Error())
	}

	for _, app := range apps {
		fmt.Printf("Adding to AppName cache. App guid:%s name:%s\n", app.Guid, app.Name)
		messages.AppNamesByGUID[app.Guid] = app.Name
	}
	fmt.Printf("Size of AppNamesByGUID:%d\n", len(messages.AppNamesByGUID))

	//TODO: should have a ping to make sure connection to OMS is good before subscribing to PCF logs
	client := client.New(*omsWorkspace, *omsKey)

	// connect to CF
	fmt.Printf("Starting with uaaAddress:%s dopplerAddress:%s\n", *uaaAddress, *dopplerAddress)

	uaaClient, err := uaago.NewClient(*uaaAddress)
	if err != nil {
		panic("Error creating uaa client:" + err.Error())
	}

	var authToken string
	authToken, err = uaaClient.GetAuthToken(*uaaClientName, *uaaClientSecret, *skipSslValidation)
	if err != nil {
		panic("Error getting Auth Token" + err.Error())
	}
	consumer := consumer.New(*dopplerAddress, &tls.Config{InsecureSkipVerify: *skipSslValidation}, nil)

	// See https://github.com/cloudfoundry-community/firehose-to-syslog/issues/82
	fmt.Printf("IDLE_TIMEOUT:%v\n", *idleTimeout)
	consumer.SetIdleTimeout(*idleTimeout)

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
	// TOD: make batching time configurable
	ticker := time.NewTicker(time.Duration(5) * time.Second)
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
						k = *omsTypePrefix + k
						err = client.PostData(&msgAsJSON, k)
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
		case msg := <-msgChan:
			// process message
			msgReceivedCount++
			var omsMessage OMSMessage
			var omsMessageType = msg.GetEventType().String()
			switch msg.GetEventType() {
			// Metrics
			case events.Envelope_ValueMetric:
				if !excludeMetricEvents {
					omsMessage = messages.NewValueMetric(msg)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}
			case events.Envelope_CounterEvent:
				if !excludeMetricEvents {
					omsMessage = messages.NewCounterEvent(msg)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			case events.Envelope_ContainerMetric:
				if !excludeMetricEvents {
					omsMessage = messages.NewContainerMetric(msg)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			// Logs Errors
			case events.Envelope_LogMessage:
				if !excludeLogEvents {
					omsMessage = messages.NewLogMessage(msg)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			case events.Envelope_Error:
				if !excludeLogEvents {
					omsMessage = messages.NewError(msg)
					pendingEvents[omsMessageType] = append(pendingEvents[omsMessageType], omsMessage)
				}

			// HTTP Start/Stop
			case events.Envelope_HttpStartStop:
				if !excludeHttpEvents {
					omsMessage = messages.NewHTTPStartStop(msg)
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

func registerGoRoutineDumpSignalChannel() chan os.Signal {
	threadDumpChan := make(chan os.Signal, 1)
	signal.Notify(threadDumpChan, syscall.SIGUSR1)

	return threadDumpChan
}

func dumpGoRoutine(dumpChan chan os.Signal) {
	for range dumpChan {
		goRoutineProfiles := pprof.Lookup("goroutine")
		if goRoutineProfiles != nil {
			goRoutineProfiles.WriteTo(os.Stdout, 2)
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
