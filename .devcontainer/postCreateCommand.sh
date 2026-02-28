#!/usr/bin/env bash

set -eou pipefail

PREFIX="🍰  "
echo "$PREFIX Running $(basename $0)"

git config --global --add safe.directory $(pwd)
echo "$PREFIX ✅ Setting up safe git repository to prevent dubious ownership errors"

git config --local --get include.path | grep -e ../.gitconfig >/dev/null 2>&1 || git config --local --add include.path ../.gitconfig
echo "$PREFIX ✅ Setting up git configuration to support .gitconfig in repo-root"

echo "$PREFIX Leaving $(basename $0)"
exit 0