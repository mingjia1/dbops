# Interfaces

## Agent Local Package Bundle

`executor.ValidateLocalPackageBundle(root, flavor, version)` returns a parsed `LocalPackageManifest` after validating a package bundle. An empty `root` selects `/opt/dbops/packages`.

Directory layout:

```text
/opt/dbops/packages/<flavor>/<version>/
├── manifest.json
├── <package-file>
└── <license-file>
```

`manifest.json` contract:

```json
{
  "flavor": "tidb",
  "version": "v8.5.7",
  "requires_license": false,
  "license_file": "license.key",
  "packages": [
    {
      "file": "tidb.tar.gz",
      "sha256": "64-character-lower-or-upper-case-hex-digest"
    }
  ]
}
```

The validator returns an error for a missing or malformed manifest, a flavor or version mismatch, an empty package list, a package path outside the bundle, a missing or non-regular file, an invalid SHA-256, a checksum mismatch, or a missing required license.

## Backend Flavor Release Baseline

`services.ReleaseForFlavor(flavor)` returns an immutable copy of the version baseline for a registered 信创 flavor. The entry identifies the version line, source URL, delivery type, and whether authorized vendor confirmation is required.

## Capability Gate

`services.RequireCapability(flavor, capability)` allows operations explicitly enabled by the flavor matrix and returns a readable error for unsupported operations. This gate protects lifecycle paths until their dedicated executors exist.

## Xinchuang Core Lifecycle Contract

`kernel.XinchuangCorePlugin` defines `Prepare`, `Execute`, `Rollback`, `Teardown`, `Join`, and `Leave` for each dedicated flavor executor. `Execute` accepts these operations: `deploy`, `configure`, `backup`, `restore`, `upgrade`, `migrate`, `ha`, `replication`, `failover`, and `monitor`.

`kernel.XinchuangCoreBase` sends `flavor`, `operation`, `task_id`, and target node details to the injected Agent task route. Agent responses succeed only with one of `completed`, `success`, `succeeded`, or `ok`; all other and missing statuses return errors for the caller to persist as task failure.
