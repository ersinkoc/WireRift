package server

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wirerift/wirerift/internal/mux"
)

func TestSerializeRequest(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Host", "example.com")
	req.Header.Set("User-Agent", "test")
	req.RemoteAddr = "192.168.1.1:12345"

	data, err := SerializeRequest(req)
	if err != nil {
		t.Fatalf("SerializeRequest: %v", err)
	}

	if len(data) == 0 {
		t.Error("Serialized request is empty")
	}

	// Check that request line is present
	if !bytes.Contains(data, []byte("GET")) {
		t.Error("Missing method in serialized request")
	}
	if !bytes.Contains(data, []byte("/test")) {
		t.Error("Missing path in serialized request")
	}
}

func TestIsWebSocketRequest(t *testing.T) {
	tests := []struct {
		headers  map[string]string
		expected bool
	}{
		{map[string]string{"Upgrade": "websocket"}, true},
		{map[string]string{"Upgrade": "WebSocket"}, true},
		{map[string]string{"Upgrade": "WEBSOCKET"}, true},
		{map[string]string{"Upgrade": "http2"}, false},
		{map[string]string{}, false},
	}

	for i, tt := range tests {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		for k, v := range tt.headers {
			req.Header.Set(k, v)
		}

		result := IsWebSocketRequest(req)
		if result != tt.expected {
			t.Errorf("[%d] IsWebSocketRequest = %v, want %v", i, result, tt.expected)
		}
	}
}

func TestIsSSE(t *testing.T) {
	tests := []struct {
		accept   string
		expected bool
	}{
		{"text/event-stream", true},
		{"text/event-stream, text/plain", true},
		{"text/plain", false},
		{"", false},
	}

	for i, tt := range tests {
		req, _ := http.NewRequest("GET", "http://example.com/events", nil)
		if tt.accept != "" {
			req.Header.Set("Accept", tt.accept)
		}

		result := IsSSE(req)
		if result != tt.expected {
			t.Errorf("[%d] IsSSE = %v, want %v", i, result, tt.expected)
		}
	}
}

func TestDeserializeResponse(t *testing.T) {
	// Create a simple HTTP response
	raw := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"Hello"

	resp, err := DeserializeResponse([]byte(raw))
	if err != nil {
		t.Fatalf("DeserializeResponse: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("Content-Type = %q, want %q", resp.Header.Get("Content-Type"), "text/plain")
	}
}

func TestShouldCloseConnection(t *testing.T) {
	tests := []struct {
		name     string
		req      *http.Request
		resp     *http.Response
		expected bool
	}{
		{
			name:     "websocket should not close",
			req:      mustMakeRequest("GET", map[string]string{"Upgrade": "websocket"}),
			expected: false,
		},
		{
			name:     "connection close header",
			req:      mustMakeRequest("GET", map[string]string{"Connection": "close"}),
			expected: true,
		},
		{
			name:     "normal request",
			req:      mustMakeRequest("GET", nil),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldCloseConnection(tt.req, tt.resp)
			if result != tt.expected {
				t.Errorf("shouldCloseConnection = %v, want %v", result, tt.expected)
			}
		})
	}
}

func mustMakeRequest(method string, headers map[string]string) *http.Request {
	req, err := http.NewRequest(method, "http://example.com", nil)
	if err != nil {
		panic(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// TestNewHTTPProxy tests the HTTP proxy creation
func TestNewHTTPProxy(t *testing.T) {
	srv := New(DefaultConfig(), nil)
	proxy := NewHTTPProxy(srv, 0)

	if proxy == nil {
		t.Fatal("NewHTTPProxy returned nil")
	}
	if proxy.server != srv {
		t.Error("Server not set correctly")
	}
	if proxy.timeout != 30*time.Second {
		t.Errorf("Default timeout = %v, want 30s", proxy.timeout)
	}
}

// TestNewHTTPProxyWithTimeout tests creating HTTP proxy with custom timeout
func TestNewHTTPProxyWithTimeout(t *testing.T) {
	srv := New(DefaultConfig(), nil)
	proxy := NewHTTPProxy(srv, 60*time.Second)

	if proxy == nil {
		t.Fatal("NewHTTPProxy returned nil")
	}
	if proxy.timeout != 60*time.Second {
		t.Errorf("Custom timeout = %v, want 60s", proxy.timeout)
	}
}

// TestNewHTTPProxyBufferPool tests that buffer pool is initialized
func TestNewHTTPProxyBufferPool(t *testing.T) {
	srv := New(DefaultConfig(), nil)
	proxy := NewHTTPProxy(srv, 0)

	// Get a buffer from the pool
	bufp := proxy.pool.Get().(*[]byte)
	buf := *bufp
	if len(buf) != 32*1024 {
		t.Errorf("Buffer size = %d, want 32768", len(buf))
	}

	// Return to pool
	proxy.pool.Put(bufp)
}

// TestProxyRequestOpenStreamError tests that ProxyRequest returns an error when the mux is nil/closed.
func TestProxyRequestOpenStreamError(t *testing.T) {
	srv := New(DefaultConfig(), nil)
	proxy := NewHTTPProxy(srv, 30*time.Second)

	// Create mock response writer and request
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	tunnel := &Tunnel{ID: "test-tunnel"}

	// Create a mux with a pipe so we can close it to trigger an error
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	m := mux.New(c1, mux.DefaultConfig())
	go m.Run()
	m.Close() // close immediately so OpenStream fails

	session := &Session{ID: "test-session", Mux: m}

	err := proxy.ProxyRequest(rw, req, tunnel, session)
	if err == nil {
		t.Error("Expected error from ProxyRequest with closed mux")
	}
	if !strings.Contains(err.Error(), "open stream") {
		t.Errorf("Expected 'open stream' error, got: %v", err)
	}
}

// TestWriteResponse tests writing an HTTP response
func TestWriteResponse(t *testing.T) {
	// Create a simple HTTP response
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(strings.NewReader("Hello, World!")),
	}

	// Create a response recorder
	rw := httptest.NewRecorder()

	err := WriteResponse(rw, resp)
	if err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}

	if rw.Code != 200 {
		t.Errorf("StatusCode = %d, want 200", rw.Code)
	}
	if ct := rw.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if body := rw.Body.String(); body != "Hello, World!" {
		t.Errorf("Body = %q, want 'Hello, World!'", body)
	}
}

// TestWriteResponseNoBody tests writing a response without body
func TestWriteResponseNoBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: 204,
		Header:     http.Header{},
		Body:       nil,
	}

	rw := httptest.NewRecorder()

	err := WriteResponse(rw, resp)
	if err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}

	if rw.Code != 204 {
		t.Errorf("StatusCode = %d, want 204", rw.Code)
	}
}

// TestShouldCloseConnectionWithResponse tests shouldCloseConnection with response
func TestShouldCloseConnectionWithResponse(t *testing.T) {
	tests := []struct {
		name     string
		req      *http.Request
		resp     *http.Response
		expected bool
	}{
		{
			name:     "websocket should not close",
			req:      mustMakeRequest("GET", map[string]string{"Upgrade": "websocket"}),
			resp:     nil,
			expected: false,
		},
		{
			name:     "connection close in response",
			req:      mustMakeRequest("GET", nil),
			resp:     &http.Response{Header: http.Header{"Connection": []string{"close"}}},
			expected: true,
		},
		{
			name:     "HTTP/1.0 request",
			req:      mustMakeRequestWithProto("GET", nil, "HTTP/1.0"),
			resp:     nil,
			expected: true,
		},
		{
			name:     "HTTP/1.1 request",
			req:      mustMakeRequestWithProto("GET", nil, "HTTP/1.1"),
			resp:     nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldCloseConnection(tt.req, tt.resp)
			if result != tt.expected {
				t.Errorf("shouldCloseConnection = %v, want %v", result, tt.expected)
			}
		})
	}
}

func mustMakeRequestWithProto(method string, headers map[string]string, proto string) *http.Request {
	req, err := http.NewRequest(method, "http://example.com", nil)
	if err != nil {
		panic(err)
	}
	req.Proto = proto
	if proto == "HTTP/1.0" {
		req.ProtoMajor = 1
		req.ProtoMinor = 0
	} else if proto == "HTTP/1.1" {
		req.ProtoMajor = 1
		req.ProtoMinor = 1
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// TestSerializeRequestWithBody tests SerializeRequest with a request body
func TestSerializeRequestWithBody(t *testing.T) {
	body := strings.NewReader("request body content")
	req, err := http.NewRequest("POST", "http://example.com/submit", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:5555"

	data, err := SerializeRequest(req)
	if err != nil {
		t.Fatalf("SerializeRequest: %v", err)
	}

	if !bytes.Contains(data, []byte("POST")) {
		t.Error("Missing method in serialized request")
	}
	if !bytes.Contains(data, []byte("request body content")) {
		t.Error("Missing body in serialized request")
	}
	if !bytes.Contains(data, []byte("X-Forwarded-For: 10.0.0.1:5555")) {
		t.Error("Missing X-Forwarded-For header")
	}
	if !bytes.Contains(data, []byte("X-Forwarded-Proto: http")) {
		t.Error("Missing X-Forwarded-Proto header")
	}
}

// TestSerializeRequestWithTLS tests SerializeRequest with TLS set
func TestSerializeRequestWithTLS(t *testing.T) {
	req, err := http.NewRequest("GET", "https://example.com/secure", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = "10.0.0.1:5555"
	req.TLS = &tls.ConnectionState{} // Set TLS to non-nil

	data, err := SerializeRequest(req)
	if err != nil {
		t.Fatalf("SerializeRequest: %v", err)
	}

	if !bytes.Contains(data, []byte("X-Forwarded-Proto: https")) {
		t.Error("Expected X-Forwarded-Proto: https for TLS request")
	}
}

// TestDeserializeResponseError tests DeserializeResponse with invalid data
func TestDeserializeResponseError(t *testing.T) {
	_, err := DeserializeResponse([]byte("this is not a valid HTTP response"))
	if err == nil {
		t.Error("Expected error for invalid response data")
	}
}

// TestShouldCloseConnectionHTTP0 tests shouldCloseConnection with HTTP/0.x (ProtoMajor < 1)
func TestShouldCloseConnectionHTTP0(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.ProtoMajor = 0
	req.ProtoMinor = 9

	result := shouldCloseConnection(req, nil)
	if !result {
		t.Error("shouldCloseConnection should return true for HTTP/0.x")
	}
}

// errReader is an io.Reader that always returns an error
type errReader struct{}

func (e *errReader) Read(p []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

// TestSerializeRequestWithErrorBody tests SerializeRequest when reading body fails
func TestSerializeRequestWithErrorBody(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.com/submit", &errReader{})
	req.RemoteAddr = "10.0.0.1:5555"

	_, err := SerializeRequest(req)
	if err == nil {
		t.Error("Expected error when reading body fails")
	}
}

// TestProxyRequestSerializeError tests ProxyRequest when request serialization fails
func TestProxyRequestSerializeError(t *testing.T) {
	srv := New(DefaultConfig(), nil)
	proxy := NewHTTPProxy(srv, 30*time.Second)

	// Create a mux with a pipe so OpenStream succeeds
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	m := mux.New(c1, mux.DefaultConfig())
	go m.Run()
	defer m.Close()

	// Drain frames on the other side so writes don't block
	m2 := mux.New(c2, mux.DefaultConfig())
	go m2.Run()
	defer m2.Close()

	rw := httptest.NewRecorder()
	// Create a request with a body that errors on read, which causes SerializeRequest to fail
	req := httptest.NewRequest("POST", "http://example.com/test", &errReader{})

	tunnel := &Tunnel{ID: "test-tunnel"}
	session := &Session{ID: "test-session", Mux: m}

	err := proxy.ProxyRequest(rw, req, tunnel, session)
	if err == nil {
		t.Error("Expected error from ProxyRequest with failing body")
	}
	if !strings.Contains(err.Error(), "serialize request") {
		t.Errorf("Expected 'serialize request' error, got: %v", err)
	}
}
