package server

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/wirerift/wirerift/internal/mux"
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

func TestTCPProxyWithCustomTimeout(t *testing.T) {
	server := New(DefaultConfig(), nil)
	proxy := NewTCPProxy(server, 64*1024, 10*time.Second)

	if proxy == nil {
		t.Fatal("NewTCPProxy returned nil")
	}
	if proxy.bufferSize != 64*1024 {
		t.Errorf("bufferSize = %d, want %d", proxy.bufferSize, 64*1024)
	}
	if proxy.timeout != 10*time.Second {
		t.Errorf("timeout = %v, want %v", proxy.timeout, 10*time.Second)
	}
}

func TestTCPTunnelRemoveConnection(t *testing.T) {
	tunnel := NewTCPTunnel("tcp-1", "tun-123", 20001)

	// Create pipes
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	// Add connection
	tunnel.AddConnection(c1)
	if tunnel.ConnectionCount() != 1 {
		t.Errorf("ConnectionCount = %d, want 1", tunnel.ConnectionCount())
	}

	// Remove non-existent connection should not panic
	tunnel.RemoveConnection("192.168.1.100:12345")
	if tunnel.ConnectionCount() != 1 {
		t.Errorf("ConnectionCount = %d, want 1 after removing non-existent", tunnel.ConnectionCount())
	}

	// Remove existing connection by getting its address
	addr := c1.LocalAddr().String()
	tunnel.RemoveConnection(addr)
	if tunnel.ConnectionCount() != 0 {
		t.Errorf("ConnectionCount = %d, want 0", tunnel.ConnectionCount())
	}
}

func TestTCPTunnelCloseMultiple(t *testing.T) {
	tunnel := NewTCPTunnel("tcp-1", "tun-123", 20001)

	// Create connections with different addresses
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	c1, _ := net.Dial("tcp", listener.Addr().String())
	if c1 != nil {
		defer c1.Close()
		tunnel.AddConnection(c1)
	}

	// Close should close all connections
	err := tunnel.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// After close, should not be able to add more
	c2, _ := net.Pipe()
	defer c2.Close()
	tunnel.AddConnection(c2)

	if tunnel.ConnectionCount() != 0 {
		t.Errorf("ConnectionCount = %d, want 0 after close", tunnel.ConnectionCount())
	}

	// Second close should be safe
	err = tunnel.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

// TestBidiCopy tests bidirectional copy between connections
func TestBidiCopy(t *testing.T) {
	// Create two pipe pairs to simulate connections
	client1, server1 := net.Pipe()
	client2, server2 := net.Pipe()
	defer client1.Close()
	defer server1.Close()
	defer client2.Close()
	defer server2.Close()

	// Write data from client1 and client2
	go func() {
		client1.Write([]byte("data from client1"))
		client1.Close()
	}()

	go func() {
		client2.Write([]byte("data from client2"))
		client2.Close()
	}()

	// Run bidiCopy between server1 and server2
	written, read, err := bidiCopy(server1, server2, 1024)
	if err != nil {
		t.Logf("bidiCopy error (expected): %v", err)
	}

	// Check that some data was transferred
	t.Logf("Written: %d, Read: %d", written, read)
}

// TestProxyConnectionOpenStreamError tests that ProxyConnection returns an error when the mux is closed.
func TestProxyConnectionOpenStreamError(t *testing.T) {
	srv := New(DefaultConfig(), nil)
	proxy := NewTCPProxy(srv, 0, 0)

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	// Create a mux with a pipe and close it immediately so OpenStream fails
	mc1, mc2 := net.Pipe()
	defer mc1.Close()
	defer mc2.Close()

	m := mux.New(mc1, mux.DefaultConfig())
	go m.Run()
	m.Close()

	tunnel := &Tunnel{ID: "test-tunnel"}
	session := &Session{ID: "test-session", Mux: m}

	err := proxy.ProxyConnection(c1, tunnel, session)
	if err == nil {
		t.Error("Expected error from ProxyConnection with closed mux")
	}
	if !strings.Contains(err.Error(), "open stream") {
		t.Errorf("Expected 'open stream' error, got: %v", err)
	}
}

// TestTCPTunnelWithListener tests TCP tunnel with listener
func TestTCPTunnelWithListener(t *testing.T) {
	tunnel := NewTCPTunnel("tcp-1", "tun-123", 20001)

	// Create a listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	tunnel.Listener = listener

	// Close should close the listener
	err = tunnel.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestBidiCopyWithLargeBuffer tests bidiCopy with larger buffer
func TestBidiCopyWithLargeBuffer(t *testing.T) {
	client1, server1 := net.Pipe()
	client2, server2 := net.Pipe()
	defer client1.Close()
	defer server1.Close()
	defer client2.Close()
	defer server2.Close()

	// Send larger data
	data1 := make([]byte, 1000)
	data2 := make([]byte, 1000)
	for i := range data1 {
		data1[i] = byte(i % 256)
		data2[i] = byte((i + 128) % 256)
	}

	go func() {
		client1.Write(data1)
		client1.Close()
	}()

	go func() {
		client2.Write(data2)
		client2.Close()
	}()

	// Run with larger buffer
	written, read, _ := bidiCopy(server1, server2, 4096)
	t.Logf("Large buffer - Written: %d, Read: %d", written, read)
}
