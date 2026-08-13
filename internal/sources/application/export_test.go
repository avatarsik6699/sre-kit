package application

import "time"

// SetClockForTest overrides Service's clock for tests (e.g. asserting MarkSeen's stamped time).
func SetClockForTest(s *Service, now func() time.Time) {
	s.now = now
}
