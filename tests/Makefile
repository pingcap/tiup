.PHONY: server

build_integration_test: build_tiup_playground_test build_tiup_cluster_test build_tiup_dms_test

build_tiup_playground_test: failpoint-enable
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-playground/bin/tiup-playground.test \
		github.com/pingcap/tiup/components/playground

build_tiup_cluster_test:
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-cluster/bin/tiup-cluster.test \
		github.com/pingcap/tiup/components/cluster;

build_tiup_dms_test:
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-dms/bin/tiup-dms.test \
		github.com/pingcap/tiup/components/dms;

build_tiup_test:
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup/bin/tiup.test \
		github.com/pingcap/tiup;