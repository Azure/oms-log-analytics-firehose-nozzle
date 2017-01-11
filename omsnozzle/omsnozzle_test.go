package omsnozzle_test

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/lizzha/pcf-oms-poc/mocks"
	"github.com/lizzha/pcf-oms-poc/omsnozzle"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Omsnozzle", func() {
	var (
		nozzle         *omsnozzle.OmsNozzle
		nozzleConfig   *omsnozzle.NozzleConfig
		firehoseClient *mocks.MockFirehoseClient
		omsClient      *mocks.MockOmsClient
		cachingClient  *mocks.MockCaching
		logger         *mocks.MockLogger
	)

	BeforeEach(func() {
		firehoseClient = mocks.NewMockFirehoseClient()
		omsClient = mocks.NewMockOmsClient()
		cachingClient = &mocks.MockCaching{}
		logger = mocks.NewMockLogger()
		nozzleConfig = &omsnozzle.NozzleConfig{
			OmsTypePrefix:       "CF_",
			OmsBatchTime:        time.Duration(5) * time.Millisecond,
			ExcludeMetricEvents: false,
			ExcludeLogEvents:    false,
			ExcludeHttpEvents:   false,
		}

		nozzle = omsnozzle.NewOmsNozzle(logger, firehoseClient, omsClient, nozzleConfig, cachingClient)
		go nozzle.Start()
	})

	It("routes a LogMessage", func() {
		eventType := events.Envelope_LogMessage
		messageType := events.LogMessage_OUT

		logMessage := events.LogMessage{
			MessageType: &messageType,
		}

		envelope := &events.Envelope{
			EventType:  &eventType,
			LogMessage: &logMessage,
		}

		firehoseClient.MessageChan <- envelope

		msgJson := "[{\"EventType\":\"LogMessage\",\"Deployment\":\"\",\"EventTime\":\"0001-01-01T00:00:00Z\",\"Job\":\"\",\"Index\":\"\",\"IP\":\"\",\"Tags\":\"\",\"NozzleInstance\":\"\",\"MessageHash\":\"d396528c711f0053685aac71a95a9637\",\"Origin\":\"\",\"Message\":\"\",\"MessageType\":\"OUT\",\"Timestamp\":0,\"AppID\":\"\",\"ApplicationName\":\"\",\"SourceType\":\"\",\"SourceInstance\":\"\",\"SourceTypeKey\":\"-OUT\"}]"
		Eventually(func() string {
			return omsClient.GetPostedMessages()["CF_LogMessage"]
		}).Should(Equal(msgJson))
	})

	It("routes a HttpStartStop", func() {
		eventType := events.Envelope_HttpStartStop
		peerType := events.PeerType_Client

		httpStartStop := events.HttpStartStop{
			PeerType: &peerType,
		}

		envelope := &events.Envelope{
			EventType:     &eventType,
			HttpStartStop: &httpStartStop,
		}

		firehoseClient.MessageChan <- envelope

		msgJson := "[{\"EventType\":\"HttpStartStop\",\"Deployment\":\"\",\"EventTime\":\"0001-01-01T00:00:00Z\",\"Job\":\"\",\"Index\":\"\",\"IP\":\"\",\"Tags\":\"\",\"NozzleInstance\":\"\",\"MessageHash\":\"b7338b4f4c40613986590b7e4ec508a9\",\"SourceInstance\":\"MISSING\",\"Origin\":\"\",\"StartTimestamp\":0,\"StopTimestamp\":0,\"RequestID\":\"\",\"PeerType\":\"Client\",\"Method\":\"GET\",\"URI\":\"\",\"RemoteAddress\":\"\",\"UserAgent\":\"\",\"StatusCode\":0,\"ContentLength\":0,\"ApplicationID\":\"\",\"ApplicationName\":\"\",\"InstanceIndex\":0,\"InstanceID\":\"\",\"Forwarded\":\"\"}]"
		Eventually(func() string {
			return omsClient.GetPostedMessages()["CF_HttpStartStop"]
		}).Should(Equal(msgJson))
	})

	It("routes an Error", func() {
		eventType := events.Envelope_Error

		envelope := &events.Envelope{
			EventType: &eventType,
			Error:     &events.Error{},
		}

		firehoseClient.MessageChan <- envelope

		msgJson := "[{\"EventType\":\"Error\",\"Deployment\":\"\",\"EventTime\":\"0001-01-01T00:00:00Z\",\"Job\":\"\",\"Index\":\"\",\"IP\":\"\",\"Tags\":\"\",\"NozzleInstance\":\"\",\"MessageHash\":\"1aeb0d10b3411300c1ad275c668c581a\",\"SourceInstance\":\"MISSING\",\"Origin\":\"\",\"Source\":\"\",\"Code\":0,\"Message\":\"\"}]"
		Eventually(func() string {
			return omsClient.GetPostedMessages()["CF_Error"]
		}).Should(Equal(msgJson))
	})

	It("routes a ContainerMetric", func() {
		eventType := events.Envelope_ContainerMetric

		envelope := &events.Envelope{
			EventType:       &eventType,
			ContainerMetric: &events.ContainerMetric{},
		}

		firehoseClient.MessageChan <- envelope

		msgJson := "[{\"EventType\":\"ContainerMetric\",\"Deployment\":\"\",\"EventTime\":\"0001-01-01T00:00:00Z\",\"Job\":\"\",\"Index\":\"\",\"IP\":\"\",\"Tags\":\"\",\"NozzleInstance\":\"\",\"MessageHash\":\"7a2415d07f1304f829a5b1fc1390aa1e\",\"SourceInstance\":\"MISSING\",\"Origin\":\"\",\"ApplicationID\":\"\",\"ApplicationName\":\"\",\"InstanceIndex\":0}]"
		Eventually(func() string {
			return omsClient.GetPostedMessages()["CF_ContainerMetric"]
		}).Should(Equal(msgJson))
	})

	It("routes a CounterEvent", func() {
		eventType := events.Envelope_CounterEvent

		envelope := &events.Envelope{
			EventType:    &eventType,
			CounterEvent: &events.CounterEvent{},
		}

		firehoseClient.MessageChan <- envelope

		msgJson := "[{\"EventType\":\"CounterEvent\",\"Deployment\":\"\",\"EventTime\":\"0001-01-01T00:00:00Z\",\"Job\":\"\",\"Index\":\"\",\"IP\":\"\",\"Tags\":\"\",\"NozzleInstance\":\"\",\"MessageHash\":\"5e28ff227b28d842fd7e08c0a764cf53\",\"SourceInstance\":\"MISSING\",\"Origin\":\"\",\"Name\":\"\",\"Delta\":0,\"Total\":0,\"CounterKey\":\"..\"}]"
		Eventually(func() string {
			return omsClient.GetPostedMessages()["CF_CounterEvent"]
		}).Should(Equal(msgJson))
	})

	It("routes a ValueMetric", func() {
		eventType := events.Envelope_ValueMetric

		envelope := &events.Envelope{
			EventType:   &eventType,
			ValueMetric: &events.ValueMetric{},
		}

		firehoseClient.MessageChan <- envelope

		msgJson := "[{\"EventType\":\"ValueMetric\",\"Deployment\":\"\",\"EventTime\":\"0001-01-01T00:00:00Z\",\"Job\":\"\",\"Index\":\"\",\"IP\":\"\",\"Tags\":\"\",\"NozzleInstance\":\"\",\"MessageHash\":\"cc4acf2df16fb78148a274ddc04800ca\",\"SourceInstance\":\"MISSING\",\"Origin\":\"\",\"Name\":\"\",\"Value\":0,\"Unit\":\"\",\"MetricKey\":\"..\"}]"
		Eventually(func() string {
			return omsClient.GetPostedMessages()["CF_ValueMetric"]
		}).Should(Equal(msgJson))
	})

	It("logs for unrecognized events", func() {
		eventType := events.Envelope_EventType(10)
		envelope := &events.Envelope{
			EventType: &eventType,
		}

		firehoseClient.MessageChan <- envelope

		Eventually(func() []mocks.Log {
			return logger.GetLogs(lager.INFO)
		}).Should(Equal([]mocks.Log{mocks.Log{
			Action: "uncategorized message",
			Data:   []lager.Data{{"message": "eventType:10 "}},
		}}))
	})
})
