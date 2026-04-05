#!/usr/bin/env bash
# Configure git to use project hooks from .githooks/ directory.
# Run once after cloning the repo: bash scripts/setup-hooks.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

git config core.hooksPath .githooks
echo "✓ Git hooks configured: core.hooksPath = .githooks"
