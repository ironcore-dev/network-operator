# Lab Test Suite

This module contains lab tests, that verify the correct behaviour of the `network-operator` against (potentially real) network devices. The style of these tests is inspired by the [Russ Cox - Go Testing By Example | GopherConAU 2023 Talk](https://youtu.be/1-o-iJlL4ak?t=1471). It is highly recommended to watch this talk in order to familiarize some of the concepts, including the [txtar](https://pkg.go.dev/golang.org/x/tools/txtar) file format and [rsc.io/script](https://pkg.go.dev/rsc.io/script) engine to create mini testing-languages.

All test cases are described as individual `.txt` files in the [testdata](./testdata) directory.

## Prerequisites

Before running the tests, ensure:

1. You have access to a Kubernetes cluster with the network-operator deployed
2. The target network device is accessible via SSH with the provided credentials
3. The network device supports the gNMI protocol on port 9339

## Run

Execute these tests by running the following command with the necessary endpoint credentials for the target device:

```bash
ADDR=192.168.0.1 USER=admin PASS=password go test
```

### Required Environment Variables

The following environment variables must be set before running the tests:

- `ADDR`: IP address or hostname of the target network device
- `USER`: Username for authentication to the device
- `PASS`: Password for authentication to the device

### Additional Options

- **Verbose Output**: Add the `-v` flag to get debug output from executed commands:

  ```bash
  ADDR=192.168.0.1 USER=admin PASS=password go test -v
  ```

- **Custom Kubernetes Context**: Specify a different Kubernetes context (defaults to current context):

  ```bash
  ADDR=192.168.0.1 USER=admin PASS=password KUBECONTEXT=my-cluster go test
  ```

## Test Execution Flow

The test suite follows this execution flow:

1. **Environment Setup**: Reads required environment variables and validates configuration
2. **SSH Connection**: Establishes SSH connection to the target network device
3. **Kubernetes Setup**: Configures Kubernetes client and creates necessary resources (`Secret` for credentials + `Device` with endpoint configuration)
4. **Test Execution**: Runs all test cases from the `testdata/*.txt` files using the script engine
5. **Cleanup**: Automatically cleans up created Kubernetes resources after test completion

## Custom Commands

The test suite provides two custom commands that extend the standard script testing capabilities:

### `apply` Command

The `apply` command applies a Kubernetes manifest to the cluster and automatically waits for the resource to transition into a `Ready` state.

**Usage:** `apply <file>`

**Behavior:**

- Reads and applies the specified YAML manifest file to the Kubernetes cluster
- Automatically sets the namespace to `default` and adds the device label `networking.metal.ironcore.dev/device: device`
- Waits for the resource to reach a Ready state (checking for a `Ready` condition with status `True`)
- Times out after 10 seconds if the resource doesn't become ready (with a polling rate of 1s)

This behavior is similar to using `kubectl wait` with a readiness condition:

```bash
kubectl wait --for=condition=Ready <resource-type>/<resource-name> --timeout=10s
```

### `vty` Command

The `vty` command executes commands on the target network device through an SSH virtual terminal session.

**Usage:** `vty <command> [args...]`

**Behavior:**

- Establishes an SSH session to the configured network device
- Executes the specified command with any provided arguments
- Returns the command output (stdout/stderr) for verification in tests
- Automatically handles session cleanup after command execution

## Deployment

The lab tests can be compiled into a test binary and packaged as a Docker container for deployment into a Kubernetes cluster where the network-operator is running.

### Docker Container

The tests are containerized using the [`Dockerfile`](./Dockerfile) which:

1. **Builds the test binary**: Compiles the Go tests into a standalone executable (`net.test`) using `go test -c`
2. **Creates minimal runtime image**: Uses Alpine Linux as the base image for a lightweight container
3. **Includes test data**: Copies the `testdata/` directory containing all test cases
4. **Sets default command**: Configures the container to run the tests with verbose output and a 30-second timeout

### Kubernetes Deployment

The containerized tests can be deployed to a Kubernetes cluster using the provided Job manifest [`deploy/job.yaml`](./deploy/job.yaml), which includes:

- **Job**: Runs the test container as a one-time execution
- **ServiceAccount**: Provides the necessary identity for the test pod
- **ClusterRole**: Grants permissions to create/delete Secrets and network-operator resources (Devices, Interfaces, etc.)
- **ClusterRoleBinding**: Associates the ServiceAccount with the required permissions

To deploy the tests:

```bash
kubectl apply -f deploy/job.yaml
```

The Job is configured with environment variables for the target device credentials (`ADDR`, `USER`, `PASS`) and will automatically clean up after completion. Check the logs to view test results:

```bash
kubectl logs job/network-operator-test
```
