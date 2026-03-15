package proto

import (
	"bytes"
	"testing"
)

// FuzzReadFrame fuzzes the frame parser with random bytes.
// Run: go test -fuzz=FuzzReadFrame -fuzztime=30s ./internal/proto/
func FuzzReadFrame(f *testing.F) {
	// Seed corpus with valid frames
	validFrame := &Frame{
		Version:  Version,
		Type:     FrameAuthReq,
		StreamID: 1,
		Payload:  []byte(`{"token":"test"}`),
	}
	var buf bytes.Buffer
	validFrame.Encode(&buf)
	f.Add(buf.Bytes())

	// Seed with magic + minimal header
	f.Add([]byte{0x57, 0x52, 0x46, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00})

	// Seed with empty payload
	f.Add([]byte{0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})

	// Seed with large length field
	f.Add([]byte{0x01, 0xFF, 0x00, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF})

	// Seed with zero bytes
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		reader := NewFrameReader(bytes.NewReader(data))
		frame, err := reader.Read()
		if err != nil {
			return // Expected for malformed input
		}

		// If we got a frame, it should be re-encodable without panic
		if frame != nil {
			var out bytes.Buffer
			frame.Encode(&out) // Should not panic
		}
	})
}

// FuzzDecodeJSONPayload fuzzes JSON payload decoding.
func FuzzDecodeJSONPayload(f *testing.F) {
	// Valid auth request
	f.Add(byte(FrameAuthReq), uint32(0), []byte(`{"token":"abc"}`))
	// Valid tunnel request
	f.Add(byte(FrameTunnelReq), uint32(1), []byte(`{"type":"http","local_addr":"localhost:8080"}`))
	// Empty JSON
	f.Add(byte(FrameAuthReq), uint32(0), []byte(`{}`))
	// Invalid JSON
	f.Add(byte(FrameAuthReq), uint32(0), []byte(`{invalid`))
	// Huge nested JSON
	f.Add(byte(FrameAuthReq), uint32(0), []byte(`{"a":{"b":{"c":"d"}}}`))

	f.Fuzz(func(t *testing.T, frameType byte, streamID uint32, payload []byte) {
		frame := &Frame{
			Version:  Version,
			Type:     FrameType(frameType),
			StreamID: streamID,
			Payload:  payload,
		}

		// Try decoding as various message types - should not panic
		var auth AuthRequest
		DecodeJSONPayload(frame, &auth)

		var tunnel TunnelRequest
		DecodeJSONPayload(frame, &tunnel)

		var resp TunnelResponse
		DecodeJSONPayload(frame, &resp)

		var streamOpen StreamOpen
		DecodeJSONPayload(frame, &streamOpen)
	})
}

// FuzzReadMagic fuzzes the magic byte reader.
func FuzzReadMagic(f *testing.F) {
	f.Add([]byte{0x57, 0x52, 0x46, 0x01})         // valid
	f.Add([]byte{0x57, 0x52, 0x46, 0x02})         // wrong version
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})         // zeros
	f.Add([]byte{0x57, 0x52})                      // too short
	f.Add([]byte{})                                 // empty

	f.Fuzz(func(t *testing.T, data []byte) {
		ReadMagic(bytes.NewReader(data)) // Should not panic
	})
}
