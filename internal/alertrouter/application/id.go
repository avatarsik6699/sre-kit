package application

import "github.com/google/uuid"

// newID generates a fresh UUID for a new Alert, AlertRule, or NotificationChannel — always
// assigned by the core, mirroring internal/sources/application's Create (docs/SPEC.md §4).
func newID() string {
	return uuid.NewString()
}
