#!/bin/bash

pushd ..
  rm -f pcf-tile/resources/oms-log-analytics-firehose-nozzle.zip
  zip -r pcf-tile/resources/oms-log-analytics-firehose-nozzle.zip caching/ firehose/ client/ Godeps/ main.go messages/ omsnozzle/ vendor/ \
    -x vendor/github.com/onsi/\* vendor/golang.org/x/sys/unix/\* vendor/gopkg.in/yaml.v2/\* *_test.go
popd

if [ "$1" = "-major" ]; then
  tile build major
elif [ "$1" = "-minor" ]; then
  tile build minor
else
  tile build
fi
