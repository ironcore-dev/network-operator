# syntax=docker/dockerfile:1
# SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

FROM node:25-alpine3.22

LABEL source_repository="https://github.com/ironcore-dev/network-operator" \
    org.opencontainers.image.url="https://github.com/ironcore-dev/network-operator/tree/main/docs"

WORKDIR /workspace

COPY package.json package-lock.json ./

RUN --mount=type=cache,target=/root/.npm \
    npm ci

EXPOSE 5173

CMD ["npm", "run", "docs:dev", "--", "--host=0.0.0.0"]
