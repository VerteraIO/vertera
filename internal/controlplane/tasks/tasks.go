package tasks

import (
	"encoding/json"
	"sync"
	"time"
	"github.com/google/uuid"
)

type Type string

const (
	TypeInstallPackages Type = "INSTALL_PACKAGES"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Task struct {
	ID        string          `json:"id"`
	HostID    string          `json:"hostId"`
	Type      Type            `json:"type"`
	Params    json.RawMessage `json:"params"`
	Status    Status          `json:"status"`
	Logs      string          `json:"logs,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	StartedAt *time.Time      `json:"startedAt,omitempty"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// UpdateLogs sets/overwrites the last log snippet for a task.
func (m *Manager) UpdateLogs(id string, logs string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if t, ok := m.tasks[id]; ok {
        t.Logs = logs
    }
}

type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewManager() *Manager {
	return &Manager{tasks: make(map[string]*Task)}
}

var Default = NewManager()

type InstallPackagesParams struct {
	Packages  []string `json:"packages"`
	Version   string   `json:"version,omitempty"`
	OSVersion string   `json:"os_version,omitempty"`
}

func (m *Manager) EnqueueInstallPackages(hostID string, p InstallPackagesParams) (*Task, error) {
	bytes, _ := json.Marshal(p)
	id := uuid.NewString()
	t := &Task{
		ID:        id,
		HostID:    hostID,
		Type:      TypeInstallPackages,
		Params:    bytes,
		Status:    StatusQueued,
		CreatedAt: time.Now().UTC(),
	}
	m.mu.Lock()
	m.tasks[id] = t
	m.mu.Unlock()
	return t, nil
}

func (m *Manager) Get(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	return t, ok
}

// UpdateStatusRunning sets a task to running and stamps StartedAt if not already set.
func (m *Manager) UpdateStatusRunning(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		now := time.Now().UTC()
		t.Status = StatusRunning
		if t.StartedAt == nil {
			t.StartedAt = &now
		}
	}
}

// UpdateStatusSucceeded sets a task to succeeded and stamps FinishedAt.
func (m *Manager) UpdateStatusSucceeded(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		now := time.Now().UTC()
		t.Status = StatusSucceeded
		t.FinishedAt = &now
		t.Error = ""
	}
}

// UpdateStatusFailed sets a task to failed with an error and stamps FinishedAt.
func (m *Manager) UpdateStatusFailed(id string, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		now := time.Now().UTC()
		t.Status = StatusFailed
		t.FinishedAt = &now
		t.Error = errMsg
	}
}
