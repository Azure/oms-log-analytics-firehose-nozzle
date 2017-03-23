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
Operators should run at least two instances of the nozzle to reduce message loss. The Firehose will evenly distribute events across all instances of the nozzle. Scale to more instances if the nozzle cannot handle the workload.

### 5. Push the app
```
cf push
```

# View in OMS Portal
The OMS view of Cloud Foundry will be added to the OMS Solutions Gallery soon. For the intermediate period, you could import the view manually.
### Import [omsview](./omsview)
From the main OMS Overview page, go to **View Designer** -> **Import** -> **Browse**, select the Cloud Foundry (Preview).omsview file and save the view. Now a **Tile** will be displayed on the main OMS Overview page. Click the **Tile**, it shows visualized metrics.

# Test
You need [ginkgo](https://github.com/onsi/ginkgo) to run the test. Run the following command to execute test:
```
ginkgo -r
```
