package v1_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpserver "github.com/VerteraIO/vertera/internal/http"
)

type taskResp struct {
	ID     string `json:"id"`
	HostID string `json:"hostId"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

func TestInstallPackagesEnqueue(t *testing.T) {
	// Start test server
	ts := httptest.NewServer(httpserver.NewServer())
	defer ts.Close()

	body := `{"packages":["ovs"],"version":"3.6.0","os_version":"el9"}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/hosts/host-123/packages/install", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, string(b))
	}

	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/api/v1/tasks/") {
		t.Fatalf("expected Location header to be /api/v1/tasks/<id>, got %q", loc)
	}

	var tr taskResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if tr.ID == "" || tr.HostID != "host-123" || tr.Status != "queued" {
		t.Fatalf("unexpected task body: %+v", tr)
	}

	// Fetch task status
	res2, err := http.Get(ts.URL + loc)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("expected 200, got %d: %s", res2.StatusCode, string(b))
	}
}
