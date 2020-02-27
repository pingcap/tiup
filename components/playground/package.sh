#!/usr/bin/env bash

GOOS=darwin go build
tar -czf playground-v0.0.1-darwin-amd64.tar.gz playground
shasum playground-v0.0.1-darwin-amd64.tar.gz | awk '{print $1}' > playground-v0.0.1-darwin-amd64.sha1

GOOS=linux go build
tar -czf playground-v0.0.1-linux-amd64.tar.gz playground
shasum playground-v0.0.1-linux-amd64.tar.gz | awk '{print $1}' > playground-v0.0.1-linux-amd64.sha1

