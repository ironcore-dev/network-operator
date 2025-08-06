# Fake GNMI Test Server

This is a fake GNMI server that can be used to test GNMI clients.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)

## Build

All the commands below should be executed in the directory containing this `README.md` file.

Build the fake GNMI server:

```sh
docker build -t ghcr.io/ironcore-dev/gnmi-test-server .
```

## Run

Run the fake GNMI server:

```sh
docker run -d -p 9339:9339 ghcr.io/ironcore-dev/gnmi-test-server
```

Now, it's possible to connect to the server using a GNMI client such as [gnmic](https://gnmic.openconfig.net) on `127.0.0.1:9339`.

```sh
λ gnmic -a 127.0.0.1 --port 9339  --insecure get --path /System/name
[
  {
    "source": "127.0.0.1",
    "timestamp": 1753363982688366597,
    "time": "2025-07-24T15:33:02.688366597+02:00",
    "updates": [
      {
        "Path": "System/name",
        "values": {
          "System/name": null
        }
      }
    ]
  }
]

λ gnmic -a 127.0.0.1 --port 9339  --insecure set --update-path /System/name --update-value "leaf1"
{
  "source": "127.0.0.1",
  "timestamp": 1753364001109266411,
  "time": "2025-07-24T15:33:21.109266411+02:00",
  "results": [
    {
      "operation": "UPDATE",
      "path": "System/name"
    }
  ]
}

λ gnmic -a 127.0.0.1 --port 9339  --insecure get --path /System/name
[
  {
    "source": "127.0.0.1",
    "timestamp": 1753364003723688653,
    "time": "2025-07-24T15:33:23.723688653+02:00",
    "updates": [
      {
        "Path": "System/name",
        "values": {
          "System/name": "leaf1"
        }
      }
    ]
  }
]
```

## HTTP API

In addition to the GNMI gRPC interface, the server also provides an HTTP API for convenient state management and inspection.

### HTTP Server Configuration

The HTTP server runs on port 8000 by default and can be configured using the `--http-port` flag:

```sh
docker run -d -p 9339:9339 -p 8000:8000 ghcr.io/ironcore-dev/gnmi-test-server --http-port 8000
```

### Available Endpoints

#### GET /v1/state

Retrieves the current state of the GNMI server as compacted JSON (no whitespace/indentation).

**Example:**
```sh
# Get the current state
curl -s http://127.0.0.1:8000/v1/state

# Response when state is empty:
{}

# Response after setting some values via GNMI (compacted):
{"System":{"name":"leaf1"}}
```

#### DELETE /v1/state

Clears all state from the GNMI server, resetting it to an empty state.

**Example:**
```sh
# Clear all state
curl -X DELETE http://127.0.0.1:8000/v1/state

# Returns HTTP 204 No Content on success
```

### Usage Examples

1. **Inspect state after GNMI operations:**
   ```sh
   # Set a value via GNMI
   gnmic -a 127.0.0.1 --port 9339 --insecure set --update-path /System/name --update-value "leaf1"
   
   # Check the state via HTTP
   curl http://127.0.0.1:8000/v1/state
   ```

2. **Reset state for testing:**
   ```sh
   # Clear all state
   curl -X DELETE http://127.0.0.1:8000/v1/state
   
   # Verify state is empty
   curl http://127.0.0.1:8000/v1/state
   ```

The HTTP API is particularly useful for:
- Debugging and inspecting the current state
- Automated testing scenarios where you need to reset state between tests
- Integration with monitoring tools that can consume JSON over HTTP
