package proto

// Magic bytes for protocol identification.
// Sent by client immediately upon connection: "WRF" + version byte.
var Magic = []byte{0x57, 0x52, 0x46, 0x01} // "WRF\x01"

// Protocol constants.
const (
	// Version is the current protocol version.
	Version byte = 0x01

	// HeaderSize is the size of the frame header in bytes.
	HeaderSize = 9

	// MaxPayloadSize is the maximum payload size (16 MB).
	MaxPayloadSize = 16 << 20 // 16,777,216 bytes

	// MaxStreamID is the maximum stream ID (3 bytes, big-endian).
	MaxStreamID = 1<<24 - 1 // 16,777,215

	// ControlStreamID is the stream ID for control messages.
	ControlStreamID = 0

	// DefaultWindowSize is the default flow control window size (256 KB).
	DefaultWindowSize = 256 << 10 // 262,144 bytes
)

// FrameType represents the type of a frame.
type FrameType byte

// Frame type constants.
const (
	// Control frames (0x01-0x0F)
	FrameAuthReq     FrameType = 0x01 // Authentication request (C→S)
	FrameAuthRes     FrameType = 0x02 // Authentication response (S→C)
	FrameTunnelReq   FrameType = 0x03 // Tunnel open request (C→S)
	FrameTunnelRes   FrameType = 0x04 // Tunnel open response (S→C)
	FrameTunnelClose FrameType = 0x05 // Tunnel close (both)

	// Stream frames (0x10-0x1F)
	FrameStreamOpen   FrameType = 0x10 // New stream opened (S→C)
	FrameStreamData   FrameType = 0x11 // Stream data (both)
	FrameStreamClose  FrameType = 0x12 // Stream closed (both)
	FrameStreamRst    FrameType = 0x13 // Stream reset (both)
	FrameStreamWindow FrameType = 0x14 // Window update (both)

	// Heartbeat frames (0x20-0x2F)
	FrameHeartbeat    FrameType = 0x20 // Heartbeat ping (both)
	FrameHeartbeatAck FrameType = 0x21 // Heartbeat pong (both)

	// Error frames (0xF0-0xFF)
	FrameGoAway FrameType = 0xFE // Graceful shutdown (both)
	FrameError  FrameType = 0xFF // Protocol error (both)
)

// String returns a human-readable name for the frame type.
func (ft FrameType) String() string {
	switch ft {
	case FrameAuthReq:
		return "AUTH_REQ"
	case FrameAuthRes:
		return "AUTH_RES"
	case FrameTunnelReq:
		return "TUNNEL_REQ"
	case FrameTunnelRes:
		return "TUNNEL_RES"
	case FrameTunnelClose:
		return "TUNNEL_CLOSE"
	case FrameStreamOpen:
		return "STREAM_OPEN"
	case FrameStreamData:
		return "STREAM_DATA"
	case FrameStreamClose:
		return "STREAM_CLOSE"
	case FrameStreamRst:
		return "STREAM_RST"
	case FrameStreamWindow:
		return "STREAM_WINDOW"
	case FrameHeartbeat:
		return "HEARTBEAT"
	case FrameHeartbeatAck:
		return "HEARTBEAT_ACK"
	case FrameGoAway:
		return "GO_AWAY"
	case FrameError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
