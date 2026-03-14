package proto

import (
	"bytes"
	"testing"
	"time"
)

func TestHeartbeatPayload(t *testing.T) {
	before := time.Now()
	payload := HeartbeatPayload()
	after := time.Now()

	parsed := ParseHeartbeat(payload)

	if parsed.Before(before.Add(-time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("Parsed timestamp %v not in expected range [%v, %v]", parsed, before, after)
	}
}

func TestHeartbeatPayloadRoundTrip(t *testing.T) {
	original := time.Now().Truncate(time.Nanosecond)
	payload := HeartbeatPayload()
	parsed := ParseHeartbeat(payload)

	// Allow 1ms tolerance for test execution
	diff := parsed.Sub(original)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Millisecond {
		t.Errorf("Timestamp drift = %v, want < 1ms", diff)
	}
}

func TestParseHeartbeatEmpty(t *testing.T) {
	parsed := ParseHeartbeat([]byte{})
	if !parsed.IsZero() {
		t.Errorf("Expected zero time for empty payload, got %v", parsed)
	}
}

func TestEncodeDecodeJSONPayload(t *testing.T) {
	authReq := &AuthRequest{
		Token:    "tk_abc123",
		ClientID: "cli_xyz",
		Version:  "1.0.0",
		OS:       "linux",
		Arch:     "amd64",
		Hostname: "test-host",
	}

	// Encode
	frame, err := EncodeJSONPayload(FrameAuthReq, ControlStreamID, authReq)
	if err != nil {
		t.Fatalf("EncodeJSONPayload failed: %v", err)
	}

	if frame.Type != FrameAuthReq {
		t.Errorf("Type = %v, want %v", frame.Type, FrameAuthReq)
	}
	if frame.StreamID != ControlStreamID {
		t.Errorf("StreamID = %d, want %d", frame.StreamID, ControlStreamID)
	}

	// Decode
	var got AuthRequest
	if err := DecodeJSONPayload(frame, &got); err != nil {
		t.Fatalf("DecodeJSONPayload failed: %v", err)
	}

	if got.Token != authReq.Token {
		t.Errorf("Token = %q, want %q", got.Token, authReq.Token)
	}
	if got.ClientID != authReq.ClientID {
		t.Errorf("ClientID = %q, want %q", got.ClientID, authReq.ClientID)
	}
}

func TestTunnelRequestHTTP(t *testing.T) {
	req := &TunnelRequest{
		Type:      TunnelTypeHTTP,
		Subdomain: "myapp",
		LocalAddr: "localhost:3000",
		Inspect:   true,
		Headers: map[string]string{
			"X-Forwarded-Proto": "https",
		},
	}

	frame, err := EncodeJSONPayload(FrameTunnelReq, ControlStreamID, req)
	if err != nil {
		t.Fatalf("EncodeJSONPayload failed: %v", err)
	}

	var got TunnelRequest
	if err := DecodeJSONPayload(frame, &got); err != nil {
		t.Fatalf("DecodeJSONPayload failed: %v", err)
	}

	if got.Type != TunnelTypeHTTP {
		t.Errorf("Type = %q, want %q", got.Type, TunnelTypeHTTP)
	}
	if got.Subdomain != "myapp" {
		t.Errorf("Subdomain = %q, want %q", got.Subdomain, "myapp")
	}
}

func TestTunnelRequestTCP(t *testing.T) {
	req := &TunnelRequest{
		Type:      TunnelTypeTCP,
		RemotePort: 0,
		LocalAddr: "localhost:5432",
	}

	frame, err := EncodeJSONPayload(FrameTunnelReq, ControlStreamID, req)
	if err != nil {
		t.Fatalf("EncodeJSONPayload failed: %v", err)
	}

	var got TunnelRequest
	if err := DecodeJSONPayload(frame, &got); err != nil {
		t.Fatalf("DecodeJSONPayload failed: %v", err)
	}

	if got.Type != TunnelTypeTCP {
		t.Errorf("Type = %q, want %q", got.Type, TunnelTypeTCP)
	}
}

func TestAuthResponse(t *testing.T) {
	tests := []struct {
		name string
		resp *AuthResponse
	}{
		{
			name: "success",
			resp: &AuthResponse{
				OK:                 true,
				SessionID:          "sess_abc123",
				ServerVersion:      "1.0.0",
				HeartbeatInterval:  30000,
				MaxTunnels:         10,
				MaxStreamsPerTunnel: 256,
			},
		},
		{
			name: "error",
			resp: &AuthResponse{
				OK:    false,
				Error: "invalid token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := EncodeJSONPayload(FrameAuthRes, ControlStreamID, tt.resp)
			if err != nil {
				t.Fatalf("EncodeJSONPayload failed: %v", err)
			}

			var got AuthResponse
			if err := DecodeJSONPayload(frame, &got); err != nil {
				t.Fatalf("DecodeJSONPayload failed: %v", err)
			}

			if got.OK != tt.resp.OK {
				t.Errorf("OK = %v, want %v", got.OK, tt.resp.OK)
			}
			if got.SessionID != tt.resp.SessionID {
				t.Errorf("SessionID = %q, want %q", got.SessionID, tt.resp.SessionID)
			}
		})
	}
}

func TestStreamWindowEncoding(t *testing.T) {
	sw := &StreamWindow{
		StreamID: 42,
		Delta:    65536,
	}

	frame, err := EncodeJSONPayload(FrameStreamWindow, 42, sw)
	if err != nil {
		t.Fatalf("EncodeJSONPayload failed: %v", err)
	}

	var got StreamWindow
	if err := DecodeJSONPayload(frame, &got); err != nil {
		t.Fatalf("DecodeJSONPayload failed: %v", err)
	}

	if got.StreamID != 42 {
		t.Errorf("StreamID = %d, want 42", got.StreamID)
	}
	if got.Delta != 65536 {
		t.Errorf("Delta = %d, want 65536", got.Delta)
	}
}

func TestGoAwayEncoding(t *testing.T) {
	ga := &GoAway{
		Reason:         "server_shutdown",
		Message:        "Server is shutting down for maintenance",
		ReconnectAfter: 5000,
	}

	frame, err := EncodeJSONPayload(FrameGoAway, ControlStreamID, ga)
	if err != nil {
		t.Fatalf("EncodeJSONPayload failed: %v", err)
	}

	var got GoAway
	if err := DecodeJSONPayload(frame, &got); err != nil {
		t.Fatalf("DecodeJSONPayload failed: %v", err)
	}

	if got.Reason != "server_shutdown" {
		t.Errorf("Reason = %q, want %q", got.Reason, "server_shutdown")
	}
}

func TestFullProtocolRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFrameWriter(&buf)
	fr := NewFrameReader(&buf)

	// Write magic
	if err := WriteMagic(&buf); err != nil {
		t.Fatalf("WriteMagic failed: %v", err)
	}

	// Client sends auth request
	authReq := &AuthRequest{Token: "tk_secret"}
	frame, _ := EncodeJSONPayload(FrameAuthReq, ControlStreamID, authReq)
	if err := fw.Write(frame); err != nil {
		t.Fatalf("Write auth request failed: %v", err)
	}

	// Server sends auth response
	authRes := &AuthResponse{OK: true, SessionID: "sess_123"}
	frame, _ = EncodeJSONPayload(FrameAuthRes, ControlStreamID, authRes)
	if err := fw.Write(frame); err != nil {
		t.Fatalf("Write auth response failed: %v", err)
	}

	// Read magic
	if err := ReadMagic(&buf); err != nil {
		t.Fatalf("ReadMagic failed: %v", err)
	}

	// Read auth request
	gotFrame, err := fr.Read()
	if err != nil {
		t.Fatalf("Read auth request failed: %v", err)
	}
	var gotAuthReq AuthRequest
	DecodeJSONPayload(gotFrame, &gotAuthReq)
	if gotAuthReq.Token != "tk_secret" {
		t.Errorf("Token = %q, want %q", gotAuthReq.Token, "tk_secret")
	}

	// Read auth response
	gotFrame, err = fr.Read()
	if err != nil {
		t.Fatalf("Read auth response failed: %v", err)
	}
	var gotAuthRes AuthResponse
	DecodeJSONPayload(gotFrame, &gotAuthRes)
	if gotAuthRes.SessionID != "sess_123" {
		t.Errorf("SessionID = %q, want %q", gotAuthRes.SessionID, "sess_123")
	}
}

func BenchmarkEncodeJSONPayload(b *testing.B) {
	authReq := &AuthRequest{
		Token:    "tk_abc123",
		ClientID: "cli_xyz",
		Version:  "1.0.0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeJSONPayload(FrameAuthReq, ControlStreamID, authReq)
	}
}

func BenchmarkDecodeJSONPayload(b *testing.B) {
	authReq := &AuthRequest{
		Token:    "tk_abc123",
		ClientID: "cli_xyz",
		Version:  "1.0.0",
	}
	frame, _ := EncodeJSONPayload(FrameAuthReq, ControlStreamID, authReq)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var got AuthRequest
		DecodeJSONPayload(frame, &got)
	}
}

// TestEncodeJSONPayloadError tests that EncodeJSONPayload returns an error for invalid values
func TestEncodeJSONPayloadError(t *testing.T) {
	// Try to encode a channel (cannot be marshaled to JSON)
	ch := make(chan int)
	_, err := EncodeJSONPayload(FrameAuthReq, ControlStreamID, ch)
	if err == nil {
		t.Error("Expected error when encoding channel, got nil")
	}

	// Try to encode a function (cannot be marshaled to JSON)
	fn := func() {}
	_, err = EncodeJSONPayload(FrameAuthReq, ControlStreamID, fn)
	if err == nil {
		t.Error("Expected error when encoding function, got nil")
	}
}
