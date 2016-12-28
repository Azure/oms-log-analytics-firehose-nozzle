# Summary
The oms-log-analytics-nozzle is a CF component which forwards metrics from the [Loggregator](https://docs.cloudfoundry.org/loggregator/architecture.html) Firehose to [OMS Log Analytics](https://docs.microsoft.com/en-us/azure/log-analytics/).
> Note this is in developing phase and not for production use. It is based on private preview version of the OMS API and there may be changes before the API goes public.

# Prerequisites
### 1. Deploy a CF or PCF environment in Azure

* [Deploy Cloud Foundry on Azure](https://github.com/cloudfoundry-incubator/bosh-azure-cpi-release/blob/master/docs/guidance.md)
* [Deploy Pivotal Cloud Foundry on Azure](https://docs.pivotal.io/pivotalcf/1-8/customizing/azure.html)

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
git clone https://github.com/lizzha/pcf-oms-poc.git
cd pcf-oms-poc
```

### 4. Set environment variables in [manifest.yml](./manifest.yml)
```
OMS_WORKSPACE             : OMS workspace ID
OMS_KEY                   : OMS key
OMS_TYPE_PREFIX           : String helps to identify the CF related messags in OMS Log Analytics
OMS_POST_TIMEOUT_SEC      : HTTP post timeout seconds for sending events to OMS Log Analytics
OMS_BATCH_TIME            : Interval for posing a batch to OMS
API_ADDR                  : The api URL of the CF environment
DOPPLER_ADDR              : Loggregator's traffic controller URL
FIREHOSE_USER             : CF user who has admin and firehose access
FIREHOSE_USER_PASSWORD    : Password of the CF user
EVENT_FILTER              : If set, the specified types of events will be dropped
SKIP_SSL_VALIDATION       : If true, allows insecure connections to the UAA and the Trafficcontroller
IDLE_TIMEOUT_SEC          : Keep Alive duration for the firehose consumer
LOG_LEVEL                 : Valid log levels: DEBUG, INFO, ERROR
```
Operators should run at least two instances of the nozzle to reduce message loss. The Firehose will evenly distribute events across all instances of the nozzle. Scale to more instances if the nozzle cannot handle the workload.

### 5. Push the app
```
cf push
```