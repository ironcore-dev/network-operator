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
