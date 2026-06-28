package service

import (
	"context"
	"log"
	"time"

	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// autoCloseGrace: a session auto-closes once it's this long past its end time.
const autoCloseGrace = 2 * time.Hour

// SweepExpiredSessions closes every open session that's >2h past its end time.
// Runs on a background ticker (see StartAutoCloseSweeper) so sessions close on
// time even when nobody has the app open — a real scheduler, no AWS EventBridge.
func SweepExpiredSessions(ctx context.Context) {
	sessions, err := repository.ListOpenSessions(ctx)
	if err != nil {
		log.Printf("auto-close sweep: list open sessions: %v", err)
		return
	}
	closed := 0
	for i := range sessions {
		if AutoCloseIfExpired(ctx, &sessions[i]) {
			closed++
		}
	}
	if closed > 0 {
		log.Printf("auto-close sweep: closed %d expired session(s)", closed)
	}
}

// StartAutoCloseSweeper runs SweepExpiredSessions every interval until ctx is
// done. Call once from main on an always-on server.
func StartAutoCloseSweeper(ctx context.Context, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		SweepExpiredSessions(ctx) // run once at startup
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				SweepExpiredSessions(ctx)
			}
		}
	}()
}

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
