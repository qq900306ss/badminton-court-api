package service

import (
	"context"
	"time"

	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// autoCloseGrace: a session auto-closes once it's this long past its end time.
const autoCloseGrace = 2 * time.Hour

// AutoCloseIfExpired closes an open session that is more than 2 hours past its
// end_at. Returns true if it closed it. No-op if there's no end time set.
func AutoCloseIfExpired(ctx context.Context, s *model.Session) bool {
	if s.Status != model.SessionOpen || s.EndAt == "" {
		return false
	}
	end, err := time.Parse(time.RFC3339, s.EndAt)
	if err != nil {
		return false
	}
	if time.Now().After(end.Add(autoCloseGrace)) {
		s.Status = model.SessionClosed
		s.ClosedAt = time.Now().UTC().Format(time.RFC3339)
		_ = repository.UpdateSession(ctx, *s)
		return true
	}
	return false
}
