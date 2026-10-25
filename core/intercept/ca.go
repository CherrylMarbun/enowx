// Package intercept is a host's HTTP interceptor: a proxy on loopback that
// terminates TLS with a certificate authority of the host's own, records
// what passes (HTTP/1.1, HTTP/2, WebSocket, server-sent events, gRPC),
// and applies rules that mock, forward, transform, delay, fail or pause
// requests. See docs/plans/traffic.md.
package intercept

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CA is the host's certificate authority, which signs a certificate for
// each server name the proxy terminates TLS for. Its key never leaves the
// host. It is safe for use by several goroutines.
type CA struct {
	Cert *x509.Certificate
	PEM  []byte // the certificate, to trust where traffic is caught
	key  crypto.Signer

	mu      sync.Mutex
	leaves  map[string]*tls.Certificate
	leafKey crypto.Signer
}

// LoadCA reads the CA kept in dir, making one the first time.
func LoadCA(dir string) (*CA, error) {
	certFile, keyFile := filepath.Join(dir, "ca.pem"), filepath.Join(dir, "ca-key.pem")
	certPEM, err1 := os.ReadFile(certFile)
	keyPEM, err2 := os.ReadFile(keyFile)
	if err1 == nil && err2 == nil {
		ca, err := parseCA(certPEM, keyPEM)
		if err == nil && time.Now().Before(ca.Cert.NotAfter.Add(-30*24*time.Hour)) {
			return ca, nil
		}
	} else if !errors.Is(err1, os.ErrNotExist) && err1 != nil {
		return nil, fmt.Errorf("intercept: %w", err1)
	}
	return NewCA(dir)
}

// NewCA makes a new CA in dir, replacing one there.
func NewCA(dir string) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	host, _ := os.Hostname()
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "enowX traffic CA (" + host + ")", Organization: []string{"enowX"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(5, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("intercept: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ca-key.pem"), keyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("intercept: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ca.pem"), certPEM, 0o644); err != nil {
		return nil, fmt.Errorf("intercept: %w", err)
	}
	return parseCA(certPEM, keyPEM)
}

func parseCA(certPEM, keyPEM []byte) (*CA, error) {
	cb, _ := pem.Decode(certPEM)
	kb, _ := pem.Decode(keyPEM)
	if cb == nil || kb == nil {
		return nil, errors.New("intercept: the CA's files aren't PEM")
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return nil, fmt.Errorf("intercept: %w", err)
	}
	k, err := x509.ParsePKCS8PrivateKey(kb.Bytes)
	if err != nil {
		return nil, fmt.Errorf("intercept: %w", err)
	}
	signer, ok := k.(crypto.Signer)
	if !ok {
		return nil, errors.New("intercept: the CA's key can't sign")
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &CA{Cert: cert, PEM: certPEM, key: signer, leaves: map[string]*tls.Certificate{}, leafKey: leafKey}, nil
}

// Fingerprint is the SHA-256 of the CA's certificate, in hex.
func (ca *CA) Fingerprint() string {
	sum := sha256.Sum256(ca.Cert.Raw)
	return hex.EncodeToString(sum[:])
}

// SPKIHash is the base64 SHA-256 of the CA's public key, which Chromium's
// --ignore-certificate-errors-spki-list takes.
func (ca *CA) SPKIHash() string {
	sum := sha256.Sum256(ca.Cert.RawSubjectPublicKeyInfo)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// Leaf is a certificate for name (a host name or an IP address), signed
// by the CA, made once and kept.
func (ca *CA) Leaf(name string) (*tls.Certificate, error) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	if c, ok := ca.leaves[name]; ok && time.Now().Before(c.Leaf.NotAfter) {
		return c, nil
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		// Short enough for every client's limit on leaf lifetimes.
		NotAfter:    time.Now().AddDate(0, 0, 90),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(name); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{name}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, ca.leafKey.Public(), ca.key)
	if err != nil {
		return nil, fmt.Errorf("intercept: certificate for %s: %w", name, err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	c := &tls.Certificate{Certificate: [][]byte{der, ca.Cert.Raw}, PrivateKey: ca.leafKey, Leaf: leaf}
	ca.leaves[name] = c
	return c, nil
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 126))
	return n
}
