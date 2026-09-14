package wheelengine

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager is the authoritative in-process runtime registry introduced by D0.
// Platform-specific code discovers/transports devices, but ownership of the
// selected device, canonical input state and output health lives here.
type Manager struct {
	mu         sync.RWMutex
	generation uint64
	selectedID string
	devices    map[string]Runtime
	updatedAt  time.Time
}

func NewManager() *Manager { return &Manager{devices: make(map[string]Runtime)} }

func key(id string) string { return strings.ToLower(strings.TrimSpace(id)) }

func (m *Manager) Reconcile(devices []Device, selectedID string, now time.Time) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.devices == nil {
		m.devices = make(map[string]Runtime)
	}
	next := make(map[string]Runtime, len(devices))
	for _, d := range devices {
		k := key(d.StableID)
		if k == "" {
			continue
		}
		r := m.devices[k]
		// A PnP session change invalidates old input immediately. This prevents
		// stale samples from one physical enumeration being applied to another.
		if r.Device.SessionID != "" && !strings.EqualFold(r.Device.SessionID, d.SessionID) {
			r.Input = InputState{}
			r.Output = OutputState{}
		}
		r.Device = d
		r.Health.Connected = true
		r.Health.UpdatedAt = now
		next[k] = r
	}
	for k, old := range m.devices {
		if _, ok := next[k]; !ok {
			old.Health.Connected = false
			old.Health.InputFresh = false
			old.Health.UpdatedAt = now
			old.Input.SampleValid = false
			// Disconnected devices are deliberately not carried forever. Keeping
			// them here would make selection truth diverge from PnP truth.
		}
	}
	m.devices = next
	m.selectedID = selectedID
	m.generation++
	m.updatedAt = now
	return m.snapshotLocked()
}

func (m *Manager) UpdateInput(s InputState, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(s.WheelID)
	r, ok := m.devices[k]
	if !ok || k == "" || !strings.EqualFold(r.Device.SessionID, s.SessionID) {
		return false
	}
	if r.Input.SampleGeneration > s.SampleGeneration && s.SampleGeneration != 0 {
		return false
	}
	r.Input = s
	r.Health.InputFresh = s.Fresh(now, time.Second)
	r.Health.LastError = s.Error
	r.Health.UpdatedAt = now
	m.devices[k] = r
	m.generation++
	m.updatedAt = now
	return true
}

func (m *Manager) UpdateOutput(wheelID, sessionID string, s OutputState, recoveryPending bool, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(wheelID)
	r, ok := m.devices[k]
	if !ok || k == "" || !strings.EqualFold(r.Device.SessionID, sessionID) {
		return false
	}
	r.Output = s
	r.Health.OutputBlocked = recoveryPending || (s.LeaseActive && s.LastError != "")
	r.Health.RecoveryPending = recoveryPending
	if s.LastError != "" {
		r.Health.LastError = s.LastError
	}
	r.Health.UpdatedAt = now
	m.devices[k] = r
	m.generation++
	m.updatedAt = now
	return true
}

func (m *Manager) Selected() (Runtime, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.devices[key(m.selectedID)]
	return r, ok
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshotLocked()
}

func (m *Manager) snapshotLocked() Snapshot {
	out := Snapshot{Generation: m.generation, SelectedID: m.selectedID, UpdatedAt: m.updatedAt}
	keys := make([]string, 0, len(m.devices))
	for k := range m.devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out.Devices = append(out.Devices, m.devices[k])
	}
	return out
}
