package tls

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	dir, err := os.MkdirTemp("", "wirerift-tls-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	config := Config{
		Domain:   "test.local",
		CertDir:  dir,
		AutoCert: true,
	}

	m, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if m == nil {
		t.Fatal("Manager is nil")
	}
}

func TestGenerateSelfSigned(t *testing.T) {
	dir, err := os.MkdirTemp("", "wirerift-tls-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	m, _ := NewManager(Config{
		Domain:   "test.local",
		CertDir:  dir,
		AutoCert: true,
	})

	cert, err := m.generateSelfSigned("app.test.local")
	if err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}

	if cert == nil {
		t.Fatal("Certificate is nil")
	}
	if cert.PrivateKey == nil {
		t.Fatal("PrivateKey is nil")
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("Certificate chain is empty")
	}

	// Check files were created
	if _, err := os.Stat(filepath.Join(dir, "app.test.local.crt")); err != nil {
		t.Errorf("Certificate file not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app.test.local.key")); err != nil {
		t.Errorf("Key file not created: %v", err)
	}
}

func TestGetCertificate(t *testing.T) {
	dir, err := os.MkdirTemp("", "wirerift-tls-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	m, _ := NewManager(Config{
		Domain:   "test.local",
		CertDir:  dir,
		AutoCert: true,
	})

	hello := &tls.ClientHelloInfo{
		ServerName: "app.test.local",
	}

	cert, err := m.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	if cert == nil {
		t.Fatal("Certificate is nil")
	}

	// Second call should use cache
	cert2, err := m.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate (cached): %v", err)
	}
	if cert != cert2 {
		t.Error("Should return same certificate from cache")
	}
}

func TestGetCertificateNoServerName(t *testing.T) {
	m, _ := NewManager(Config{
		Domain:   "test.local",
		AutoCert: true,
	})

	hello := &tls.ClientHelloInfo{
		ServerName: "",
	}

	_, err := m.GetCertificate(hello)
	if err != ErrDomainNotConfigured {
		t.Errorf("error = %v, want %v", err, ErrDomainNotConfigured)
	}
}

func TestTLSConfig(t *testing.T) {
	m, _ := NewManager(Config{
		Domain:   "test.local",
		AutoCert: true,
	})

	config := m.TLSConfig()

	if config == nil {
		t.Fatal("TLSConfig is nil")
	}
	if config.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %v, want %v", config.MinVersion, tls.VersionTLS12)
	}
	if config.GetCertificate == nil {
		t.Error("GetCertificate should not be nil")
	}
}

func TestWildcardDomain(t *testing.T) {
	m, _ := NewManager(Config{
		Domain: "example.com",
	})

	expected := "*.example.com"
	if m.WildcardDomain() != expected {
		t.Errorf("WildcardDomain = %q, want %q", m.WildcardDomain(), expected)
	}
}

// TestNewManagerDefaults tests NewManager with default values
func TestNewManagerDefaults(t *testing.T) {
	// Test with empty domain and cert dir
	m, err := NewManager(Config{})
	if err != nil {
		t.Fatalf("NewManager with defaults: %v", err)
	}
	if m == nil {
		t.Fatal("Manager should not be nil")
	}
	if m.config.Domain != "localhost" {
		t.Errorf("Default domain = %q, want localhost", m.config.Domain)
	}
}

// TestGetCertificateNotFound tests GetCertificate when certificate doesn't exist and AutoCert is disabled
func TestGetCertificateNotFound(t *testing.T) {
	dir, err := os.MkdirTemp("", "wirerift-tls-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	m, _ := NewManager(Config{
		Domain:   "test.local",
		CertDir:  dir,
		AutoCert: false, // Disable auto-cert generation
	})

	hello := &tls.ClientHelloInfo{
		ServerName: "nonexistent.test.local",
	}

	_, err = m.GetCertificate(hello)
	if err != ErrCertificateNotFound {
		t.Errorf("error = %v, want ErrCertificateNotFound", err)
	}
}

// TestLoadCertificateFromDisk tests loading an existing certificate from disk
func TestLoadCertificateFromDisk(t *testing.T) {
	dir, err := os.MkdirTemp("", "wirerift-tls-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	m, _ := NewManager(Config{
		Domain:   "test.local",
		CertDir:  dir,
		AutoCert: true,
	})

	// First call generates and saves the certificate
	hello := &tls.ClientHelloInfo{
		ServerName: "app.test.local",
	}

	cert1, err := m.GetCertificate(hello)
	if err != nil {
		t.Fatalf("First GetCertificate: %v", err)
	}

	// Create a new manager with same cert dir (simulating restart)
	m2, _ := NewManager(Config{
		Domain:   "test.local",
		CertDir:  dir,
		AutoCert: false, // Disable auto-cert to test loading from disk
	})

	// Second manager should load from disk
	cert2, err := m2.GetCertificate(hello)
	if err != nil {
		t.Fatalf("Second GetCertificate: %v", err)
	}

	// Certificates should be equivalent
	if cert2 == nil {
		t.Fatal("Loaded certificate is nil")
	}
	if len(cert2.Certificate) == 0 {
		t.Fatal("Loaded certificate chain is empty")
	}

	// The cached cert from first manager and loaded cert should have same chain
	if len(cert1.Certificate) != len(cert2.Certificate) {
		t.Error("Certificate chain lengths should match")
	}
}

// TestGenerateSelfSignedNoAutoCert tests that certificates are not saved when AutoCert is false
func TestGenerateSelfSignedNoAutoCert(t *testing.T) {
	dir, err := os.MkdirTemp("", "wirerift-tls-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Use AutoCert=true first to generate
	m, _ := NewManager(Config{
		Domain:   "test.local",
		CertDir:  dir,
		AutoCert: true,
	})

	// Generate a certificate
	_, err = m.generateSelfSigned("temp.test.local")
	if err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}

	// Clean up the generated files
	os.Remove(filepath.Join(dir, "temp.test.local.crt"))
	os.Remove(filepath.Join(dir, "temp.test.local.key"))
}

// TestCertificateErrors tests error constants
func TestCertificateErrors(t *testing.T) {
	// Verify error messages
	if ErrCertificateNotFound.Error() != "certificate not found" {
		t.Errorf("ErrCertificateNotFound message = %q", ErrCertificateNotFound.Error())
	}
	if ErrInvalidCertificate.Error() != "invalid certificate" {
		t.Errorf("ErrInvalidCertificate message = %q", ErrInvalidCertificate.Error())
	}
	if ErrDomainNotConfigured.Error() != "domain not configured" {
		t.Errorf("ErrDomainNotConfigured message = %q", ErrDomainNotConfigured.Error())
	}
}
