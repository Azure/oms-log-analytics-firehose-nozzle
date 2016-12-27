package main

import (
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/lizzha/pcf-oms-poc/caching"
	"github.com/lizzha/pcf-oms-poc/client"
	"github.com/lizzha/pcf-oms-poc/messages"
	"github.com/lizzha/pcf-oms-poc/omsnozzle"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	firehoseSubscriptionID = "oms-nozzle"
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
	apiAddress     = kingpin.Flag("api-addr", "Api URL").OverrideDefaultFromEnvar("API_ADDR").Required().String()
	dopplerAddress = kingpin.Flag("doppler-addr", "Traffic controller URL").OverrideDefaultFromEnvar("DOPPLER_ADDR").Required().String()
	cfUser         = kingpin.Flag("cf-user", "CF user name").OverrideDefaultFromEnvar("CF_USER").Required().String()
	cfPassword     = kingpin.Flag("cf-password", "Password of the CF user").OverrideDefaultFromEnvar("CF_PASSWORD").Required().String()
	omsWorkspace   = kingpin.Flag("oms-workspace", "OMS workspace ID").OverrideDefaultFromEnvar("OMS_WORKSPACE").Required().String()
	omsKey         = kingpin.Flag("oms-key", "OMS workspace key").OverrideDefaultFromEnvar("OMS_KEY").Required().String()
	omsPostTimeout = kingpin.Flag("oms-post-timeout", "HTTP timeout for posting events to OMS Log Analytics").Default("5s").OverrideDefaultFromEnvar("OMS_POST_TIMEOUT").Duration()
	omsTypePrefix  = kingpin.Flag("oms-type-prefix", "Prefix to identify the CF related messags in OMS Log Analytics").Default("CF_").OverrideDefaultFromEnvar("OMS_TYPE_PREFIX").String()
	omsBatchTime   = kingpin.Flag("oms-batch-time", "Interval to post an OMS batch").Default("5s").OverrideDefaultFromEnvar("OMS_BATCH_TIME").Duration()
	// comma separated list of types to exclude.  For now use metric,log,http and revisit later
	eventFilter       = kingpin.Flag("eventFilter", "Comma separated list of types to exclude").Default("").OverrideDefaultFromEnvar("EVENT_FILTER").String()
	skipSslValidation = kingpin.Flag("skip-ssl-validation", "Skip SSL validation").Default("false").OverrideDefaultFromEnvar("SKIP_SSL_VALIDATION").Bool()
	idleTimeout       = kingpin.Flag("idle-timeout", "Keep Alive duration for the firehose consumer").Default("25s").OverrideDefaultFromEnvar("IDLE_TIMEOUT").Duration()
	logLevel          = kingpin.Flag("log-level", "Log level: DEBUG, INFO, ERROR").Default("INFO").OverrideDefaultFromEnvar("LOG_LEVEL").String()

	excludeMetricEvents = false
	excludeLogEvents    = false
	excludeHttpEvents   = false
)

func main() {
	kingpin.Version(version)
	kingpin.Parse()

	logger := lager.NewLogger("oms-nozzle")
	level := lager.INFO
	switch strings.ToUpper(*logLevel) {
	case "DEBUG":
		level = lager.DEBUG
	case "ERROR":
		level = lager.ERROR
	}
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, level))

	// enable thread dump
	threadDumpChan := registerGoRoutineDumpSignalChannel()
	defer close(threadDumpChan)
	go dumpGoRoutine(threadDumpChan)

	if maxOMSPostTimeoutSeconds >= omsPostTimeout.Seconds() && minOMSPostTimeoutSeconds <= omsPostTimeout.Seconds() {
		logger.Info("config", lager.Data{"OMS_POST_TIMEOUT": (*omsPostTimeout).String()})
	} else {
		logger.Info("invalid OMS_POST_TIMEOUT value",
			lager.Data{"invalid value": (*omsPostTimeout).String()},
			lager.Data{"min seconds": minOMSPostTimeoutSeconds},
			lager.Data{"max seconds": maxOMSPostTimeoutSeconds},
			lager.Data{"default seconds": 5})
		*omsPostTimeout = time.Duration(5) * time.Second
	}
	logger.Info("config", lager.Data{"OMS_TYPE_PREFIX": *omsTypePrefix})
	logger.Info("config", lager.Data{"SKIP_SSL_VALIDATION": *skipSslValidation})
	logger.Info("config", lager.Data{"IDLE_TIMEOUT": (*idleTimeout).String()})
	logger.Info("config", lager.Data{"OMS_BATCH_TIME": (*omsBatchTime).String()})
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
		logger.Info("config", lager.Data{"EVENT_FILTER": *eventFilter},
			lager.Data{"excludeMetricEvents": excludeMetricEvents},
			lager.Data{"excludeLogEvents": excludeLogEvents},
			lager.Data{"excludeHTTPEvents": excludeHttpEvents})
	} else {
		logger.Info("config EVENT_FILTER is nil. all events will be published")
	}

	cfClientConfig := &cfclient.Config{
		ApiAddress:        *apiAddress,
		Username:          *cfUser,
		Password:          *cfPassword,
		SkipSslValidation: *skipSslValidation,
	}

	omsClient := client.NewOmsClient(*omsWorkspace, *omsKey, *omsPostTimeout, logger)

	nozzleConfig := &omsnozzle.NozzleConfig{
		TrafficControllerUrl:   *dopplerAddress,
		SkipSslValidation:      *skipSslValidation,
		IdleTimeout:            *idleTimeout,
		FirehoseSubscriptionId: firehoseSubscriptionID,
		OmsTypePrefix:          *omsTypePrefix,
		OmsBatchTime:           *omsBatchTime,
		ExcludeMetricEvents:    excludeMetricEvents,
		ExcludeLogEvents:       excludeLogEvents,
		ExcludeHttpEvents:      excludeHttpEvents,
	}

	nozzle := omsnozzle.NewOmsNozzle(logger, cfClientConfig, omsClient, nozzleConfig)

	messages.Caching = caching.NewCaching(cfClientConfig, logger)
	messages.Caching.Initialize()
	nozzle.Start()
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
