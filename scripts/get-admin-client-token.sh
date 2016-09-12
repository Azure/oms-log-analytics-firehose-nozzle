
. ./ENV.sh
uaac --skip-ssl-validation target uaa.${ENDPOINT}
uaac token client get admin 
