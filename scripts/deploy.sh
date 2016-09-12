
. ./ENV.sh
cd ../
cf push -f oms_nozzle/manifest.yml --no-start
cf enable-diego oms_nozzle
cf start oms_nozzle
