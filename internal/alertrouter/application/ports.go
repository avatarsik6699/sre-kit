package application

import "context"

// Frame is one record pushed to Publisher on alert fire/resolve — deliberately a plain struct (not
// internal/platform/wshub.Frame) so this package doesn't import a websocket/hub dependency;
// cmd/server/main.go wires an adapter that translates Frame into wshub.Frame. Mirrors
// telemetry/application.Frame's pattern (docs/STACK.md ports-over-direct-imports).
type Frame struct {
	Type     string // always "alert", per docs/SPEC.md §4
	SourceID string
	Payload  any
}

// Publisher is the port alertrouter/application uses to fan a fired/resolved Alert out to live
// WebSocket subscribers. Optional — a Service with no Publisher configured simply doesn't push
// live updates.
type Publisher interface {
	Publish(frame Frame)
}

// Notifier is the port alertrouter/application uses to deliver a fired/resolved alert's message
// through a notification channel. v1 has exactly one implementation (internal/notify/telegram),
// but the port stays provider-agnostic in shape.
type Notifier interface {
	Send(ctx context.Context, botToken, chatID, message string) error
}

// SecretStore is the port alertrouter/application uses to store/retrieve a notification channel's
// credential (e.g. a Telegram bot token) without importing internal/platform/secrets directly.
// *secrets.Store already satisfies this structurally.
type SecretStore interface {
	Put(value string) (string, error)
	Get(ref string) (string, error)
	Delete(ref string) error
}
