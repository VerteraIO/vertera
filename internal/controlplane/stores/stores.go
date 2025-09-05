package stores

import "log"

// Stores encapsulates persistence adapters (e.g., Postgres) for
// resources like Hosts, VMs, Networks, and Tasks.
// Placeholder: wire real DB connections and migrations.
type Stores struct{}

func New() *Stores { return &Stores{} }

func (s *Stores) Start() {
	// TODO: connect to DB, run migrations
	log.Println("controlplane: stores initialized")
}
