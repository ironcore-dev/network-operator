#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

ROOTDIR=$(cd -- "$(dirname -- "$0")/.." && pwd)

# Use local kustomize binary if available, otherwise fall back to global installation
if [ -x "$ROOTDIR/bin/kustomize" ]; then
  KUSTOMIZE="$ROOTDIR/bin/kustomize"
elif command -v kustomize &>/dev/null; then
  KUSTOMIZE="kustomize"
else
  echo "Error: kustomize not found. Install it globally or run 'make kustomize' to download it locally."
  exit 1
fi

failed=0
while IFS= read -r -d '' kustomization; do
  dir=$(dirname "$kustomization")
  name=${dir#"$ROOTDIR/"}
  if $KUSTOMIZE build "$dir" >/dev/null 2>&1; then
    echo "OK: $name"
  else
    echo "FAILED: $name"
    failed=1
  fi
done < <(find "$ROOTDIR/config" -name "kustomization.yaml" -print0)

if [ "$failed" -ne 0 ]; then
  exit 1
fi
