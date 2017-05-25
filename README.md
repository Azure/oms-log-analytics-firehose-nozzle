# Summary
The oms-log-analytics-firehose-nozzle is a CF component which forwards metrics from the [Loggregator](https://docs.cloudfoundry.org/loggregator/architecture.html) Firehose to [OMS Log Analytics](https://docs.microsoft.com/en-us/azure/log-analytics/).
> Note this is preview and not for production use. The components of this tool may be changed when it is released at General Availability stage.

# Prerequisites
### 1. Deploy a CF or PCF environment in Azure

* [Deploy Cloud Foundry on Azure](https://github.com/cloudfoundry-incubator/bosh-azure-cpi-release/blob/master/docs/guidance.md)
* [Deploy Pivotal Cloud Foundry on Azure](https://docs.pivotal.io/pivotalcf/1-9/customizing/azure.html)

### 2. Install CLIs on your dev box

* [Install Cloud Foundry CLI](https://github.com/cloudfoundry/cli#downloads)
* [Install Cloud Foundry UAA Command Line Client](https://github.com/cloudfoundry/cf-uaac/blob/master/README.md)

### 3. Create an OMS Workspace in Azure

* [Get started with Log Analytics](https://docs.microsoft.com/en-us/azure/log-analytics/log-analytics-get-started)

# Deploy - Push the Nozzle as an App to Cloud Foundry
### 1. Utilize the CF CLI to authenticate with your CF instance
```
cf login -a https://api.${ENDPOINT} -u ${CF_USER} --skip-ssl-validation
```

### 2. Create a CF user and grant required privileges
The OMS Log Analytics nozzle requires a CF user who is authorized to access the loggregator firehose.
```
uaac target https://uaa.${ENDPOINT} --skip-ssl-validation
uaac token client get admin
cf create-user ${FIREHOSE_USER} ${FIREHOSE_USER_PASSWORD}
uaac member add cloud_controller.admin ${FIREHOSE_USER}
uaac member add doppler.firehose ${FIREHOSE_USER}
```

### 3. Download the latest code
```
git clone https://github.com/Azure/oms-log-analytics-firehose-nozzle.git
cd oms-log-analytics-firehose-nozzle
```

### 4. Set environment variables in [manifest.yml](./manifest.yml)
```
OMS_WORKSPACE             : OMS workspace ID
OMS_KEY                   : OMS key
OMS_POST_TIMEOUT          : HTTP post timeout for sending events to OMS Log Analytics
OMS_BATCH_TIME            : Interval for posting a batch to OMS
OMS_MAX_MSG_NUM_PER_BATCH : The max number of messages in an OMS batch
API_ADDR                  : The api URL of the CF environment
DOPPLER_ADDR              : Loggregator's traffic controller URL
FIREHOSE_USER             : CF user who has admin and firehose access
FIREHOSE_USER_PASSWORD    : Password of the CF user
EVENT_FILTER              : Event types to be filtered out. The format is a comma separated list, valid event types are METRIC,LOG,HTTP
SKIP_SSL_VALIDATION       : If true, allows insecure connections to the UAA and the Trafficcontroller
IDLE_TIMEOUT              : Keep Alive duration for the firehose consumer
LOG_LEVEL                 : Logging level of the nozzle, valid levels: DEBUG, INFO, ERROR
LOG_EVENT_COUNT           : If true, the total count of events that the nozzle has received and sent will be logged to OMS as CounterEvents
LOG_EVENT_COUNT_INTERVAL  : The time interval of logging event count to OMS
```


### 5. Push the app
```
cf push
```

# Additional logging
For the most part, the oms-log-analytics-firehose-nozzle forwards metrics from the loggregator firehose to OMS without too much processing. In a few cases the nozzle might push some additional metrics to OMS.

### 1. eventsReceived, eventsSent and eventsLost
If `LOG_EVENT_COUNT` is set to true, the nozzle will periodically send to OMS the count of received events, sent events and lost events, at intervals of `LOG_EVENT_COUNT_INTERVAL`.

The statistic count is sent as a CounterEvent, with CounterKey of one of **`nozzle.stats.eventsReceived`**, **`nozzle.stats.eventsSent`** and **`nozzle.stats.eventsLost`**. Each CounterEvent contains the value of delta count during the interval, and the total count from the beginning. **`eventsReceived`** counts all the events that the nozzle received from firehose, **`eventsSent`** counts all the events that the nozzle sent to OMS successfully, **`eventsLost`** counts all the events that the nozzle tried to send to OMS but failed after 4 attempts.

These CounterEvents themselves are not counted in the received, sent or lost count.

In normal cases, the total count of eventsSent plus eventsLost is less than total eventsReceived at the same time, as the nozzle buffers some messages and then post them in a batch to OMS. Operator can adjust the buffer size by changing the configurations `OMS_BATCH_TIME` and `OMS_MAX_MSG_NUM_PER_BATCH`.

### 2. slowConsumerAlert
When the nozzle receives slow consumer alert from loggregator in two ways:

1. the nozzle receives a WebSocket close error with error code `ClosePolicyViolation (1008)`

2. the nozzle receives a CounterEvent with the name `TruncatingBuffer.DroppedMessages`

the nozzle will send a slowConsumerAlert as a ValueMetric to OMS, with MetricKey **`nozzle.alert.slowConsumerAlert`** and value **`1`**.

This ValueMetric is not counted in the above statistic received, sent or lost count.

# Scaling guidance
### 1. Scaling Nozzle
Operators should run at least two instances of the nozzle to reduce message loss. The Firehose will evenly distribute events across all instances of the nozzle.

When the nozzle couldn't keep up with processing the logs from firehose, Loggregator alerts the nozzle and then the nozzle logs slowConsumerAlert message to OMS. Operator can [create Alert rule](#alert) for this slowConsumerAlert message in OMS Log Analytics, and when the alert is triggered, the operator should scale up the nozzle to minimize the loss of data.

We did some workload test against the nozzle and got a few data for operaters' reference:
* In our test, the size of each log and metric sent to OMS is around 550 bytes, suggest each nozzle instance should handle no more than **300000** such messages per minute. Under such workload, the CPU usage of each instance is around 40%, and the memory usage of each instance is around 80M.


### 2. Scaling Loggregator
Loggregator emits **LGR** log message to indicate problems with the logging process. When operaters see this message in OMS, they might need to [scale Loggregator](https://docs.cloudfoundry.org/running/managing-cf/logging-config.html#scaling).

# View in OMS Portal
### 1. Import OMS View
From the main OMS Overview page, go to **View Designer** -> **Import** -> **Browse**, select one of the [omsview](./omsview) files, e.g. [Cloud Foundry.omsview](./omsview/Cloud%20Foundry.omsview), and save the view. Now a **Tile** will be displayed on the main OMS Overview page. Click the **Tile**, it shows visualized metrics.

You can also customize these views or create new views through **View Designer**.

### 2. <a name="alert">Create Alert rules</a>
Operators can follow [this page](https://docs.microsoft.com/en-us/azure/log-analytics/log-analytics-alerts) to create Alert rules in OMS Portal.

**Sample Alert queries**
1. slowConsumerAlert
```
Type=CF_ValueMetric_CL Name_s=slowConsumerAlert
```

2. Loggregator emits **LGR** to indicate problems with the logging process, e.g. when log message output is too high
```
Type=CF_LogMessage_CL SourceType_s=LGR MessageType_s=ERR
```

3. When the number of lost events reaches a threshold (set the threshold value in OMS Portal)
```
Type=CF_CounterEvent_CL Job_s=nozzle Name_s=eventsLost
```

4. When the nozzle receives `TruncatingBuffer.DroppedMessages` CounterEvent
```
Type=CF_CounterEvent_CL Name_s="TruncatingBuffer.DroppedMessages"
```

# Test
You need [ginkgo](https://github.com/onsi/ginkgo) to run the test. Run the following command to execute test:
```
ginkgo -r
```

# Additional Reference
To collect syslogs and performance metrics of VMs in CloudFoundry deployment to OMS Log Analytics, please refer to [OMS Agent Bosh release](https://github.com/Azure/oms-agent-for-linux-boshrelease)
