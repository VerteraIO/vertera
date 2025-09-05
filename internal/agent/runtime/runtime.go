package runtime

// Placeholder interfaces for host runtime integrations used by the agent.
// These will wrap Cloud Hypervisor and OVS (libovsdb) operations.

// CloudHypervisor abstracts operations the agent may invoke on the local CH daemon.
// It is intentionally minimal for now; expand as features are added.
type CloudHypervisor interface {
	// TODO: Define operations like CreateVM, DeleteVM, etc.
}

// OpenvSwitch abstracts OVS operations performed via libovsdb or CLI.
type OpenvSwitch interface {
	// TODO: Define operations like EnsureBridge, AddPort, SetIfaceOptions, etc.
}
