# WireRift

[![Go Report Card](https://goreportcard.com/badge/github.com/wirerift/wirerift)](https://goreportcard.com/report/github.com/wirerift/wirerift)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Tear a rift through the wire.** Expose localhost to the world.

Open-source, zero-dependency tunnel server and client. Written in Go.

## Features

- **Zero dependencies** — uses only Go standard library
- **Single binary** — no runtime dependencies
- **Self-hosted** — run your own tunnel server
- **HTTP & TCP tunnels** — expose web services or raw TCP
- **Auto TLS** — automatic HTTPS with Let's Encrypt
- **WebSocket support** — real-time applications work out of the box
- **Custom domains** — use your own domain names
- **Traffic inspection** — built-in dashboard for debugging
- **Prometheus metrics** — observability out of the box

## Quick Start

### Install

```bash
go install github.com/wirerift/wirerift/cmd/wirerift@latest
```

### Expose a local HTTP server

```bash
# Start your local app on port 3000
wirerift http 3000
# → https://random-subdomain.wirerift.dev
```

### Expose a TCP service

```bash
# Expose PostgreSQL
wirerift tcp 5432
# → tcp://wirerift.dev:20001
```

## Server Setup

```bash
# Run the server
wirerift-server start --domain wirerift.dev --http 80 --https 443
```

## Building

```bash
make build
```

## License

MIT License — see [LICENSE](LICENSE)
