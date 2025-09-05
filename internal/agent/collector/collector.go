package collector

// Collector defines a metric/inventory collector that reports information
// from a host back to the control plane (e.g., CPU, memory, NICs, OVS DB state).
type Collector interface {
	Name() string
	Collect() (map[string]any, error)
}
