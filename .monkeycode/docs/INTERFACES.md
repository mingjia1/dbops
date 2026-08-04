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

`completedSingleNodeCapabilities` is the per-flavor record used to activate completed single-node executors. The initial flavor task handlers only validate local media and detect versions, so their records are empty. `singleNodeExecutorCapabilities` only accepts lifecycle capabilities compatible with a completed single-node executor; it always keeps `replication`, `failover`, `scale`, `node_rebuild`, and `cluster_deploy` disabled.

## Xinchuang Core Lifecycle Contract

`kernel.XinchuangCorePlugin` defines `Prepare`, `Execute`, `Rollback`, `Teardown`, `Join`, and `Leave` for each dedicated flavor executor. `Execute` accepts these operations: `deploy`, `configure`, `backup`, `restore`, `upgrade`, `migrate`, `ha`, `replication`, `failover`, and `monitor`.

`kernel.XinchuangCoreBase` sends `flavor`, `operation`, `task_id`, and target node details to the injected Agent task route. Agent responses succeed only with one of `completed`, `success`, `succeeded`, or `ok`; all other and missing statuses return errors for the caller to persist as task failure.

## Agent Flavor Task Route

`POST /agent/tasks/flavor` accepts `executor.FlavorTaskRequest`. The payload requires `task_id`, `instance_id`, `flavor`, `version`, `operation`, `package_path`, and a `tls` object. `package_path` must exactly equal `/opt/dbops/packages/<flavor>/<version>` after path cleaning.

The Agent registers isolated version command builders for OceanBase, GaussDB MySQL, PolarDB MySQL, TDSQL MySQL, TiDB, Kingbase, openGauss, HighGo, GBase 8a, Shentong, DM, and GBase 8s. `validate-package` verifies the local manifest and hashes. `version-detect` runs the registered fixed binary with `--version` and requires the output to contain the requested version.

For OceanBase, `deploy` accepts a structured `oceanbase` object and only executes approved RPM installation, the fixed observer and OBProxy binaries, resource-unit and tenant SQL, and the parameter allowlist `enable_perf_event`, `enable_sql_audit`, and `syslog_level`. TLS enables OceanBase file-mode certificates and client authentication, then updates the OBProxy file-certificate configuration. The deploy flow verifies `SELECT VERSION()`, `SHOW OB SERVERS`, and tenant presence before reporting completion. `configure` applies the same parameter and TLS controls without installation. `backup` requires an approved `file:///opt/dbops/backups/oceanbase/` destination, sets `DATA_BACKUP_DEST`, triggers a database-plus-archivelog backup, and waits for `CDB_OB_BACKUP_JOBS` to reach `COMPLETED`. `restore` and `migrate` restore a backup into a new tenant and require schema, row-count, and checksum validation. `upgrade` creates this backup before invoking the fixed `obshell cluster upgrade` command. `monitor` returns a version, server, and tenant-status snapshot; `ha`, `replication`, and `failover` return a single-node capability error. `teardown` requires `confirm_uninstall: true`, a completed backup configuration, and a data directory under `/data/oceanbase`; it invokes `obshell cluster stop`, then removes that data directory and fixed OceanBase and OBProxy installation directories. Other lifecycle actions return a failed task until their dedicated flavor implementation is available.

For TiDB, `deploy` accepts `tidb.cluster_name`, `address`, `architecture`, `deploy_user`, a restricted root password, and an optional parameter allowlist. The local bundle must contain SHA-256-verified `tidb-community-server-<version>-linux-<architecture>.tar.gz` and toolkit archives. The executor extracts only those archives, runs the official local TiUP installer, writes a local PD, TiKV, and TiDB topology, then runs `tiup cluster deploy`, `start --init`, `display`, and SQL version/configuration queries. It changes the one-time initialization password within the Agent before using the requested root password. TLS configures TiDB client and internal certificates plus PD and TiKV component certificates. `configure` writes the validated topology and runs a TiUP reload for the three components.

TiDB `backup` runs BR full backup and may start and verify log backup. `restore` runs full BR restore or PITR based on `restored_ts`, then checks configured table row counts and checksums. All BR locations must use `local:///opt/dbops/backups/tidb/` paths. `migrate` accepts controlled export and sorted-KV directories under `/opt/dbops/backups/tidb/`, runs Dumpling export and Lightning import, and writes the Lightning configuration with restrictive file permissions. `upgrade` requires an approved BR backup destination and target version before running TiUP rolling upgrade and cluster health verification.

TiDB `monitor` returns TiUP component status and the SQL engine version. `ha`, `replication`, and `failover` return a clear single-host capability error. `teardown` requires `confirm_uninstall: true` and an approved BR backup destination; it completes a final BR backup before calling TiUP cluster destroy.

For Dameng DM9, `deploy` accepts the structured `dameng` object with address, port, installation and data paths under `/opt/dbops/dm/`, SYSDBA and application credentials, and a parameter allowlist. The local SHA-256-verified bundle must include `DMInstall.bin`. The executor writes the documented silent-install XML, runs `DMInstall.bin -q`, initializes with `dminit`, starts `dmserver`, applies the allowlisted parameters, creates the application user, and queries the server version through `disql`. TLS validates supplied certificate files and enables the documented `ENABLE_ENCRYPT=5` server setting. The task reports `restart_required: true` because DM applies this security setting after a controlled server restart.

Dameng `backup` supports online DIsql full backup and offline DMRMAN full backup. `restore` validates the offline backup set before DMRMAN restore, and can restore and recover archive logs before updating the database magic. `migrate` runs dexp then dimp against a controlled dump file. Backup sets, archive sets, and dump files must reside under `/opt/dbops/backups/dm/`. DM upgrade remains unavailable pending an official, verified DM9 upgrade procedure and matching offline delivery media.

Dameng `monitor` queries instance and archive states through `V$INSTANCE` and `V$ARCH_STATUS`. `ha`, `replication`, `failover`, and `scale` return a single-instance capability error. `teardown` requires `confirm_uninstall: true` and an approved final backup destination; it runs the final backup before the official `uninstall.sh -i` command.

For openGauss, the structured `opengauss` object accepts installation and data directories under `/opt/dbops/opengauss/`, fixed backup, restore, and migration paths under `/opt/dbops/backups/opengauss/`, and `confirm_uninstall`. Its monitoring and controlled teardown use the dedicated flavor task route. `monitor` returns `gs_ctl query -D <data>` output, SQL `version()`, active connection count, and `pg_stat_archiver` output. `teardown` requires `confirm_uninstall: true` and a valid backup destination, runs `gs_basebackup` first, then invokes only `<install>/bin/gs_uninstall --delete-data -L`. `ha`, `replication`, `failover`, and `scale` return explicit single-node capability errors; `upgrade` stays unavailable pending a verified cluster XML delivery chain.

For GBase 8s, the structured `gbase8s` object accepts an installation directory under `/opt/dbops/gbase8s/`, controlled `ontape` archive and logical-log directories under `/opt/dbops/backups/gbase8s/`, and `confirm_uninstall`. `monitor` returns output from fixed `onstat -c`, `onstat -g cfg`, `onstat -V`, `ps -C oninit`, and `df -P <install>` commands. `teardown` requires `confirm_uninstall: true` and valid final `ontape` directories, then returns a controlled failure because public official documentation has no verified uninstall executable and parameter set; it performs no backup, shutdown, or cleanup command. `ha`, `replication`, `failover`, and `scale` return explicit single-node capability errors.

The executor fixture suite injects its command runner and creates local package bundles in temporary directories. It automatically verifies successful version detection, missing binary handling, package checksum failures, and permission errors. Backend lifecycle fixtures verify that a failed Agent task is surfaced to the caller and that `Rollback` dispatches a separate `rollback` Agent task.

For Kingbase KES V9R1C10, the `kingbase` object accepts an address, port, installation and data directories under `/opt/dbops/kingbase/`, SCRAM superuser and application credentials, an allowlisted parameter map, and an optional TLS CIDR. `deploy` requires a manifest with `requires_license: true`, `license_file`, and SHA-256 verified `setup.sh` and `silent.conf`. It executes the fixed silent setup command, writes the initdb password via the Agent input runner, initializes with `initdb -D <data> -U superuser -A scram-sha-256 --pwfile <securefile>`, and starts through `sys_ctl`. `configure` applies `ALTER SYSTEM SET`, runs `sys_reload_conf` and `sys_ctl reload`, and creates the application user over standard input. TLS accepts only ordinary absolute CA, certificate, and key files plus an exact canonical CIDR with a nonzero mask; it sets `ssl`, `ssl_ca_file`, `ssl_key_file`, and `ssl_cert_file`, then appends `hostssl all all <cidr> cert`. `backup.destination`, `restore.backup_source`, and `migration.dump_file` must stay under `/opt/dbops/backups/kingbase/`. `backup` uses fixed `sys_basebackup -D <destination> -Fp -Pv -Xf`; `restore` only accepts a `.dump` source and a validated target database, then uses `sys_restore -d <target> <dump>`; `migrate` exports through `sys_dump -Fc -f <dump> <source>` and restores through `sys_restore -d <target> <dump>`. The `-f` option provides the same custom dump result as shell redirection while preserving argument-safe execution. A supplied recovery target returns a PITR refusal until formal `sys_rman` repository and archive configuration is verified. Upgrade returns a refusal until the official `sys_upgrade --check` procedure, separate backup, controlled old and new paths, and exact target version are all verified.
