package automation

import "time"

func (s *Service) Fire(triggerID int, due time.Time) {
	s.engine.fire(triggerID, due)
}
