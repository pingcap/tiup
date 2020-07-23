#!/bin/bash

set -eu

source script/cmd_subtest.sh

echo "test cluster for verision v4.0.2 with CDC, via easy ssh"
cmd_subtest v4.0.2 true false

echo "test cluster for verision v4.0.2 with CDC, via native ssh"
cmd_subtest v4.0.2 true true
