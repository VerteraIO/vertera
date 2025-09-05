//go:build grpcgen

package main

import (
	"log"
	"os"
	"time"

	controllerserver "github.com/VerteraIO/vertera/internal/grpc/controller"
	"github.com/VerteraIO/vertera/internal/security/pki"
	"google.golang.org/grpc/credentials"
)

func init() {
	// Start gRPC server in background when built with -tags=grpcgen
	go func() {
		addr := os.Getenv("VERTERA_GRPC_ADDR")
		if addr == "" {
			addr = ":9090"
		}
		// BYO certs (preferred if provided)
		byoCACert := os.Getenv("VERTERA_CA_CERT")
		byoServerCert := os.Getenv("VERTERA_SERVER_CERT")
		byoServerKey := os.Getenv("VERTERA_SERVER_KEY")
		var caCertPath, serverCertPath, serverKeyPath string
		if byoCACert != "" && byoServerCert != "" && byoServerKey != "" {
			caCertPath, serverCertPath, serverKeyPath = byoCACert, byoServerCert, byoServerKey
		} else {
			// mTLS setup: create/load CA and issue server cert
			pkiDir := os.Getenv("VERTERA_PKI_DIR")
			if pkiDir == "" { pkiDir = "/tmp/vertera/pki" }
			caCert, caKey, err := pki.EnsureCA(pkiDir, "Vertera Root CA", 365*24*time.Hour)
			if err != nil {
				log.Printf("PKI EnsureCA error: %v", err)
				return
			}
			// Issue server cert for controller with SANs localhost and 127.0.0.1
			_, serverKeyPath = pki.Paths(pkiDir, "controller")
			serverCertPath, serverKeyPath, err = pki.IssueCertificate(pkiDir, "controller", "vertera-controller", true, caCert, caKey, 365*24*time.Hour, []string{"localhost", "127.0.0.1"})
			if err != nil {
				log.Printf("PKI IssueCertificate error: %v", err)
				return
			}
			caCertPath, _, _, _ = pki.Paths(pkiDir, "")
		}
		tlsCfg, err := pki.ServerTLSConfig(caCertPath, serverCertPath, serverKeyPath)
		if err != nil {
			log.Printf("server TLS config error: %v", err)
			return
		}
		creds := credentials.NewTLS(tlsCfg)
		if err := controllerserver.RunTLS(addr, creds); err != nil {
			log.Printf("gRPC server (mTLS) error: %v", err)
		}
	}()
	// tiny delay to make logs ordering nicer during startup
	time.Sleep(10 * time.Millisecond)
}
