package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecover(t *testing.T) {
	logger := slog.Default()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	recovered := Recover(logger)(handler)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	recovered.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRecoverNoPanic(t *testing.T) {
	logger := slog.Default()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	recovered := Recover(logger)(handler)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	recovered.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCORSAllowed(t *testing.T) {
	allowedOrigins := []string{"http://example.com", "http://localhost:3000"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := CORS(allowedOrigins)(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	corsHandler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("CORS header = %q, want %q", rec.Header().Get("Access-Control-Allow-Origin"), "http://example.com")
	}
}

func TestCORSWildcard(t *testing.T) {
	allowedOrigins := []string{"*"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := CORS(allowedOrigins)(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	rec := httptest.NewRecorder()

	corsHandler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://any-origin.com" {
		t.Errorf("CORS header = %q, want %q", rec.Header().Get("Access-Control-Allow-Origin"), "http://any-origin.com")
	}
}

func TestCORSPreflight(t *testing.T) {
	allowedOrigins := []string{"http://example.com"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for preflight")
	})

	corsHandler := CORS(allowedOrigins)(handler)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	corsHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRequestID(t *testing.T) {
	var capturedID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = w.Header().Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	})

	requestIDHandler := RequestID()(handler)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	requestIDHandler.ServeHTTP(rec, req)

	if capturedID == "" {
		t.Error("Request ID should not be empty")
	}
}

func TestRequestIDExisting(t *testing.T) {
	existingID := "existing-id-123"
	var capturedID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = w.Header().Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	})

	requestIDHandler := RequestID()(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", existingID)
	rec := httptest.NewRecorder()

	requestIDHandler.ServeHTTP(rec, req)

	if capturedID != existingID {
		t.Errorf("Request ID = %q, want %q", capturedID, existingID)
	}
}

func TestHealthCheck(t *testing.T) {
	handler := HealthCheck()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != `{"status":"healthy"}` {
		t.Errorf("Body = %q, want %q", rec.Body.String(), `{"status":"healthy"}`)
	}
}

func TestReadyCheckAllPass(t *testing.T) {
	check1 := func() bool { return true }
	check2 := func() bool { return true }

	handler := ReadyCheck(check1, check2)

	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != `{"status":"ready"}` {
		t.Errorf("Body = %q, want %q", rec.Body.String(), `{"status":"ready"}`)
	}
}

func TestReadyCheckOneFails(t *testing.T) {
	check1 := func() bool { return true }
	check2 := func() bool { return false }

	handler := ReadyCheck(check1, check2)

	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestChain(t *testing.T) {
	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	chained := Chain(handler, m1, m2)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	chained.ServeHTTP(rec, req)

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Errorf("Order length = %d, want %d", len(order), len(expected))
	}
}

func TestResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)

	if rw.status != http.StatusCreated {
		t.Errorf("Status = %d, want %d", rw.status, http.StatusCreated)
	}

	// Second WriteHeader should be ignored
	rw.WriteHeader(http.StatusBadRequest)

	if rw.status != http.StatusCreated {
		t.Errorf("Status should remain %d", http.StatusCreated)
	}
}
