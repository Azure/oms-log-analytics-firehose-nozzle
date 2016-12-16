# Summary
The oms-log-analytics-nozzle is a CF component which forwards metrics from the Loggregator Firehose to [OMS Log Analytics](https://docs.microsoft.com/en-us/azure/log-analytics/).
> Note this is in developing phase and not for production use. It is based on private preview version of the OMS API and there may be changes before the API goes public.

# Prerequisites
### 1. Deploy an CF/PCF environment in Azure

* [Deploy Cloud Foundry on Azure](https://github.com/cloudfoundry-incubator/bosh-azure-cpi-release/blob/master/docs/guidance.md)
* [Deploy Pivotal Cloud Foundry on Azure](https://docs.pivotal.io/pivotalcf/1-8/customizing/azure.html)
* (optional) [Configure Logging on PCF](http://docs.pivotal.io/pivotalcf/1-8/opsguide/logging-config-opsman.html)

### 2. Install CLIs on your dev box

* [Install Cloud Foundry CLI](https://github.com/cloudfoundry/cli#downloads)
* [Install Cloud Foundry UAA Command Line Client](https://github.com/cloudfoundry/cf-uaac/blob/master/README.md)

### 3. Create an OMS Workspace in Azure

* [Get started with Log Analytics](https://docs.microsoft.com/en-us/azure/log-analytics/log-analytics-get-started)

# Deploy
## Push the Nozzle as an App to Cloud Foundry
### 1. Create a UAA client and grant required privileges
The OMS Log Analytics nozzle requires a UUA user who is authorized to access the loggregator firehose. You can add a user using UAA CLI.
```
uaac target https://uaa.${ENDPOINT} --skip-ssl-validation
uaac token client get admin
uaac client add ${ID} --name ${UAA_CLIENT_NAME} --scope openid,oauth.approvals,doppler.firehose --authorized_grant_types authorization_code,client_credentials,refresh_token --authorities oauth.login,doppler.firehose --access_token_validity 31557600 --refresh_token_validity 31557600
```

### 2. Download the latest code
```
git clone https://github.com/lizzha/pcf-oms-poc.git
cd pcf-oms-poc
```

### 3. Utilize the CF CLI to authenticate with your CF instance
```
cf login -a https://api.${ENDPOINT} -u ${CF_USER} --skip-ssl-validation
```

### 4. Set environment variables in [manifest.yml](./manifest.yml)
```
OMS_WORKSPACE        : OMS workspace ID
OMS_KEY              : OMS key
OMS_TYPE_PREFIX      : String helps to identify the CF related messags in OMS Log Analytics
OMS_POST_TIMEOUT_SEC : HTTP post timeout seconds for sending events to OMS Log Analytics
OMS_BATCH_TIME       : Interval for posing a batch to OMS
API_ADDR             : The api URL of the CF environment
DOPPLER_ADDR         : Loggregator's traffic controller URL
UAA_ADDR             : UAA URL which the nozzle uses to get an authentication token for the firehose
UAA_CLIENT_NAME      : Client who has access to the firehose
UAA_CLIENT_SECRET    : Secret for the client
CF_USER              : CF user who has admin access
CF_PASSWORD          : Password of the CF user
EVENT_FILTER         : If set, the specified types of events will be dropped
SKIP_SSL_VALIDATION  : If true, allows insecure connections to the UAA and the Trafficcontroller
IDLE_TIMEOUT_SEC     : Keep Alive duration for the firehose consumer
```
Operators should run at least two instances of the nozzle to reduce message loss. The Firehose will evenly distribute events across all instances of the nozzle. Scale to more instances if the nozzle cannot handle the workload.

### 5. Push the app
```
cf push
```