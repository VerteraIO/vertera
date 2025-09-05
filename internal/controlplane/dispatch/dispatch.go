package dispatch

import (
	"sync"

	"github.com/VerteraIO/vertera/internal/controlplane/tasks"
)

// Manager keeps per-host pending tasks and subscribers to stream tasks.
type Manager struct {
	mu       sync.Mutex
	pending  map[string][]*tasks.Task          // hostID -> pending tasks
	subs     map[string]map[chan *tasks.Task]struct{} // hostID -> subscribers
}

func NewManager() *Manager {
	return &Manager{
		pending: make(map[string][]*tasks.Task),
		subs:    make(map[string]map[chan *tasks.Task]struct{}),
	}
}

var Default = NewManager()

// AddPending enqueues a task to the host's pending list and notifies subscribers.
func (m *Manager) AddPending(hostID string, t *tasks.Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// queue pending
	m.pending[hostID] = append(m.pending[hostID], t)
	// notify subscribers if any
	for ch := range m.subs[hostID] {
		select {
		case ch <- t:
		default:
			// drop if subscriber is slow; pending still holds it
		}
	}
}

// DrainPending returns and clears all pending tasks for a host.
func (m *Manager) DrainPending(hostID string) []*tasks.Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.pending[hostID]
	if len(s) == 0 {
		return nil
	}
	// copy and clear
	out := make([]*tasks.Task, len(s))
	copy(out, s)
	m.pending[hostID] = nil
	return out
}

// Subscribe creates a channel subscription for a host's tasks. Caller must call Unsubscribe.
func (m *Manager) Subscribe(hostID string) (chan *tasks.Task, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan *tasks.Task, 8)
	if m.subs[hostID] == nil {
		m.subs[hostID] = make(map[chan *tasks.Task]struct{})
	}
	m.subs[hostID][ch] = struct{}{}
	return ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if subs := m.subs[hostID]; subs != nil {
			delete(subs, ch)
			close(ch)
			if len(subs) == 0 {
				delete(m.subs, hostID)
			}
		}
	}
}
