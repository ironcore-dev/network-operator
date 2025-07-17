# Fake GNMI Test Server

This is a fake GNMI server that can be used to test GNMI clients.

Server Configuration can be found [`config.pb.txt`](./config.pb.txt), bootstrapped from [gen_fake_config](https://github.com/openconfig/gnmi/tree/master/testing/fake/gnmi/cmd/gen_fake_config).

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [OpenSSL](https://www.openssl.org/)

## Build

All the commands below should be executed in the directory containing this `README.md` file.

Generate a x509 certificate and private key for the server:

```sh
openssl req -x509 -sha256 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/O=SAP SE/OU=GCID PlusOne/CN=localhost" -addext "subjectAltName = DNS:localhost"
```

Build the fake GNMI server:

```sh
docker build -t ghcr.io/ironcore-dev/gnmi-test-server .
```

## Run

Run the fake GNMI server:

```sh
docker run -d -p 9339:9339 -v $(pwd)/config.pb.txt:/config.pb.txt -v $(pwd)/key.pem:/key.pem -v $(pwd)/cert.pem:/cert.pem ghcr.io/ironcore-dev/gnmi-test-server
```
