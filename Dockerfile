# syntax=docker/dockerfile:1
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.22 AS builder

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

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
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOTOOLCHAIN=local CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}" -o /usr/bin/network-operator ./cmd

FROM gcr.io/distroless/static:nonroot

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

LABEL source_repository="https://github.com/ironcore-dev/network-operator" \
    org.opencontainers.image.url="https://github.com/ironcore-dev/network-operator" \
    org.opencontainers.image.created=${BUILD_DATE} \
    org.opencontainers.image.revision=${GIT_COMMIT} \
    org.opencontainers.image.version=${VERSION} \
    org.opencontainers.image.licenses="Apache-2.0"

COPY --from=builder /usr/bin/network-operator /manager

USER 65532:65532
WORKDIR /
ENTRYPOINT [ "/manager" ]
