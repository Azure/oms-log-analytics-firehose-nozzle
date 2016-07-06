# pcf-oms-poc - graphite_listener
This is a standalone process to recieve graphite formatted messages from the Bosh heath monitor and collector.  

>> Note this is a proof of concept based on private preview version of the OMS API and there may be changes before the API goes public. It is provided for demonstration purposes and should not be considered production quality.
>> 
An alternative approach would have been to write handlers for collector and heath monitor that published directly to 
the OMS API.  However the long term strategy for PCF is to move to the firehose/nozzle model.   This approach assumes the message
from this listener will eventually be available via the loggregator firehose.


The required parameters are:

- LISTEN_PORT
- OMS_WORKSPACE
- OMS_KEY

To enable publishing of collector metrics via graphite, the following needs
to be added to the collector job spec in the elastic-runtime deployment manifest:

```yaml
 properties:
      collector:
        deployment_name: collector
        use_graphite: true
        graphite:
          address: 10.0.0.100
          port: 8888
```

To enable publishing of BOSH health monitor via graphite, the folowing
needs to be added to the BOSH deployment manifest

```yaml
hm:
...
graphite_enabled: true
      graphite:
           address: 10.0.0.100
           port: 8888
```
