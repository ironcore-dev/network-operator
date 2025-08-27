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

ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-s -w -X 'main.version=${VERSION}' -X 'main.gitCommit=${GIT_COMMIT}' -X 'main.buildDate=${BUILD_DATE}'" -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot AS manager

ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION

LABEL source_repository="https://github.com/ironcore-dev/metal-operator" \
    org.opencontainers.image.url="https://github.com/ironcore-dev/network-operator" \
    org.opencontainers.image.created=${BUILD_DATE} \
    org.opencontainers.image.revision=${GIT_COMMIT} \
    org.opencontainers.image.version=${VERSION} \
    org.opencontainers.image.licenses="Apache-2.0"

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
