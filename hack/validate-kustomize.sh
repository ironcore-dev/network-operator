#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

BASEDIR=$(cd -- "$(dirname -- "$0")" && pwd)

for kustomization in $(find "$BASEDIR/../config" -name "kustomization.yaml"); do
  dir=$(dirname "$kustomization")
  name=${dir#"$BASEDIR/../"}
  if kustomize build "$dir" >/dev/null 2>&1; then
    echo "OK: $name"
  else
    echo "FAILED: $name"
    exit 1
  fi
done
