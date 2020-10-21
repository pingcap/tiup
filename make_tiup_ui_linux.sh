#!/bin/sh

make embed_cluster_ui
GOOS=linux GOARCH=amd64 UI=1 make
