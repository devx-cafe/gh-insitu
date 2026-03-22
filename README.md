<!-- cspell:ignore insitu markdownlint cspell golangci -->

# gh-insitu

> A self-contained parallel task runner for the [GitHub CLI](https://cli.github.com/).

**InSitu** brings parity between local development and CI by letting you define checks once in `.insitu.yml` and run them identically on your machine and inside GitHub Actions workflows.

[![Latest release](https://img.shields.io/github/v/release/devx-cafe/gh-insitu)](https://github.com/devx-cafe/gh-insitu/releases/latest)
[![Release workflow](https://github.com/devx-cafe/gh-insitu/actions/workflows/release.yml/badge.svg)](https://github.com/devx-cafe/gh-insitu/actions/workflows/release.yml)

---

## Installation

Requires the [GitHub CLI](https://cli.github.com/) (`gh`) to be installed.

```bash
gh extension install devx-cafe/gh-insitu
```

Verify:

```bash
gh insitu --help
```

---

## Quick start

```bash
# Bootstrap a starter .insitu.yml and a pre-commit hook
gh insitu init

# Validate the config and preview the execution plan
gh insitu plan

# Run all waves
gh insitu run

# Run a specific wave
gh insitu run static
```

---

## `.insitu.yml` syntax

The configuration file uses three top-level sections:

```yaml
# .insitu.yml

# 1. Global defaults – inherited by all checks
defaults:
  die-on-error: true   # stop after the first failing wave
  timeout: 5m          # default per-check timeout (Go duration string)
  verbose: false       # print command output for every check (not just failures)

# 2. Inventory – the library of named checks
inventory:
  - id: "build"
    name: "Build"           # human-readable label (optional; falls back to id)
    command: "make build"
    timeout: 10m            # per-check timeout override (optional)
    die-on-error: false     # per-check die-on-error override (optional)

  - id: "coverage"
    name: "Unit Test with Coverage"
    command: "make coverage"

# 3. Waves – ordered execution groups that reference inventory ids
waves:
  - id: "static"
    name: "Static Analysis & Build"
    parallel: true   # run all checks in this wave concurrently
    checks:
      - "build"

  - id: "test"
    name: "Post-Build Validation"
    parallel: true
    checks:
      - "coverage"
```

### Field reference

| Field | Required | Description |
| --- | --- | --- |
| `defaults.die-on-error` | no | Stop after the first wave that has a failure. Default: `false`. |
| `defaults.timeout` | no | Fallback timeout for every check. Go duration string (e.g. `5m`, `30s`). Default: `5m`. |
| `defaults.verbose` | no | Print full command output for all checks. Default: `false` (CI: `true`). |
| `inventory[].id` | **yes** | Unique identifier, referenced by waves. |
| `inventory[].name` | no | Display name shown in output. |
| `inventory[].command` | **yes** | Shell command to execute. |
| `inventory[].timeout` | no | Override `defaults.timeout` for this check. |
| `inventory[].die-on-error` | no | Override `defaults.die-on-error` for this check. |
| `waves[].id` | **yes** | Unique wave identifier. |
| `waves[].name` | no | Display name for the wave. |
| `waves[].parallel` | no | When `true`, all checks in the wave run concurrently. Default: `false`. |
| `waves[].checks` | **yes** | List of inventory `id` values to execute. |

### Config file discovery

By default every command looks for `.insitu.yml` in the current working directory. You can override this with the `--config` / `-c` flag on any command:

```bash
gh insitu run --config path/to/custom.yml
gh insitu plan -c ci/checks.yml
```

---

## Commands

### `insitu init`

Creates a starter `.insitu.yml` and installs a `.git/hooks/pre-commit` hook that runs `insitu run` before every commit.

```bash
gh insitu init               # create .insitu.yml + pre-commit hook
gh insitu init --force       # overwrite an existing .insitu.yml
gh insitu init -c custom.yml # write config to a custom path
```

### `insitu plan`

Validates the configuration and prints the resolved execution plan (timeouts, commands, die-on-error settings). No checks are actually run.

```bash
gh insitu plan
gh insitu plan --config custom.yml
```

### `insitu run`

Executes one or more waves. Without arguments all waves run in order.

```bash
gh insitu run                            # run all waves
gh insitu run static                     # run only the 'static' wave
gh insitu run static test                # run 'static' then 'test'
gh insitu run --verbose                  # always print all command output
gh insitu run --mark-pending             # mark all checks as 'pending' (CI only)
gh insitu run trunk-worthy --mark-pending
```

**Inside a GitHub Actions workflow** (`GITHUB_ACTIONS=true`):

- Each check result is automatically reported as a [GitHub commit status](https://docs.github.com/en/rest/commits/statuses).
- Output is wrapped in `::group::` / `::endgroup::` for collapsible log sections.
- Use `--mark-pending` as an early step to show checks as pending while the workflow runs.

---

## GitHub Actions example

```yaml
- name: Mark checks pending
  run: gh insitu run --mark-pending
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}

- name: Run all checks
  run: gh insitu run
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## Releases

See the [Releases page](https://github.com/devx-cafe/gh-insitu/releases) for all published versions and release notes. Releases are built and published automatically by the [Release workflow](https://github.com/devx-cafe/gh-insitu/actions/workflows/release.yml).

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines, testing practices, and how to set up the local development environment.
