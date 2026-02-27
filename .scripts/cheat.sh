#!/bin/bash

# step 1
.scripts/co-sample "Create a workspace file in the root of your repo" \
  origin/shadow/sample-batch1 \
  default.code-workspace

# step 2
.scripts/co-sample "Add a Dev Container" \
  origin/shadow/sample-batch1 \
  .devcontainer/devcontainer.json \
  .github/dependabot.yml

# step 3
.scripts/co-sample "Setup default linters" \
  origin/shadow/sample-batch2 \
  .cspell.jsonc \
  .devcontainer/devcontainer.json \
  .dict/repo.dictionary \
  .markdownlint-cli2.jsonc \
  .prettierignore \
  .prettierrc \
  .vscode/settings.json

# step 4
.scripts/co-sample "Setup git" \
  origin/shadow/sample-batch3 \
  .gitconfig \
  .gitignore \
  .devcontainer/postCreateCommand.sh \
  .scripts/trunk-worthy \
  .githooks/pre-commit

# step 5
.scripts/co-sample "Setup TakT and workflows" \
  origin/shadow/sample-batch4 \
    .devcontainer/.gh_alias.yml \
    .devcontainer/devcontainer.json \
    .devcontainer/postCreateCommand.sh \
    .github/actions/prep-runner/action.yml \
    .github/workflows/ready.yml \
    .github/workflows/release.yml \
    .github/workflows/stage.yml \
    .github/workflows/wrapup.yml \
    .scripts/trunk-worthy \
    docs/ready_pusher.md

#step 6
.scripts/co-sample "Create boilerplate Go project" \
  origin/shadow/sample-batch5 \
    .devcontainer/postCreateCommand.sh \
    .github/actions/build-go-binaries/action.yml \
    .github/workflows/ready.yml \
    .github/workflows/release.yml \
    .golangci.yml \
    .scripts/trunk-worthy \
    .vscode/settings.json \
    Makefile \
    cmd/mkissue.go \
    cmd/mkissue/mkissue.go \
    cmd/mkissue/mkissue_test.go \
    cmd/root.go \
    go.mod \
    go.sum \
    main.go \
    specs/template.issue.md

# step 7
.scripts/co-sample "Set up for Copilot collaboration" \
  origin/shadow/sample-batch5 \
    .github/copilot-instructions.md \
    .github/instructions/go-standards.instructions.md \
    .github/workflows/copilot-setup-steps.yml \
    .github/workflows/pr-to-ready.yml \
    CONTRIBUTING.md