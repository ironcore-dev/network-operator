# syntax=docker/dockerfile:1
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

ARG GO_VERSION=1.24
ARG ALPINE_VERSION=3.22

FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build

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
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go test -c -o /usr/bin/net.test ./test/lab

FROM alpine:${ALPINE_VERSION}

RUN addgroup -g 65532 -S nonroot && \
    adduser -u 65532 -S nonroot -G nonroot

WORKDIR /

COPY --from=build --chown=nonroot:nonroot /usr/bin/net.test  /
COPY --chown=nonroot:nonroot ./test/lab/testdata/ /testdata/

USER nonroot

CMD ["/net.test", "-test.v", "-test.timeout=30s", "-test.run=TestAll"]
