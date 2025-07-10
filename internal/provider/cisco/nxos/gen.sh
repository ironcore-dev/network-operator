#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0
VERSION=10.4-4

set -e

cd "$(dirname "$0")" || exit 1

if [ ! -f "Cisco-NX-OS-device.yang" ]; then
  curl -fsSLO https://raw.githubusercontent.com/YangModels/yang/refs/heads/main/vendor/cisco/nx/$VERSION/Cisco-NX-OS-device.yang
fi

rm -r genyang
mkdir genyang

go run github.com/openconfig/ygot/generator@v0.32.0 \
  -structs_split_files_count=50 \
  -exclude_state \
  -generate_append \
  -generate_getters \
  -generate_simple_unions \
  -generate_populate_defaults \
  -output_dir=./genyang \
  -package_name=genyang \
  Cisco-NX-OS-device.yang

go install golang.org/x/tools/cmd/goimports@latest
goimports -w .

go install github.com/google/addlicense@latest
addlicense -c "SAP SE or an SAP affiliate company and IronCore contributors" -s=only -y "$(date +%Y)" .

find . -type f -name "*.go" -exec sed -i.bak '1s|// Copyright|// SPDX-FileCopyrightText:|' {} \;
find . -type f -name "*.bak" -delete
