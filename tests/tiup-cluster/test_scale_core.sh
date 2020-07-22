#!/bin/bash

set -eu

source script/scale_core.sh

echo "test scaling of core components in cluster for verision v4.0.2"
scale_core v4.0.2 true
