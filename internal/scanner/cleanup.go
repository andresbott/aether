package scanner

import "time"

func (s *Scanner) cleanup(scanStart time.Time) error {
	return s.store.Cleanup(scanStart)
}
