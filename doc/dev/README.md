# Developer documentation

## Building

You can build TiUp on any platform that supports Go.

Prerequisites:

* Go (minimum version: 1.13; [installation instructions](https://golang.org/doc/install))
* golint (`go get -u golang.org/x/lint/golint`)
* make

To build TiUp, run `make`.

## Running locally for development

For development, you don't want to use any global directories. You may also want to supply your own metadata. TiUp can be modified using the following environment variables:

* `TIUP_HOME` the profile directory, where TiUp stores its metadata. If not set, `~/.tiup` will be used.
* `TIUP_MIRRORS` set the location of TiUp's registry, can be a directory or URL. If not set, `https://tiup-mirrors.pingcap.com` will be used.

> **Note**
> TiUP need a certificate file (root.json) installed in `${TIUP_HOME}/bin` directory. If this is your first time getting TiUP, you can run `tiup mirror set <tiup-mirror>` to get it installed.

## Testing

TiUp has unit and integration tests; you can run both by running `make test`.

Unit tests are alongside the code they test, following the Go convention of using a `_test` suffix for test files. Integration tests are in the [tests](tests) directory.

## Architecture overview

Each TiUp command has its own executable, their source is in the [cmd](cmd) directory. The main TiUp executable is [root.go](cmd/root.go).

The core of TiUp is defined in the [pkg](pkg) directory.

[localdata](pkg/localdata) manages TiUp's metadata held on the user's computer.

[meta](pkg/meta) contains high-level functions for managing components.

[repository](pkg/repository) handles remote repositories.

The [set](pkg/set), [tui](pkg/tui), and [utils](pkg/utils) packages contain utility types and functions. The [version](pkg/version) package contains version data for TiUp and utility functions for handling that data.

The [mock](pkg/mock) package is a utility for testing.

Some key concepts:

* *Repository* a source of components and metadata concerning those components and TiUp in general.
* *Profile* the state of an installation of TiUp and the components it manages.
* *Component* a piece of software that can be managed by TiUp, e.g., TiDB or the playground.
* *Command* a top-level command run by TiUp, e.g., `update`, `list`.

### TiUp registry structure

* tiup-manifest.index: the manifest file in json format.
* Manifests for each component named tiup-component-$name.index, where %name is the name of the component.
* Component tarballs, one per component, per version; named $name-$version-$os-$arch.tar.gz, where $name and $version identify the component, and $os and $arch are a supported platform. Each tarball has a sha1 hash with the same name, but extension .sha1, instead of .tar.gz.

### Manifest formats

See `ComponentManifest` and `VersionManifest` data structures in [component.go](pkg/repository/component.go) and [version.go](pkg/repository/version.go).
