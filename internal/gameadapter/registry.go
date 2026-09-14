package gameadapter

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	IDLocalJSON         = "logimate-json"
	IDWreckfestPino     = "wreckfest2-pino"
	LegacyIDOpenG27Pino = "openg27-pino"
)

type Factory func() (Adapter, error)

type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
	aliases   map[string]string
}

func NewRegistry() *Registry {
	r := &Registry{factories: map[string]Factory{}, aliases: map[string]string{LegacyIDOpenG27Pino: IDWreckfestPino}}
	r.Register(IDLocalJSON, func() (Adapter, error) {
		return NewUDPAdapter(Descriptor{ID: IDLocalJSON, Name: "LogiMate Local JSON", Game: "Generic", DefaultPort: 27100, Automatic: false, Setup: "Send normalized LogiMate JSON telemetry via UDP to 127.0.0.1:27100. Manual listener only.", Caps: CapabilityTelemetry | CapabilityRPM | CapabilityForce | CapabilityPlayerControl | CapabilityFFBAuthorization}, ParseLocalJSON)
	})
	r.Register(IDWreckfestPino, func() (Adapter, error) {
		return NewUDPAdapter(Descriptor{ID: IDWreckfestPino, Name: "Wreckfest 2 / Pino", Game: "Wreckfest 2", DefaultPort: 23123, Automatic: true, Setup: "Enable the Pino telemetry output for Wreckfest 2 and send Main packets to UDP 127.0.0.1:23123.", Caps: CapabilityTelemetry | CapabilityRPM | CapabilityForce | CapabilityPlayerControl | CapabilityFFBAuthorization}, ParseWreckfestPino)
	})
	return r
}

func normalize(id string) string { return strings.ToLower(strings.TrimSpace(id)) }

func (r *Registry) CanonicalID(id string) string {
	id = normalize(id)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if x := r.aliases[id]; x != "" {
		return x
	}
	return id
}

func (r *Registry) Register(id string, f Factory) {
	id = normalize(id)
	if id == "" || f == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[id] = f
}

func (r *Registry) New(id string) (Adapter, error) {
	id = r.CanonicalID(id)
	r.mu.RLock()
	f := r.factories[id]
	r.mu.RUnlock()
	if f == nil {
		return nil, fmt.Errorf("telemetry adapter %q is not registered", id)
	}
	return f()
}

func (r *Registry) Descriptors() []Descriptor {
	r.mu.RLock()
	ids := make([]string, 0, len(r.factories))
	for id := range r.factories {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fs := make([]Factory, len(ids))
	for i, id := range ids {
		fs[i] = r.factories[id]
	}
	r.mu.RUnlock()
	out := make([]Descriptor, 0, len(ids))
	for _, f := range fs {
		a, err := f()
		if err == nil {
			out = append(out, a.Descriptor())
		}
	}
	return out
}
