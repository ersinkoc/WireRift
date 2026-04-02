# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.6.0] - 2025-04-02

### Added
- **Persistent Tunnels**: Tunnels now survive server restarts with automatic reconnection
- **Dashboard Session Heartbeat**: Real-time session status with uptime and last seen tracking
- **Comprehensive Test Coverage**: 95-100% test coverage across all packages
- **Additional CLI Tests**: Complete coverage for edge cases in argument parsing

### Fixed
- **Defensive Crypto**: Added proper error checks for crypto/rand operations
- **Type Safety**: Safe type assertions throughout the codebase
- **Benchmark Stability**: Fixed thread exhaustion on Windows with shared HTTP client pool
- **Webhook Removal**: Cleaned up unused webhook functionality

### Security
- Improved error handling in cryptographic operations
- Safe type assertions to prevent panics

## [1.5.0] - Previous Release

### Added
- Let's Encrypt ACME support with HTTP-01 challenge
- Self-signed TLS certificate generation
- Web Dashboard with traffic inspection
- Request replay functionality
- cURL export for captured requests
- Rate limiting support
- IP whitelist (CIDR) protection
- Basic Auth for tunnels
- PIN protection for tunnel access

### Features
- HTTP and TCP tunneling
- WebSocket passthrough
- Stream multiplexing with flow control
- Graceful shutdown
- Auto-reconnect
- Health check endpoint (`/healthz`)
- X-Request-ID tracing
- Dark/light theme for dashboard

## [1.4.0] - [1.4.3]

### Added
- Initial release with core tunneling functionality
- Multiplexing protocol
- TLS support
- Docker support
- Configuration via CLI, env vars, and files

---

For older releases, see [GitHub Releases](https://github.com/WireRift/WireRift/releases).
