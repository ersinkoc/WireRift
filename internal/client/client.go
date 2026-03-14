package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wirerift/wirerift/internal/mux"
	"github.com/wirerift/wirerift/internal/proto"
)

// Errors returned by client operations.
var (
	ErrClientClosed    = errors.New("client is closed")
	ErrNotConnected    = errors.New("not connected to server")
	ErrAuthFailed      = errors.New("authentication failed")
	ErrTunnelFailed    = errors.New("tunnel creation failed")
	ErrReconnectFailed = errors.New("reconnect failed")
)

// Config holds client configuration.
type Config struct {
	// ServerAddr is the address of the tunnel server.
	ServerAddr string

	// Token is the authentication token.
	Token string

	// TLSConfig is the TLS configuration.
	TLSConfig *tls.Config

	// Reconnect enables automatic reconnection.
	Reconnect bool

	// ReconnectInterval is the initial reconnect interval.
	ReconnectInterval time.Duration

	// MaxReconnectInterval is the maximum reconnect interval.
	MaxReconnectInterval time.Duration

	// HeartbeatInterval is the interval for sending heartbeats.
	HeartbeatInterval time.Duration
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() Config {
	return Config{
		ServerAddr:           "wirerift.dev:4443",
		Reconnect:            true,
		ReconnectInterval:    1 * time.Second,
		MaxReconnectInterval: 30 * time.Second,
		HeartbeatInterval:    30 * time.Second,
	}
}

// Client is the tunnel client.
type Client struct {
	config Config
	logger *slog.Logger

	// Connection
	conn net.Conn
	mux  *mux.Mux

	// Session state
	sessionID  string
	connected  atomic.Bool
	maxTunnels int

	// Tunnels
	tunnels sync.Map // map[string]*Tunnel

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Tunnel represents an active tunnel.
type Tunnel struct {
	ID        string
	Type      proto.TunnelType
	PublicURL string
	LocalAddr string
	Subdomain string
	Port      int
	client    *Client
}

// New creates a new client.
func New(config Config, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Connect connects to the tunnel server.
func (c *Client) Connect() error {
	if err := c.connect(); err != nil {
		return err
	}

	// Start heartbeat
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.heartbeatLoop()
	}()

	// Start reconnect loop if enabled
	if c.config.Reconnect {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.reconnectLoop()
		}()
	}

	return nil
}

// connect establishes the connection and authenticates.
func (c *Client) connect() error {
	// Dial server
	var conn net.Conn
	var err error

	if c.config.TLSConfig != nil {
		conn, err = tls.Dial("tcp", c.config.ServerAddr, c.config.TLSConfig)
	} else {
		conn, err = net.Dial("tcp", c.config.ServerAddr)
	}

	if err != nil {
		return fmt.Errorf("dial server: %w", err)
	}

	c.conn = conn

	// Send magic bytes
	if err := proto.WriteMagic(conn); err != nil {
		conn.Close()
		return fmt.Errorf("send magic: %w", err)
	}

	// Create mux
	c.mux = mux.New(conn, mux.DefaultConfig())

	// Authenticate
	if err := c.authenticate(); err != nil {
		conn.Close()
		return err
	}

	c.connected.Store(true)
	c.logger.Info("connected", "session", c.sessionID)

	// Start mux run loop
	go c.mux.Run()

	return nil
}

// authenticate sends authentication request.
func (c *Client) authenticate() error {
	authReq := &proto.AuthRequest{
		Token:   c.config.Token,
		Version: "1.0.0",
	}

	frame, err := proto.EncodeJSONPayload(proto.FrameAuthReq, proto.ControlStreamID, authReq)
	if err != nil {
		return fmt.Errorf("encode auth request: %w", err)
	}

	if err := c.mux.GetFrameWriter().Write(frame); err != nil {
		return fmt.Errorf("send auth request: %w", err)
	}

	// Read response
	respFrame, err := c.mux.GetFrameReader().Read()
	if err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}

	if respFrame.Type != proto.FrameAuthRes {
		return fmt.Errorf("unexpected frame type: %v", respFrame.Type)
	}

	var authRes proto.AuthResponse
	if err := proto.DecodeJSONPayload(respFrame, &authRes); err != nil {
		return fmt.Errorf("decode auth response: %w", err)
	}

	if !authRes.OK {
		return fmt.Errorf("%w: %s", ErrAuthFailed, authRes.Error)
	}

	c.sessionID = authRes.SessionID
	c.maxTunnels = authRes.MaxTunnels

	return nil
}

// Close closes the client.
func (c *Client) Close() error {
	c.cancel()

	if c.mux != nil {
		c.mux.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}

	c.wg.Wait()
	return nil
}

// HTTP opens an HTTP tunnel.
func (c *Client) HTTP(localAddr string, opts ...HTTPOption) (*Tunnel, error) {
	if !c.connected.Load() {
		return nil, ErrNotConnected
	}

	req := &proto.TunnelRequest{
		Type:      proto.TunnelTypeHTTP,
		LocalAddr: localAddr,
	}

	for _, opt := range opts {
		opt(req)
	}

	return c.openTunnel(req)
}

// TCP opens a TCP tunnel.
func (c *Client) TCP(localAddr string, port int) (*Tunnel, error) {
	if !c.connected.Load() {
		return nil, ErrNotConnected
	}

	req := &proto.TunnelRequest{
		Type:      proto.TunnelTypeTCP,
		LocalAddr: localAddr,
		RemotePort: port,
	}

	return c.openTunnel(req)
}

// openTunnel sends a tunnel request.
func (c *Client) openTunnel(req *proto.TunnelRequest) (*Tunnel, error) {
	frame, err := proto.EncodeJSONPayload(proto.FrameTunnelReq, proto.ControlStreamID, req)
	if err != nil {
		return nil, fmt.Errorf("encode tunnel request: %w", err)
	}

	if err := c.mux.GetFrameWriter().Write(frame); err != nil {
		return nil, fmt.Errorf("send tunnel request: %w", err)
	}

	// Read response
	respFrame, err := c.mux.GetFrameReader().Read()
	if err != nil {
		return nil, fmt.Errorf("read tunnel response: %w", err)
	}

	if respFrame.Type != proto.FrameTunnelRes {
		return nil, fmt.Errorf("unexpected frame type: %v", respFrame.Type)
	}

	var res proto.TunnelResponse
	if err := proto.DecodeJSONPayload(respFrame, &res); err != nil {
		return nil, fmt.Errorf("decode tunnel response: %w", err)
	}

	if !res.OK {
		return nil, fmt.Errorf("%w: %s", ErrTunnelFailed, res.Error)
	}

	tunnel := &Tunnel{
		ID:        res.TunnelID,
		Type:      res.Type,
		PublicURL: res.PublicURL,
		LocalAddr: req.LocalAddr,
		Subdomain: req.Subdomain,
		Port:      req.RemotePort,
		client:    c,
	}

	c.tunnels.Store(res.TunnelID, tunnel)

	c.logger.Info("tunnel opened", "id", res.TunnelID, "url", res.PublicURL)

	return tunnel, nil
}

// CloseTunnel closes a tunnel.
func (c *Client) CloseTunnel(id string) error {
	if c.mux == nil {
		return ErrNotConnected
	}

	closeReq := &proto.TunnelClose{TunnelID: id}
	frame, err := proto.EncodeJSONPayload(proto.FrameTunnelClose, proto.ControlStreamID, closeReq)
	if err != nil {
		return err
	}

	if err := c.mux.GetFrameWriter().Write(frame); err != nil {
		return err
	}

	c.tunnels.Delete(id)
	c.logger.Info("tunnel closed", "id", id)

	return nil
}

// SessionID returns the current session ID.
func (c *Client) SessionID() string {
	return c.sessionID
}

// IsConnected returns true if connected.
func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// FrameWriter returns the frame writer (for internal use).
func (c *Client) FrameWriter() *proto.FrameWriter {
	if c.mux == nil {
		return nil
	}
	return c.mux.GetFrameWriter()
}

// FrameReader returns the frame reader (for internal use).
func (c *Client) FrameReader() *proto.FrameReader {
	if c.mux == nil {
		return nil
	}
	return c.mux.GetFrameReader()
}

// heartbeatLoop sends periodic heartbeats.
func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if !c.connected.Load() {
				continue
			}

			frame := &proto.Frame{
				Version:  proto.Version,
				Type:     proto.FrameHeartbeat,
				StreamID: 0,
				Payload:  proto.HeartbeatPayload(),
			}

			if err := c.mux.GetFrameWriter().Write(frame); err != nil {
				c.logger.Warn("heartbeat failed", "error", err)
			}
		case <-c.mux.Done():
			c.connected.Store(false)
			c.logger.Warn("connection lost")
			return
		}
	}
}

// reconnectLoop handles automatic reconnection.
func (c *Client) reconnectLoop() {
	interval := c.config.ReconnectInterval

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.mux.Done():
			if !c.config.Reconnect {
				return
			}

			c.connected.Store(false)
			c.logger.Info("reconnecting", "interval", interval)

			select {
			case <-c.ctx.Done():
				return
			case <-time.After(interval):
			}

			if err := c.connect(); err != nil {
				c.logger.Warn("reconnect failed", "error", err)
				interval = interval * 2
				if interval > c.config.MaxReconnectInterval {
					interval = c.config.MaxReconnectInterval
				}
				continue
			}

			interval = c.config.ReconnectInterval
		}
	}
}

// HTTPOption is an option for HTTP tunnels.
type HTTPOption func(*proto.TunnelRequest)

// WithSubdomain sets the subdomain.
func WithSubdomain(subdomain string) HTTPOption {
	return func(req *proto.TunnelRequest) {
		req.Subdomain = subdomain
	}
}

// WithInspect enables traffic inspection.
func WithInspect() HTTPOption {
	return func(req *proto.TunnelRequest) {
		req.Inspect = true
	}
}

// WithAuth sets tunnel authentication.
func WithAuth(username, password string) HTTPOption {
	return func(req *proto.TunnelRequest) {
		req.Auth = &proto.TunnelAuth{
			Type:     "basic",
			Username: username,
			Password: password,
		}
	}
}

// WithHeaders sets custom headers.
func WithHeaders(headers map[string]string) HTTPOption {
	return func(req *proto.TunnelRequest) {
		req.Headers = headers
	}
}

// Close closes the tunnel.
func (t *Tunnel) Close() error {
	return t.client.CloseTunnel(t.ID)
}

// GetPublicURL returns the public URL.
func (t *Tunnel) GetPublicURL() string {
	return t.PublicURL
}

// GetLocalAddr returns the local address.
func (t *Tunnel) GetLocalAddr() string {
	return t.LocalAddr
}
