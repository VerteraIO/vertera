package tasks

import (
	"encoding/json"
	"testing"
)

func TestEnqueueInstallPackages(t *testing.T) {
	m := NewManager()
	params := InstallPackagesParams{
		Packages:  []string{"ovs"},
		Version:   "3.6.0",
		OSVersion: "el9",
	}
	task, err := m.EnqueueInstallPackages("host-123", params)
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if task.ID == "" {
		t.Fatalf("expected task ID to be set")
	}
	if task.HostID != "host-123" {
		t.Fatalf("unexpected host id: %s", task.HostID)
	}
	if task.Type != TypeInstallPackages {
		t.Fatalf("unexpected task type: %s", task.Type)
	}
	if task.Status != StatusQueued {
		t.Fatalf("unexpected status: %s", task.Status)
	}
	var decoded InstallPackagesParams
	if err := json.Unmarshal(task.Params, &decoded); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	if len(decoded.Packages) != 1 || decoded.Packages[0] != "ovs" {
		t.Fatalf("unexpected params: %+v", decoded)
	}
}
