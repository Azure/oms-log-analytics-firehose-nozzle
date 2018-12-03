package messages_test

import (
	"crypto/md5"
	hex "encoding/hex"
	"time"

	"github.com/Azure/oms-log-analytics-firehose-nozzle/caching"

	"github.com/Azure/oms-log-analytics-firehose-nozzle/messages"
	"github.com/Azure/oms-log-analytics-firehose-nozzle/mocks"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Messages", func() {
	var (
		instanceName    string
		environmentName string
		cache           *mocks.MockCaching
	)

	BeforeEach(func() {
		instanceName = "nozzleinstace"
		environmentName = "dev"
		cache = &mocks.MockCaching{
			InstanceName:    instanceName,
			EnvironmentName: environmentName,
		}
	})

	It("creates BaseMessage from Envelope", func() {
		eventType := events.Envelope_LogMessage
		timestamp := int64(2)
		origin := "originName"
		job := "jobName"
		index := "index"
		deployment := "deploymentName"
		ip := "192.168.0.1"
		tags := map[string]string{
			"tag1": "value1",
		}

		envelope := &events.Envelope{
			EventType:  &eventType,
			Origin:     &origin,
			Job:        &job,
			Index:      &index,
			Deployment: &deployment,
			Timestamp:  &timestamp,
			Ip:         &ip,
			Tags:       tags,
		}

		m := *messages.NewBaseMessage(envelope, cache)

		Expect(m.NozzleInstance).To(Equal(instanceName))
		Expect(m.EventType).To(Equal("LogMessage"))
		Expect(m.Origin).To(Equal(origin))
		Expect(m.SourceInstance).To(Equal("deploymentName.jobName.index"))
		Expect(m.EventTime).To(Equal(time.Unix(0, timestamp)))
		Expect(m.Index).To(Equal(index))
		Expect(m.Deployment).To(Equal(deployment))
		Expect(m.IP).To(Equal(ip))
		Expect(m.Tags).To(Equal("map[tag1:value1]"))
		Expect(m.Job).To(Equal(job))
		hash := md5.Sum([]byte(envelope.String()))
		Expect(m.MessageHash).To(Equal(hex.EncodeToString(hash[:])))
		Expect(m.Environment).To(Equal(environmentName))
	})

	It("creates LogMessage from Envelope", func() {
		eventType := events.Envelope_LogMessage
		messageType := events.LogMessage_OUT
		appId := "DAC65549-1667-4C84-9B62-5BCF1A08EDD5"
		posixStart := int64(1)
		sourceType := "sourceTypeName"
		sourceInstance := "1"
		logMsg := "This is a test log message"
		appName := "appName"
		appOrg := "appOrg"
		appOrgID := "ASDF1234"
		appSpace := "appSpace"
		appSpaceID := "	QWER5678"

		logMessage := events.LogMessage{
			Message:        []byte(logMsg),
			AppId:          &appId,
			Timestamp:      &posixStart,
			SourceType:     &sourceType,
			MessageType:    &messageType,
			SourceInstance: &sourceInstance,
		}

		envelope := &events.Envelope{
			EventType:  &eventType,
			LogMessage: &logMessage,
		}

		cache.MockGetAppInfo = func(appGuid string) caching.AppInfo {
			Expect(appGuid).To(Equal(appId))
			return caching.AppInfo{
				Name:      appName,
				Org:       appOrg,
				Space:     appSpace,
				OrgID:     appOrgID,
				SpaceID:   appSpaceID,
				Monitored: true,
			}
		}

		m := *messages.NewLogMessage(envelope, cache)

		Expect(m).NotTo(Equal(nil))
		Expect(m.ApplicationName).To(Equal(appName))
		Expect(m.ApplicationOrg).To(Equal(appOrg))
		Expect(m.ApplicationOrgID).To(Equal(appOrgID))
		Expect(m.ApplicationSpace).To(Equal(appSpace))
		Expect(m.ApplicationSpaceID).To(Equal(appSpaceID))
		Expect(m.Message).To(Equal(logMsg))
		Expect(m.MessageType).To(Equal("OUT"))
		Expect(m.Timestamp).To(Equal(posixStart))
		Expect(m.AppID).To(Equal(appId))
		Expect(m.SourceType).To(Equal(sourceType))
		Expect(m.SourceInstance).To(Equal(sourceInstance))
		Expect(m.SourceTypeKey).To(Equal("sourceTypeName-OUT"))
		Expect(m.BaseMessage.SourceInstance).To(Equal(""))
		Expect(m.BaseMessage.EventType).To(Equal("LogMessage"))
		Expect(m.BaseMessage.Environment).To(Equal(environmentName))

		cache.MockGetAppInfo = func(appGuid string) caching.AppInfo {
			Expect(appGuid).To(Equal(appId))
			return caching.AppInfo{
				Name:      appName,
				Org:       appOrg,
				Space:     appSpace,
				OrgID:     appOrgID,
				SpaceID:   appSpaceID,
				Monitored: false,
			}
		}

		m2 := messages.NewLogMessage(envelope, cache)

		Expect(m2).To(BeNil())
	})

	It("creates HttpStartStop from Envelope", func() {
		eventType := events.Envelope_HttpStartStop
		peerType := events.PeerType_Client
		low := uint64(16592826980310057976)
		high := uint64(791876040070101076)
		appId := events.UUID{
			Low:  &low,
			High: &high,
		}
		requestId := appId
		formattedUUID := "f803e6dc-3990-45e6-5478-851a104ffd0a"

		startTimestamp := int64(10)
		stopTimestamp := int64(20)
		method := events.Method_GET
		uri := "http://api.sys.mslovelinux"
		remoteAddress := "10.0.0.23:47426"
		userAgent := "Go-http-client/1.1"
		statusCode := int32(200)
		contentLength := int64(82)
		instanceIndex := int32(0)
		instanceId := "4489c965-c445-4fd3-481b-9fd35e12222f"
		forwarded := []string{"10.0.0.1", "10.0.0.2"}
		appName := "applicationName"
		appOrg := "applicationOrg"
		appOrgID := "4103e7f8-c44b-43a6-b46f-0df52295113f"
		appSpace := "applicationSpace"
		appSpaceID := "60c1d57c-8a71-4c38-9ea1-28bdabd59014"

		httpStartStop := events.HttpStartStop{
			StartTimestamp: &startTimestamp,
			StopTimestamp:  &stopTimestamp,
			RequestId:      &requestId,
			PeerType:       &peerType,
			Method:         &method,
			Uri:            &uri,
			RemoteAddress:  &remoteAddress,
			UserAgent:      &userAgent,
			StatusCode:     &statusCode,
			ContentLength:  &contentLength,
			ApplicationId:  &appId,
			InstanceIndex:  &instanceIndex,
			InstanceId:     &instanceId,
			Forwarded:      forwarded,
		}

		envelope := &events.Envelope{
			EventType:     &eventType,
			HttpStartStop: &httpStartStop,
		}

		cache.MockGetAppInfo = func(appGuid string) caching.AppInfo {
			Expect(appGuid).To(Equal(formattedUUID))
			return caching.AppInfo{
				Name:      appName,
				Org:       appOrg,
				Space:     appSpace,
				OrgID:     appOrgID,
				SpaceID:   appSpaceID,
				Monitored: true,
			}
		}

		m := *messages.NewHTTPStartStop(envelope, cache)

		Expect(m).NotTo(Equal(nil))
		Expect(m.ApplicationID).To(Equal("f803e6dc-3990-45e6-5478-851a104ffd0a"))
		Expect(m.ApplicationName).To(Equal(appName))
		Expect(m.ApplicationOrg).To(Equal(appOrg))
		Expect(m.ApplicationOrgID).To(Equal(appOrgID))
		Expect(m.ApplicationSpace).To(Equal(appSpace))
		Expect(m.ApplicationSpaceID).To(Equal(appSpaceID))
		Expect(m.StartTimestamp).To(Equal(startTimestamp))
		Expect(m.StopTimestamp).To(Equal(stopTimestamp))
		Expect(m.RequestID).To(Equal(formattedUUID))
		Expect(m.PeerType).To(Equal(peerType.String()))
		Expect(m.Method).To(Equal(method.String()))
		Expect(m.URI).To(Equal(uri))
		Expect(m.RemoteAddress).To(Equal(remoteAddress))
		Expect(m.UserAgent).To(Equal(userAgent))
		Expect(m.StatusCode).To(Equal(statusCode))
		Expect(m.ContentLength).To(Equal(contentLength))
		Expect(m.ApplicationID).To(Equal(formattedUUID))
		Expect(m.InstanceIndex).To(Equal(instanceIndex))
		Expect(m.InstanceID).To(Equal(instanceId))
		Expect(m.Forwarded).To(Equal("10.0.0.1,10.0.0.2"))
		Expect(m.BaseMessage.EventType).To(Equal(eventType.String()))
		Expect(m.BaseMessage.Environment).To(Equal(environmentName))

		cache.MockGetAppInfo = func(appGuid string) caching.AppInfo {
			Expect(appGuid).To(Equal("f803e6dc-3990-45e6-5478-851a104ffd0a"))
			return caching.AppInfo{
				Name:      appName,
				Org:       appOrg,
				Space:     appSpace,
				OrgID:     appOrgID,
				SpaceID:   appSpaceID,
				Monitored: false,
			}
		}

		m2 := messages.NewHTTPStartStop(envelope, cache)

		Expect(m2).To(BeNil())
	})

	It("creates Error from Envelope", func() {
		eventType := events.Envelope_Error
		msg := "Error message"
		code := int32(1)
		source := "app"

		err := events.Error{
			Message: &msg,
			Code:    &code,
			Source:  &source,
		}

		envelope := &events.Envelope{
			EventType: &eventType,
			Error:     &err,
		}

		m := *messages.NewError(envelope, cache)

		Expect(m.Message).To(Equal(msg))
		Expect(m.Code).To(Equal(code))
		Expect(m.Source).To(Equal(source))
		Expect(m.BaseMessage.EventType).To(Equal(eventType.String()))
		Expect(m.BaseMessage.Environment).To(Equal(environmentName))
	})

	It("creates ContainerMetric from Envelope", func() {
		eventType := events.Envelope_ContainerMetric
		appId := "98B26B8D-6642-4070-A3B1-0EB3B43FF9AD"
		instanceIndex := int32(4)
		cpuPercentage := 0.0396875881867705
		memoryBytes := uint64(497360896)
		diskBytes := uint64(158560256)
		memoryBytesQuota := uint64(1073741824)
		diskBytesQuota := uint64(1073741800)
		appName := "cf"
		appOrg := "system"
		appOrgID := "123456-1234-1234-12345678"
		appSpace := "oms_nozzle"
		appSpaceID := "ABCDEF-ABCD-ABCD-ABCDEFGH"

		cache.MockGetAppInfo = func(appGuid string) caching.AppInfo {
			Expect(appGuid).To(Equal(appId))
			return caching.AppInfo{
				Name:      appName,
				Org:       appOrg,
				Space:     appSpace,
				OrgID:     appOrgID,
				SpaceID:   appSpaceID,
				Monitored: true,
			}
		}

		metric := events.ContainerMetric{
			ApplicationId:    &appId,
			InstanceIndex:    &instanceIndex,
			CpuPercentage:    &cpuPercentage,
			MemoryBytes:      &memoryBytes,
			DiskBytes:        &diskBytes,
			MemoryBytesQuota: &memoryBytesQuota,
			DiskBytesQuota:   &diskBytesQuota,
		}

		envelope := &events.Envelope{
			EventType:       &eventType,
			ContainerMetric: &metric,
		}

		m := *messages.NewContainerMetric(envelope, cache)

		Expect(m).NotTo(Equal(nil))
		Expect(m.ApplicationID).To(Equal(appId))
		Expect(m.ApplicationName).To(Equal(appName))
		Expect(m.ApplicationOrg).To(Equal(appOrg))
		Expect(m.ApplicationOrgID).To(Equal(appOrgID))
		Expect(m.ApplicationSpace).To(Equal(appSpace))
		Expect(m.ApplicationSpaceID).To(Equal(appSpaceID))
		Expect(m.InstanceIndex).To(Equal(instanceIndex))
		Expect(m.CPUPercentage).To(Equal(cpuPercentage))
		Expect(m.MemoryBytes).To(Equal(memoryBytes))
		Expect(m.DiskBytes).To(Equal(diskBytes))
		Expect(m.MemoryBytesQuota).To(Equal(memoryBytesQuota))
		Expect(m.DiskBytesQuota).To(Equal(diskBytesQuota))
		Expect(m.BaseMessage.EventType).To(Equal(eventType.String()))
		Expect(m.BaseMessage.Environment).To(Equal(environmentName))

		cache.MockGetAppInfo = func(appGuid string) caching.AppInfo {
			Expect(appGuid).To(Equal(appId))
			return caching.AppInfo{
				Name:      appName,
				Org:       appOrg,
				Space:     appSpace,
				OrgID:     appOrgID,
				SpaceID:   appSpaceID,
				Monitored: false,
			}
		}

		m2 := messages.NewContainerMetric(envelope, cache)

		Expect(m2).To(BeNil())
	})

	It("creates CounterEvent from Envelope", func() {
		eventType := events.Envelope_CounterEvent
		name := "dropsondeUnmarshaller.receivedEnvelopes"
		delta := uint64(2)
		total := uint64(369199)
		job := "job"
		origin := "MetronAgent"

		event := events.CounterEvent{
			Name:  &name,
			Delta: &delta,
			Total: &total,
		}

		envelope := &events.Envelope{
			EventType:    &eventType,
			CounterEvent: &event,
			Job:          &job,
			Origin:       &origin,
		}

		m := *messages.NewCounterEvent(envelope, cache)

		Expect(m.Name).To(Equal(name))
		Expect(m.Delta).To(Equal(delta))
		Expect(m.Total).To(Equal(total))
		Expect(m.CounterKey).To(Equal("job.MetronAgent.dropsondeUnmarshaller.receivedEnvelopes"))
		Expect(m.BaseMessage.EventType).To(Equal(eventType.String()))
		Expect(m.BaseMessage.Environment).To(Equal(environmentName))
	})

	It("creates ValueMetric from Envelope", func() {
		eventType := events.Envelope_ValueMetric
		name := "memoryStats.numBytesAllocatedHeap"
		value := float64(54632)
		unit := "count"
		origin := "rep"
		job := "job"

		metric := events.ValueMetric{
			Name:  &name,
			Value: &value,
			Unit:  &unit,
		}

		envelope := &events.Envelope{
			EventType:   &eventType,
			ValueMetric: &metric,
			Job:         &job,
			Origin:      &origin,
		}

		m := *messages.NewValueMetric(envelope, cache)

		Expect(m.Name).To(Equal(name))
		Expect(m.Value).To(Equal(value))
		Expect(m.Unit).To(Equal(unit))
		Expect(m.MetricKey).To(Equal("job.rep.memoryStats.numBytesAllocatedHeap"))
		Expect(m.BaseMessage.EventType).To(Equal(eventType.String()))
		Expect(m.BaseMessage.Environment).To(Equal(environmentName))
	})
})
