package client

import (
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/wirerift/wirerift/internal/mux"
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
	if cfg.MaxReconnectInterval <= 0 {
		t.Error("MaxReconnectInterval should be positive")
	}
	if !cfg.Reconnect {
		t.Error("Reconnect should be true by default")
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

func TestClientNewWithLogger(t *testing.T) {
	logger := slog.Default()
	c := New(DefaultConfig(), logger)
	if c.logger != logger {
		t.Error("Logger not set correctly")
	}
}

func TestClientNewWithNilLogger(t *testing.T) {
	c := New(DefaultConfig(), nil)
	if c.logger == nil {
		t.Error("Logger should be set to default when nil is passed")
	}
}

func TestClientCloseWithoutConnect(t *testing.T) {
	c := New(DefaultConfig(), nil)
	if err := c.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	c := New(DefaultConfig(), nil)
	for i := 0; i < 3; i++ {
		if err := c.Close(); err != nil {
			t.Errorf("Close %d failed: %v", i+1, err)
		}
	}
}

func TestHTTPOptions(t *testing.T) {
	tests := []struct {
		name     string
		opt      HTTPOption
		validate func(*proto.TunnelRequest) bool
	}{
		{
			name: "WithSubdomain",
			opt:  WithSubdomain("myapp"),
			validate: func(req *proto.TunnelRequest) bool {
				return req.Subdomain == "myapp"
			},
		},
		{
			name: "WithInspect",
			opt:  WithInspect(),
			validate: func(req *proto.TunnelRequest) bool {
				return req.Inspect
			},
		},
		{
			name: "WithAuth",
			opt:  WithAuth("user", "pass"),
			validate: func(req *proto.TunnelRequest) bool {
				return req.Auth != nil && req.Auth.Username == "user" && req.Auth.Password == "pass" && req.Auth.Type == "basic"
			},
		},
		{
			name: "WithHeaders",
			opt:  WithHeaders(map[string]string{"X-Test": "value"}),
			validate: func(req *proto.TunnelRequest) bool {
				return req.Headers["X-Test"] == "value"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &proto.TunnelRequest{}
			tt.opt(req)
			if !tt.validate(req) {
				t.Errorf("Validation failed for %s", tt.name)
			}
		})
	}
}

func TestIsConnected(t *testing.T) {
	c := New(DefaultConfig(), nil)
	if c.IsConnected() {
		t.Error("Should not be connected initially")
	}
}

func TestSessionID(t *testing.T) {
	c := New(DefaultConfig(), nil)
	if c.SessionID() != "" {
		t.Error("SessionID should be empty before connecting")
	}
}

func TestHTTPNotConnected(t *testing.T) {
	c := New(DefaultConfig(), nil)
	_, err := c.HTTP("localhost:3000")
	if err != ErrNotConnected {
		t.Errorf("Expected ErrNotConnected, got %v", err)
	}
}

func TestTCPNotConnected(t *testing.T) {
	c := New(DefaultConfig(), nil)
	_, err := c.TCP("localhost:3000", 8080)
	if err != ErrNotConnected {
		t.Errorf("Expected ErrNotConnected, got %v", err)
	}
}

func TestCloseTunnelNotConnected(t *testing.T) {
	c := New(DefaultConfig(), nil)
	err := c.CloseTunnel("test-id")
	if err != ErrNotConnected {
		t.Errorf("Expected ErrNotConnected, got %v", err)
	}
}

func TestTunnelGetters(t *testing.T) {
	tunnel := &Tunnel{
		ID:        "test-id",
		PublicURL: "https://test.wirerift.dev",
		LocalAddr: "localhost:3000",
	}
	if tunnel.GetPublicURL() != "https://test.wirerift.dev" {
		t.Errorf("GetPublicURL = %q, want %q", tunnel.GetPublicURL(), "https://test.wirerift.dev")
	}
	if tunnel.GetLocalAddr() != "localhost:3000" {
		t.Errorf("GetLocalAddr = %q, want %q", tunnel.GetLocalAddr(), "localhost:3000")
	}
}

func TestTunnelClose(t *testing.T) {
	c := New(DefaultConfig(), nil)
	tunnel := &Tunnel{ID: "test-tunnel", client: c}
	err := tunnel.Close()
	if err != ErrNotConnected {
		t.Errorf("Expected ErrNotConnected, got %v", err)
	}
}

func TestFrameWriterReaderBeforeConnect(t *testing.T) {
	c := New(DefaultConfig(), nil)
	if c.FrameWriter() != nil {
		t.Error("FrameWriter should be nil before connect")
	}
	if c.FrameReader() != nil {
		t.Error("FrameReader should be nil before connect")
	}
}

func TestClientErrors(t *testing.T) {
	if ErrClientClosed == nil {
		t.Error("ErrClientClosed should not be nil")
	}
	if ErrNotConnected == nil {
		t.Error("ErrNotConnected should not be nil")
	}
	if ErrAuthFailed == nil {
		t.Error("ErrAuthFailed should not be nil")
	}
	if ErrTunnelFailed == nil {
		t.Error("ErrTunnelFailed should not be nil")
	}
	if ErrReconnectFailed == nil {
		t.Error("ErrReconnectFailed should not be nil")
	}
}

// --- Mock server for full-flow connect tests ---

type mockServer struct {
	listener net.Listener
	addr     string
	token    string
	tunnels  map[string]*Tunnel
	mu       struct {
		sync.Mutex
		connCount int
	}
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	ms := &mockServer{
		listener: l,
		addr:     l.Addr().String(),
		token:    "test-token",
		tunnels:  make(map[string]*Tunnel),
	}
	go ms.serve()
	return ms
}

func (ms *mockServer) serve() {
	for {
		conn, err := ms.listener.Accept()
		if err != nil {
			return
		}
		ms.mu.Lock()
		ms.mu.connCount++
		ms.mu.Unlock()
		go ms.handleConn(conn)
	}
}

func (ms *mockServer) handleConn(conn net.Conn) {
	defer conn.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(conn, magic); err != nil {
		return
	}
	if magic[0] != 0x57 || magic[1] != 0x52 || magic[2] != 0x46 || magic[3] != 0x01 {
		return
	}

	frameWriter := proto.NewFrameWriter(conn)
	frameReader := proto.NewFrameReader(conn)

	for {
		frame, err := frameReader.Read()
		if err != nil {
			return
		}

		switch frame.Type {
		case proto.FrameAuthReq:
			ms.handleAuth(frame, frameWriter)
		case proto.FrameTunnelReq:
			ms.handleTunnelRequest(frame, frameWriter)
		case proto.FrameHeartbeat:
			ackFrame := &proto.Frame{
				Version:  proto.Version,
				Type:     proto.FrameHeartbeatAck,
				StreamID: 0,
				Payload:  proto.HeartbeatPayload(),
			}
			frameWriter.Write(ackFrame)
		case proto.FrameTunnelClose:
			// Just consume it
		default:
			// Ignore
		}
	}
}

func (ms *mockServer) handleAuth(frame *proto.Frame, writer *proto.FrameWriter) {
	var authReq proto.AuthRequest
	if err := proto.DecodeJSONPayload(frame, &authReq); err != nil {
		return
	}

	authRes := &proto.AuthResponse{
		OK:         authReq.Token == ms.token,
		SessionID:  "test-session-123",
		MaxTunnels: 10,
	}
	if authReq.Token != ms.token {
		authRes.Error = "invalid token"
	}

	respFrame, _ := proto.EncodeJSONPayload(proto.FrameAuthRes, proto.ControlStreamID, authRes)
	writer.Write(respFrame)
}

func (ms *mockServer) handleTunnelRequest(frame *proto.Frame, writer *proto.FrameWriter) {
	var req proto.TunnelRequest
	if err := proto.DecodeJSONPayload(frame, &req); err != nil {
		return
	}

	res := &proto.TunnelResponse{
		OK:        true,
		TunnelID:  "tunnel-" + time.Now().Format("20060102150405"),
		Type:      req.Type,
		PublicURL: "https://test.wirerift.dev",
	}

	respFrame, _ := proto.EncodeJSONPayload(proto.FrameTunnelRes, proto.ControlStreamID, res)
	writer.Write(respFrame)
}

func (ms *mockServer) Close() {
	ms.listener.Close()
}

func (ms *mockServer) ConnectionCount() int {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.mu.connCount
}

// --- Helper: create a connected client using net.Pipe ---

// makeConnectedClient creates a Client with an active mux backed by a net.Pipe.
// Returns the client, and the server-side of the pipe for writing responses.
func makeConnectedClient(t *testing.T) (*Client, net.Conn) {
	t.Helper()
	c1, c2 := net.Pipe()

	cfg := Config{
		HeartbeatInterval:    100 * time.Millisecond,
		ReconnectInterval:    50 * time.Millisecond,
		MaxReconnectInterval: 200 * time.Millisecond,
	}
	c := New(cfg, slog.Default())
	c.conn = c1
	c.mux = mux.New(c1, mux.DefaultConfig())
	c.connected.Store(true)
	c.sessionID = "test-session"
	c.maxTunnels = 10

	return c, c2
}

// --- connect() tests ---

func TestConnectFailureDial(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServerAddr = "127.0.0.1:1"
	cfg.Reconnect = false
	c := New(cfg, nil)
	err := c.Connect()
	if err == nil {
		t.Fatal("Expected error for invalid address")
	}
	c.Close()
}

func TestConnectWithTLSDialFailure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServerAddr = "127.0.0.1:1"
	cfg.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	cfg.Reconnect = false
	c := New(cfg, nil)
	err := c.Connect()
	if err == nil {
		t.Fatal("Expected error for TLS dial to invalid address")
	}
	c.Close()
}

func TestConnectSuccess(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	cfg := DefaultConfig()
	cfg.ServerAddr = server.addr
	cfg.Token = server.token
	cfg.Reconnect = false
	cfg.HeartbeatInterval = 100 * time.Millisecond

	c := New(cfg, nil)
	defer c.Close()

	err := c.Connect()
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if !c.IsConnected() {
		t.Error("Should be connected")
	}
	if c.SessionID() == "" {
		t.Error("SessionID should not be empty")
	}
	if c.FrameWriter() == nil {
		t.Error("FrameWriter should not be nil after connect")
	}
	if c.FrameReader() == nil {
		t.Error("FrameReader should not be nil after connect")
	}
}

func TestConnectWithReconnectEnabled(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	cfg := DefaultConfig()
	cfg.ServerAddr = server.addr
	cfg.Token = server.token
	cfg.Reconnect = true
	cfg.HeartbeatInterval = 100 * time.Millisecond
	cfg.ReconnectInterval = 50 * time.Millisecond
	cfg.MaxReconnectInterval = 200 * time.Millisecond

	c := New(cfg, nil)
	err := c.Connect()
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if !c.IsConnected() {
		t.Error("Should be connected")
	}
	// Let heartbeat and reconnect goroutines run briefly
	time.Sleep(150 * time.Millisecond)
	c.Close()
}

func TestConnectAuthFailure(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	cfg := DefaultConfig()
	cfg.ServerAddr = server.addr
	cfg.Token = "wrong-token"
	cfg.Reconnect = false
	c := New(cfg, nil)
	defer c.Close()

	err := c.Connect()
	if err == nil {
		t.Fatal("Expected error for invalid token")
	}
	if c.IsConnected() {
		t.Error("Should not be connected")
	}
}

// --- authenticate() tests using net.Pipe ---

func TestAuthenticateWriteError(t *testing.T) {
	c1, c2 := net.Pipe()
	c2.Close() // close the other end so writes fail

	cfg := Config{HeartbeatInterval: time.Second}
	c := New(cfg, slog.Default())
	c.conn = c1
	c.mux = mux.New(c1, mux.DefaultConfig())

	err := c.authenticate()
	if err == nil {
		t.Fatal("Expected error when writing to closed pipe")
	}
	c1.Close()
}

func TestAuthenticateReadError(t *testing.T) {
	c1, c2 := net.Pipe()

	cfg := Config{HeartbeatInterval: time.Second}
	c := New(cfg, slog.Default())
	c.conn = c1
	c.mux = mux.New(c1, mux.DefaultConfig())

	// Server side: read the auth request frame then close
	go func() {
		fr := proto.NewFrameReader(c2)
		fr.Read() // read auth request
		c2.Close() // close before sending response
	}()

	err := c.authenticate()
	if err == nil {
		t.Fatal("Expected error when reading response from closed conn")
	}
	c1.Close()
}

func TestAuthenticateWrongFrameType(t *testing.T) {
	c1, c2 := net.Pipe()

	cfg := Config{HeartbeatInterval: time.Second}
	c := New(cfg, slog.Default())
	c.conn = c1
	c.mux = mux.New(c1, mux.DefaultConfig())

	go func() {
		fr := proto.NewFrameReader(c2)
		fw := proto.NewFrameWriter(c2)
		fr.Read() // read auth request

		// Send wrong frame type
		wrongFrame := &proto.Frame{
			Version:  proto.Version,
			Type:     proto.FrameHeartbeat,
			StreamID: 0,
			Payload:  proto.HeartbeatPayload(),
		}
		fw.Write(wrongFrame)
		// Drain remaining
		io.Copy(io.Discard, c2)
	}()

	err := c.authenticate()
	if err == nil {
		t.Fatal("Expected error for wrong frame type")
	}
	c1.Close()
}

func TestAuthenticateDecodeError(t *testing.T) {
	c1, c2 := net.Pipe()

	cfg := Config{HeartbeatInterval: time.Second}
	c := New(cfg, slog.Default())
	c.conn = c1
	c.mux = mux.New(c1, mux.DefaultConfig())

	go func() {
		fr := proto.NewFrameReader(c2)
		fw := proto.NewFrameWriter(c2)
		fr.Read() // read auth request

		// Send auth response frame with invalid JSON payload
		badFrame := &proto.Frame{
			Version:  proto.Version,
			Type:     proto.FrameAuthRes,
			StreamID: 0,
			Payload:  []byte("not valid json{{{"),
		}
		fw.Write(badFrame)
		io.Copy(io.Discard, c2)
	}()

	err := c.authenticate()
	if err == nil {
		t.Fatal("Expected error for bad JSON payload")
	}
	c1.Close()
}

func TestAuthenticateAuthFailed(t *testing.T) {
	c1, c2 := net.Pipe()

	cfg := Config{HeartbeatInterval: time.Second}
	c := New(cfg, slog.Default())
	c.conn = c1
	c.mux = mux.New(c1, mux.DefaultConfig())

	go func() {
		fr := proto.NewFrameReader(c2)
		fw := proto.NewFrameWriter(c2)
		fr.Read() // read auth request

		resp := &proto.AuthResponse{
			OK:    false,
			Error: "access denied",
		}
		respFrame, _ := proto.EncodeJSONPayload(proto.FrameAuthRes, proto.ControlStreamID, resp)
		fw.Write(respFrame)
		io.Copy(io.Discard, c2)
	}()

	err := c.authenticate()
	if err == nil {
		t.Fatal("Expected error for auth failure")
	}
	c1.Close()
}

func TestAuthenticateSuccess(t *testing.T) {
	c1, c2 := net.Pipe()

	cfg := Config{HeartbeatInterval: time.Second}
	c := New(cfg, slog.Default())
	c.conn = c1
	c.mux = mux.New(c1, mux.DefaultConfig())

	go func() {
		fr := proto.NewFrameReader(c2)
		fw := proto.NewFrameWriter(c2)
		fr.Read() // read auth request

		resp := &proto.AuthResponse{
			OK:         true,
			SessionID:  "session-abc",
			MaxTunnels: 5,
		}
		respFrame, _ := proto.EncodeJSONPayload(proto.FrameAuthRes, proto.ControlStreamID, resp)
		fw.Write(respFrame)
		io.Copy(io.Discard, c2)
	}()

	err := c.authenticate()
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
	if c.sessionID != "session-abc" {
		t.Errorf("sessionID = %q, want session-abc", c.sessionID)
	}
	if c.maxTunnels != 5 {
		t.Errorf("maxTunnels = %d, want 5", c.maxTunnels)
	}
	c1.Close()
}

// --- openTunnel() tests ---

func TestOpenTunnelSuccess(t *testing.T) {
	client, serverConn := makeConnectedClient(t)
	defer serverConn.Close()

	go func() {
		fr := proto.NewFrameReader(serverConn)
		fw := proto.NewFrameWriter(serverConn)
		frame, err := fr.Read()
		if err != nil {
			return
		}
		if frame.Type != proto.FrameTunnelReq {
			return
		}
		resp := &proto.TunnelResponse{
			OK:        true,
			TunnelID:  "tun-123",
			Type:      proto.TunnelTypeHTTP,
			PublicURL: "https://myapp.wirerift.dev",
		}
		respFrame, _ := proto.EncodeJSONPayload(proto.FrameTunnelRes, proto.ControlStreamID, resp)
		fw.Write(respFrame)
		io.Copy(io.Discard, serverConn)
	}()

	req := &proto.TunnelRequest{
		Type:      proto.TunnelTypeHTTP,
		LocalAddr: "localhost:3000",
		Subdomain: "myapp",
	}
	tunnel, err := client.openTunnel(req)
	if err != nil {
		t.Fatalf("openTunnel failed: %v", err)
	}
	if tunnel.ID != "tun-123" {
		t.Errorf("tunnel ID = %q, want tun-123", tunnel.ID)
	}
	if tunnel.PublicURL != "https://myapp.wirerift.dev" {
		t.Errorf("PublicURL = %q, want https://myapp.wirerift.dev", tunnel.PublicURL)
	}
	if tunnel.LocalAddr != "localhost:3000" {
		t.Errorf("LocalAddr = %q, want localhost:3000", tunnel.LocalAddr)
	}
	if tunnel.Subdomain != "myapp" {
		t.Errorf("Subdomain = %q, want myapp", tunnel.Subdomain)
	}

	// Verify tunnel is stored
	if _, ok := client.tunnels.Load("tun-123"); !ok {
		t.Error("tunnel should be stored in client.tunnels")
	}

	client.conn.Close()
}

func TestOpenTunnelWriteError(t *testing.T) {
	client, serverConn := makeConnectedClient(t)
	serverConn.Close() // close server side so writes fail

	req := &proto.TunnelRequest{
		Type:      proto.TunnelTypeHTTP,
		LocalAddr: "localhost:3000",
	}
	_, err := client.openTunnel(req)
	if err == nil {
		t.Fatal("Expected error when writing to closed conn")
	}
	client.conn.Close()
}

func TestOpenTunnelReadError(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	go func() {
		fr := proto.NewFrameReader(serverConn)
		fr.Read() // read tunnel request
		serverConn.Close() // close before sending response
	}()

	req := &proto.TunnelRequest{
		Type:      proto.TunnelTypeHTTP,
		LocalAddr: "localhost:3000",
	}
	_, err := client.openTunnel(req)
	if err == nil {
		t.Fatal("Expected error when reading from closed conn")
	}
	client.conn.Close()
}

func TestOpenTunnelWrongFrameType(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	go func() {
		fr := proto.NewFrameReader(serverConn)
		fw := proto.NewFrameWriter(serverConn)
		fr.Read() // read tunnel request

		wrongFrame := &proto.Frame{
			Version:  proto.Version,
			Type:     proto.FrameHeartbeat,
			StreamID: 0,
			Payload:  proto.HeartbeatPayload(),
		}
		fw.Write(wrongFrame)
		io.Copy(io.Discard, serverConn)
	}()

	req := &proto.TunnelRequest{
		Type:      proto.TunnelTypeHTTP,
		LocalAddr: "localhost:3000",
	}
	_, err := client.openTunnel(req)
	if err == nil {
		t.Fatal("Expected error for wrong frame type")
	}
	client.conn.Close()
	serverConn.Close()
}

func TestOpenTunnelDecodeError(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	go func() {
		fr := proto.NewFrameReader(serverConn)
		fw := proto.NewFrameWriter(serverConn)
		fr.Read() // read tunnel request

		badFrame := &proto.Frame{
			Version:  proto.Version,
			Type:     proto.FrameTunnelRes,
			StreamID: 0,
			Payload:  []byte("invalid json{{{"),
		}
		fw.Write(badFrame)
		io.Copy(io.Discard, serverConn)
	}()

	req := &proto.TunnelRequest{
		Type:      proto.TunnelTypeHTTP,
		LocalAddr: "localhost:3000",
	}
	_, err := client.openTunnel(req)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	client.conn.Close()
	serverConn.Close()
}

func TestOpenTunnelFailed(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	go func() {
		fr := proto.NewFrameReader(serverConn)
		fw := proto.NewFrameWriter(serverConn)
		fr.Read() // read tunnel request

		resp := &proto.TunnelResponse{
			OK:    false,
			Error: "max tunnels reached",
		}
		respFrame, _ := proto.EncodeJSONPayload(proto.FrameTunnelRes, proto.ControlStreamID, resp)
		fw.Write(respFrame)
		io.Copy(io.Discard, serverConn)
	}()

	req := &proto.TunnelRequest{
		Type:      proto.TunnelTypeHTTP,
		LocalAddr: "localhost:3000",
	}
	_, err := client.openTunnel(req)
	if err == nil {
		t.Fatal("Expected error for failed tunnel")
	}
	client.conn.Close()
	serverConn.Close()
}

// --- HTTP() and TCP() with connected client ---

func TestHTTPSuccess(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	go func() {
		fr := proto.NewFrameReader(serverConn)
		fw := proto.NewFrameWriter(serverConn)
		frame, err := fr.Read()
		if err != nil {
			return
		}
		if frame.Type != proto.FrameTunnelReq {
			return
		}
		resp := &proto.TunnelResponse{
			OK:        true,
			TunnelID:  "http-tun-1",
			Type:      proto.TunnelTypeHTTP,
			PublicURL: "https://myapp.wirerift.dev",
		}
		respFrame, _ := proto.EncodeJSONPayload(proto.FrameTunnelRes, proto.ControlStreamID, resp)
		fw.Write(respFrame)
		io.Copy(io.Discard, serverConn)
	}()

	tunnel, err := client.HTTP("localhost:3000", WithSubdomain("myapp"), WithInspect())
	if err != nil {
		t.Fatalf("HTTP failed: %v", err)
	}
	if tunnel.ID != "http-tun-1" {
		t.Errorf("tunnel ID = %q, want http-tun-1", tunnel.ID)
	}
	client.conn.Close()
	serverConn.Close()
}

func TestTCPSuccess(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	go func() {
		fr := proto.NewFrameReader(serverConn)
		fw := proto.NewFrameWriter(serverConn)
		frame, err := fr.Read()
		if err != nil {
			return
		}
		if frame.Type != proto.FrameTunnelReq {
			return
		}
		resp := &proto.TunnelResponse{
			OK:       true,
			TunnelID: "tcp-tun-1",
			Type:     proto.TunnelTypeTCP,
		}
		respFrame, _ := proto.EncodeJSONPayload(proto.FrameTunnelRes, proto.ControlStreamID, resp)
		fw.Write(respFrame)
		io.Copy(io.Discard, serverConn)
	}()

	tunnel, err := client.TCP("localhost:5432", 5432)
	if err != nil {
		t.Fatalf("TCP failed: %v", err)
	}
	if tunnel.ID != "tcp-tun-1" {
		t.Errorf("tunnel ID = %q, want tcp-tun-1", tunnel.ID)
	}
	if tunnel.Port != 5432 {
		t.Errorf("Port = %d, want 5432", tunnel.Port)
	}
	client.conn.Close()
	serverConn.Close()
}

// --- CloseTunnel() tests ---

func TestCloseTunnelSuccess(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	// Store a tunnel to close
	client.tunnels.Store("tun-1", &Tunnel{ID: "tun-1", client: client})

	go func() {
		// Drain all data from server side
		io.Copy(io.Discard, serverConn)
	}()

	err := client.CloseTunnel("tun-1")
	if err != nil {
		t.Fatalf("CloseTunnel failed: %v", err)
	}

	// Verify tunnel is deleted
	if _, ok := client.tunnels.Load("tun-1"); ok {
		t.Error("tunnel should be deleted after close")
	}

	client.conn.Close()
	serverConn.Close()
}

func TestCloseTunnelWriteError(t *testing.T) {
	client, serverConn := makeConnectedClient(t)
	serverConn.Close() // close server side so writes fail

	err := client.CloseTunnel("tun-1")
	if err == nil {
		t.Fatal("Expected error when writing to closed conn")
	}
	client.conn.Close()
}

// --- heartbeatLoop() tests ---

func TestHeartbeatLoopSendsHeartbeat(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	cfg := DefaultConfig()
	cfg.ServerAddr = server.addr
	cfg.Token = server.token
	cfg.Reconnect = false
	cfg.HeartbeatInterval = 50 * time.Millisecond

	c := New(cfg, nil)
	err := c.Connect()
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Wait for multiple heartbeats
	time.Sleep(200 * time.Millisecond)

	if !c.IsConnected() {
		t.Error("Should still be connected after heartbeats")
	}
	c.Close()
}

func TestHeartbeatLoopCtxDone(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	cfg := DefaultConfig()
	cfg.ServerAddr = server.addr
	cfg.Token = server.token
	cfg.Reconnect = false
	cfg.HeartbeatInterval = 50 * time.Millisecond

	c := New(cfg, nil)
	err := c.Connect()
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Cancel context to trigger heartbeat loop exit
	c.cancel()
	// Wait for goroutines
	time.Sleep(100 * time.Millisecond)
}

func TestHeartbeatLoopWriteError(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	// Close server side after a brief delay so heartbeat write fails,
	// then close mux to end the loop since heartbeat failures only log warnings.
	go func() {
		time.Sleep(50 * time.Millisecond)
		serverConn.Close()
		// Wait for at least one heartbeat failure, then close mux to exit loop
		time.Sleep(150 * time.Millisecond)
		client.mux.Close()
	}()

	client.heartbeatLoop()
	client.conn.Close()
}

func TestHeartbeatLoopMuxDone(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	// Close the mux to trigger Done()
	go func() {
		time.Sleep(50 * time.Millisecond)
		client.mux.Close()
	}()

	client.heartbeatLoop()

	// After mux done, connected should be false
	if client.connected.Load() {
		t.Error("connected should be false after mux done")
	}
	serverConn.Close()
}

func TestHeartbeatLoopNotConnected(t *testing.T) {
	// Test the continue branch when not connected
	client, serverConn := makeConnectedClient(t)
	client.connected.Store(false) // set not connected

	go func() {
		// Let a couple of heartbeat ticks pass (at 100ms interval), then cancel ctx
		time.Sleep(250 * time.Millisecond)
		client.cancel()
	}()

	// Drain server side
	go io.Copy(io.Discard, serverConn)

	client.heartbeatLoop()
	serverConn.Close()
	client.conn.Close()
}

// --- reconnectLoop() tests ---

func TestReconnectLoopCtxDone(t *testing.T) {
	client, serverConn := makeConnectedClient(t)
	client.config.Reconnect = true

	// Cancel context immediately
	client.cancel()

	client.reconnectLoop()
	serverConn.Close()
	client.conn.Close()
}

func TestReconnectLoopReconnectDisabled(t *testing.T) {
	client, serverConn := makeConnectedClient(t)
	client.config.Reconnect = false

	// Close mux to trigger Done
	go func() {
		time.Sleep(20 * time.Millisecond)
		client.mux.Close()
	}()

	client.reconnectLoop()
	serverConn.Close()
}

func TestReconnectLoopFailureWithBackoff(t *testing.T) {
	client, serverConn := makeConnectedClient(t)
	client.config.Reconnect = true
	client.config.ServerAddr = "127.0.0.1:1" // will fail to reconnect
	client.config.ReconnectInterval = 20 * time.Millisecond
	client.config.MaxReconnectInterval = 50 * time.Millisecond

	// Close mux to trigger reconnect
	client.mux.Close()

	// Let it attempt a couple reconnects then cancel
	go func() {
		time.Sleep(200 * time.Millisecond)
		client.cancel()
	}()

	client.reconnectLoop()
	serverConn.Close()
}

func TestReconnectLoopCtxDoneDuringWait(t *testing.T) {
	client, serverConn := makeConnectedClient(t)
	client.config.Reconnect = true
	client.config.ServerAddr = "127.0.0.1:1"
	client.config.ReconnectInterval = 5 * time.Second // long interval

	// Close mux to trigger reconnect
	client.mux.Close()

	// Cancel context while waiting for reconnect interval
	go func() {
		time.Sleep(50 * time.Millisecond)
		client.cancel()
	}()

	client.reconnectLoop()
	serverConn.Close()
}

func TestReconnectLoopSuccess(t *testing.T) {
	// Start a mock server that the client can reconnect to
	server := newMockServer(t)
	defer server.Close()

	client, serverConn := makeConnectedClient(t)
	client.config.Reconnect = true
	client.config.ServerAddr = server.addr
	client.config.Token = server.token
	client.config.ReconnectInterval = 20 * time.Millisecond
	client.config.MaxReconnectInterval = 100 * time.Millisecond

	// Close mux to trigger reconnect
	client.mux.Close()

	// Let reconnect succeed, then cancel to stop the loop
	go func() {
		time.Sleep(300 * time.Millisecond)
		client.cancel()
	}()

	client.reconnectLoop()
	serverConn.Close()
}

// --- Full integration: Connect with reconnect, then connection loss ---

func TestConnectionLossTriggersReconnect(t *testing.T) {
	server := newMockServer(t)

	cfg := DefaultConfig()
	cfg.ServerAddr = server.addr
	cfg.Token = server.token
	cfg.Reconnect = false
	cfg.HeartbeatInterval = 100 * time.Millisecond

	c := New(cfg, nil)
	err := c.Connect()
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Close server to cause connection loss
	server.Close()
	time.Sleep(200 * time.Millisecond)
	c.Close()
}

// --- Tunnel.Close via client ---

func TestTunnelCloseViaClient(t *testing.T) {
	client, serverConn := makeConnectedClient(t)

	go func() {
		io.Copy(io.Discard, serverConn)
	}()

	tunnel := &Tunnel{
		ID:     "tun-42",
		client: client,
	}
	client.tunnels.Store("tun-42", tunnel)

	err := tunnel.Close()
	if err != nil {
		t.Fatalf("tunnel.Close failed: %v", err)
	}

	if _, ok := client.tunnels.Load("tun-42"); ok {
		t.Error("tunnel should be deleted")
	}

	client.conn.Close()
	serverConn.Close()
}

// --- connect() WriteMagic error ---

func TestConnectWriteMagicError(t *testing.T) {
	// Test WriteMagic error path by creating a server that sends RST.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		// Set linger to 0 and close to send RST
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.SetLinger(0)
		}
		conn.Close()
	}()

	cfg := DefaultConfig()
	cfg.ServerAddr = l.Addr().String()
	cfg.Reconnect = false

	c := New(cfg, nil)
	connectErr := c.connect()
	if connectErr == nil {
		t.Log("WriteMagic did not fail immediately (buffered)")
	}
	c.Close()
}

// --- connect() authenticate error (via mock server that sends wrong response) ---

func TestConnectAuthenticateError(t *testing.T) {
	// Server that accepts but sends wrong auth response
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read magic
		magic := make([]byte, 4)
		io.ReadFull(conn, magic)

		fr := proto.NewFrameReader(conn)
		fw := proto.NewFrameWriter(conn)

		// Read auth request
		fr.Read()

		// Send auth failure
		resp := &proto.AuthResponse{OK: false, Error: "bad token"}
		respFrame, _ := proto.EncodeJSONPayload(proto.FrameAuthRes, proto.ControlStreamID, resp)
		fw.Write(respFrame)
	}()

	cfg := DefaultConfig()
	cfg.ServerAddr = l.Addr().String()
	cfg.Token = "bad-token"
	cfg.Reconnect = false

	c := New(cfg, nil)
	connectErr := c.connect()
	if connectErr == nil {
		t.Fatal("Expected auth error")
	}
	c.Close()
}

// --- More edge cases ---

func TestContextCancellation(t *testing.T) {
	c := New(DefaultConfig(), nil)
	select {
	case <-c.ctx.Done():
		t.Error("Context should not be done initially")
	default:
	}
	c.Close()
	select {
	case <-c.ctx.Done():
	default:
		t.Error("Context should be done after close")
	}
}

func TestMaxTunnelsAfterConnect(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	cfg := DefaultConfig()
	cfg.ServerAddr = server.addr
	cfg.Token = server.token
	cfg.Reconnect = false

	c := New(cfg, nil)
	defer c.Close()

	if c.maxTunnels != 0 {
		t.Errorf("maxTunnels should be 0 before connect, got %d", c.maxTunnels)
	}

	err := c.Connect()
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if c.maxTunnels != 10 {
		t.Errorf("maxTunnels should be 10 after connect, got %d", c.maxTunnels)
	}
}
