This folder contains all tests which relies on external service.

## Preprations

1. Set up test environment by running `./docker/up.sh`, see [README.sh](https://github.com/pingcap/tiup/components/cluster/tree/master/docker) for detail.
2. run `docker exec -it tiup-cluster-control bash` in another terminal to proceed.

## Running

> Note all this must be run inside the control node set up by docker in the last step.

To run all the test:

```
./tests/run.sh
```

To run a specify test:

```
# ./test/run.sh <TEST_NAME>
./tests/run.sh test_upgrade
```

where TEST_NAME is the file name of `tests/*.sh`

The flowing environment can control the testing version of cluster:

- `version` The version of cluster to be deploy and test.
- `old_version` The version of cluster to be deploy and upgrade to `version`

For example:

```
version=v4.0.0-rc old_version=v3.0.12 ./tests/run.sh
```

This will test using version `v4.0.0-rc` , and upgrade from `v3.0.12` to `v4.0.0-rc` when testing upgrade in `tests/test_upgrade.sh`

## Writing new tests

New integration tests can be written as shell script in `tests/TEST_NAME.sh`.
