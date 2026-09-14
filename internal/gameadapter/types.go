package gameadapter

import "time"

// Capability describes semantic data an adapter can provide. Adapters never
// write wheel HID directly; they only publish normalized Frames.
type Capability uint32

const (
	CapabilityTelemetry Capability = 1 << iota
	CapabilityRPM
	CapabilityForce
	CapabilityPlayerControl
	CapabilityFFBAuthorization
)

type Descriptor struct {
	ID          string
	Name        string
	Game        string
	DefaultPort int
	Implemented bool
	Automatic   bool
	Setup       string
	Caps        Capability
}

type Frame struct {
	Adapter       string
	Game          string
	RPM           int
	RPMMax        int
	RPMRedline    int
	SpeedKPH      float64
	Force         float64
	Physics       bool
	PlayerControl bool
	FFBEnabled    bool
	ReceivedAt    time.Time
}

type Health struct {
	Running   bool
	Address   string
	Packets   uint64
	Frames    uint64
	Invalid   uint64
	LastError string
	StartedAt time.Time
	LastFrame time.Time
}

type Snapshot struct {
	Descriptor Descriptor
	Health     Health
	LastFrame  Frame
	Stale      bool
}

type StartOptions struct {
	Port      int
	Automatic bool
}

type Adapter interface {
	Descriptor() Descriptor
	Start(StartOptions) error
	Stop() error
	Snapshot() Snapshot
	RecentFrames() []Frame
}
