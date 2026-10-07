package automation

import (
	"context"
	"time"
)

func (s *Service) Fire(triggerID int, due time.Time) {
	s.engine.fire(triggerKey(triggerID), due)
}

func (s *Service) ResumeScheduled(runID int) bool {
	_, ok := s.engine.scheduler.due(resumeKey(runID))
	return ok
}

func (s *Service) CheckMargins(ctx context.Context) error {
	return s.checkMargins(ctx)
}
