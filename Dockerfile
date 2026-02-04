# syntax=docker/dockerfile:1
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.22 AS builder

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

RUN --mount=type=bind,target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOTOOLCHAIN=local CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/sapcc/go-api-declarations/bininfo.binName=network-operator -X github.com/sapcc/go-api-declarations/bininfo.version=${BININFO_VERSION} -X github.com/sapcc/go-api-declarations/bininfo.commit=${BININFO_COMMIT_HASH} -X github.com/sapcc/go-api-declarations/bininfo.buildDate=${BININFO_BUILD_DATE}" -o /usr/bin/network-operator ./cmd

FROM gcr.io/distroless/static:nonroot

ARG BININFO_BUILD_DATE
ARG BININFO_COMMIT_HASH
ARG BININFO_VERSION

LABEL source_repository="https://github.com/ironcore-dev/network-operator" \
    org.opencontainers.image.url="https://github.com/ironcore-dev/network-operator" \
    org.opencontainers.image.created=${BININFO_BUILD_DATE} \
    org.opencontainers.image.revision=${BININFO_COMMIT_HASH} \
    org.opencontainers.image.version=${BININFO_VERSION} \
    org.opencontainers.image.licenses="Apache-2.0"

COPY --from=builder /usr/bin/network-operator /manager

USER 65532:65532
WORKDIR /
ENTRYPOINT [ "/manager" ]
