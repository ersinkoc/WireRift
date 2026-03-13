package mux

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/wirerift/wirerift/internal/proto"
)

func newTestPipe(t *testing.T) (client, server *Mux) {
	t.Helper()

	c1, c2 := net.Pipe()

	client = New(c1, DefaultConfig())
	server = New(c2, DefaultConfig())

	return client, server
}

func TestMuxOpenStream(t *testing.T) {
	client, _ := newTestPipe(t)

	stream, err := client.OpenStream()
	if err != nil {
		t.Fatalf("OpenStream failed: %v", err)
	}

	if stream.ID() != 0 {
		t.Errorf("First stream ID = %d, want 0", stream.ID())
	}

	stream2, err := client.OpenStream()
	if err != nil {
		t.Fatalf("Second OpenStream failed: %v", err)
	}

	if stream2.ID() != 2 {
		t.Errorf("Second stream ID = %d, want 2", stream2.ID())
	}
}

func TestMuxHeartbeat(t *testing.T) {
	client, server := newTestPipe(t)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		server.Run()
	}()

	go func() {
		defer wg.Done()
		client.Run()
	}()

	// Send heartbeat from client
	frame := &proto.Frame{
		Version:  proto.Version,
		Type:     proto.FrameHeartbeat,
		StreamID: 0,
		Payload:  proto.HeartbeatPayload(),
	}

	if err := client.GetFrameWriter().Write(frame); err != nil {
		t.Fatalf("Send heartbeat: %v", err)
	}

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	client.Close()
	server.Close()
	wg.Wait()
}

func TestMuxClose(t *testing.T) {
	client, _ := newTestPipe(t)

	if err := client.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Should not be able to open new streams
	_, err := client.OpenStream()
	if err == nil {
		t.Error("Expected error after close")
	}

	// Done channel should be closed
	select {
	case <-client.Done():
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Error("Done channel not closed")
	}
}

func TestMuxMultipleStreams(t *testing.T) {
	client, _ := newTestPipe(t)

	// Open multiple streams
	streams := make([]*Stream, 10)
	for i := 0; i < 10; i++ {
		s, err := client.OpenStream()
		if err != nil {
			t.Fatalf("OpenStream %d: %v", i, err)
		}
		streams[i] = s
	}

	// Verify IDs are sequential even numbers
	for i, s := range streams {
		expected := uint32(i * 2)
		if s.ID() != expected {
			t.Errorf("Stream %d ID = %d, want %d", i, s.ID(), expected)
		}
	}

	client.Close()
}

func TestStreamIsClosed(t *testing.T) {
	client, _ := newTestPipe(t)
	stream, _ := client.OpenStream()

	if stream.IsClosed() {
		t.Error("New stream should not be closed")
	}
}

func TestStreamMetadata(t *testing.T) {
	stream := &Stream{
		readBuf:  newRingBuffer(4096),
		readCh:   make(chan struct{}, 1),
		windowCh: make(chan struct{}, 1),
	}
	stream.window.Store(proto.DefaultWindowSize)

	stream.SetMetadata("192.168.1.1:12345", "http")

	if stream.RemoteAddr() != "192.168.1.1:12345" {
		t.Errorf("RemoteAddr = %q, want %q", stream.RemoteAddr(), "192.168.1.1:12345")
	}
	if stream.Protocol() != "http" {
		t.Errorf("Protocol = %q, want %q", stream.Protocol(), "http")
	}
}

func TestMuxLocalRemoteAddr(t *testing.T) {
	client, server := newTestPipe(t)

	if client.LocalAddr() == nil {
		t.Error("LocalAddr should not be nil")
	}
	if client.RemoteAddr() == nil {
		t.Error("RemoteAddr should not be nil")
	}

	client.Close()
	server.Close()
}

func TestMuxConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxStreams <= 0 {
		t.Error("MaxStreams should be positive")
	}
	if cfg.WindowSize <= 0 {
		t.Error("WindowSize should be positive")
	}
	if cfg.MaxFrameSize <= 0 {
		t.Error("MaxFrameSize should be positive")
	}
	if cfg.HeartbeatInterval <= 0 {
		t.Error("HeartbeatInterval should be positive")
	}
}

func TestMuxHandleGoAway(t *testing.T) {
	client, server := newTestPipe(t)

	go server.Run()

	// Send GO_AWAY frame
	frame, _ := proto.EncodeJSONPayload(proto.FrameGoAway, 0, &proto.GoAway{
		Reason: "test",
	})
	if err := client.GetFrameWriter().Write(frame); err != nil {
		t.Fatalf("Send GO_AWAY: %v", err)
	}

	// Wait for mux to close
	select {
	case <-server.Done():
		// OK
	case <-time.After(1 * time.Second):
		t.Error("Server should have closed after GO_AWAY")
	}

	client.Close()
}

func BenchmarkMuxOpenStream(b *testing.B) {
	c1, c2 := net.Pipe()
	client := New(c1, DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.OpenStream()
		client.removeStream(uint32(i * 2))
	}

	c1.Close()
	c2.Close()
}

func BenchmarkMuxDataFrame(b *testing.B) {
	c1, c2 := net.Pipe()
	client := New(c1, DefaultConfig())
	// Start server reader to drain
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := c2.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	stream, _ := client.OpenStream()
	data := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.sendDataFrame(stream.ID(), data)
	}

	c1.Close()
	c2.Close()
}
