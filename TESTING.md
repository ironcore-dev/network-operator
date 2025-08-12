# Testing Guide

This guide covers how the testing framework used for the `network-operator` project.

## Test Types

The project uses three types of tests:

1. **Unit Tests** - Fast feedback with mocked external dependencies (run on every PR via GitHub Actions)
2. **E2E Tests** - Full system tests in a Kind cluster against mock gNMI server (run on every PR via GitHub Actions)
3. **Lab Tests** - Real device tests using Containerlab for virtualizing devices (run before releases in internal environment)

## Unit Tests

### Prerequisites

- Go 1.24+
- Make

### Running Unit Tests

```sh
# Run all unit tests and collect coverage information
make build/cover.out

# Display coverage information in Browser
open build/cover.html # Use xdg-open on Linux
```

### What Gets Tested

- Kubernetes controller reconciliation logic
- Provider functionality with mocked clients
- Resource status management and cleanup

**Important Note**: When creating tests for [controllers](./internal/controller), use the stub provider implementation located in [internal/controller/suite_test.go](./internal/controller/suite_test.go). This approach ensures controller logic is tested in isolation. Provider implementations should be tested separately within their own packages, with additional validation provided by E2E and Lab Tests to verify integration.

## E2E Tests

### Prerequisites

- Docker
- Kind
- Make

### Running E2E Tests

```sh
# Create test cluster
make kind-create

# Run E2E tests
make test-e2e

# Cleanup (optional)
make kind-delete
```

### What Gets Tested

- Operator deployment in Kubernetes
- CRD installation and validation
- Resource reconciliation with mock gNMI server
- Error recovery scenarios

**Target Outcome**: Validates the operator works correctly in a real Kubernetes environment without requiring actual network devices.

E2E Tests make use of the fake gNMI test server located in [`./test/gnmi/`](./test/gnmi), that simulates a real gNMI server for E2E testing without hardware dependencies.

## Lab Tests

### Prerequisites

- Access to a network device (real or virtual) that supports gNMI
- Kubernetes cluster with the `network-operator` deployed
- Docker image with packaged lab tests

> [!NOTE]
> For our purposes, we use the [lab-operator](https://github.com/cobaltcore-dev/lab-operator) to setup environments provided by [Containerlab](https://containerlab.dev). This allows us to run tests against virtualized network devices in a controlled environment.

### Running Lab Tests

```sh
# Build and push the lab test image
docker build --file=test/lab/Dockerfile --tag=<tag> --push .

# Deploy lab test job (adjust container image!)
kubectl apply -f ./test/lab/deploy/job.yaml

# Check test results
kubectl logs job/network-operator-lab-test
```

### What Gets Tested

- Real network device interactions
- Provider compatibility with actual network hardware
- Resource reconciliation against live device state

**Target Outcome**: Ensures the operator works with real/virtual network devices and validates provider implementations against the running-config of the network device.

Please refer to the [lab test documentation](./test/lab/README.md) for more details on the lab tests.
