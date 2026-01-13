.PHONY: build_integration_test
build_integration_test: build_tiup_playground_test build_tiup_cluster_test build_tiup_dm_test
	@# Target: run the playground, cluster, and dm integration tests.

.PHONY: build_tiup_playground_test
build_tiup_playground_test: failpoint-enable
	@# Target: run the tiup-playground tests
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-playground/bin/tiup-playground.test \
		github.com/pingcap/tiup/components/playground
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-playground-ng/bin/tiup-playground-ng.test \
		github.com/pingcap/tiup/components/playground-ng
	@$(FAILPOINT_DISABLE)

.PHONY: build_tiup_cluster_test
build_tiup_cluster_test: failpoint-enable
	@# Target: run the tiup-cluster tests
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-cluster/bin/tiup-cluster.test \
		github.com/pingcap/tiup/components/cluster;
	@$(FAILPOINT_DISABLE)

.PHONY: build_tiup_dm_test
build_tiup_dm_test: failpoint-enable
	@# Target: run the tiup-dm tests
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-dm/bin/tiup-dm.test \
		github.com/pingcap/tiup/components/dm;
	@$(FAILPOINT_DISABLE)

.PHONY: build_tiup_test
build_tiup_test: failpoint-enable
	@# Target: run the tiup tests
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup/bin/tiup.test \
		github.com/pingcap/tiup;
	@$(FAILPOINT_DISABLE)
