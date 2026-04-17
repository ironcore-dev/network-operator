#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0
set -o errexit
set -o nounset
set -o pipefail

CONTAINER_TOOL="${CONTAINER_TOOL:-docker}"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kind}"
export KIND_EXPERIMENTAL_PROVIDER="${CONTAINER_TOOL}"

# Exit early if the cluster already exists
if kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
  echo "Cluster ${KIND_CLUSTER_NAME} already exists"
  exit 0
fi

# 1. Create registry container unless it already exists
REGISTRY_NAME='kind-registry'
REGISTRY_PORT="${KIND_REGISTRY_PORT:-5000}"
if [ "$("${CONTAINER_TOOL}" inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)" != 'true' ]; then
  "${CONTAINER_TOOL}" run -d --restart=always -p "127.0.0.1:${REGISTRY_PORT}:5000" --network bridge --name "${REGISTRY_NAME}" registry:3
fi

# 2. Create kind cluster
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${KIND_CLUSTER_NAME}
nodes:
- role: control-plane
  labels:
    topology.kubernetes.io/zone: eu-de-1a
EOF

# 3. Add the registry config to the nodes
#
# This is necessary because localhost resolves to loopback addresses that are
# network-namespace local.
# In other words: localhost in the container is not localhost on the host.
#
# We want a consistent name that works from both ends, so we tell containerd to
# alias localhost:${reg_port} to the registry container when pulling images
REGISTRY_DIR="/etc/containerd/certs.d/localhost:${REGISTRY_PORT}"
for node in $(kind get nodes); do
  "${CONTAINER_TOOL}" exec "${node}" mkdir -p "${REGISTRY_DIR}"
  cat <<EOF | "${CONTAINER_TOOL}" exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://${REGISTRY_NAME}:5000"]
EOF
done

# 4. Connect the registry to the cluster network if not already connected
# This allows kind to bootstrap the network but ensures they're on the same network
if [ "$("${CONTAINER_TOOL}" inspect -f='{{json .NetworkSettings.Networks.kind}}' "${REGISTRY_NAME}")" = 'null' ]; then
  "${CONTAINER_TOOL}" network connect "kind" "${REGISTRY_NAME}"
fi

# 5. Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REGISTRY_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

kubectl wait --for=condition=Ready --timeout=300s "node/${KIND_CLUSTER_NAME}-control-plane"
