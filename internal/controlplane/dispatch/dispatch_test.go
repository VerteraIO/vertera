package dispatch

import (
	"testing"
	"time"

	"github.com/VerteraIO/vertera/internal/controlplane/tasks"
)

func TestDispatchSubscribeAndNotify(t *testing.T) {
	m := NewManager()
	host := "host-123"
	ch, cancel := m.Subscribe(host)
	defer cancel()

	// Enqueue a task and notify
	tt, _ := tasks.NewManager().EnqueueInstallPackages(host, tasks.InstallPackagesParams{Packages: []string{"ovs"}})
	m.AddPending(host, tt)

	select {
	case got := <-ch:
		if got == nil || got.ID != tt.ID {
			t.Fatalf("expected task %s, got %+v", tt.ID, got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for task notification")
	}

	// Pending queue is independent of subscriptions; first drain should return the first task
	drained := m.DrainPending(host)
	if len(drained) != 1 || drained[0].ID != tt.ID {
		t.Fatalf("unexpected first drain result: %+v", drained)
	}

	// Enqueue again, drain again -> should return exactly one
	m.AddPending(host, tt)
	drained2 := m.DrainPending(host)
	if len(drained2) != 1 || drained2[0].ID != tt.ID {
		t.Fatalf("unexpected second drain result: %+v", drained2)
	}
}
