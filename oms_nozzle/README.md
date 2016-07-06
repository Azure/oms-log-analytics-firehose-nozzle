# OMS Log Analytics Nozzle
This is a sample nozzle that uses the OMS API for log/metric ingestion.

>> Note this is based on private preview version of the API and there may be changes before the API goes public. It is provided for demonstration purposes and should not be considered production quality


As with other nozzles you'll need to crate a UAA user and grant required privileges.

The required parameters are:

- doppler URL
- UAA URL
- PCF user
- PCR password
- workspace id
- workspace key

You can get the workspace id and key from the OMS portal.

There is sample manifest provided to allow deploying the nozzle as a CF app.  The project uses godep to manage vendor dependencies, so cf push needs to be run from the top of the repo with reference to the manifest:

```
cf push -f oms_nozzle\manifest.yml
```
