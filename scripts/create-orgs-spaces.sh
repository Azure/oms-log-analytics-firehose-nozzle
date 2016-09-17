ENDPOINT=${ENDPOINT_IP}.cf.pcfazure.com
cf login --skip-ssl-validation -a https://api.${ENDPOINT}
cf create-org azcat
cf target -o azcat
cf create-space Development
cf target -o azcat -s Development

