package client

import (
	"testing"

	"github.com/wirerift/wirerift/internal/proto"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ServerAddr == "" {
		t.Error("ServerAddr should not be empty")
	}
	if cfg.ReconnectInterval <= 0 {
		t.Error("ReconnectInterval should be positive")
	}
	if cfg.HeartbeatInterval <= 0 {
		t.Error("HeartbeatInterval should be positive")
	}
}

func TestClientNew(t *testing.T) {
	cfg := DefaultConfig()
	c := New(cfg, nil)

	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.config.ServerAddr != cfg.ServerAddr {
		t.Errorf("ServerAddr = %q, want %q", c.config.ServerAddr, cfg.ServerAddr)
	}
}

func TestClientCloseWithoutConnect(t *testing.T) {
	c := New(DefaultConfig(), nil)

	// Should not panic when closing without connecting
	if err := c.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestHTTPOptions(t *testing.T) {
	req := &proto.TunnelRequest{
		Type:      proto.TunnelTypeHTTP,
		LocalAddr: "localhost:3000",
	}

	// Apply options
	WithSubdomain("myapp")(req)
	WithInspect()(req)
	WithAuth("user", "pass")(req)
	WithHeaders(map[string]string{"X-Custom": "value"})(req)

	if req.Subdomain != "myapp" {
		t.Errorf("Subdomain = %q, want %q", req.Subdomain, "myapp")
	}
	if !req.Inspect {
		t.Error("Inspect should be true")
	}
	if req.Auth == nil {
		t.Fatal("Auth should not be nil")
	}
	if req.Auth.Username != "user" {
		t.Errorf("Username = %q, want %q", req.Auth.Username, "user")
	}
	if req.Headers["X-Custom"] != "value" {
		t.Errorf("Headers[X-Custom] = %q, want %q", req.Headers["X-Custom"], "value")
	}
}

func TestIsConnected(t *testing.T) {
	c := New(DefaultConfig(), nil)

	if c.IsConnected() {
		t.Error("Should not be connected initially")
	}
}
