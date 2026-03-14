package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestTokenValidation(t *testing.T) {
	m := NewManager()

	token, account, err := m.ValidateToken(m.devToken.Secret)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if token == nil {
		t.Fatal("Token should not be nil")
	}
	if account == nil {
		t.Fatal("Account should not be nil")
	}
	if account.ID != "dev_account" {
		t.Errorf("Account ID = %q, want %q", account.ID, "dev_account")
	}
}

func TestInvalidToken(t *testing.T) {
	m := NewManager()

	_, _, err := m.ValidateToken("invalid_token")
	if err != ErrInvalidToken {
		t.Errorf("Error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestTokenIsExpired(t *testing.T) {
	// Not expired
	token := &Token{
		ID:        "test",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if token.IsExpired() {
		t.Error("Token should not be expired")
	}

	// Expired
	expiredToken := &Token{
		ID:        "test",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	if !expiredToken.IsExpired() {
		t.Error("Token should be expired")
	}

	// Never expires (zero time)
	neverExpires := &Token{
		ID:        "test",
		CreatedAt: time.Now(),
		ExpiresAt: time.Time{},
	}
	if neverExpires.IsExpired() {
		t.Error("Token with zero ExpiresAt should not expire")
	}
}

func TestCreateAccount(t *testing.T) {
	m := NewManager()

	account, err := m.CreateAccount("test@example.com", "Test User")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if account.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", account.Email, "test@example.com")
	}
	if !account.Active {
		t.Error("Account should be active")
	}
}

func TestCreateAndRevokeToken(t *testing.T) {
	m := NewManager()

	// Create account and token
	account, _ := m.CreateAccount("test@example.com", "Test User")
	token, err := m.CreateToken(account.ID, "Test Token", 1*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// Validate token works
	_, _, err = m.ValidateToken(token.Secret)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	// Revoke token
	m.RevokeToken(token.ID)

	// Should fail now
	_, _, err = m.ValidateToken(token.Secret)
	if err != ErrInvalidToken {
		t.Errorf("Error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestMiddleware(t *testing.T) {
	m := NewManager()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := m.Middleware()
	protected := middleware(handler)

	// Test without auth
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Test with valid token
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+m.devToken.Secret)
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestBasicAuth(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := BasicAuth("admin", "secret")
	protected := middleware(handler)

	// Test without auth
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Test with valid credentials
	req = httptest.NewRequest("GET", "/", nil)
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	req.Header.Set("Authorization", "Basic "+auth)
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusOK)
	}

	// Test with invalid credentials
	req = httptest.NewRequest("GET", "/", nil)
	auth = base64.StdEncoding.EncodeToString([]byte("admin:wrong"))
	req.Header.Set("Authorization", "Basic "+auth)
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestDevToken tests the DevToken method
func TestDevToken(t *testing.T) {
	m := NewManager()

	// DevToken should return the development token secret
	token := m.DevToken()
	if token == "" {
		t.Error("DevToken should not be empty")
	}

	// DevToken should start with "dev_" prefix
	if len(token) < 4 || token[:4] != "dev_" {
		t.Errorf("DevToken should start with 'dev_', got: %s", token)
	}
}

// TestMiddlewareInvalidAuthHeader tests middleware with invalid auth headers
func TestMiddlewareInvalidAuthHeader(t *testing.T) {
	m := NewManager()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := m.Middleware()
	protected := middleware(handler)

	// Test with Bearer prefix but empty token
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Test with non-Bearer prefix
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestValidateTokenExpiredInStore tests validation of a token found in the store that is expired
func TestValidateTokenExpiredInStore(t *testing.T) {
	m := NewManager()

	// Create account
	account, _ := m.CreateAccount("test@example.com", "Test User")

	// Manually store an expired token
	expiredToken := &Token{
		ID:        "tk_expired",
		AccountID: account.ID,
		Name:      "Expired Token",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Secret:    "sk_expired_secret_12345",
	}
	m.tokens.Store(expiredToken.ID, expiredToken)

	// Validate should return ErrInvalidToken because the token is expired
	_, _, err := m.ValidateToken(expiredToken.Secret)
	if err != ErrInvalidToken {
		t.Errorf("Error = %v, want %v", err, ErrInvalidToken)
	}
}

// TestValidateTokenAccountNotFound tests validation of a token whose account doesn't exist
func TestValidateTokenAccountNotFound(t *testing.T) {
	m := NewManager()

	// Store a token with an account ID that doesn't exist
	orphanToken := &Token{
		ID:        "tk_orphan",
		AccountID: "nonexistent_account",
		Name:      "Orphan Token",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Secret:    "sk_orphan_secret_12345",
	}
	m.tokens.Store(orphanToken.ID, orphanToken)

	// Validate should return ErrInvalidToken because the account is not found
	_, _, err := m.ValidateToken(orphanToken.Secret)
	if err != ErrInvalidToken {
		t.Errorf("Error = %v, want %v", err, ErrInvalidToken)
	}
}

// TestCreateTokenInactiveAccount tests creating a token for an inactive account
func TestCreateTokenInactiveAccount(t *testing.T) {
	m := NewManager()

	// Create account and then deactivate it
	account, _ := m.CreateAccount("test@example.com", "Test User")
	account.Active = false

	// Try to create token for inactive account
	_, err := m.CreateToken(account.ID, "Test Token", 1*time.Hour)
	if err != ErrUnauthorized {
		t.Errorf("Error = %v, want %v", err, ErrUnauthorized)
	}
}

// TestDevTokenNil tests DevToken when devToken is nil
func TestDevTokenNil(t *testing.T) {
	m := &Manager{} // Create manager without calling NewManager, so devToken is nil

	token := m.DevToken()
	if token != "" {
		t.Errorf("DevToken() = %q, want empty string", token)
	}
}

// TestBasicAuthNonBasicPrefix tests BasicAuth with a non-Basic authorization prefix
func TestBasicAuthNonBasicPrefix(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := BasicAuth("admin", "secret")
	protected := middleware(handler)

	// Test with Bearer prefix instead of Basic
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestBasicAuthInvalidHeader tests BasicAuth with various invalid headers
func TestBasicAuthInvalidHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := BasicAuth("admin", "secret")
	protected := middleware(handler)

	// Test with Basic prefix but empty credentials
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic ")
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Test with Basic prefix but invalid base64
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic !!!invalid!!!")
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Test with Basic prefix but missing colon in decoded string
	req = httptest.NewRequest("GET", "/", nil)
	auth := base64.StdEncoding.EncodeToString([]byte("adminonly"))
	req.Header.Set("Authorization", "Basic "+auth)
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
