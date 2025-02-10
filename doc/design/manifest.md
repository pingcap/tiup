# Manifest format and repository layout

See [#120](https://github.com/pingcap/tiup/issues/120).

## The Update Framework

[The Update Framework (TUF)](https://theupdateframework.io/overview/) is the state of the art for update systems. It has a strong focus on security.

TUF is a [specification](https://github.com/theupdateframework/specification/blob/master/tuf-spec.md#the-update-framework-specification) for building update systems, not an implementation (though reference implementations exist). It does not fully specify how such a system should work, e.g., it does not discuss how updates are applied or how servers are managed. However, it defines what metadata should be kept, how data and metadata should be signed, and how to ensure downloads are secure.

We build on and adapt TUF, all places where we deviate from TUF are enumerated below. I believe we still fulfil all TUF security guarantees.


## Components

A component is a program which can be run by TiUp. In this document, we do not consider what constitutes a component, for our purposes it is an opaque file.

A component may have many versions. We do not discuss how versions are specified or applied, only that a client may specify a version to download.

A component may be supplied by PingCAP or a third party, we detail how components are *uploaded*, but how components are packaged and checked by the client is out of scope. We assume that a component is not 'opened', i.e., a component is uploaded to a server and later downloaded, the server does not inspect the contents of the component. Note that this design document only addresses part of the security issue with handling third party code or binaries - we ensure that the code that a user downloads is the same code that the publisher uploads, however, we cannot ensure that the code uploaded is not malicious.

We do not support channels or groupings of components. We expect components will support something here, but it will be up to the client to implement.

TiUp itself can be treated as a component and I think does not need to be treated any differently on the server side.

## Other terms

* repository: a source of components and manifests (aka server, mirror)
* manifest: a JSON metadata file
* platform: the hardware platform and operating system where a binary will run (c.f., built).


## Principles

* URLs are persistent.
* All data is immutable
* Most metadata is immutable, achieved using versioning.
* Components cannot be removed or deleted.
* Zero-trust access control.
* Minimise the number of keys and the frequency with which they must be replaced.
* Uploads and downloads should be secure.
* All operations should be idempotent or at least repeatable. If an operation is interrupted, the user can just retry until it succeeds.
* There should be no 'transaction' mechanism between client and server, groups of operations should not require atomicity.
* Minimise required knowledge in the client - the metadata should direct the client how to perform operations with minimal knowledge of the server's data layout, etc.
* We should not require consistency between files in the repo (beyond hashes and keys).


## Assumptions

* Updates are per-component, updating all components is interpreted by the client.
* There is a secure channel to distribute initial and updated keys for each server
* Don't need to change the owner of a component.
* Don't need to delegate ownership of a component (owners can delegate themselves with client-side tooling).


## Repository organisation

We require snapshot.json and root.json to exist at the root of the repository. There will be many n.root.json files, these must also be at the repository root.

The rest of the repository organisation can be changed freely since all other locations are specified by URLs. All files and manifests should be in the repository root, i.e., there is a single directory.

Each non-manifest file has the form `id-version-platform.ext`, where `ext` is `tar.gz` for tarballs, `id` is the component id, `platform` is a target triple, and `version` is a semver version containing exactly two periods (`.`, e.g., `1.0.0`).


## Manifests

Each manifest is a [canonical json](http://wiki.laptop.org/go/Canonical_JSON) file with a `.json` extension.

Manifest format:

```
{
    "signatures": [
        {
            "keyid": "",
            "sig": "",
        }
    ],
    "signed": {
        "_type": "TY",
        "spec_version": "",
        "expires": "TS",
        // version is only present for versioned manifests.
        "version": n,
        ROLE
    },
}
```

where

* `""` is a string.
* `n` is an integer > 0.
* `"TS"` is a timestamp (date and time) in ISO 8601 format; timezone should always be `Z`.
* `"TY"` is one of `"root"`, `"index"`, `"component"`, `"snapshot"`, or `"timestamp"`.
* `ROLE` is specific to the type of manifest.
* The value of `"spec_version"` should be valid semver.

The timestamp and snapshot manifests should expire after one day. All other manifests should expire after one year.

URLs are relative to the repository root, which allows a repository to be cloned to a new location without changing URLs.

The spec version should be the same for all files in a given snapshot, if the client finds a spec version which is inconsistent, or that the client cannot interpret, it should abort its operation.

### Keys

A key definition is:

```json
"KEYID": {
    "keytype": "ed25519",
    "keyval": {
        "public": "KEY",
    },
    "scheme": "ed25519",
},
```

where

* `"KEYID"` is a globally unique identifier (i.e., keys must not share key ids); it is the hexdigest of the SHA-256 hash of the canonical JSON form of the key.
* We support `"sha256"` and `"sha512"` hashing (only SHA256 in first iteration).
* We support `"rsa"`, `"ed25519"`, and `"ecdsa-sha2-nistp256"` key types (only `"rsa"` in first iteration).
* We support `"rsassa-pss-sha256"`, `"ed25519"`, and `"ecdsa-sha2-nistp256"` schemes (only `"rsassa-pss-sha256"` in first iteration)
  - "rsassa-pss-sha256" : RSA Probabilistic signature scheme with appendix. The underlying hash function is SHA256. https://tools.ietf.org/html/rfc3447#page-29
  - "ed25519" : Elliptic curve digital signature algorithm based on Twisted Edwards curves. https://ed25519.cr.yp.to/
  - "ecdsa-sha2-nistp256" : Elliptic Curve Digital Signature Algorithm with NIST P-256 curve signing and SHA-256 hashing. https://en.wikipedia.org/wiki/Elliptic_Curve_Digital_Signature_Algorithm

Note that the non-rsa schemes only need implementing if needed.

The private keys for the timestamp, snapshot, and index roles will be stored online (with the server software, not in the repository itself), they must not be accessible to the internet. Private keys for the root role should be stored offline; any action requiring re-signing of the root manifest is an admin action which must involve a user with access to the private keys.

Every manifest except root.json has key threshold 1, i.e., each manifest can be signed by a single key (except during key migrations, see the [TUF spec](https://github.com/theupdateframework/specification/blob/master/tuf-spec.md#6-usage) for more info).

root.json should have a threshold of at least 3. We should create 4 (or 5?) root keys which are stored offline and separately (to reduce the risk of an attack if the place where a key is stored is compromised).

For each owner, we create a new key pair in the client, the public key is stored in index.json, the private key is stored by the client. Every component manifest can be signed by either the component's owner or by the index role (this is implicit, an alternative would be to add these keyids to the component metadata in index.json, which would be more flexible and make key revocation explicit).

Rotating keys is specified by [TUF](https://github.com/theupdateframework/specification/blob/master/tuf-spec.md#6-usage). Neither TUF nor TiUp will dictate how often this should happen. The TiUp server software should provide a facility for doing so, both for the root keys and for owner keys. The TiUp client should automatically rotate owner keys on a regular basis (question: how often?). TiUp server should ensure that all keys are unique, this should be verified by the client whenever it downloads a new manifest.

### root.json and n.root.json

```
"_type": "root"
```

Indexes other top-level manifests and roles. n is the version, versioned files are immutable, root.json is a copy of the most recent version of n.root.json (or a symlink, if that works with the web server) and is mutable.

Example:

```
"roles": {
    "root": {
        "url": "/root.json",
        "keys": {
            "65171251a9aff5a8b3143a813481cb07f6e0de4eb197c767837fe4491b739093": {
                "keytype": "ed25519",
                "keyval": {
                    "public": "edcd0a32a07dce33f7c7873aaffbff36d20ea30787574ead335eefd337e4dacd",
                },
                "scheme": "ed25519",
            },
        },
        "threshold": 1,
    },
    "index": {
        "url": "/index.json",
        "keys": {
            "5e777de0d275f9d28588dd9a1606cc748e548f9e22b6795b7cb3f63f98035fcb": {
                "keytype": "rsa",
                "keyval": {
                    "public": "-----BEGIN PUBLIC KEY-----\nMIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEA0GjPoVrjS9eCqzoQ8VRe\nPkC0cI6ktiEgqPfHESFzyxyjC490Cuy19nuxPcJuZfN64MC48oOkR+W2mq4pM51i\nxmdG5xjvNOBRkJ5wUCc8fDCltMUTBlqt9y5eLsf/4/EoBU+zC4SW1iPU++mCsity\nfQQ7U6LOn3EYCyrkH51hZ/dvKC4o9TPYMVxNecJ3CL1q02Q145JlyjBTuM3Xdqsa\nndTHoXSRPmmzgB/1dL/c4QjMnCowrKW06mFLq9RAYGIaJWfM/0CbrOJpVDkATmEc\nMdpGJYDfW/sRQvRdlHNPo24ZW7vkQUCqdRxvnTWkK5U81y7RtjLt1yskbWXBIbOV\nz94GXsgyzANyCT9qRjHXDDz2mkLq+9I2iKtEqaEePcWRu3H6RLahpM/TxFzw684Y\nR47weXdDecPNxWyiWiyMGStRFP4Cg9trcwAGnEm1w8R2ggmWphznCd5dXGhPNjfA\na82yNFY8ubnOUVJOf0nXGg3Edw9iY3xyjJb2+nrsk5f3AgMBAAE=\n-----END PUBLIC KEY-----",
                },
                "scheme": "rsassa-pss-sha256",
            },
        },
        "threshold": 1,
    },
    "snapshot": {...},
    "timestamp": {...},
},
```

### n.index.json

```
"_type": "index"
```

Lists owners and components. File is versioned and immutable, n is the version.

Replaces the targets role in TUF.

Example:

```json
"owners": {
    "pingcap": {
        "name": "PingCAP",
        "threshold": 1,
        "keys": {
            "4e777de0d275f9d28588dd9a1606cc748e548f9e22b6795b7cb3f63f98035fcb": {
                "keytype": "rsa",
                "keyval": {
                    "public": "-----BEGIN PUBLIC KEY-----\nMIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEA0GjPoVrjS9eCqzoQ8VRe\nPkC0cI6ktiEgqPfHESFzyxyjC490Cuy19nuxPcJuZfN64MC48oOkR+W2mq4pM51i\nxmdG5xjvNOBRkJ5wUCc8fDCltMUTBlqt9y5eLsf/4/EoBU+zC4SW1iPU++mCsity\nfQQ7U6LOn3EYCyrkH51hZ/dvKC4o9TPYMVxNecJ3CL1q02Q145JlyjBTuM3Xdqsa\nndTHoXSRPmmzgB/1dL/c4QjMnCowrKW06mFLq9RAYGIaJWfM/0CbrOJpVDkATmEc\nMdpGJYDfW/sRQvRdlHNPo24ZW7vkQUCqdRxvnTWkK5U81y7RtjLt1yskbWXBIbOV\nz94GXsgyzANyCT9qRjHXDDz2mkLq+9I2iKtEqaEePcWRu3H6RLahpM/TxFzw684Y\nR47weXdDecPNxWyiWiyMGStRFP4Cg9trcwAGnEm1w8R2ggmWphznCd5dXGhPNjfA\na82yNFY8ubnOUVJOf0nXGg3Edw9iY3xyjJb2+nrsk5f3AgMBAAE=\n-----END PUBLIC KEY-----",
                },
                "scheme": "rsassa-pss-sha256",
            },
            "59a4df8af818e9ed7abe0764c0b47b4240952aa0d179b5b78346c470ac30278d": {
                "keytype": "ed25519",
                "keyval": {
                    "public": "edcd0a32a07dce33f7c7873aaffbff36d20ea30787574ead335eefd337e4dacd",
                },
                "scheme": "ed25519",
            },
        },
    },
},
"components": {
    "tidb": {
        "yanked": false,
        "owner": "pingcap",
        "url": "/name_of.json",
    },
},
"default components": [
    "tidb",
    "tikv",
    "pd",
]
```

Each owner id and component id must be unique (TiUp should treat owner and component ids as distinct types, but ids must be unique within the union of the types). 

### n.xxx.json

```
"_type": "component"
```

Where xxx is the id of the component, n is the version of the manifest, file is immutable.

```
"description": "This is a component",
"nightly": "v0.2.0-nightly+20200525",
"platforms": {
    "x86_64-apple-darwin": {
        "v0.0.1": {
            "yanked": false,
            "url": "/name-of_x86_64-apple-darwin_v0.0.1.tar.gz",
            "hashes": {
                "sha256": "141f740f53781d1ca54b8a50af22cbf74e44c21a998fa2a8a05aaac2c002886b",
                "sha512": "ef5beafa16041bcdd2937140afebd485296cd54f7348ecd5a4d035c09759608de467a7ac0eb58753d0242df873c305e8bffad2454aa48f44480f15efae1cacd0"
            },
            "length": 1001000499,
            "dependencies": {
                "foo": "v1.0.0",
            },
        },
        "v0.2.0": { ... },
    },
    "aarch64-unknown-linux": { ... },
},
```

The platform id should be one of the supported TiUp target triples. Version ids must be valid semver. Dependencies are a map from component id to a semver version string.

The "nightly" points to the version number of latest daily build, that version should be in the version list of all supported platforms. The version number of nightly build should (but not forced to) be in the following format:

```
vX.Y.Z-nightly-YYYY-mm-dd
```

Where `vX.Y.Z` is the version of the last released version of that component.

### snapshot.json

```
"_type": "snapshot"
```

Lists the latest version number for each manifest.

Used for consistency, to check for updates, and security.

Not versioned.


```json
"meta": {
    "root.json": {
        "version": 1
        "length": 500
    },
    "index.json": {
        "version": 1
        "length": 500
    },
    "name_of.json": {
        "version": 1,
        "length": 500
    },
    "foo.json": {
        "version": 1,
        "length": 240
    }
},
```

### timestamp.json

```
"_type": "timestamp"
```

Short timeout, basically just a hash of snapshot.json.

This file has a version, but we don't keep versioned files on the server, just one timestamp.json. This file is mutable.

```json
"meta": {
    "snapshot.json": {
        "hashes": {
            "sha256": "8f88e2ba48b412c3843e9bb26e1b6f8fc9e98aceb0fbaa97ba37b4c98717d7ab"
        },
        "length": 515,
    }
},
```

## Deviations from TUF

* Exactly one layer of delegation to owners; no terminating of delegation.
* keys are inlined since we do not support sharing keys.
* We use some aspects of consistent snapshots for security, availability, and convenience, but we do not guarantee consistent snapshots.
* targets.json -> index.json; no delegations section, replaced with list of owners (no delegated targets manifests, these are effectively inlined into index).
* Added component.json to deal with multiple versions and platforms of each component.
* No mirrors.json (optional anyway).
* index and snapshot role keys must be kept online since these manifests must be updated by the server when a new component or version is uploaded by a user.


## Workflows

* All downloaded manifests are kept between operations. Once an updated version has been downloaded and verified, the old version can be deleted.
* All syntax is just a strawman and is not proposed as part of the design for TiUp.
* In these workflows, if verification of a component or manifest fails, the operation may be retried or an error given to the user. In most cases we can retry as verification error may have been caused by a network error or otherwise be transient.


### Install TiUp client

* Assume there is some secure mechanism for shipping
* TiUp ships with the URL of the PingCAP repository and a recent copy of n.root.json
* Go through the update root flow (see below)
* Download and verify timestamp.json
* Use timestamp.json to download and verify snapshot.json
* Use snapshot.json to download and verify index.json
* Use index.json and snapshot.json to install default components using the install/update component flow

### Download a component version for a target

#### one component

e.g., `tiup update foo`

* Download and verify timestamp.json, if the hash of snapshot.json is unchanged, then finish with nothing to update.
  - If timestamp.json cannot be verified, it is possible that the key has changed. Retry after updating root.json
* Update root.json
* Download and verify timestamp.json (finish if it has not changed)
* Use timestamp.json to download and verify snapshot.json
* Use snapshot.json to download and verify index.json
* Check index.json for new default components, install them using the install/update component flow
* If the component has been yanked, inform the user.
* Use index.json to download and verify the component manifest (finish if the signature has not changed)
* For each platform
  - for each major version the user has installed, find the most recent, non-yanked recent version.
  - if the most recent version is more recent than the installed version, download and verify it, then install the new component and delete the old component.
  - if there is a new major version, download the most recent version of it and tell the user

Note that we do not specify what should happen to existing versions or what should be communicated to the user

#### all components

e.g., `tiup update --all`

* Download and verify timestamp.json, if the hash of snapshot.json is unchanged, then finish with nothing to update.
  - If timestamp.json cannot be verified, it is possible that the key has changed. Retry after updating root.json
* Update root.json
* Download and verify timestamp.json (finish if it has not changed)
* Use timestamp.json to download and verify snapshot.json
* Use snapshot.json to download and verify index.json
* Check index.json for new default components, install them using the install/update component flow
* For each component the user has installed (excluding TiUp)
  - If the component has been yanked, inform the user and continue to next component.
  - Use index.json to download and verify the component manifest (continue to next component if the signature has not changed)
  - For each platform
    * for each major version the user has installed, find the most recent, non-yanked version.
    * if the most recent version is more recent than the installed version, download and verify it, then install the new component and delete the old component.
    * if there is a new major version, download the most recent version of it and tell the user

#### Self update

e.g., `tiup update --self`

Follow steps for one component workflow.

### Add a new owner

Probably an implicit action when a user publishes their first component.

* Generate a new key pair in the client.
  - The private key is stored securely.
  - The private key should persist if TiUp is updated or reinstalled (assuming the user does not find and delete the data)
  - The private key may be exported and imported for transfer to a different TiUp.
* Update root.json
* Send new owner request from TiUp client to server including the client's public key but not private key; request must be sent over TLS to ensure that the public key is not tampered with.
* Check that owner id and name are unique.
* Update index.json (by adding new owner), snapshot.json, and timestamp.json.
* Return success.

### Add a new component

e.g., `tiup publish foo`. Not specifying the client-side work, assume we end up with a tar ball and a generated manifest containing a single version for at least one platform.

* Update root.json
* Download and verify timestamp.json
* Use timestamp.json to download and verify snapshot.json
* Use snapshot.json to download and verify index.json
* Verify that the user's entry in index.json is as expected.
* Client signs manifest and sends manifest and tar ball to the server
* Server verifies the manifest using the clients public key from most recent index.json.
* Verify that the component id is unique and that the platform is valid.
* Create a component directory and platform sub-directory.
* Store the component tar ball and manifest (with version number 1) in the new directories.
* Return success to the client.
* Client downloads timestamp.json, snapshot.json, index.json, and the new component manifest and verifies they are as expected.

### Add a new version of a component

* Update root.json
* Download and verify timestamp.json
* Use timestamp.json to download and verify snapshot.json
* Use snapshot.json to download and verify index.json
* Verify that the user's entry and the component's entry in index.json are as expected.
* Use snapshot.json to download and verify the component's manifest, check it is as expected.
* Create a new manifest by adding the new version (and a new platform, if required).
* Client signs manifest and sends manifest and tar ball to the server
* Server verifies the manifest using the clients public key from most recent index.json.
* Verify that the platform is valid.
* Create a platform sub-directory, if necessary.
* Store the component tar ball and manifest (with incremented version number) in the new directories.
* Return success to the client.
* Client downloads timestamp.json, snapshot.json, index.json, and the new component manifest and verifies they are as expected.

### Yank a version of a component

* The version of the component is marked as yanked in the component manifest (for all platforms on which it exists).
* snapshot.json and timestamp.json are updated

### Yank all versions of a component

* The component is marked as yanked in index.json
* Every version of the component is marked as yanked in the component manifest.
* snapshot.json and timestamp.json are updated

### Add a repository to the client.

Adding a repo is trivial - we just need a URL and trusted root manifest, then follow the above workflow for installing the TiUp client to initialise the repo on the client side. However, the root manifest must be securely distributed. I believe this can be done by requesting the most recent root.json from the server using TLS (question: is this good enough?).

### Rotate a top-level key

Whether a key is rotated due to it being compromised or due to regular rotation, the procedure is the same. A top-level key cannot be removed without being replaced. This is an entirely server-side action.

* Produce a new key pair.
* Store the new private key or give it to the administrator to store securely.
* Update root.json to add the new public key and remove the old one.
* If replacing a root key, the new root.json should be signed with both the old and the new keys.
* If a non-root key is replaced, create a new version of the relevant manifest and sign with the new key (and any older, non-replaced keys).
  - Note that if the index role is having a key rotated, then any component manifests signed with that key must be re-signed, as well as index.json.
* Any copies of the new private key are securely destroyed
* snapshot.json and timestamp.json are updated

### Rotate an owner's key

* The user initiates a key rotation with TiUp client.
* The client generates a new key pair.
* The client sends the new public key to the server, signed using the old private key.
* The client downloads (via timestamp.json, etc.), verifies, re-signs, and sends back to the server a component manifest for each component the user owns.
* The server replaces the owner's entry in index.json with the new public key.
* The server replaces each component manifest with the new one.
* snapshot.json and timestamp.json are updated.
* The server returns success to the client.
* The client downloads all its manfests using via the new timestamp.json and verifies they are as expected.

### Owner key is compromised or lost

This is an admin action. The identity of the owner must be established offline, this is an obvious channel for a phishing attack, so identifying the owner should be taken very seriously.

* The owner's client generates a new key pair.
* The client re-signs all component manifests.
* The client sends the public key and all component manifests to the server using TLS.
* The client's identify is verified offline.
* index.json and all component manifests are updated; snapshot.json and timestamp.json are updated.
* Client updates and verifies index.json and component manifests.

### Ban/suspend an owner

This is an admin action performed on the server.

* All an owners components are marked as yanked.
* The owner's public key is removed from index.json (it should be stored somewhere off-server).
* Update snapshot.json and timestamp.json

We might want to add a notification mechanism so that any clients who use one of the owner's components can be notified to remove it if there is a security risk. This could be checked on update and when a yanked component is downloaded.

### Clone part or all of a repo

* The server uses snapshot.json to copy the most recent version of root.json, index.json, and each component manifest and component.
  - If only part of the repository is required, components and their manifests can be filtered in this step.
* Each manifest is set to version 1, existing versions are discarded.
* All non-owner keys are removed from manifests.
* The altered copies and moved to the new location.
* New root keys are generated and saved into the manifests.
* The new server creates snapshot and timestamp manifests.

## Underlying flows

### Update root

The client must have some trusted version of root.json, this is either the last version that has been successfully downloaded or the version shipped with the client. For a new repository, a trusted root.json must have been sent some how.

* The client repeatedly downloads new versions of n.root.json (by incrementing n) until a download fails.
* At each increment, the client verifies the signature of the new root.json using the keys in both the old and new root.json. At each increment, the version number and spec version number must be equal or greater than the old ones.
* Once we have the most recent root.json, the client can then verify this is indeed the most up to date version by comparing it to the unversioned root.json.
* The client must check the expiration of the timestamp of this version.
* If keys have changed from the starting root.json for any role, delete the corresponding manifests (so they will be downloaded fresh as needed).


### Download and verify manifest

Whenever downloading a component or manifest, the first step is to download timestamp.json, if that has not changed since the previous access, then nothing else in the repository has changed either. The downloaded timestamp.json must not have expired, if it has then report an error to the user.

For any download, if there is a known length of file, download only that many bytes. Files without a known length should have a default maximum size known by the client.

Preconditions: the client has downloaded snapshot.json and timestamp.json. The client has trusted keys for all top-level roles stored locally.

* Verify the signature of timestamp.json,
  - if it is signed with a new key, then follow the flow for updating the root manifest and then verify with the new timestamp key.
* Verify the expiration timestamp of timestamp.json.
* Verify the signature and expiration of the timestamp of snapshot.json.
* Use snapshot.json to find the URL of the manifest to download and fetch it.
  - For a component manifest, also download index.json
  - Verify the signature and timestamp of index.json using the index role's keys in root.json
  - Find the component's owner's keys via index.json (a component may also be signed by the index role)
* Verify the manifest's signature with its role's key. Verify its timestamp has not expired. Verify its version and spec version have increased or are equal to the old version numbers.


### Download and verify component

Precondition: the client has downloaded and verified the component's manifest and index.json.

* Find the required platform and version in the component. If it has been yanked or the component has been yanked in index.json, abort and/or warn the user.
* Download the component from the URL in the component manifest up to its specified length.
* Verify the downloaded file with the hash in the component manifest.

### Update manifest

#### Versioned manifest

* Read the existing version of the manifest to ensure it has the same version number as the manifest when we read it, if not, abort.
* Increment the version number, call the new version number `m` and write the new manifest to m.manifest-name.json, this write operation should fail if there is a file with that name which already exists, in that case, abort.
* Note that the old version of the manifest is kept.

#### Unversioned manifest

* Only the admin user should be able to modify an unversioned manifest, we should ensure in software that there is no concurrent modification.
* The new manifest is written to a temporary file on disk
* The old manifest is deleted
* The temporary manifest is renamed to the name of the manifest
