// Package domain holds the adapter Manifest shape and its execution Mode, per docs/SPEC.md §4.
package domain

import (
	"encoding/json"
	"fmt"
)

// Mode is an adapter's execution model.
type Mode string

const (
	ModePull   Mode = "pull"
	ModeStream Mode = "stream"
)

// Manifest is the manifest.json every adapter subprocess declares: name, version, execution mode,
// which entity types it emits, its config's JSON Schema, and (stream mode only) how often it must
// heartbeat so the core can tell "quiet" from "hung/dead".
type Manifest struct {
	Name             string          `json:"name"`
	Version          string          `json:"version"`
	Mode             Mode            `json:"mode"`
	Emits            []string        `json:"emits"`
	ConfigSchema     json.RawMessage `json:"config_schema"`
	HeartbeatSeconds int             `json:"heartbeat_seconds,omitempty"`
}

// Validate checks the structural requirements docs/SPEC.md §4 places on a manifest: a name, a
// known mode, and (for stream mode) a positive heartbeat interval.
func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("adapterengine: manifest: name is required")
	}
	switch m.Mode {
	case ModePull:
		// no further constraints
	case ModeStream:
		if m.HeartbeatSeconds <= 0 {
			return fmt.Errorf("adapterengine: manifest %q: heartbeat_seconds must be > 0 in stream mode", m.Name)
		}
	default:
		return fmt.Errorf("adapterengine: manifest %q: mode must be %q or %q, got %q", m.Name, ModePull, ModeStream, m.Mode)
	}
	return nil
}
