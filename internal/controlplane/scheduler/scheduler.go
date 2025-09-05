package scheduler

import "log"

// Scheduler assigns units of work to agents/nodes.
// Placeholder for queueing, scoring, and assignment logic.
type Scheduler struct{}

func New() *Scheduler { return &Scheduler{} }

func (s *Scheduler) Start() {
	log.Println("controlplane: scheduler started")
}
