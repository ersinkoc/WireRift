package mux

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestRingBufferWriteRead(t *testing.T) {
	rb := newRingBuffer(1024)

	data := []byte("hello world")
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}

	p := make([]byte, len(data))
	n, err = rb.Read(p)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Read returned %d, want %d", n, len(data))
	}
	if string(p) != string(data) {
		t.Errorf("Read = %q, want %q", string(p), string(data))
	}
}

func TestRingBufferWrapAround(t *testing.T) {
	rb := newRingBuffer(16)

	// Write more than buffer size to force wrap-around
	data := make([]byte, 32)
	for i := range data {
		data[i] = byte(i)
	}

	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	// Should grow to fit
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}

	// Read all
	p := make([]byte, len(data))
	n, err = rb.Read(p)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Read returned %d, want %d", n, len(data))
	}
	if !bytes.Equal(p, data) {
		t.Error("Data mismatch after wrap-around")
	}
}

func TestRingBufferGrow(t *testing.T) {
	rb := newRingBuffer(16)

	// Write more than initial size
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}

	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}
	if rb.size < len(data) {
		t.Errorf("Buffer size = %d, want >= %d", rb.size, len(data))
	}
}

func TestRingBufferEmpty(t *testing.T) {
	rb := newRingBuffer(1024)

	p := make([]byte, 10)
	n, err := rb.Read(p)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 0 {
		t.Errorf("Read on empty buffer returned %d, want 0", n)
	}
}

func TestRingBufferReset(t *testing.T) {
	rb := newRingBuffer(1024)

	rb.Write([]byte("hello"))
	if rb.Len() != 5 {
		t.Errorf("Len = %d, want 5", rb.Len())
	}

	rb.Reset()
	if rb.Len() != 0 {
		t.Errorf("After Reset, Len = %d, want 0", rb.Len())
	}
}

func TestRingBufferPartialRead(t *testing.T) {
	rb := newRingBuffer(1024)

	data := []byte("hello world")
	rb.Write(data)

	// Read less than available
	p := make([]byte, 5)
	n, _ := rb.Read(p)
	if string(p[:n]) != "hello" {
		t.Errorf("First read = %q, want %q", string(p[:n]), "hello")
	}

	// Read rest
	p = make([]byte, 10)
	n, _ = rb.Read(p)
	if string(p[:n]) != " world" {
		t.Errorf("Second read = %q, want %q", string(p[:n]), " world")
	}
}

func TestRingBufferConcurrent(t *testing.T) {
	rb := newRingBuffer(4096)
	done := make(chan bool)

	// Writer
	go func() {
		for i := 0; i < 1000; i++ {
			data := []byte{byte(i % 256)}
			rb.Write(data)
		}
		done <- true
	}()

	// Reader
	go func() {
		count := 0
		p := make([]byte, 1)
		for count < 1000 {
			n, _ := rb.Read(p)
			count += n
		}
		done <- true
	}()

	<-done
	<-done
}

func BenchmarkRingBufferWrite(b *testing.B) {
	rb := newRingBuffer(64 * 1024)
	data := make([]byte, 1024)
	rand.Read(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Write(data)
		rb.Reset()
	}
}

func BenchmarkRingBufferRead(b *testing.B) {
	rb := newRingBuffer(64 * 1024)
	data := make([]byte, 1024)
	rb.Write(data)
	p := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Read(p)
		rb.Write(data)
	}
}
