//go:build grpcgen

package main

import (
	"context"
	"log"
	"os"
	"time"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"

	agentgrpc "github.com/VerteraIO/vertera/internal/grpc/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"github.com/VerteraIO/vertera/internal/security/pki"
)

func init() {
	go func() {
		addr := os.Getenv("VERTERA_CONTROLLER_ADDR")
		if addr == "" {
			addr = "localhost:9090"
		}
		hostname, _ := os.Hostname()
		agentID := os.Getenv("VERTERA_AGENT_ID")
		if agentID == "" {
			agentID = hostname
		}
		ctx := context.Background()
		// mTLS: BYO certs via env or create CA/client cert in VERTERA_PKI_DIR
		caCertPath := os.Getenv("VERTERA_CA_CERT")
		clientCertPath := os.Getenv("VERTERA_CLIENT_CERT")
		clientKeyPath := os.Getenv("VERTERA_CLIENT_KEY")

		// Auto-enroll if no client certs provided and an enrollment token is available
		if (clientCertPath == "" || clientKeyPath == "") && os.Getenv("VERTERA_ENROLL_TOKEN") != "" {
			if err := autoEnrollClientCert(agentID, hostname); err != nil {
				log.Printf("agent auto-enroll failed: %v", err)
			} else {
				// Re-evaluate paths after successful enrollment
				pkiDir := os.Getenv("VERTERA_PKI_DIR")
				if pkiDir == "" { pkiDir = "/tmp/vertera/pki" }
				_, _, clientCertPath, clientKeyPath = pki.Paths(pkiDir, "agent-"+agentID)
				if caCertPath == "" {
					caCertPath, _, _, _ = pki.Paths(pkiDir, "")
				}
			}
		}

		if caCertPath == "" || clientCertPath == "" || clientKeyPath == "" {
			pkiDir := os.Getenv("VERTERA_PKI_DIR")
			if pkiDir == "" { pkiDir = "/tmp/vertera/pki" }
			caCert, caKey, err := pki.EnsureCA(pkiDir, "Vertera Root CA", 365*24*time.Hour)
			if err != nil {
				log.Printf("agent PKI EnsureCA error: %v", err)
				return
			}
			// Issue client cert for this agent id
			_ = caKey
			clientCertPath, clientKeyPath, err = pki.IssueCertificate(pkiDir, "agent-"+agentID, agentID, false, caCert, caKey, 365*24*time.Hour, []string{hostname})
			if err != nil {
				log.Printf("agent PKI IssueCertificate error: %v", err)
				return
			}
			caCertPath, _, _, _ = pki.Paths(pkiDir, "")
		}
		tlsCfg, err := pki.ClientTLSConfig(caCertPath, clientCertPath, clientKeyPath, "vertera-controller")
		if err != nil {
			log.Printf("agent TLS config error: %v", err)
			return
		}
		creds := credentials.NewTLS(tlsCfg)
		if err := agentgrpc.Run(ctx, addr, agentID, hostname, grpc.WithTransportCredentials(creds)); err != nil {
			log.Printf("agent gRPC client error: %v", err)
		}
	}()
	// tiny delay to order logs nicely
	time.Sleep(10 * time.Millisecond)
}

// autoEnrollClientCert generates a key+CSR and requests a signed certificate from the controller
// using the token in VERTERA_ENROLL_TOKEN. The certificate and key are saved under VERTERA_PKI_DIR
// as agent-<agentID>.{pem,key}. The CA cert is expected to be provided via VERTERA_CA_CERT or to
// exist in VERTERA_PKI_DIR/ca.pem for dev mode.
func autoEnrollClientCert(agentID, hostname string) error {
	token := os.Getenv("VERTERA_ENROLL_TOKEN")
	if token == "" { return nil }
	pkiDir := os.Getenv("VERTERA_PKI_DIR")
	if pkiDir == "" { pkiDir = "/tmp/vertera/pki" }
	if err := pki.EnsureDir(pkiDir); err != nil { return fmt.Errorf("ensure pki dir: %w", err) }

	// Generate key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil { return fmt.Errorf("generate key: %w", err) }
	_, _, certPath, keyPath := pki.Paths(pkiDir, "agent-"+agentID)
	// Write key now
	if err := writePrivateKeyPEM(keyPath, key); err != nil { return fmt.Errorf("write key: %w", err) }

	// Build CSR
	csrTmpl := &x509.CertificateRequest{ Subject: pkix.Name{ CommonName: agentID } }
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTmpl, key)
	if err != nil { return fmt.Errorf("create csr: %w", err) }
	var csrPEM bytes.Buffer
	_ = pem.Encode(&csrPEM, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	// POST CSR to controller HTTP endpoint
	baseURL := os.Getenv("VERTERA_CONTROLLER_HTTP")
	if baseURL == "" { baseURL = "http://localhost:8080" }
	url := fmt.Sprintf("%s/api/v1/agents/enroll/csr", baseURL)
	payload := map[string]string{
		"token":  token,
		"csr_pem": csrPEM.String(),
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil { return fmt.Errorf("post csr: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("csr sign failed: status %d", resp.StatusCode)
	}
	var out struct{ CertPEM string `json:"cert_pem"` }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return fmt.Errorf("decode csr response: %w", err) }
	if out.CertPEM == "" { return fmt.Errorf("empty cert pem from controller") }
	if err := os.WriteFile(certPath, []byte(out.CertPEM), 0644); err != nil { return fmt.Errorf("write cert: %w", err) }
	return nil
}

func writePrivateKeyPEM(path string, key *rsa.PrivateKey) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil { return err }
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}
