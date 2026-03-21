---
title: "github-cli feature installs extensions by git clone, bypassing gh extension install semantics"
assign: []
labels:
  - name: "bug"
  - name: "github-cli"
  - name: "devcontainer feature"
---

## Summary

The GitHub CLI Dev Container feature currently installs extensions listed in the extensions option by cloning extension repositories directly into the gh extensions folder.

This differs from gh extension install owner/repo behavior and can produce broken or inconsistent runtime results, especially for precompiled extensions.

Relevant script section:

<https://github.com/devcontainers/features/blob/20aa81fff7953399bdb89f6874b034aa7e01274f/src/github-cli/scripts/install-extensions.sh#L19-L31>

## Environment

- Host: macOS
- Dev Container: Ubuntu 24.04 (devcontainers base:noble)
- GitHub CLI: 2.88.1
- Feature used: ghcr.io/devcontainers/features/github-cli:1
- Feature option:

```jsonc
"ghcr.io/devcontainers/features/github-cli:1": {
  "extensions": "devx-cafe/gh-insitu"
}
```

## Reproduction

1. Configure a devcontainer with the github-cli feature and set extensions to an extension repository.
2. Use a postCreateCommand that expects the extension to be runnable, for example:

```jsonc
"postCreateCommand": "gh insitu run -c .insitu.prep.yml"
```

1. Build and start container.
2. Observe:

```bash
gh insitu
failed to run extension: fork/exec /home/vscode/.local/share/gh/extensions/gh-insitu/gh-insitu: no such file or directory
```

1. Observe extension appears installed:

```bash
gh ext list
# output: devx-cafe/gh-insitu
```

1. Observe installed folder contains repository source tree instead of a runnable extension payload:

```bash
ls /home/vscode/.local/share/gh/extensions/gh-insitu/
# cmd CONTRIBUTING.md default.code-workspace docs go.mod go.sum internal main.go Makefile specs
```

## Control Comparison

Manual install with GitHub CLI works as expected:

```bash
gh ext remove insitu && gh ext install devx-cafe/gh-insitu
ls /home/vscode/.local/share/gh/extensions/gh-insitu/
# gh-insitu  manifest.yml
```

That path yields a runnable extension.

## Expected Behavior

- The github-cli feature should install extensions with behavior equivalent to gh extension install owner/repo.
- Post-create commands should be able to run the extension immediately.
- Installed layout should include a runnable entry point and manifest consistent with gh extension install.

## Actual Behavior

- Feature installs by direct git clone into extension path.
- gh ext list can still report the extension.
- Running the extension fails if no root executable with expected name is present in the cloned source.
- Behavior diverges from normal gh extension install semantics and from precompiled extension support.

## Why This Is a Problem

- Breaks user expectations for the extensions feature option.
- Produces install-time/runtime mismatch: listed does not imply runnable.
- Bypasses official GitHub CLI extension resolution logic for precompiled releases.
- Creates non-deterministic outcomes depending on extension repository layout.

## Root Cause Analysis

Current implementation:

```bash
install_extension() {
    local extension="$1"
    local extensions_root
    local repo_name

    extensions_root="${XDG_DATA_HOME:-"${HOME}/.local/share"}/gh/extensions"
    repo_name="${extension##*/}"

    mkdir -p "${extensions_root}"
    if [ ! -d "${extensions_root}/${repo_name}" ]; then
        git clone --depth 1 "https://github.com/${extension}.git" "${extensions_root}/${repo_name}"
    fi
}
```

This does not call gh extension install and therefore does not preserve GitHub CLI install semantics.

## Proposed Fix

Use gh extension install as the primary installer, and keep clone only as optional fallback if absolutely required.

Suggested replacement for install_extension:

```bash
install_extension() {
    local extension="$1"

    # Prefer official gh installer so precompiled assets, manifest handling,
    # and extension discovery semantics match normal user installs.
    if gh extension install "$extension" --force; then
        return 0
    fi

    # Optional fallback to source clone for resiliency in constrained environments.
    # If fallback is retained, keep behavior explicit and visible in logs.
    echo "Warning: gh extension install failed for $extension, falling back to git clone" >&2

    local extensions_root
    local repo_name
    extensions_root="${XDG_DATA_HOME:-"${HOME}/.local/share"}/gh/extensions"
    repo_name="${extension##*/}"

    mkdir -p "${extensions_root}"
    rm -rf "${extensions_root}/${repo_name}"
    git clone --depth 1 "https://github.com/${extension}.git" "${extensions_root}/${repo_name}"
}
```

## Notes

If compatibility with older gh versions is needed, the function can perform a capability check and select the best path:

- if gh extension install supports the current syntax, use it
- otherwise fallback to clone with clear warning

## Acceptance Criteria

- extensions option produces a runnable extension for precompiled and interpreted extensions.
- behavior matches manual gh extension install owner/repo semantics.
- postCreateCommand can invoke installed extensions reliably without requiring extra install steps.
- gh ext list and actual runnable extension state remain consistent.

## Additional Context

The issue is reproducible with devx-cafe/gh-insitu and appears to stem from installation method, not from devcontainer build failure itself.
