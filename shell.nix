# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company
# SPDX-License-Identifier: Apache-2.0

{ pkgs ? import <nixpkgs> { } }:

with pkgs;

mkShell {
  nativeBuildInputs = [
    addlicense
    go-licence-detector
    go_1_25
    golangci-lint
    goreleaser
    gotools # goimports
    kubernetes-controller-tools # controller-gen
    renovate
    setup-envtest
    syft
    typos
    # keep this line if you use bash
    bashInteractive
  ];
}
