package v1_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	httpserver "github.com/VerteraIO/vertera/internal/http"
)

type tokenResp struct {
	Token string `json:"token"`
}

type csrResp struct {
	CertPEM string `json:"cert_pem"`
}

func TestEnrollTokenAndSignCSR(t *testing.T) {
	if err := os.Setenv("VERTERA_ENROLL_JWT_SECRET", "test-secret"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("VERTERA_ENROLL_JWT_SECRET"); err != nil {
			t.Fatalf("unsetenv: %v", err)
		}
	}()
	// ensure PKI dir is writable temp
	pkiDir := t.TempDir()
	if err := os.Setenv("VERTERA_PKI_DIR", pkiDir); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("VERTERA_PKI_DIR"); err != nil {
			t.Fatalf("unsetenv: %v", err)
		}
	}()

	ts := httptest.NewServer(httpserver.NewServer())
	defer ts.Close()

	// 1) Issue token
	tr := struct{ TTL string `json:"ttl"` }{TTL: "2m"}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal token: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agents/enroll/token", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("token req: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("close response body: %v", err)
		}
	}()
	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read response body: %v", err)
		}
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
	var trOut tokenResp
	if err := json.NewDecoder(resp.Body).Decode(&trOut); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if trOut.Token == "" {
		t.Fatal("empty token")
	}

	// 2) Generate CSR
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	csrTmpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: "agent-test"}}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTmpl, key)
	if err != nil {
		t.Fatalf("create csr: %v", err)
	}
	var csrPEM bytes.Buffer
	if err := pem.Encode(&csrPEM, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}); err != nil {
		t.Fatalf("pem encode: %v", err)
	}

	// 3) Submit CSR for signing
	csrReq := map[string]any{"token": trOut.Token, "csr_pem": csrPEM.String()}
	payload, err := json.Marshal(csrReq)
	if err != nil {
		t.Fatalf("marshal csr: %v", err)
	}
	resp2, err := http.Post(ts.URL+"/api/v1/agents/enroll/csr", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("csr post: %v", err)
	}
	defer func() {
		if err := resp2.Body.Close(); err != nil {
			t.Fatalf("close response body: %v", err)
		}
	}()
	if resp2.StatusCode != 200 {
		body, err := io.ReadAll(resp2.Body)
		if err != nil {
			t.Fatalf("read response body: %v", err)
		}
		t.Fatalf("expected 200 for csr, got %d: %s", resp2.StatusCode, string(body))
	}
	var csrOut csrResp
	if err := json.NewDecoder(resp2.Body).Decode(&csrOut); err != nil {
		t.Fatalf("decode csr: %v", err)
	}
	if csrOut.CertPEM == "" { t.Fatal("empty cert pem") }
}
