package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	buf := proxy.pool.Get().([]byte)
	if len(buf) != 32*1024 {
		t.Errorf("Buffer size = %d, want 32768", len(buf))
	}

	// Return to pool
	proxy.pool.Put(buf)
}

// TestProxyRequestNotImplemented tests that ProxyRequest returns not implemented error
func TestProxyRequestNotImplemented(t *testing.T) {
	srv := New(DefaultConfig(), nil)
	proxy := NewHTTPProxy(srv, 30*time.Second)

	// Create mock response writer and request
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	tunnel := &Tunnel{ID: "test-tunnel"}
	session := &Session{ID: "test-session"}

	err := proxy.ProxyRequest(rw, req, tunnel, session)
	if err == nil {
		t.Error("Expected error from unimplemented ProxyRequest")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("Expected 'not implemented' error, got: %v", err)
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
