package httpserver

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	v1 "github.com/VerteraIO/vertera/internal/http/v1"
)

// NewServer builds the root router and mounts all versioned subrouters under /api/{version}.
func NewServer() http.Handler {
	r := chi.NewRouter()

	// Global middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Root-level docs: redirect to Swagger UI for v1
	r.Get("/docs", serveRootDocs)

	// Default 404: nudge callers toward versioned paths
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found","message":"Use a versioned path like /api/v1/...","supported":["v1"]}`))
	})

	// Mount versioned APIs
	r.Route("/api", func(api chi.Router) {
		api.Mount("/v1", v1.Router())

		// Placeholder for future versions and deprecation headers on v1
		// api.Mount("/v2", v2.Router())
		// api.With(Deprecation("true", "2027-01-01", "/api/v2/docs")).Mount("/v1", v1.Router())
	})

	return r
}

// Deprecation is a middleware that emits deprecation and sunset headers for an API subtree.
func Deprecation(deprecated string, sunsetISO string, successorLink string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Deprecation", deprecated) // e.g., "true"
			if sunsetISO != "" {
				w.Header().Set("Sunset", sunsetISO)
			}
			if successorLink != "" {
				w.Header().Set("Link", "<"+successorLink+">; rel=\"successor-version\"")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// serveRootDocs renders Scalar UI for the latest GA API version.
// Today this points to the v1 OpenAPI document.
func serveRootDocs(w http.ResponseWriter, r *http.Request) {
	// Redirect to the versioned Swagger UI
	http.Redirect(w, r, "/api/v1/docs/index.html", http.StatusFound)
}
