package server

import (
	"net"
	"testing"
	"time"
)

func TestNewTCPTunnel(t *testing.T) {
	tunnel := NewTCPTunnel("tcp-1", "tun-123", 20001)

	if tunnel.ID != "tcp-1" {
		t.Errorf("ID = %q, want %q", tunnel.ID, "tcp-1")
	}
	if tunnel.TunnelID != "tun-123" {
		t.Errorf("TunnelID = %q, want %q", tunnel.TunnelID, "tun-123")
	}
	if tunnel.Port != 20001 {
		t.Errorf("Port = %d, want 20001", tunnel.Port)
	}
	if tunnel.ConnectionCount() != 0 {
		t.Error("New tunnel should have 0 connections")
	}
}

func TestTCPTunnelAddRemoveConnection(t *testing.T) {
	tunnel := NewTCPTunnel("tcp-1", "tun-123", 20001)

	// Create a mock connection
	c1, _ := net.Pipe()
	defer c1.Close()

	tunnel.AddConnection(c1)
	if tunnel.ConnectionCount() != 1 {
		t.Errorf("ConnectionCount = %d, want 1", tunnel.ConnectionCount())
	}

	tunnel.RemoveConnection(c1.RemoteAddr().String())
	if tunnel.ConnectionCount() != 0 {
		t.Errorf("ConnectionCount = %d, want 0", tunnel.ConnectionCount())
	}
}

func TestTCPTunnelClose(t *testing.T) {
	tunnel := NewTCPTunnel("tcp-1", "tun-123", 20001)

	// Close twice should not panic
	if err := tunnel.Close(); err != nil {
		t.Errorf("First close: %v", err)
	}
	if err := tunnel.Close(); err != nil {
		t.Errorf("Second close: %v", err)
	}
}

func TestTCPTunnelClosedRejectsConnections(t *testing.T) {
	tunnel := NewTCPTunnel("tcp-1", "tun-123", 20001)
	tunnel.Close()

	c1, _ := net.Pipe()
	defer c1.Close()

	// Should not add connection when closed
	tunnel.AddConnection(c1)
	if tunnel.ConnectionCount() != 0 {
		t.Error("Should not add connection to closed tunnel")
	}
}

func TestTCPProxyNew(t *testing.T) {
	server := New(DefaultConfig(), nil)
	proxy := NewTCPProxy(server, 0, 0)

	if proxy == nil {
		t.Fatal("NewTCPProxy returned nil")
	}
	if proxy.bufferSize != 32*1024 {
		t.Errorf("bufferSize = %d, want %d", proxy.bufferSize, 32*1024)
	}
	if proxy.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want %v", proxy.timeout, 30*time.Second)
	}
}

func TestStreamOpenForTCP(t *testing.T) {
	frame, err := StreamOpenForTCP("tun-123", 42, "192.168.1.1:12345")
	if err != nil {
		t.Fatalf("StreamOpenForTCP: %v", err)
	}

	if frame.Type != 0x10 { // proto.FrameStreamOpen
		t.Errorf("Frame type = %d, want 0x10", frame.Type)
	}
	if frame.StreamID != 42 {
		t.Errorf("StreamID = %d, want 42", frame.StreamID)
	}
	if len(frame.Payload) == 0 {
		t.Error("Payload should not be empty")
	}
}
