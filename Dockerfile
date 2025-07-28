# syntax=docker/dockerfile:1
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.24-alpine3.22 AS builder

RUN apk add --no-cache --no-progress git make

ARG BININFO_BUILD_DATE
ARG BININFO_COMMIT_HASH
ARG BININFO_VERSION

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    go mod download -x

RUN --mount=type=bind,target=.,readwrite \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOTOOLCHAIN=local make install

FROM gcr.io/distroless/static:nonroot

ARG BININFO_BUILD_DATE
ARG BININFO_COMMIT_HASH
ARG BININFO_VERSION

LABEL source_repository="https://github.com/ironcore-dev/network-operator" \
    org.opencontainers.image.url="https://github.com/ironcore-dev/network-operator" \
    org.opencontainers.image.created=${BININFO_BUILD_DATE} \
    org.opencontainers.image.revision=${BININFO_COMMIT_HASH} \
    org.opencontainers.image.version=${BININFO_VERSION}

COPY --from=builder /usr/bin/network-operator /manager

USER 65532:65532
WORKDIR /
ENTRYPOINT [ "/manager" ]
