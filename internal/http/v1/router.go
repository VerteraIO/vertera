package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	openapi "github.com/VerteraIO/vertera/api/openapi"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Router returns the chi.Router for REST API v1.
func Router() chi.Router {
	r := chi.NewRouter()

	// Docs (Swagger UI) and spec under the versioned prefix
	r.Get("/docs/*", httpSwagger.Handler(
		httpSwagger.URL("/api/v1/openapi.yaml"), // point Swagger UI at our embedded OpenAPI spec
	))
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
