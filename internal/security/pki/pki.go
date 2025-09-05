package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"bytes"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Paths returns default file paths for a given PKI directory and name prefix.
func Paths(dir, name string) (caCert, caKey, cert, key string) {
	return filepath.Join(dir, "ca.pem"), filepath.Join(dir, "ca.key"), filepath.Join(dir, name+".pem"), filepath.Join(dir, name+".key")
}

// SignCSR signs a PEM-encoded PKCS#10 CSR with the provided CA and returns a PEM-encoded certificate.
// The issued certificate will have either client or server EKU depending on isServer and will be valid for 'validity'.
func SignCSR(caCert *x509.Certificate, caKey *rsa.PrivateKey, csrPEM []byte, isServer bool, validity time.Duration) ([]byte, error) {
    block, _ := pem.Decode(csrPEM)
    if block == nil || block.Type != "CERTIFICATE REQUEST" {
        return nil, errors.New("invalid CSR PEM")
    }
    csr, err := x509.ParseCertificateRequest(block.Bytes)
    if err != nil {
        return nil, err
    }
    if err := csr.CheckSignature(); err != nil {
        return nil, fmt.Errorf("csr signature invalid: %w", err)
    }
    serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
    tmpl := &x509.Certificate{
        SerialNumber: serial,
        Subject:      csr.Subject,
        NotBefore:    time.Now().Add(-5 * time.Minute),
        NotAfter:     time.Now().Add(validity),
        KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
        ExtKeyUsage: func() []x509.ExtKeyUsage {
            if isServer { return []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth} }
            return []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
        }(),
        DNSNames:     csr.DNSNames,
        IPAddresses:  csr.IPAddresses,
    }
    der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, csr.PublicKey, caKey)
    if err != nil { return nil, err }
    var b bytes.Buffer
    if err := pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil { return nil, err }
    return b.Bytes(), nil
}

// EnsureDir ensures a directory exists.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0700)
}

// EnsureCA creates a self-signed CA if not present and returns the CA cert and key.
func EnsureCA(dir string, commonName string, validity time.Duration) (*x509.Certificate, *rsa.PrivateKey, error) {
	if err := EnsureDir(dir); err != nil {
		return nil, nil, err
	}
	caCertPath, caKeyPath, _, _ := Paths(dir, "")
	// Try to load existing
	if _, err := os.Stat(caCertPath); err == nil {
		cert, key, err := LoadCA(caCertPath, caKeyPath)
		return cert, key, err
	}
	// Create new CA
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{CommonName: commonName, Organization: []string{"Vertera"}},
		NotBefore: time.Now().Add(-5 * time.Minute),
		NotAfter:  time.Now().Add(validity),
		IsCA:      true,
		KeyUsage:  x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	if err := writeCertKey(caCertPath, caKeyPath, der, key); err != nil {
		return nil, nil, err
	}
	cert, _ := x509.ParseCertificate(der)
	return cert, key, nil
}

func LoadCA(certPath, keyPath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	crt, err := os.ReadFile(certPath)
	if err != nil { return nil, nil, err }
	blk, _ := pem.Decode(crt)
	if blk == nil { return nil, nil, errors.New("invalid ca cert pem") }
	cert, err := x509.ParseCertificate(blk.Bytes)
	if err != nil { return nil, nil, err }
	kb, err := os.ReadFile(keyPath)
	if err != nil { return nil, nil, err }
	kblk, _ := pem.Decode(kb)
	if kblk == nil { return nil, nil, errors.New("invalid ca key pem") }
	key, err := x509.ParsePKCS1PrivateKey(kblk.Bytes)
	if err != nil { return nil, nil, err }
	return cert, key, nil
}

// IssueCertificate issues a server or client certificate signed by the CA.
// If hosts is non-empty, they are added as DNS and IP SANs.
func IssueCertificate(dir, name, commonName string, isServer bool, caCert *x509.Certificate, caKey *rsa.PrivateKey, validity time.Duration, hosts []string) (certPath, keyPath string, err error) {
	if err = EnsureDir(dir); err != nil { return "", "", err }
	_, _, certPath, keyPath = Paths(dir, name)
	// If already exists, do not overwrite
	if _, err = os.Stat(certPath); err == nil {
		return certPath, keyPath, nil
	}
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil { return "", "", err }
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{CommonName: commonName, Organization: []string{"Vertera"}},
		NotBefore: time.Now().Add(-5 * time.Minute),
		NotAfter:  time.Now().Add(validity),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: func() []x509.ExtKeyUsage {
			if isServer { return []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth} }
			return []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}(),
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else if h != "" {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil { return "", "", err }
	if err := writeCertKey(certPath, keyPath, der, key); err != nil { return "", "", err }
	return certPath, keyPath, nil
}

func writeCertKey(certPath, keyPath string, certDER []byte, key *rsa.PrivateKey) error {
	cf, err := os.Create(certPath)
	if err != nil { return err }
	defer func() { _ = cf.Close() }()
	if err := pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil { return err }
	if err := os.Chmod(certPath, 0644); err != nil { return err }
	kf, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil { return err }
	defer func() { _ = kf.Close() }()
	if err := pem.Encode(kf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil { return err }
	return nil
}

// ServerTLSConfig loads a server-side mTLS config requiring verified client certs.
func ServerTLSConfig(caCertPath, serverCertPath, serverKeyPath string) (*tls.Config, error) {
	caPool, err := loadCertPool(caCertPath)
	if err != nil { return nil, err }
	cert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	if err != nil { return nil, err }
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	return cfg, nil
}

// ClientTLSConfig loads a client-side mTLS config that verifies the server against the CA.
func ClientTLSConfig(caCertPath, clientCertPath, clientKeyPath, serverName string) (*tls.Config, error) {
	caPool, err := loadCertPool(caCertPath)
	if err != nil { return nil, err }
	cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil { return nil, err }
	cfg := &tls.Config{
		RootCAs:      caPool,
		Certificates: []tls.Certificate{cert},
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}
	return cfg, nil
}

func loadCertPool(caCertPath string) (*x509.CertPool, error) {
	pemBytes, err := os.ReadFile(caCertPath)
	if err != nil { return nil, err }
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("failed to append CA certs from %s", caCertPath)
	}
	return pool, nil
}
