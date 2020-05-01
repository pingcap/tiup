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
* Make it easy to try out TiDB, on a single machine and a cluster

## Use cases

* A new user with no experience with TiDB can try out TiDB with minimal friction.
* A developer of TiDB or TiKV can setup a local environment to test, benchmark, and debug their code.
* An oncall DBA or developer can quickly setup a cluster matching one where a bug was reported.
* A user reporting a bug can provide clear steps to reproduce based on a TiUp cluster configuration
* A TiDB DBA can manage a production cluster, including TODO


## Requirements

TODO Do we need to support different platforms? (E.g., mac/linux/ARM)

### Manage components

* Install and update components
* Uninstall/remove
* Provide information about installed components and how to use them to the user
* How must components be written? Restrictions on language, API? Sandboxing?
* Components can be run directly from TiUp, components may run other components if they are dependencies (see versioning).
  - E.g., running tidb can launch pd and tikv components

#### Versioning

* Components have an explicit version
* Components may depend on other components and specify versions of those dependencies with which they are compatible
* Components should use strict semver versioning
* Component publishers will provide all versions
* Version and semver compatibility is trusted, not verified
* Users can specify which versions of each component to install
* Component versions can be 'yanked', this means they are not listed and if the user tries to install them, they will be prompted for confirmation
* Component versions cannot be deleted from servers
* Component versions can be deleted locally (without removing the whole component)
* Update
  - patch revisions will be updated by default (i.e., the user must opt-in to keeping older versions)
  - major and minor revisions will be duplicated by default (i.e., old versions are kept)
  - when updating a component, by default, dependent components will be updated if necessary
* Users can have multiple versions of components installed and can specify which version will be launched by default
* Users can run any version of a component that they have installed
* By default, TiUp will not let components run incompatible components, this may be forced by the user
* Components may not specify which version of a dependent component they run.
* TiUP will select which version to run (whether run by the user or another component) as follows:
  - if the version is constrained by the user, that constraint has highest priority
  - if the version is constrained by a (transitive-)dependee component, that constraint has second priority
    - these constraints can be ignored if specified by the user
  - if an override is specified by the user that overrides any version which fits the above constraints, then it is run
  - otherwise, the most recent installed version of a component which satisfies the constraints is run
  - if the constraints cannot be satisfied by any installed version, TiUp will show an error
* The user may specify overrides of a component
  - any component may be overridden
  - an override may apply to any version, a specific versions, or a range of versions
  - an override may use another version or another component (of the same or different version) or a local binary
  - overrides apply to ways of running a component
  - when the user runs a component explicitly, the user can cause the override to be ignored

### Third-party components

* Users should be able to publish their own components to public and private servers
* Users can install third-party components and use them like the standard components
* TiUp should manage version compatibility between components
* Users should be able to search for components
* Users who have published a component should be able to hide it (but not delete it), or do so with specific versions
* Only the user who published a component should be able to publish new versions of that component
* Users should be able to publish multiple components and be able to list them

### Self-update

* Update TiUp from remote and local servers
* Uninstall TiUp

### Deployment and managing a deployed cluster

TODO

Better error messages when setting up and managing a cluster

### Sources

Also called mirrors and servers

* PingCAP will host a publicly available server
* Any third party may host a public or private server
  - Users must opt-in to using a source
  - Sources are strictly ordered for a given user (and can be changed by the user)
  - Sources cannot be specified per-component
  - All sources are checked in turn for a component
  - If updating a component causes it to be installed from a different source, the user should be prompted
* The user may create a local server, that server is created empty
* Components can be cloned from any source to a local source, either a specific version, all versions, or a range of versions
* TiUp can manage a server (possibly with a non-standard component); users should not have to manually edit any part of a server
* Hosts of servers are responsible for any authentication, TiUp will not keep servers private but will provide API hooks for hosts to do so

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
* No intent to support custom commands (c.f., components).
* We should avoid providing any build system functionality
