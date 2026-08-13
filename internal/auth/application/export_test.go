package application

import "time"

// SetClockForTest overrides Service's clock for tests (e.g. simulating session expiry).
func SetClockForTest(s *Service, now func() time.Time) {
	s.now = now
}
