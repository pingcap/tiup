#!/usr/bin/env bash

# check for io/ioutil
GREP_IOUTIL=`find . -name "*.go" | xargs grep -ns "io\/ioutil"`

if [[ ! -z $GREP_IOUTIL ]]; then
    echo $GREP_IOUTIL
    echo "use of \"io/ioutil\" is deprecated in Go 1.16, please update to use \"io\" and \"os\""
    exit 1
fi
