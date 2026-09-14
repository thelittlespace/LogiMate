package wheelengine

import (
	"context"
	"time"
)

type CommandKind string

const (
	CommandRange      CommandKind = "range"
	CommandLEDs       CommandKind = "leds"
	CommandAutocenter CommandKind = "autocenter"
	CommandFFB        CommandKind = "ffb"
	CommandNeutral    CommandKind = "neutral"
	CommandNativeMode CommandKind = "native-mode"
)

type Command struct {
	Kind      CommandKind
	WheelID   string
	SessionID string
	Model     ModelID
	Payload   []byte
	Motor     bool
	Purpose   string
	Deadline  time.Time
}

// Transport is the D0 hardware boundary. Implementations own serialization,
// cancellation semantics and the physical handle; upper layers never write a
// HID report directly once migrated to this interface.
type Transport interface {
	Execute(context.Context, Command) error
	Close() error
}
