package server

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
)

// SerializeRequest serializes an HTTP request for tunneling.
func SerializeRequest(r *http.Request) ([]byte, error) {
	var buf bytes.Buffer

	// Write request line
	buf.WriteString(r.Method)
	buf.WriteByte(' ')
	buf.WriteString(r.URL.String())
	buf.WriteByte(' ')
	buf.WriteString(r.Proto)
	buf.WriteString("\r\n")

	// Write headers
	for key, values := range r.Header {
		for _, value := range values {
			buf.WriteString(key)
			buf.WriteString(": ")
			buf.WriteString(value)
			buf.WriteString("\r\n")
		}
	}

	// Add forwarding headers
	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}
	buf.WriteString("X-Forwarded-For: ")
	if existing := r.Header.Get("X-Forwarded-For"); existing != "" {
		buf.WriteString(existing + ", " + clientIP)
	} else {
		buf.WriteString(clientIP)
	}
	buf.WriteString("\r\n")
	buf.WriteString("X-Forwarded-Proto: ")
	if r.TLS != nil {
		buf.WriteString("https")
	} else {
		buf.WriteString("http")
	}
	buf.WriteString("\r\n")
	buf.WriteString("X-Forwarded-Host: ")
	buf.WriteString(r.Host)
	buf.WriteString("\r\n")

	// End headers
	buf.WriteString("\r\n")

	// Write body (limit to 32 MB to prevent memory exhaustion)
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 32*1024*1024))
		if err != nil {
			return nil, err
		}
		buf.Write(body)
	}

	return buf.Bytes(), nil
}

// DeserializeResponse deserializes an HTTP response from tunneling.
func DeserializeResponse(data []byte) (*http.Response, error) {
	reader := bufio.NewReader(bytes.NewReader(data))
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// WriteResponse writes an HTTP response to the client.
func WriteResponse(w http.ResponseWriter, resp *http.Response) error {
	// Copy headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Copy body
	if resp.Body != nil {
		_, err := io.Copy(w, resp.Body)
		resp.Body.Close()
		return err
	}

	return nil
}

// IsWebSocketRequest checks if a request is a WebSocket upgrade.
func IsWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
