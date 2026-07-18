#!/bin/bash

set -eu

version=${version-v6.2.0}

source script/external_labels.sh

echo "test prometheus external_labels for version $version"
external_labels "$version"
