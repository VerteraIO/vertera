package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIPrefixEnforced(t *testing.T) {
	s := NewServer()

	// Unversioned path should 404
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unversioned path, got %d", rec.Code)
	}

	// Versioned path should 200
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for versioned path, got %d", rec2.Code)
	}
}
