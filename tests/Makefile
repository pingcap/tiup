build_integration_test: build_tiup_playground_test build_tiup_cluster_test build_tiup_dm_test

build_tiup_playground_test: failpoint-enable
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-playground/bin/tiup-playground.test \
		github.com/pingcap/tiup/components/playground

build_tiup_cluster_test: failpoint-enable
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-cluster/bin/tiup-cluster.test \
		github.com/pingcap/tiup/components/cluster;

build_tiup_dm_test: failpoint-enable
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-dm/bin/tiup-dm.test \
		github.com/pingcap/tiup/components/dm;

build_tiup_test: failpoint-enable
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup/bin/tiup.test \
		github.com/pingcap/tiup;
