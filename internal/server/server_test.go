package server

import (
	"testing"
)

func TestExtractSubdomain(t *testing.T) {
	tests := []struct {
		host     string
		domain   string
		expected string
	}{
		{"myapp.wirerift.dev", "wirerift.dev", "myapp"},
		{"myapp.wirerift.dev:8080", "wirerift.dev", "myapp"},
		{"test.wirerift.dev", "wirerift.dev", "test"},
		{"wirerift.dev", "wirerift.dev", ""},
		{"other.example.com", "wirerift.dev", ""},
		{"sub.sub.wirerift.dev", "wirerift.dev", "sub.sub"},
		{"", "wirerift.dev", ""},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			result := extractSubdomain(tt.host, tt.domain)
			if result != tt.expected {
				t.Errorf("extractSubdomain(%q, %q) = %q, want %q", tt.host, tt.domain, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Domain == "" {
		t.Error("Domain should not be empty")
	}
	if cfg.ControlAddr == "" {
		t.Error("ControlAddr should not be empty")
	}
	if cfg.HTTPAddr == "" {
		t.Error("HTTPAddr should not be empty")
	}
	if cfg.MaxTunnelsPerSession <= 0 {
		t.Error("MaxTunnelsPerSession should be positive")
	}
}

func TestAllocatePort(t *testing.T) {
	s := New(DefaultConfig(), nil)

	// Allocate several ports
	ports := make(map[int]bool)
	for i := 0; i < 100; i++ {
		port, err := s.allocatePort()
		if err != nil {
			t.Fatalf("allocatePort failed: %v", err)
		}
		if port < s.tcpPortStart || port > s.tcpPortEnd {
			t.Errorf("port %d out of range [%d, %d]", port, s.tcpPortStart, s.tcpPortEnd)
		}
		if ports[port] {
			t.Errorf("port %d allocated twice", port)
		}
		ports[port] = true
	}

	// Release and reallocate
	firstPort := 20000
	s.releasePort(firstPort)

	port, err := s.allocatePort()
	if err != nil {
		t.Fatalf("allocatePort after release failed: %v", err)
	}
	_ = port
}

func TestServerNew(t *testing.T) {
	cfg := DefaultConfig()
	s := New(cfg, nil)

	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.config.Domain != cfg.Domain {
		t.Errorf("Domain = %q, want %q", s.config.Domain, cfg.Domain)
	}
}

func TestServerStopWithoutStart(t *testing.T) {
	s := New(DefaultConfig(), nil)

	// Should not panic when stopping without starting
	if err := s.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}
