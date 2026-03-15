package server

import (
	"github.com/wirerift/wirerift/internal/proto"
)

// StreamOpenForTCP creates a STREAM_OPEN frame for a TCP connection.
func StreamOpenForTCP(tunnelID string, streamID uint32, remoteAddr string) (*proto.Frame, error) {
	msg := &proto.StreamOpen{
		TunnelID:   tunnelID,
		StreamID:   streamID,
		RemoteAddr: remoteAddr,
		Protocol:   "tcp",
	}
	return proto.EncodeJSONPayload(proto.FrameStreamOpen, streamID, msg)
}

// StreamOpenForHTTP creates a STREAM_OPEN frame for an HTTP connection.
func StreamOpenForHTTP(tunnelID string, streamID uint32, remoteAddr string) (*proto.Frame, error) {
	msg := &proto.StreamOpen{
		TunnelID:   tunnelID,
		StreamID:   streamID,
		RemoteAddr: remoteAddr,
		Protocol:   "http",
	}
	return proto.EncodeJSONPayload(proto.FrameStreamOpen, streamID, msg)
}
