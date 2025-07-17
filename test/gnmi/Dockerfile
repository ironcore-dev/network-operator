# syntax=docker/dockerfile:1
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

ARG GO_VERSION=1.24
ARG ALPINE_VERSION=3.22

FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build

ARG TARGETOS
ARG TARGETARCH

# Clone the openconfig/gnmi repository containing the fake server
RUN apk add --no-cache --no-progress git && \
    git clone --depth=1 --filter=blob:none https://github.com/openconfig/gnmi.git /go/src/github.com/openconfig/gnmi && \
    apk del --no-cache --no-progress apk-tools alpine-keys alpine-release libc-utils

# Switch into workspace
WORKDIR /go/src/github.com/openconfig/gnmi

# Install dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x && \
    go mod tidy

# Build the application into a static executable while removing the symbol table and debugging information
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o ./build/server ./testing/fake/gnmi/cmd/fake_server

FROM alpine:${ALPINE_VERSION}

# Switch into workspace
WORKDIR /app

# Copy executable from build
COPY --from=build /go/src/github.com/openconfig/gnmi/build/server  /app/

# Start the server application
CMD ["/app/server", "--config=/config.pb.txt", "--text", "--port=9339", "--server_crt=/cert.pem", "--server_key=/key.pem", "--allow_no_client_auth", "--logtostderr"]
