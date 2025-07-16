#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0
VERSION=5.2.0

set -e

cd "$(dirname "$0")" || exit 1

find . ! -name "gen.go" ! -name "gen.sh" ! -name "client.go" ! -name "provider.go" ! -name ".gitignore" -delete

mkdir -p yang
curl -fsSL https://github.com/openconfig/public/archive/refs/tags/v$VERSION.tar.gz | tar -C yang --strip-components=1 -xf - public-$VERSION

go run github.com/openconfig/ygnmi/app/ygnmi@v0.12.0 generator \
  --fakeroot_name=device \
  --root_package_name=openconfig \
  --trim_module_prefix=openconfig \
  --prefer_operational_state=false \
  --split_top_level_packages=false \
  --exclude_modules=ietf-interfaces \
  --paths="$(find . -type f -name "*.yang" -exec dirname {} \; | sort -u | paste -sd, -)" \
  --base_package_path=github.com/ironcore-dev/network-operator/internal/provider/openconfig \
  ./yang/release/models/interfaces/openconfig-interfaces.yang \
  ./yang/release/models/interfaces/openconfig-if-ip.yang \
  ./yang/third_party/ietf/iana-if-type.yang

mv structs-0.go structs.go

sed 's/oc\.//g; s/oc\ "github.com\/ironcore-dev\/network-operator\/internal\/provider\/openconfig"//' openconfig/openconfig.go >path.go
rm -rf openconfig

go run golang.org/x/tools/cmd/goimports@v0.35.0 -w .
go run github.com/google/addlicense@v1.1.1 -c "SAP SE or an SAP affiliate company and IronCore contributors" -s=only -y "$(date +%Y)" .

find . -type f -name "*.go" -exec sed -i.bak '1s|// Copyright|// SPDX-FileCopyrightText:|' {} \;
find . -type f -name "*.bak" -delete
