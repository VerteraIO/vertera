package v1

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/VerteraIO/vertera/internal/security/enroll"
	"github.com/VerteraIO/vertera/internal/security/pki"
)

type enrollTokenReq struct {
	TTL string `json:"ttl"` // Go duration, e.g. "15m"
}

type enrollTokenResp struct {
	Token string `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type csrSignReq struct {
	Token string `json:"token"`
	CSRPEM string `json:"csr_pem"`
}

type csrSignResp struct {
	CertPEM string `json:"cert_pem"`
}

// createEnrollToken handles POST /agents/enroll/token
func createEnrollToken(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("VERTERA_ENROLL_JWT_SECRET")
	if secret == "" {
		http.Error(w, "enrollment disabled: VERTERA_ENROLL_JWT_SECRET not set", http.StatusForbidden)
		return
	}
	var req enrollTokenReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	ttl := 15 * time.Minute
	if req.TTL != "" {
		if d, err := time.ParseDuration(req.TTL); err == nil {
			ttl = d
		}
	}
	tok, err := enroll.IssueToken([]byte(secret), ttl)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to issue token: %v", err), http.StatusInternalServerError)
		return
	}
	resp := enrollTokenResp{Token: tok, ExpiresAt: time.Now().Add(ttl)}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

// signCsr handles POST /agents/enroll/csr
func signCsr(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("VERTERA_ENROLL_JWT_SECRET")
	if secret == "" {
		http.Error(w, "enrollment disabled: VERTERA_ENROLL_JWT_SECRET not set", http.StatusForbidden)
		return
	}
	var req csrSignReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}
	if req.Token == "" || req.CSRPEM == "" {
		http.Error(w, "token and csr_pem are required", http.StatusBadRequest)
		return
	}
	if _, err := enroll.VerifyToken([]byte(secret), req.Token); err != nil {
		http.Error(w, fmt.Sprintf("invalid token: %v", err), http.StatusUnauthorized)
		return
	}
	// Load CA for signing
	var caCertPath, caKeyPath string
	byoCA := os.Getenv("VERTERA_CA_CERT")
	byoCAKey := os.Getenv("VERTERA_CA_KEY") // required if BYO CA is used for signing
	if byoCA != "" && byoCAKey != "" {
		caCertPath, caKeyPath = byoCA, byoCAKey
	} else {
		pkiDir := os.Getenv("VERTERA_PKI_DIR")
		if pkiDir == "" { pkiDir = "/tmp/vertera/pki" }
		caCert, caKey, err := pki.EnsureCA(pkiDir, "Vertera Root CA", 365*24*time.Hour)
		if err != nil {
			http.Error(w, fmt.Sprintf("EnsureCA error: %v", err), http.StatusInternalServerError)
			return
		}
		// Write paths for TLS config builders
		caCertPath, _, _, _ = pki.Paths(pkiDir, "")
		// Also keep parsed caCert/caKey in memory below
		_ = caCert; _ = caKey
	}
	// If BYO paths provided, load them
	var caCertObj *x509.Certificate
	var caKeyObj *rsa.PrivateKey
	if caCertPath != "" && caKeyPath != "" {
		cert, key, err := pki.LoadCA(caCertPath, caKeyPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("load CA failed: %v", err), http.StatusInternalServerError)
			return
		}
		caCertObj, caKeyObj = cert, key
	} else {
		// Fall back: reload from pkiDir paths
		pkiDir := os.Getenv("VERTERA_PKI_DIR")
		if pkiDir == "" { pkiDir = "/tmp/vertera/pki" }
		cp, kp, _, _ := pki.Paths(pkiDir, "")
		cert, key, err := pki.LoadCA(cp, kp)
		if err != nil {
			http.Error(w, fmt.Sprintf("load CA failed: %v", err), http.StatusInternalServerError)
			return
		}
		caCertObj, caKeyObj = cert, key
	}
	// Sign CSR
	certPEM, err := pki.SignCSR(caCertObj, caKeyObj, []byte(req.CSRPEM), false, 365*24*time.Hour)
	if err != nil {
		http.Error(w, fmt.Sprintf("sign CSR failed: %v", err), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(csrSignResp{CertPEM: string(certPEM)})
}
