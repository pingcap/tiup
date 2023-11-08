# Developer documentation

## Building

You can build TiUP on any platform that supports Go.

Prerequisites:

* Go (minimum version: 1.21; [installation instructions](https://golang.org/doc/install))
* golint (`go get -u golang.org/x/lint/golint`)
* make

To build TiUP, run `make`.

## Running locally for development

For development, you don't want to use any global directories. You may also want to supply your own metadata. TiUP can be modified using the following environment variables:

* `TIUP_VERBOSE` this enables verbose logging when set to `enabled`.
* `TIUP_HOME` the profile directory, where TiUP stores its metadata. If not set, `~/.tiup` will be used.
* `TIUP_MIRRORS` set the location of TiUP's registry, can be a directory or URL. If not set, `https://tiup-mirrors.pingcap.com` will be used.

> **Note**
> TiUP need a certificate file (root.json) installed in `${TIUP_HOME}/bin` directory. If this is your first time getting TiUP, you can run `curl https://tiup-mirrors.pingcap.com/root.json -o ${TIUP_HOME}/bin/root.json` to get it installed.

## Testing

TiUP has unit and integration tests; you can run unit tests by running `make test`.

Unit tests are alongside the code they test, following the Go convention of using a `_test` suffix for test files. Integration tests are in the [tests](../../tests) directory.

## Architecture overview

Each TiUP command has its own executable, their source is in the [cmd](../../cmd) directory. The main TiUP executable is [root.go](../../cmd/root.go).

The core of TiUP is defined in the [pkg](../../pkg) directory.

[localdata](../../pkg/localdata) manages TiUP's metadata held on the user's computer.

[meta](../../pkg/meta) contains high-level functions for managing components.

[repository](../../pkg/repository) handles remote repositories.

The [set](../../pkg/set), [tui](../../pkg/tui), and [utils](../../pkg/utils) packages contain utility types and functions. The [version](../../pkg/version) package contains version data for TiUP and utility functions for handling that data.

The [mock](../../pkg/utils/mock) package is a utility for testing.

[embed](../../embed) contains static files used by builtin components (mainly `cluster` as of now), the template files are in [embed/templates](../../embed/templates) directory.

Some key concepts:

* *Repository* a source of components and metadata concerning those components and TiUP in general.
* *Profile* the state of an installation of TiUP and the components it manages.
* *Component* a piece of software that can be managed by TiUP, e.g., TiDB or the playground.
* *Command* a top-level command run by TiUP, e.g., `update`, `list`.

### TiUP registry structure

* tiup-manifest.index: the manifest file in json format.
* Manifests for each component named tiup-component-$name.index, where %name is the name of the component.
* Component tarballs, one per component, per version; named $name-$version-$os-$arch.tar.gz, where $name and $version identify the component, and $os and $arch are a supported platform. Each tarball has a sha1 hash with the same name, but extension .sha1, instead of .tar.gz.

### Manifest formats

See `ComponentManifest` and `VersionManifest` data structures in [component.go](../../pkg/repository/types.go) and [version.go](../../pkg/version/version.go).
