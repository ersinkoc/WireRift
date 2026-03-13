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
