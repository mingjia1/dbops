# GBase 8a 10.1 Requirements

## Introduction

Add a constrained GBase 8a 10.1 single-node lifecycle path that runs only through the dedicated flavor Agent.

## Glossary

- **Dedicated flavor Agent**: The Agent flavor task route that dispatches `gbase8a` tasks.
- **Approved manifest**: A verified local package manifest containing the required GBase 8a files and license.

## Requirements

### Requirement 1

**User Story:** AS an operations engineer, I want a verified GBase 8a deployment path, so that installation uses only approved local media and commands.

#### Acceptance Criteria

1. WHEN a GBase 8a deploy task is received, the Agent SHALL require a licensed manifest containing `SetSysEnv.py`, `gcinstall.py`, `demo.options`, and `gcChangeInfo.xml`.
2. WHEN the manifest passes validation, the Agent SHALL run `SetSysEnv.py`, `gcinstall.py`, and the fixed single-node distribution command with the manifest-provided XML.
3. IF a required artifact or license is unavailable, the Agent SHALL return a failed task before executing a command.

### Requirement 2

**User Story:** AS an operations engineer, I want controlled GBase 8a configuration, so that runtime changes are bounded.

#### Acceptance Criteria

1. WHEN a deploy or configure task includes `gcluster_rebalancing_concurrent_count` from 1 through 64, the Agent SHALL issue the corresponding `SET GLOBAL` statement through `gccli`.
2. IF a parameter name or value falls outside the allowlist, the Agent SHALL return a failed task before executing a command.
3. WHERE a task uses GBase 8a paths, the Agent SHALL accept `/opt/dbops/gbase8a` and its descendants.

### Requirement 3

**User Story:** AS a platform operator, I want explicit GBase 8a safety boundaries, so that generic workflows and unverified TLS actions are contained.

#### Acceptance Criteria

1. WHEN a generic platform capability is requested for GBase 8a, the capability gate and console mirror SHALL keep the operation unavailable.
2. IF a TLS request is received, the Agent SHALL require regular absolute certificate files below `/opt/dbops/gbase8a/certs/` and return a controlled refusal until a vendor restart command is verified.
3. WHEN a TLS configuration file is written after restart verification, the Agent SHALL write the `[gbased]` TLS fields to `<install_prefix>/gbase_8a_gcluster.cnf` with mode `0600`.

### Requirement 4

**User Story:** AS an operations engineer, I want a controlled full backup workflow, so that GBase 8a backup media and cluster state are bounded.

#### Acceptance Criteria

1. WHEN a backup task provides a destination below `/opt/dbops/backups/gbase8a/`, the Agent SHALL switch the cluster to `readonly`, run `python <install_prefix>/server/bin/gcrcman.py -d <destination> -P gbasedba -e 'backup level 0'`, and switch the cluster to `normal`.
2. IF the backup command returns an error after the readonly switch, the Agent SHALL issue `gcadmin switchmode normal` before returning a failed task.
3. IF a backup destination leaves the controlled root, the Agent SHALL return a failed task before executing a command.

### Requirement 5

**User Story:** AS an operations engineer, I want constrained recovery and file migration, so that lifecycle input stays auditable and argument-safe.

#### Acceptance Criteria

1. WHEN a restore task selects `full: true` or validated database and table objects, the Agent SHALL switch to `recovery`, run a generated `gcrcman` recover expression, and switch to `normal`.
2. IF a restore task supplies neither a full scope nor validated objects, the Agent SHALL return a failed task before executing a command.
3. WHEN a migration task supplies a `file:///opt/dbops/backups/gbase8a/` URI and validated database and table identifiers, the Agent SHALL issue one generated `LOAD DATA INFILE` statement through `gccli`.
4. IF a migration URI uses another scheme, leaves the controlled root, or contains an invalid identifier, the Agent SHALL return a failed task before executing a command.
5. WHEN an upgrade or rollback task is received, the Agent SHALL return a failed task that identifies complex cluster topology and version prerequisites as the vendor-verification boundary.
