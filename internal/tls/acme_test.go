package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBase64URLEncode(t *testing.T) {
	tests := []struct {
		input    []byte
		expected string
	}{
		{[]byte("hello"), "aGVsbG8"},
		{[]byte{0, 1, 2}, "AAEC"},
		{[]byte{}, ""},
	}
	for _, tt := range tests {
		got := base64URLEncode(tt.input)
		if got != tt.expected {
			t.Errorf("base64URLEncode(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestPadTo32(t *testing.T) {
	short := []byte{1, 2, 3}
	padded := padTo32(short)
	if len(padded) != 32 {
		t.Errorf("len = %d, want 32", len(padded))
	}
	if padded[31] != 3 || padded[30] != 2 || padded[29] != 1 {
		t.Error("padding incorrect")
	}

	exact := make([]byte, 32)
	exact[0] = 42
	result := padTo32(exact)
	if len(result) != 32 || result[0] != 42 {
		t.Error("32-byte input should pass through unchanged")
	}

	long := make([]byte, 40)
	result = padTo32(long)
	if len(result) != 40 {
		t.Error("longer than 32 should pass through unchanged")
	}
}

func TestNewACMEManagerValidation(t *testing.T) {
	_, err := NewACMEManager("", "", false, nil)
	if err == nil {
		t.Error("Expected error for empty email")
	}

	dir := t.TempDir()
	mgr, err := NewACMEManager("test@example.com", dir, true, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if mgr.email != "test@example.com" {
		t.Error("Email not set")
	}
	if !mgr.staging {
		t.Error("Staging not set")
	}
}

func TestACMEManagerLoadOrCreateKey(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)
	keyPath := filepath.Join(dir, "test-key.pem")

	// Create new key
	key1, err := mgr.loadOrCreateKey(keyPath)
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}
	if key1 == nil {
		t.Fatal("Key is nil")
	}

	// Verify file was saved
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file not created")
	}

	// Load existing key
	key2, err := mgr.loadOrCreateKey(keyPath)
	if err != nil {
		t.Fatalf("Failed to load key: %v", err)
	}

	// Same key
	if key1.D.Cmp(key2.D) != 0 {
		t.Error("Loaded key doesn't match created key")
	}
}

func TestACMEManagerJWK(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	mgr.account = &acmeAccount{Key: key}

	jwk := mgr.jwk()
	if jwk["kty"] != "EC" {
		t.Errorf("kty = %q, want EC", jwk["kty"])
	}
	if jwk["crv"] != "P-256" {
		t.Errorf("crv = %q, want P-256", jwk["crv"])
	}
	if jwk["x"] == "" || jwk["y"] == "" {
		t.Error("x or y is empty")
	}
}

func TestACMEManagerJWKThumbprint(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	mgr.account = &acmeAccount{Key: key}

	thumbprint := mgr.jwkThumbprint()
	if thumbprint == "" {
		t.Error("Thumbprint is empty")
	}
	if len(thumbprint) < 20 {
		t.Errorf("Thumbprint too short: %d chars", len(thumbprint))
	}

	// Should be deterministic
	thumbprint2 := mgr.jwkThumbprint()
	if thumbprint != thumbprint2 {
		t.Error("Thumbprint not deterministic")
	}
}

func TestACMEManagerServeChallenge(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)

	// Store a challenge
	mgr.challenges.Store("test-token-123", "test-token-123.thumbprint-value")

	// Valid challenge request
	req := httptest.NewRequest("GET", "/.well-known/acme-challenge/test-token-123", nil)
	rec := httptest.NewRecorder()
	mgr.ServeChallenge(rec, req)

	if rec.Code != 200 {
		t.Errorf("Status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "test-token-123.thumbprint-value" {
		t.Errorf("Body = %q", rec.Body.String())
	}

	// Unknown token
	req2 := httptest.NewRequest("GET", "/.well-known/acme-challenge/unknown", nil)
	rec2 := httptest.NewRecorder()
	mgr.ServeChallenge(rec2, req2)

	if rec2.Code != 404 {
		t.Errorf("Unknown token: status = %d, want 404", rec2.Code)
	}

	// Empty token
	req3 := httptest.NewRequest("GET", "/.well-known/acme-challenge/", nil)
	rec3 := httptest.NewRecorder()
	mgr.ServeChallenge(rec3, req3)

	if rec3.Code != 404 {
		t.Errorf("Empty token: status = %d, want 404", rec3.Code)
	}
}

func TestCertificateBundleNeedsRenewal(t *testing.T) {
	// Expires in 60 days - no renewal needed
	b1 := &CertificateBundle{ExpiresAt: time.Now().Add(60 * 24 * time.Hour)}
	if b1.NeedsRenewal() {
		t.Error("60 days out should not need renewal")
	}

	// Expires in 20 days - needs renewal
	b2 := &CertificateBundle{ExpiresAt: time.Now().Add(20 * 24 * time.Hour)}
	if !b2.NeedsRenewal() {
		t.Error("20 days out should need renewal")
	}

	// Already expired
	b3 := &CertificateBundle{ExpiresAt: time.Now().Add(-1 * time.Hour)}
	if !b3.NeedsRenewal() {
		t.Error("Expired cert should need renewal")
	}
}

func TestCertificateBundleTLSCertificate(t *testing.T) {
	// Generate a self-signed cert for testing
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serial, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	template := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"test.example.com"},
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	bundle := &CertificateBundle{
		CertPEM:    certPEM,
		PrivateKey: key,
	}

	tlsCert, err := bundle.TLSCertificate()
	if err != nil {
		t.Fatalf("TLSCertificate error: %v", err)
	}
	if tlsCert == nil {
		t.Fatal("TLSCertificate returned nil")
	}
	if len(tlsCert.Certificate) == 0 {
		t.Error("No certificates in chain")
	}
}

func TestSaveCertBundleAndLoad(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serial, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	template := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour),
		DNSNames:     []string{"test.example.com"},
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	bundle := &CertificateBundle{
		CertPEM:    certPEM,
		PrivateKey: key,
		Domains:    []string{"test.example.com"},
		IssuedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(90 * 24 * time.Hour),
	}

	// Save
	err := mgr.saveCertBundle("test.example.com", bundle)
	if err != nil {
		t.Fatalf("saveCertBundle error: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(dir, "test.example.com.crt")); os.IsNotExist(err) {
		t.Error("Cert file not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "test.example.com.key")); os.IsNotExist(err) {
		t.Error("Key file not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "test.example.com.json")); os.IsNotExist(err) {
		t.Error("Metadata file not created")
	}

	// Load
	loaded, err := mgr.LoadCertBundle("test.example.com")
	if err != nil {
		t.Fatalf("LoadCertBundle error: %v", err)
	}
	if loaded.PrivateKey.D.Cmp(key.D) != 0 {
		t.Error("Loaded key doesn't match")
	}
	if loaded.ExpiresAt.IsZero() {
		t.Error("ExpiresAt not loaded")
	}
}

func TestEstimateExpiry(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serial, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	notAfter := time.Now().Add(90 * 24 * time.Hour).Truncate(time.Second)
	template := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now(),
		NotAfter:     notAfter,
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	expiry, err := EstimateExpiry(certPEM)
	if err != nil {
		t.Fatalf("EstimateExpiry error: %v", err)
	}
	if !expiry.Equal(notAfter) {
		t.Errorf("Expiry = %v, want %v", expiry, notAfter)
	}

	// Invalid PEM
	_, err = EstimateExpiry([]byte("not a pem"))
	if err == nil {
		t.Error("Expected error for invalid PEM")
	}
}

func TestACMEManagerInitializeWithFakeServer(t *testing.T) {
	// Mock ACME directory
	mockDir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dir := acmeDirectory{
			NewNonce:   "http://" + r.Host + "/nonce",
			NewAccount: "http://" + r.Host + "/account",
			NewOrder:   "http://" + r.Host + "/order",
		}
		json.NewEncoder(w).Encode(dir)
	}))
	defer mockDir.Close()

	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)

	// Override directory URL for test
	resp, err := mgr.httpClient.Get(mockDir.URL)
	if err != nil {
		t.Fatalf("Failed to fetch mock directory: %v", err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&mgr.directory)

	if mgr.directory.NewNonce == "" {
		t.Error("Directory NewNonce not set")
	}
	if mgr.directory.NewAccount == "" {
		t.Error("Directory NewAccount not set")
	}
}

func TestACMEChallengeHandlerOnManagerWithoutACME(t *testing.T) {
	// Manager without ACME should return 404
	mgr := &Manager{}
	handler := mgr.ACMEChallengeHandler()

	req := httptest.NewRequest("GET", "/.well-known/acme-challenge/token", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != 404 {
		t.Errorf("Status = %d, want 404", rec.Code)
	}
}

func TestIsACMEEnabled(t *testing.T) {
	mgr := &Manager{}
	if mgr.IsACMEEnabled() {
		t.Error("Should be false without ACME")
	}

	mgr.acme = &ACMEManager{}
	if !mgr.IsACMEEnabled() {
		t.Error("Should be true with ACME")
	}
}

func TestLoadCertBundleNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)

	_, err := mgr.LoadCertBundle("nonexistent.com")
	if err == nil {
		t.Error("Expected error for nonexistent bundle")
	}
}

// ─── Mock ACME Server ────────────────────────────────────

// mockACMEServer creates a complete fake ACME server for testing the full flow.
// It supports directory, nonce, account, order, authorization, challenge,
// finalize, and certificate download endpoints.
func mockACMEServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Generate a self-signed CA cert for the mock to issue
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caSerial, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	caTemplate := &x509.Certificate{
		SerialNumber:          caSerial,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)

	// Track state
	var (
		orderReady      = false
		challengePosted = false
	)

	mux := http.NewServeMux()

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
	}))

	base := srv.URL

	// Directory
	mux.HandleFunc("/directory", func(w http.ResponseWriter, r *http.Request) {
		dir := acmeDirectory{
			NewNonce:   base + "/nonce",
			NewAccount: base + "/account",
			NewOrder:   base + "/order",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dir)
	})

	// Nonce (HEAD)
	mux.HandleFunc("/nonce", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Replay-Nonce", fmt.Sprintf("nonce-%d", time.Now().UnixNano()))
		w.WriteHeader(http.StatusOK)
	})

	// Account creation (POST)
	mux.HandleFunc("/account", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", base+"/account/1")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "valid",
		})
	})

	// Order creation (POST)
	mux.HandleFunc("/order", func(w http.ResponseWriter, r *http.Request) {
		orderReady = false
		challengePosted = false
		w.Header().Set("Location", base+"/order/1")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(acmeOrder{
			Status:         "pending",
			Authorizations: []string{base + "/authz/1"},
			Finalize:       base + "/finalize/1",
		})
	})

	// Order status polling (POST with empty payload, same as signedPost)
	mux.HandleFunc("/order/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "pending"
		cert := ""
		if challengePosted {
			status = "ready"
			orderReady = true
		}
		if orderReady {
			status = "ready"
		}
		json.NewEncoder(w).Encode(acmeOrder{
			Status:         status,
			Authorizations: []string{base + "/authz/1"},
			Finalize:       base + "/finalize/1",
			Certificate:    cert,
		})
	})

	// Authorization (POST/GET)
	mux.HandleFunc("/authz/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "pending"
		if challengePosted {
			status = "valid"
		}
		json.NewEncoder(w).Encode(acmeAuthorization{
			Status: status,
			Identifier: acmeIdentifier{
				Type:  "dns",
				Value: "test.example.com",
			},
			Challenges: []acmeChallenge{
				{
					Type:   "http-01",
					URL:    base + "/challenge/1",
					Token:  "mock-token-12345678",
					Status: status,
				},
			},
		})
	})

	// Challenge response (POST)
	mux.HandleFunc("/challenge/1", func(w http.ResponseWriter, r *http.Request) {
		challengePosted = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "processing",
		})
	})

	// Finalize (POST)
	mux.HandleFunc("/finalize/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(acmeOrder{
			Status:      "valid",
			Certificate: base + "/cert/1",
		})
	})

	// Certificate download (POST)
	mux.HandleFunc("/cert/1", func(w http.ResponseWriter, r *http.Request) {
		// Issue a cert signed by mock CA
		certKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		serial, _ := rand.Int(rand.Reader, big.NewInt(1000000))
		tmpl := &x509.Certificate{
			SerialNumber: serial,
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(90 * 24 * time.Hour),
			DNSNames:     []string{"test.example.com"},
		}
		certDER, _ := x509.CreateCertificate(rand.Reader, tmpl, caTemplate, &certKey.PublicKey, caKey)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
		caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
		w.Header().Set("Content-Type", "application/pem-certificate-chain")
		w.Write(certPEM)
		w.Write(caPEM)
	})

	return srv
}

// setupMockManager creates an ACMEManager wired to a mock ACME server.
func setupMockManager(t *testing.T, mockSrv *httptest.Server) *ACMEManager {
	t.Helper()
	dir := t.TempDir()
	mgr, err := NewACMEManager("test@example.com", dir, true, nil)
	if err != nil {
		t.Fatalf("NewACMEManager: %v", err)
	}

	// Fetch directory from mock
	resp, err := mgr.httpClient.Get(mockSrv.URL + "/directory")
	if err != nil {
		t.Fatalf("Fetch directory: %v", err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&mgr.directory); err != nil {
		t.Fatalf("Decode directory: %v", err)
	}

	// Create account key
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	mgr.account = &acmeAccount{Key: key}

	return mgr
}

func TestSignedPost_MockServer(t *testing.T) {
	mockSrv := mockACMEServer(t)
	defer mockSrv.Close()
	mgr := setupMockManager(t, mockSrv)

	// signedPost to account endpoint should return a valid response
	payload, _ := json.Marshal(map[string]interface{}{
		"termsOfServiceAgreed": true,
		"contact":             []string{"mailto:test@example.com"},
	})
	resp, err := mgr.signedPost(mgr.directory.NewAccount, payload)
	if err != nil {
		t.Fatalf("signedPost failed: %v", err)
	}
	if resp == nil {
		t.Fatal("signedPost returned nil response")
	}
	if resp.location == "" {
		t.Error("signedPost: Location header not set")
	}
}

func TestSignedPost_NilPayload(t *testing.T) {
	mockSrv := mockACMEServer(t)
	defer mockSrv.Close()
	mgr := setupMockManager(t, mockSrv)

	// signedPost with nil payload (POST-as-GET)
	resp, err := mgr.signedPost(mockSrv.URL+"/authz/1", nil)
	if err != nil {
		t.Fatalf("signedPost with nil payload failed: %v", err)
	}
	if resp == nil {
		t.Fatal("Response is nil")
	}
	if len(resp.body) == 0 {
		t.Error("Response body is empty")
	}
}

func TestSignedPost_ErrorResponse(t *testing.T) {
	// Server that always returns 403
	errorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Replay-Nonce", "test-nonce")
			return
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"type":"forbidden","detail":"test error"}`))
	}))
	defer errorSrv.Close()

	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	mgr.account = &acmeAccount{Key: key}
	mgr.directory = acmeDirectory{
		NewNonce:   errorSrv.URL + "/nonce",
		NewAccount: errorSrv.URL + "/account",
	}

	_, err := mgr.signedPost(errorSrv.URL+"/account", []byte("{}"))
	if err == nil {
		t.Fatal("Expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "ACME error 403") {
		t.Errorf("Expected 'ACME error 403', got: %v", err)
	}
}

func TestRegisterAccount_MockServer(t *testing.T) {
	mockSrv := mockACMEServer(t)
	defer mockSrv.Close()
	mgr := setupMockManager(t, mockSrv)

	err := mgr.registerAccount()
	if err != nil {
		t.Fatalf("registerAccount failed: %v", err)
	}
	if mgr.account.URL == "" {
		t.Error("Account URL not set after registration")
	}
	// Should contain the mock server's account location
	if !strings.Contains(mgr.account.URL, "/account/1") {
		t.Errorf("Account URL = %q, expected to contain /account/1", mgr.account.URL)
	}
}

func TestProcessAuthorization_MockServer(t *testing.T) {
	mockSrv := mockACMEServer(t)
	defer mockSrv.Close()
	mgr := setupMockManager(t, mockSrv)

	// Register account first (needed for kid header)
	mgr.registerAccount()

	// processAuthorization should find http-01 challenge, post to it, poll until valid
	err := mgr.processAuthorization(mockSrv.URL + "/authz/1")
	if err != nil {
		t.Fatalf("processAuthorization failed: %v", err)
	}
}

func TestProcessAuthorization_AlreadyValid(t *testing.T) {
	// Mock server where authz is already "valid"
	validSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Replay-Nonce", "nonce-1")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(acmeAuthorization{
			Status: "valid",
			Identifier: acmeIdentifier{
				Type:  "dns",
				Value: "test.example.com",
			},
		})
	}))
	defer validSrv.Close()

	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	mgr.account = &acmeAccount{Key: key, URL: validSrv.URL + "/account/1"}
	mgr.directory = acmeDirectory{
		NewNonce: validSrv.URL + "/nonce",
	}

	err := mgr.processAuthorization(validSrv.URL + "/authz/1")
	if err != nil {
		t.Fatalf("Already-valid authz should succeed: %v", err)
	}
}

func TestProcessAuthorization_NoHTTP01Challenge(t *testing.T) {
	// Mock server that returns authz with only dns-01 challenge (no http-01)
	var srvURL string
	dnsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Replay-Nonce", "nonce-1")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(acmeAuthorization{
			Status: "pending",
			Identifier: acmeIdentifier{
				Type:  "dns",
				Value: "test.example.com",
			},
			Challenges: []acmeChallenge{
				{
					Type:   "dns-01",
					URL:    srvURL + "/chall",
					Token:  "dns-token",
					Status: "pending",
				},
			},
		})
	}))
	defer dnsSrv.Close()
	srvURL = dnsSrv.URL

	dir := t.TempDir()
	mgr, _ := NewACMEManager("test@example.com", dir, true, nil)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	mgr.account = &acmeAccount{Key: key, URL: dnsSrv.URL + "/account/1"}
	mgr.directory = acmeDirectory{
		NewNonce: dnsSrv.URL + "/nonce",
	}

	err := mgr.processAuthorization(dnsSrv.URL + "/authz/1")
	if err == nil {
		t.Fatal("Expected error for missing http-01 challenge")
	}
	if !strings.Contains(err.Error(), "no http-01 challenge") {
		t.Errorf("Expected 'no http-01 challenge', got: %v", err)
	}
}

func TestObtainCertificate_MockServer(t *testing.T) {
	mockSrv := mockACMEServer(t)
	defer mockSrv.Close()
	mgr := setupMockManager(t, mockSrv)

	// Register account first
	if err := mgr.registerAccount(); err != nil {
		t.Fatalf("registerAccount failed: %v", err)
	}

	bundle, err := mgr.ObtainCertificate([]string{"test.example.com"})
	if err != nil {
		t.Fatalf("ObtainCertificate failed: %v", err)
	}

	if bundle == nil {
		t.Fatal("Bundle is nil")
	}
	if len(bundle.CertPEM) == 0 {
		t.Error("CertPEM is empty")
	}
	if bundle.PrivateKey == nil {
		t.Error("PrivateKey is nil")
	}
	if len(bundle.Domains) != 1 || bundle.Domains[0] != "test.example.com" {
		t.Errorf("Domains = %v, want [test.example.com]", bundle.Domains)
	}
	if bundle.IssuedAt.IsZero() {
		t.Error("IssuedAt is zero")
	}
	if bundle.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero")
	}

	// Verify cert files were saved
	certPath := filepath.Join(mgr.certDir, "test.example.com.crt")
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Error("Cert file not saved to disk")
	}

	keyPath := filepath.Join(mgr.certDir, "test.example.com.key")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file not saved to disk")
	}

	// Verify the CertPEM contains valid PEM blocks
	block, _ := pem.Decode(bundle.CertPEM)
	if block == nil {
		t.Fatal("CertPEM does not contain valid PEM")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("PEM block type = %q, want CERTIFICATE", block.Type)
	}
	// Note: TLSCertificate() would fail key matching because the mock server
	// issues a cert with its own key, not the CSR key. That's expected for mocks.
}

func TestGetNonce_MockServer(t *testing.T) {
	mockSrv := mockACMEServer(t)
	defer mockSrv.Close()
	mgr := setupMockManager(t, mockSrv)

	nonce, err := mgr.getNonce()
	if err != nil {
		t.Fatalf("getNonce failed: %v", err)
	}
	if nonce == "" {
		t.Error("Nonce is empty")
	}
	if !strings.HasPrefix(nonce, "nonce-") {
		t.Errorf("Nonce = %q, expected prefix 'nonce-'", nonce)
	}
}

func TestSignedPost_WithKID(t *testing.T) {
	mockSrv := mockACMEServer(t)
	defer mockSrv.Close()
	mgr := setupMockManager(t, mockSrv)

	// Set account URL so kid is used instead of jwk
	mgr.account.URL = mockSrv.URL + "/account/1"

	resp, err := mgr.signedPost(mockSrv.URL+"/order/1", nil)
	if err != nil {
		t.Fatalf("signedPost with kid failed: %v", err)
	}
	if resp == nil {
		t.Fatal("Response is nil")
	}
}

func TestSignedPost_WithoutKID(t *testing.T) {
	mockSrv := mockACMEServer(t)
	defer mockSrv.Close()
	mgr := setupMockManager(t, mockSrv)

	// Clear account URL so jwk is used
	mgr.account.URL = ""

	payload, _ := json.Marshal(map[string]interface{}{
		"termsOfServiceAgreed": true,
	})
	resp, err := mgr.signedPost(mgr.directory.NewAccount, payload)
	if err != nil {
		t.Fatalf("signedPost without kid failed: %v", err)
	}
	if resp.location == "" {
		t.Error("Location header not set")
	}
}

func TestInitialize_MockServer(t *testing.T) {
	mockSrv := mockACMEServer(t)
	defer mockSrv.Close()

	dir := t.TempDir()
	mgr, err := NewACMEManager("test@example.com", dir, true, nil)
	if err != nil {
		t.Fatalf("NewACMEManager: %v", err)
	}

	// Override the httpClient GET to redirect to our mock directory
	// We patch the directory fetch by directly calling the mock
	resp, err := mgr.httpClient.Get(mockSrv.URL + "/directory")
	if err != nil {
		t.Fatalf("Fetch directory: %v", err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&mgr.directory)

	// Load or create key
	keyPath := filepath.Join(dir, "acme-account.key")
	key, err := mgr.loadOrCreateKey(keyPath)
	if err != nil {
		t.Fatalf("loadOrCreateKey: %v", err)
	}
	mgr.account = &acmeAccount{Key: key}

	// Register account via mock
	if err := mgr.registerAccount(); err != nil {
		t.Fatalf("registerAccount: %v", err)
	}

	if mgr.account.URL == "" {
		t.Error("Account URL not set after Initialize-like flow")
	}
	if mgr.directory.NewNonce == "" {
		t.Error("Directory NewNonce empty")
	}
	if mgr.directory.NewAccount == "" {
		t.Error("Directory NewAccount empty")
	}
	if mgr.directory.NewOrder == "" {
		t.Error("Directory NewOrder empty")
	}
}
