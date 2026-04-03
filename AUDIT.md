# WireRift Go Codebase Audit Report

**Date:** 2026-04-03
**Auditor:** Automated deep audit (Claude Opus 4.6)
**Codebase:** github.com/wirerift/wirerift — 30,065 lines Go, zero external dependencies
**Go Version:** 1.23
**Test Coverage:** 99.1% statement coverage (3 failing tests)

---

## Executive Summary

WireRift is a well-structured, zero-dependency tunnel server with impressively high test coverage (99.1%) and clean architecture. The codebase demonstrates strong Go idioms in most areas — constant-time auth comparisons, proper use of `crypto/rand`, context-based lifecycle management, and thorough edge-case testing including fuzz tests.

However, the audit identified **7 critical/high severity issues** that warrant immediate attention, primarily around: (1) an X-Forwarded-For header spoofing vulnerability enabling SSRF preconditions, (2) a replay endpoint that bypasses all tunnel access controls, (3) a TOCTOU race condition on tunnel limits, (4) several nil-dereference paths from ignored errors, and (5) three failing tests indicating incomplete feature implementations. Additionally, there are ~20 medium/low severity findings around resource management, error handling consistency, and protocol-level hardening.

The overall architecture is sound — clean package separation, proper use of `internal/`, and well-chosen abstractions (mux/stream/frame layering). The zero-dependency constraint is admirably maintained throughout.

---

## Risk Assessment

| Dimension | Score (1-10) | Notes |
|---|---|---|
| **Code Health** | 8 | Clean architecture, high coverage, good idioms |
| **Security** | 6 | Auth/crypto solid, but header injection + replay bypass |
| **Concurrency Safety** | 7 | Generally well-synchronized, 2 notable races |
| **Performance** | 7 | Good use of pools/atomics, some buffering concerns |
| **Maintainability** | 8 | Clear structure, consistent patterns, good test suite |
| **Test Coverage** | 9 | 99.1% statements, fuzz tests, integration tests |

**Overall Risk Level: MEDIUM** — No data-loss or RCE-class vulnerabilities, but the header injection and auth-bypass issues need prompt remediation for any internet-facing deployment.

---

## Metrics Summary

| Severity | Count |
|----------|-------|
| CRITICAL | 2 |
| HIGH | 5 |
| MEDIUM | 12 |
| LOW | 17 |
| **Total** | **36** |

---

## Top 10 Critical Issues

### 1. [CRITICAL] X-Forwarded-For Header Spoofing

**Category:** Security — Header Injection / SSRF Precondition
**File:** `internal/server/http_proxy.go`
**Lines:** 36–45
**Impact:** Attacker can spoof origin IP to downstream services, bypassing IP-based access controls.

**Current Code:**
```go
buf.WriteString("X-Forwarded-For: ")
if existing := r.Header.Get("X-Forwarded-For"); existing != "" {
    buf.WriteString(existing + ", " + clientIP)
} else {
    buf.WriteString(clientIP)
}
```

**Problem:** The server blindly appends to client-supplied `X-Forwarded-For`. An external attacker can set `X-Forwarded-For: 127.0.0.1` and the backend will see `127.0.0.1, <real-ip>`. Backends that trust the first/leftmost entry will believe the request came from localhost, enabling SSRF-style attacks and IP whitelist bypasses.

**Recommendation:**
```go
// Always set X-Forwarded-For to the actual client IP only (strip untrusted values)
buf.WriteString("X-Forwarded-For: ")
buf.WriteString(clientIP)
buf.WriteString("\r\n")
```

Or for proxy-chain compatibility, use a configurable trusted-proxy list.

---

### 2. [CRITICAL] ReplayRequest Bypasses All Tunnel Access Controls

**Category:** Security — Authorization Bypass
**File:** `internal/server/inspect.go`
**Lines:** 110–188
**Impact:** Dashboard users can replay requests that bypass IP whitelist, PIN protection, and Basic Auth.

**Current Code:**
```go
func (s *Server) ReplayRequest(logID string) (*RequestLog, error) {
    // ... finds original log, opens stream, forwards directly ...
    // No IP whitelist check, no PIN check, no Basic Auth check
}
```

**Problem:** `ReplayRequest` reconstructs an HTTP request from the stored log and forwards it directly through the mux stream, completely bypassing `handleHTTPRequest`'s security middleware (IP whitelist, PIN, Basic Auth). Any authenticated dashboard user can replay requests that were originally made from whitelisted IPs or with proper credentials.

**Recommendation:** Route replay requests through the full `handleHTTPRequest` pipeline, or explicitly re-check all tunnel access controls before forwarding.

---

### 3. [HIGH] TOCTOU Race on Max Tunnels Per Session

**Category:** Concurrency — Race Condition
**File:** `internal/server/server.go`
**Lines:** 518–536
**Impact:** Two concurrent tunnel requests can exceed `MaxTunnelsPerSession`.

**Current Code:**
```go
session.mu.Lock()
if len(session.Tunnels) >= s.config.MaxTunnelsPerSession {
    session.mu.Unlock()
    sendTunnelError(m, "max tunnels exceeded")
    return
}
session.mu.Unlock()
// ... lock released, then createHTTPTunnel eventually locks again to insert ...
```

**Problem:** The lock is released between the count check and the actual insertion in `createHTTPTunnel`/`createTCPTunnel`. Two concurrent requests can both pass the check and insert, exceeding the limit.

**Recommendation:** Hold the session lock across both the check and the insertion, or perform the check-and-insert atomically within a single critical section.

---

### 4. [HIGH] Nil Dereference in ReplayRequest

**Category:** Bug — Nil Pointer Dereference
**File:** `internal/server/inspect.go`
**Line:** 138
**Impact:** Server panic (crash) on corrupted request log.

**Current Code:**
```go
req, _ := http.NewRequest(original.Method, original.Path, nil)
for k, v := range original.ReqHeaders {
    req.Header.Set(k, v) // panics if req is nil
}
```

**Problem:** If `http.NewRequest` fails (e.g., invalid method from a corrupted log), `req` is nil and the next line panics. The error is discarded with `_`.

**Recommendation:**
```go
req, err := http.NewRequest(original.Method, original.Path, nil)
if err != nil {
    return nil, fmt.Errorf("create replay request: %w", err)
}
```

---

### 5. [HIGH] Three Failing Tests Indicate Missing Features

**Category:** Bug — Test Failures
**File:** `internal/server/server_test.go`
**Lines:** 4435, 4469, 4503
**Impact:** Tests for /healthz, X-Request-ID generation, and X-Request-ID preservation are failing.

**Details:**
- `TestHealthzEndpoint`: `/healthz` returns 403 "Access denied: private_ipv4_blocked" instead of 200 — the healthcheck is blocked by some IP filtering that runs before the healthz handler when using `:0` (all interfaces).
- `TestRequestIDHeader`: No `X-Request-ID` header in response — the request ID is generated and set on `r.Header` but never copied to the response when the request doesn't reach a tunnel (falls through to "Invalid host" or "Tunnel not found" error paths).
- `TestRequestIDPreserved`: Same root cause — X-Request-ID is set after the healthz check but the response header `w.Header().Set("X-Request-ID", ...)` at line 36 only runs for requests that reach the subdomain routing, not error paths.

**Root Cause:** The `/healthz` handler returns before the `X-Request-ID` header is set. Error paths (`http.Error`) also return before the request ID is written to the response.

---

### 6. [HIGH] Mutable Exported `Magic` Slice — Data Race

**Category:** Concurrency — Data Race
**File:** `internal/proto/constants.go`
**Line:** 5
**Impact:** Any code can mutate `proto.Magic`, causing data races in concurrent `ReadMagic`/`WriteMagic` calls.

**Current Code:**
```go
var Magic = []byte{0x57, 0x52, 0x46, 0x01}
```

**Problem:** Go does not have `const` byte slices. This exported `var` can be mutated by any caller. Concurrent reads in `ReadMagic` and writes from external code would be a data race.

**Recommendation:** Use a function that returns a fresh copy, or compare against a hardcoded literal in `ReadMagic`:
```go
// unexported, immutable via convention
var magic = [4]byte{0x57, 0x52, 0x46, 0x01}

func MagicBytes() []byte {
    b := magic
    return b[:]
}
```

---

### 7. [HIGH] `DecodeJSONPayload` Nil Frame Panic

**Category:** Bug — Nil Pointer Dereference
**File:** `internal/proto/messages.go`
**Line:** 136
**Impact:** Any caller passing nil frame causes panic.

**Current Code:**
```go
func DecodeJSONPayload(f *Frame, v any) error {
    return json.Unmarshal(f.Payload, v) // panics if f is nil
}
```

**Recommendation:**
```go
func DecodeJSONPayload(f *Frame, v any) error {
    if f == nil {
        return errors.New("nil frame")
    }
    return json.Unmarshal(f.Payload, v)
}
```

---

## All Findings by Category

### Security (10 issues)

| # | Severity | File | Line | Issue |
|---|----------|------|------|-------|
| S1 | CRITICAL | `server/http_proxy.go` | 36-45 | X-Forwarded-For header spoofing |
| S2 | CRITICAL | `server/inspect.go` | 110-188 | ReplayRequest bypasses auth/PIN/IP whitelist |
| S3 | MEDIUM | `server/pin.go` | 56-67 | PIN exposed in URL query parameter (logs, Referer, history) |
| S4 | MEDIUM | `server/pin.go` | 98-131 | `errorHTML` double-injected (into `<style>` + body) — latent XSS risk |
| S5 | MEDIUM | `server/pin.go` | 56-67 | Path-relative redirect could be open redirect via `//evil.com` |
| S6 | MEDIUM | `server/http_edge.go` | 31-36 | Client-supplied X-Request-ID accepted unvalidated |
| S7 | MEDIUM | `proto/constants.go` | 5 | Mutable exported `Magic` byte slice |
| S8 | LOW | `proto/messages.go` | 10, 39-42 | Auth tokens/passwords transmitted as plaintext JSON (requires TLS) |
| S9 | LOW | `proto/frame.go` | 165-169 | `ReadMagic` uses non-constant-time comparison (timing side-channel on magic bytes) |
| S10 | LOW | `config/domains.go` | 213-218 | `generateVerificationCode` generates different codes on each call (not stored immutably) |

### Concurrency & Race Conditions (7 issues)

| # | Severity | File | Line | Issue |
|---|----------|------|------|-------|
| C1 | HIGH | `server/server.go` | 518-536 | TOCTOU race on max-tunnels-per-session check |
| C2 | HIGH | `proto/constants.go` | 5 | `Magic` global mutable slice — data race |
| C3 | MEDIUM | `server/server.go` | 706-714 | TCP listener shutdown goroutine not tracked in `s.wg` (goroutine leak) |
| C4 | MEDIUM | `proto/frame.go` | 147-151 | `FrameWriter.Write` doesn't protect caller's `*Frame` from concurrent mutation |
| C5 | LOW | `server/server.go` | 1045-1059 | `cleanupInactiveSessions` double-removal race via mux close + deferred removeSession |
| C6 | LOW | `server/inspect.go` | 20-31 | `inspectResponseWriter.written` not mutex-protected (theoretical race under WebSocket hijack) |
| C7 | LOW | `mux/stream.go` | 126-168 | `Write` checks state without CAS; concurrent Close+Write could send data on half-closed stream |

### Nil/Zero Value Handling (5 issues)

| # | Severity | File | Line | Issue |
|---|----------|------|------|-------|
| N1 | HIGH | `proto/messages.go` | 136 | `DecodeJSONPayload(nil, v)` panics |
| N2 | HIGH | `server/inspect.go` | 138 | `http.NewRequest` error ignored; nil `req` dereference |
| N3 | MEDIUM | `proto/messages.go` | 121-132 | `EncodeJSONPayload` doesn't validate StreamID eagerly |
| N4 | LOW | `proto/messages.go` | 23-25, 48, 89 | `HeartbeatInterval`, `RemotePort`, `ReconnectAfter` are `int` not `uint`; negative values accepted |
| N5 | LOW | `proto/messages.go` | 117 | `ParseHeartbeat` casts `uint64` to `int64` — sign flip on large values |

### Error Handling (8 issues)

| # | Severity | File | Line | Issue |
|---|----------|------|------|-------|
| E1 | HIGH | `server/server_test.go` | 4435+ | 3 failing tests: TestHealthzEndpoint, TestRequestIDHeader, TestRequestIDPreserved |
| E2 | MEDIUM | Multiple server files | — | ~15 instances of `EncodeJSONPayload` error discarded with `_`; nil frame passed to Write |
| E3 | MEDIUM | `server/inspect.go` | 156-159 | ReplayRequest discards errors from SerializeRequest, Write, ReadAll |
| E4 | MEDIUM | `server/server.go` | 231-245 | Partial Start() failure leaves control listener open (resource leak) |
| E5 | LOW | `proto/fuzz_test.go` | 19, 71-82 | Fuzz test corpus seeding ignores Encode errors |
| E6 | LOW | `proto/messages_test.go` | 236, 243 | `TestFullProtocolRoundTrip` ignores EncodeJSONPayload error; nil frame → panic |
| E7 | LOW | `server/server.go` | 349-350 | `generateSubdomain` ignores `rand.Read` error |
| E8 | LOW | `ratelimit/ratelimit.go` | 64-72 | `WaitN` uses busy-wait with `time.Sleep(10ms)` — should use time-based reservation |

### Resource Management (6 issues)

| # | Severity | File | Line | Issue |
|---|----------|------|------|-------|
| R1 | MEDIUM | `server/http_edge.go` | 156 | 64 MB response fully buffered in memory before streaming |
| R2 | MEDIUM | `server/http_edge.go` | 221-240 | WebSocket tunnels have no idle timeout (goroutine leak on idle connections) |
| R3 | MEDIUM | `server/http_proxy.go` | 62 | 32 MB request body fully buffered in memory |
| R4 | LOW | `server/server.go` | 706-730 | TCP listener `Close()` called twice (defer + goroutine) — harmless but redundant |
| R5 | LOW | `server/inspect.go` | 80-86 | Request log slice grows/compacts on every overflow — could use ring buffer |
| R6 | LOW | `mux/ringbuffer.go` | 36-66 | Ring buffer Write is byte-by-byte copy; `copy()` would be significantly faster |

### Performance (5 issues)

| # | Severity | File | Line | Issue |
|---|----------|------|------|-------|
| P1 | MEDIUM | `server/inspect.go` | 192-202 | `getTunnelByID` does O(N) full scan of all tunnels |
| P2 | LOW | `proto/messages.go` | 102-105, 113-116 | Manual bit-shifting instead of `binary.BigEndian.PutUint64/Uint64` |
| P3 | LOW | `proto/frame_test.go` | 270 | Benchmark allocates `bytes.Buffer` inside loop (inaccurate measurement) |
| P4 | LOW | `server/server.go` | 858-873 | Port allocator `int32` wraps after 2^31 calls (theoretical) |
| P5 | LOW | `auth/auth.go` | 107-122 | `ValidateToken` iterates all tokens with `Range` — O(N) per auth request |

### Code Quality (5 issues)

| # | Severity | File | Line | Issue |
|---|----------|------|------|-------|
| Q1 | LOW | `proto/frame.go` | 121-133 | `FrameReader` is a trivial wrapper adding no value over `ReadFrame()` |
| Q2 | LOW | `proto/frame_test.go` | 83-117 | `wantErr` field declared but never set to `true` in TestFrameEncodeDecode |
| Q3 | LOW | `server/server_test.go` | 557-562 | Incorrect assertion uses `t.Logf` instead of `t.Errorf` — masks regression |
| Q4 | LOW | `server/inspect.go` | 178 | Replayed request always stores `Duration: 0` |
| Q5 | LOW | `server/pin.go` | 98-131 | PIN page `fmt.Fprintf` uses `%%` escaping + `%s` mixing — fragile template |

---

## Architecture & Design Assessment

### Strengths

1. **Zero-dependency constraint** — Admirably maintained. All crypto, HTTP, TLS, and protocol handling uses stdlib only. This eliminates supply-chain risk entirely.

2. **Clean package layering** — `proto` → `mux` → `server`/`client` is well-designed. No circular dependencies. Proper use of `internal/` for all packages.

3. **Protocol design** — The binary frame protocol is compact (9-byte header), well-specified (magic bytes, version, type system), and includes flow control (window-based backpressure). Frame sizes are bounded (16 MB max).

4. **Concurrency patterns** — Good use of `sync.Map` for hot-path lookups, `atomic` operations for counters/state, `sync.Pool` for header buffers, and `context.Context` for lifecycle management.

5. **Auth security** — Constant-time comparisons throughout (`crypto/subtle`), `crypto/rand` for all random generation, rejection sampling for unbiased random strings, HMAC-based PIN cookies.

6. **HTTP server hardening** — Proper timeouts configured (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`), `MaxHeaderBytes` set, graceful shutdown with context.

7. **Test quality** — 99.1% coverage, table-driven tests, fuzz tests for protocol parsing, integration tests with real TCP connections, benchmark tests.

### Areas for Improvement

1. **Streaming responses** — Both request serialization (32 MB buffer) and response handling (64 MB buffer) load everything into memory. For a tunnel product, streaming with chunked transfer would be more appropriate.

2. **Secondary indexes** — Tunnels are keyed by subdomain/port but `getTunnelByID` requires O(N) scan. A secondary `tunnelsByID sync.Map` would fix this.

3. **Dashboard CSRF** — Cookie-based auth is restricted to GET requests only (good), but the dashboard should set `SameSite=Strict` on session cookies and add CSRF tokens for state-changing operations.

4. **Healthcheck path** — The `/healthz` endpoint should be unconditionally accessible (before any filtering), which is the intent but currently fails when the server binds to all interfaces.

---

## Recommended Action Plan

### Phase 1 — Immediate (Security Fixes)

1. **Fix X-Forwarded-For spoofing** (`http_proxy.go:36-45`) — Strip or replace untrusted XFF headers
2. **Add auth checks to ReplayRequest** (`inspect.go:110-188`) — Re-verify IP whitelist, PIN, Basic Auth
3. **Fix nil dereference in ReplayRequest** (`inspect.go:138`) — Check `http.NewRequest` error
4. **Fix nil guard in DecodeJSONPayload** (`messages.go:136`) — Add nil frame check
5. **Fix 3 failing tests** — Ensure /healthz and X-Request-ID work on all bind addresses

### Phase 2 — Soon (Concurrency & Resource)

6. **Fix TOCTOU on tunnel count** (`server.go:518-536`) — Hold lock across check+insert
7. **Make Magic immutable** (`constants.go:5`) — Use unexported `[4]byte` or function
8. **Track TCP listener goroutine** (`server.go:706-714`) — Add to `s.wg`
9. **Fix partial Start() cleanup** (`server.go:231-245`) — Close control listener on HTTP failure
10. **Add WebSocket idle timeout** — Set deadline on hijacked connections

### Phase 3 — Planned (Quality & Performance)

11. **Stream responses instead of buffering** — Replace `io.ReadAll` with streaming copy
12. **Add tunnel ID secondary index** — `sync.Map` keyed by tunnel ID
13. **Fix PIN page template** — Separate CSS placeholder from error HTML injection
14. **Replace busy-wait in rate limiter** — Use timer-based reservation
15. **Optimize ring buffer** — Use `copy()` instead of byte-by-byte loop

---

## Static Analysis Results

| Tool | Result |
|------|--------|
| `go build ./cmd/... ./internal/...` | PASS — clean compilation |
| `go vet ./cmd/... ./internal/...` | PASS — no vet issues |
| `go test -race` | FAIL — 3 test failures (TestHealthzEndpoint, TestRequestIDHeader, TestRequestIDPreserved); no race conditions detected |
| `go test -cover` | 99.1% statement coverage |
| `go.mod` | Zero external dependencies, Go 1.23 |

---

## Dependency Analysis

**Module:** `github.com/wirerift/wirerift`
**Go Version:** 1.23
**External Dependencies:** None (zero entries in `go.mod` beyond module declaration)
**CGO:** Not required (`CGO_ENABLED=0` compatible)

This is the ideal state — zero supply-chain risk, fully reproducible builds, maximum portability. The zero-dependency constraint is consistently maintained across all packages.

---

## Conclusion

WireRift is a high-quality Go codebase with excellent test coverage and clean architecture. The critical security issues (XFF spoofing, replay auth bypass) are straightforward to fix. The TOCTOU race on tunnel limits is a concurrency design issue that needs an atomic check-and-insert pattern. The failing tests indicate recently added features that are not yet fully implemented. After addressing the Phase 1 and Phase 2 items, this codebase would be suitable for production internet-facing deployment.
