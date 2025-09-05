package executor

// Executor defines a unit of work the agent can execute on a host.
// Examples: install CH/OVS RPMs, configure OVS, create bridges/ports.
// Implementations should be idempotent when possible.
type Executor interface {
	Name() string
	Run() error
}
