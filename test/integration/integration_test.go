package integration

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/wirerift/wirerift/internal/mux"
	"github.com/wirerift/wirerift/internal/proto"
)

// TestMuxRoundTrip tests basic mux communication
func TestMuxRoundTrip(t *testing.T) {
	// Create a pipe for client-server communication
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// Server side
	go func() {
		defer wg.Done()
		// Read magic
		if err := proto.ReadMagic(serverConn); err != nil {
			t.Errorf("Server read magic: %v", err)
			return
		}
		serverMux := mux.New(serverConn, mux.DefaultConfig())
		go serverMux.Run()
	}()

	// Client side
	go func() {
		defer wg.Done()
		// Write magic
		if err := proto.WriteMagic(clientConn); err != nil {
			t.Errorf("Client write magic: %v", err)
			return
		}
		clientMux := mux.New(clientConn, mux.DefaultConfig())
		go clientMux.Run()

		// Wait for mux to be ready
		time.Sleep(100 * time.Millisecond)

		// Clean close
		clientMux.Close()
	}()

	wg.Wait()
}
