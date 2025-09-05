package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	scalar "github.com/MarceloPetrucio/go-scalar-api-reference"
	openapi "github.com/VerteraIO/vertera/api/openapi"
)

// Router returns the chi.Router for REST API v1.
func Router() chi.Router {
	r := chi.NewRouter()

	// Docs and spec under the versioned prefix
	r.Get("/docs", serveScalarUI)
	r.Get("/openapi.yaml", serveOpenAPIStaticAsset)

	// Package management endpoints
	r.Get("/packages/info", getPackageInfo)
	r.Post("/hosts/{hostId}/packages/install", installPackages)

	// Task endpoints
	r.Get("/tasks/{taskId}", getTaskStatus)

	// Agent enrollment endpoints
	r.Post("/agents/enroll/token", createEnrollToken)
	r.Post("/agents/enroll/csr", signCsr)

	// API endpoints
	r.Get("/projects", listProjects)

	return r
}

func serveScalarUI(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	specURL := fmt.Sprintf("%s://%s/api/v1/openapi.yaml", scheme, r.Host)
	htmlContent, err := scalar.ApiReferenceHTML(&scalar.Options{
		SpecURL: specURL,
		CustomOptions: scalar.CustomOptions{
			PageTitle: "Vertera API v1",
		},
		DarkMode: false,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to render docs: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := fmt.Fprintln(w, htmlContent); err != nil {
		http.Error(w, fmt.Sprintf("failed to write response: %v", err), http.StatusInternalServerError)
		return
	}
}

func serveOpenAPIStaticAsset(w http.ResponseWriter, r *http.Request) {
	data, err := openapi.FS.ReadFile("v1/vertera.yaml")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read spec: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(data)
}

func listProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Stub payload; replace with real store-backed implementation
	resp := map[string]any{
		"items": []map[string]any{
			{"id": "00000000-0000-0000-0000-000000000001", "name": "default"},
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
