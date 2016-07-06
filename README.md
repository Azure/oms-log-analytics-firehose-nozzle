# pcf-oms-poc

This is a proof of concept based on private preview version of the OMS API and Dashboard designer. 

>> It is provided for demonstration purposes and should not be considered production quality.

There are two projects in the repo.

- Sample nozzle - is in the oms_nozzle folder.  It connects to the doppler endpoint to consume firehose messages and forwards each message to OMS Log Analytics via the private preview API.
- Graphite listener - is in the graphite_listener folder.  This is a temporary endpoint to capture health and metrics from the BOSH health monitor and the Collector job.


