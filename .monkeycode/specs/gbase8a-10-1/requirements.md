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
