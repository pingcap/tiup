# Requirements and goals

Includes the overlapping projects of TiUp, TiUp Cluster, and TiUp Playground.

Priorities:

* P0 must be shipped in initial release (blockers)
* P1 should be shipped in initial release but don't block
* P2 should ship in an early release
* P3 would be nice to have

## Vision and high-level goals

* Help expand the TiDB ecosystem by making it easy to share, install, and deploy components (P2)
* Manage and deploy TiDB and TiKV clusters (P0)
* Open source project, with a community of users and contributors (P1)
* Help TiDB and TiKV developers run and test their code in a production-like environment (P2)
* Make TiDB an easy to use database (P1)
* Be the standard way for TiDB users to manage their clusters (P2)
* Make it easy to try out TiDB, on a single machine and a cluster (P2)

## Use cases

* A new user with no experience with TiDB can try out TiDB with minimal friction (P2)
* A developer of TiDB or TiKV can setup a local environment to test, benchmark, and debug their code (P2)
* An oncall DBA or developer can quickly setup a cluster matching one where a bug was reported (P1)
* A user reporting a bug can provide clear steps to reproduce based on a TiUp cluster configuration (P1)
* A TiDB DBA can manage a production cluster, including TODO (P0,1,2,3)


## Requirements

### Manage components

* Install and update components (P0)
* Uninstall/remove (P1)
* Provide information about installed components and how to use them to the user (P0)
* How must components be written? Restrictions on language, API? Sandboxing? (P2)
* Components can be run directly from TiUp, components may run other components if they are dependencies (see versioning) (P0)
  - E.g., running tidb can launch pd and tikv components
  - Initially, this does not need explicit support in TiUp

#### Versioning

* Components have an explicit version (P0)
* Components should use strict semver versioning (P0)
* Version and semver compatibility is trusted, not verified (P0)
* Users can specify which versions of each component to install (P1)
* Component versions can be 'yanked', this means they are not listed and if the user tries to install them, they will be prompted for confirmation (P2)
* Component versions cannot be deleted from servers (P0)
* Component versions can be deleted locally (without removing the whole component) (P1)
* Update  (P1)
  - patch revisions will be updated by default (i.e., the user must opt-in to keeping older versions)
  - major and minor revisions will be duplicated by default (i.e., old versions are kept)
* Users can have multiple versions of components installed and can specify which version will be launched by default (P2)
* Users can run any version of a component that they have installed (P2)
* The user may specify overrides of a component (P3)
  - any component may be overridden
  - an override may apply to any version, a specific versions, or a range of versions
  - an override may use another version or another component (of the same or different version) or a local binary
  - overrides apply to ways of running a component
  - when the user runs a component explicitly, the user can cause the override to be ignored

#### Dependencies (P3)

* Components can run other components, these are dependent components
* Dependent components may or may not be managed by TiUp, but managed is recommended
  - If a dependent component is managed by TiUp it is specified explicitly and declaratively in the component's manifest.
  - If a dependent component is not managed by TiUp, then it is not specified explicitly and the component must handle download, version management, platform handling, running, etc.

Managed dependent components:

* Components may depend on other components and specify versions of those dependencies with which they are compatible
* By default, TiUp will not let components run incompatible components, this may be forced by the user
* TiUP will select which version to run (whether run by the user or another component) as follows:
  - if the version is constrained by the user, that constraint has highest priority
  - if the version is constrained by a (transitive-)dependee component, that constraint has second priority
    - these constraints can be ignored if specified by the user
  - if an override is specified by the user that overrides any version which fits the above constraints, then it is run
  - otherwise, the most recent installed version of a component which satisfies the constraints is run
  - if the constraints cannot be satisfied by any installed version, TiUp will show an error
* When updating a component, by default, dependent components will be updated if necessary

### Platforms (P2)

* Components may support multiple platforms (i.e., combinations of processor and operating system)
* TiUp will install and run the correct platform of a component depending on where the component is run (which is not necessarily where TiUp is run)
* A single version of TiUP shall support targets on different platforms (P3)

### Third-party components (P2)

* Users should be able to publish their own components to public and private servers
* Users can install third-party components and use them like the standard components
* TiUp should manage version compatibility between components
* Users should be able to search for components
* Users who have published a component should be able to hide it (but not delete it), or do so with specific versions
* Only the user who published a component should be able to publish new versions of that component
* Users should be able to publish multiple components and be able to list them

### Self-update (P1)

* Update TiUp from remote and local servers
* Uninstall TiUp

### Deployment and managing a deployed cluster

TODO

Better error messages when setting up and managing a cluster

### Sources

Also called mirrors and servers

* PingCAP will host a publicly available server (P0)
* Any third party may host a public or private server (P2)
  - Users must opt-in to using a source
  - Sources are strictly ordered for a given user (and can be changed by the user)
  - Sources cannot be specified per-component
  - All sources are checked in turn for a component
  - If updating a component causes it to be installed from a different source, the user should be prompted
* The user may create a local server, that server is created empty (P1)
* Components can be cloned from any source to a local source, either a specific version, all versions, or a range of versions (P3)
* TiUp can manage a server (possibly with a non-standard component); users should not have to manually edit any part of a server (P3)
* Hosts of servers are responsible for any authentication, TiUp will not keep servers private but will provide API hooks for hosts to do so (P0)

### Supported products (P0)

* TiDB
* TiKV (with and without TiDB)
* TiFlash
* Binlog
* Blackbox probe
* Prometheus and Grafana

TiUp should be extensible with user-provided components (P2).

### Non-functional requirements

In priority order

* Reliability - could bugs in cluster management cause severe issues? That might be important. I think delaying update is not critical as long as it is not for too long, perhaps <1hr?
* Security - we must be able to securely download files and manage clusters since the DB is likely to be security-critical for users. Download binaries is likely to be over the internet. Cluster management might include cross-data center communication on a public network. No need to protect data at rest.
* Ease of contribution and development velocity - should be prioritised to help OSS community building. 
* Performance - performance is not critical, but the CLI should be responsive (low latency). Since most tasks are expected to be long-running, there is not much pressure for high performance.
* Memory/disk usage - unlikely to be a problem. We should provide mechanisms for automatic and manual recovery of disk space, since in the long-term it might become an issue.
* Scale - should support our largest users, i.e., 1000s of nodes in a cluster and nodes in different data centers.
* Backwards compatibility - TODO how much do we have to preserve workflows and scripts between versions?

### Non-goals

Things we intend not to do, either for now or ever.

* Designed to be primarily used by humans, secondarily by scripts. Not intended as an API.
* No GUI.
* Project is specific to TiDB and TiKV, not a generic database or software management tool (but if components that do such things work, then that is ok).
* No intent to support custom commands (c.f., components).
* We should avoid providing any build system functionality
