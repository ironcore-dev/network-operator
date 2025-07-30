# syntax=docker/dockerfile:1
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

ARG GO_VERSION=1.24
ARG ALPINE_VERSION=3.22

FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Install dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    go mod download -x

# Build the application into a static executable while removing the symbol table and debugging information
RUN --mount=type=bind,target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /usr/bin/server ./main.go

FROM alpine:${ALPINE_VERSION}

# Switch into workspace
WORKDIR /

# Copy executable from build
COPY --from=build /usr/bin/server  /

# Start the server application
CMD ["/server", "--port=9339"]
