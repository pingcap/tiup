# Requirements and goals

Includes the overlapping projects of TiUp, TiUp Cluster, and TiUp Playground.

We should distinguish between goals and requirements for the long term vs an initial release.

## Vision and high-level goals

* Help expand the TiDB ecosystem by making it easy to share, install, and deploy components
* Manage and deploy TiDB and TiKV clusters
* Open source project, with a community of users and contributors
* Help TiDB and TiKV developers run and test their code in a production-like environment
* Make TiDB an easy to use database
* Be the standard way for TiDB users to manage their clusters
* Make it easy to try out TiDB, both on a single machine and a cluster

## Requirements

### Manage components

* Install and update from a remote or local server, or from user-provided binaries
* Uninstall/remove
* Provide information about installed components and how to use them to the user

### Self-update

* Update TiUp from remote server
* Uninstall

### Deployment and managing a deployed cluster

TODO

### Supported products

* TiDB
* TiKV (with and without TiDB)
* TiFlash
* Binlog
* Blackbox probe
* Prometheus and Grafana

TiUp should be extensible with user-provided components.

### Interfaces

This section is getting beyond requirements and should be removed, but might be useful for deriving requirements

* SQL client
* playground
* cluster

### Non-functional requirements

* Security - we must be able to securely download files and manage clusters since the DB is likely to be security-critical for users. Download binaries is likely to be over the internet. Cluster management might include cross-data center communication on a public network. No need to protect data at rest.
* Reliability - could bugs in cluster management cause severe issues? That might be important. I think delaying update is not critical as long as it is not for too long, perhaps <1hr?
* Performance - performance is not critical, but the CLI should be responsive (low latency). Since most tasks are expected to be long-running, there is not much pressure for high performance.
* Memory/disk usage - unlikely to be a problem. We should provide mechanisms for automatic and manual recovery of disk space, since in the long-term it might become an issue.
* Ease of contribution and development velocity - should be prioritised to help OSS community building. 
* Scale - should support our largest users, i.e., 1000s of nodes in a cluster and nodes in different data centers.
* Backwards compatibility - TODO how much do we have to preserve workflows and scripts between versions?

### Non-goals

Things we intend not to do, either for now or ever.

* Designed to be primarily used by humans, secondarily by scripts. Not intended as an API.
* No GUI.
* Project is specific to TiDB and TiKV, not a generic database or software management tool.
* No intent to support custom commands.
