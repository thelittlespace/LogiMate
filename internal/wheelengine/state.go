package wheelengine

import "time"

type ConnectionState string

const (
	ConnectionUnknown      ConnectionState = "unknown"
	ConnectionConnected    ConnectionState = "connected"
	ConnectionReconnecting ConnectionState = "reconnecting"
	ConnectionDisconnected ConnectionState = "disconnected"
	ConnectionError        ConnectionState = "error"
	ConnectionAmbiguous    ConnectionState = "ambiguous"
)

type InputSource string

const (
	InputSourceUnknown   InputSource = "unknown"
	InputSourceDirectHID InputSource = "direct-hid"
	InputSourceWinMM     InputSource = "winmm"
)

type Device struct {
	StableID            string
	SessionID           string
	HardwareFingerprint string
	PersistentIdentity  bool
	Name                string
	Model               ModelID
	Mode                OperatingMode
	Supported           bool
	ModelConfirmed      bool
	PnPVerified         bool
	NativePath          string
}

type InputState struct {
	WheelID          string
	SessionID        string
	Model            ModelID
	Source           InputSource
	LayoutID         string
	Connection       ConnectionState
	SampleValid      bool
	SampleGeneration uint64
	SampleAt         time.Time
	Steering         float64 // normalized -1..+1 when known
	SteeringDegrees  float64
	Throttle         float64 // normalized 0..1 when known
	Brake            float64
	Clutch           float64
	Buttons          uint64
	DPad             int
	Gear             int
	PaddleLeft       bool
	PaddleRight      bool
	Error            string
}

func (s InputState) Fresh(now time.Time, maxAge time.Duration) bool {
	if !s.SampleValid || s.SampleAt.IsZero() || maxAge <= 0 {
		return false
	}
	return !now.Before(s.SampleAt) && now.Sub(s.SampleAt) <= maxAge
}

type OutputState struct {
	LeaseActive bool
	Generation  uint64
	Motor       bool
	Purpose     string
	LastCommand string
	LastError   string
	UpdatedAt   time.Time
}

type Health struct {
	Connected       bool
	InputFresh      bool
	OutputBlocked   bool
	RecoveryPending bool
	LastError       string
	UpdatedAt       time.Time
}

type Runtime struct {
	Device Device
	Input  InputState
	Output OutputState
	Health Health
}

type Snapshot struct {
	Generation uint64
	SelectedID string
	Devices    []Runtime
	UpdatedAt  time.Time
}
