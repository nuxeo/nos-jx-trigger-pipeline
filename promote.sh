#!/bin/sh

echo "promoting the new version ${VERSION} to downstream repositories"

jx step create pr go --name github.com/nuxeo/jxlabs-nos-trigger-pipeline --version ${VERSION} --build "make build" --repo https://github.com/nuxeo/jxlabs-nos-jxl.git

jx step create pr regex --regex "^(?m)\s+name: tp\s+version: \"(.*)\"$"  --version ${VERSION} --files alpha/plugins.yml --repo https://github.com/nuxeo/jxlabs-nos-jxl.git
