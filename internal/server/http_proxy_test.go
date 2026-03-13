package server

import (
	"bytes"
	"net/http"
	"testing"
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
