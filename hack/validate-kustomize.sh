#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

BASEDIR=$(cd -- "$(dirname -- "$0")" && pwd)

failed=0
while IFS= read -r -d '' kustomization; do
  dir=$(dirname "$kustomization")
  name=${dir#"$BASEDIR/../"}
  if kustomize build "$dir" >/dev/null 2>&1; then
    echo "OK: $name"
  else
    echo "FAILED: $name"
    failed=1
  fi
done < <(find "$BASEDIR/../config" -name "kustomization.yaml" -print0)

if [ "$failed" -ne 0 ]; then
  exit 1
fi
