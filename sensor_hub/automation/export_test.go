package automation

import "time"

// Fire makes a trigger come due at due, as the scheduler does when the clock
// reaches it.
func (s *Service) Fire(triggerID int, due time.Time) {
	s.engine.fire(triggerID, due)
}
