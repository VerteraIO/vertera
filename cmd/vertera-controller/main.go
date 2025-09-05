package main

import (
	"log"
	"net/http"

	httpserver "github.com/VerteraIO/vertera/internal/http"
	"github.com/VerteraIO/vertera/internal/controlplane/reconciler"
	"github.com/VerteraIO/vertera/internal/controlplane/scheduler"
	"github.com/VerteraIO/vertera/internal/controlplane/stores"
)

func main() {
	// Initialize control plane components
	st := stores.New()
	st.Start()

	sch := scheduler.New()
	go sch.Start()

	rec := reconciler.New()
	go rec.Start()

	// Start HTTP server
	srv := httpserver.NewServer()
	addr := ":8080" // TODO: switch to :8443 with TLS when certs are wired
	log.Printf("vertera-controller listening on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
