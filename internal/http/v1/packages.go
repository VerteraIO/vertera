package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/VerteraIO/vertera/internal/packages"
	"github.com/VerteraIO/vertera/internal/controlplane/tasks"
	"github.com/VerteraIO/vertera/internal/controlplane/dispatch"
)

// installPackages handles POST /hosts/{hostId}/packages/install
func installPackages(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "hostId")
	if hostID == "" {
		http.Error(w, "hostId is required", http.StatusBadRequest)
		return
	}

	var req packages.PackageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if len(req.Packages) == 0 {
		http.Error(w, "at least one package must be specified", http.StatusBadRequest)
		return
	}

	// Enqueue a task for the agent via the controller's in-memory task manager.
	// Convert package types to strings for the task params.
	var pkgs []string
	for _, t := range req.Packages {
		pkgs = append(pkgs, string(t))
	}
	params := tasks.InstallPackagesParams{
		Packages:  pkgs,
		Version:   req.Version,
		OSVersion: req.OSVersion,
	}
	t, err := tasks.Default.EnqueueInstallPackages(hostID, params)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to enqueue task: %v", err), http.StatusInternalServerError)
		return
	}

	// Notify dispatch so subscribed agents can receive the task immediately.
	dispatch.Default.AddPending(hostID, t)

	// Return 202 with Location header to the task resource.
	w.Header().Set("Location", fmt.Sprintf("/api/v1/tasks/%s", t.ID))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(t)
}

// getPackageInfo handles GET /packages/info
func getPackageInfo(w http.ResponseWriter, r *http.Request) {
	pkgType := r.URL.Query().Get("type")
	version := r.URL.Query().Get("version")
	osVersion := r.URL.Query().Get("os_version")

	if pkgType == "" {
		http.Error(w, "type parameter is required", http.StatusBadRequest)
		return
	}

	cacheDir := os.Getenv("VERTERA_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "/tmp/vertera/packages"
	}
	pkgService := packages.NewService(cacheDir)
	
	packageInfos, err := pkgService.GetPackageInfo(packages.PackageType(pkgType), version, osVersion)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get package info: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"package_type": pkgType,
		"version":      version,
		"os_version":   osVersion,
		"packages":     packageInfos,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// getTaskStatus handles GET /tasks/{taskId}
func getTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskId")
	if id == "" {
		http.Error(w, "taskId is required", http.StatusBadRequest)
		return
	}
	t, ok := tasks.Default.Get(id)
	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(t)
}
