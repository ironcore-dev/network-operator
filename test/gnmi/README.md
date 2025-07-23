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
