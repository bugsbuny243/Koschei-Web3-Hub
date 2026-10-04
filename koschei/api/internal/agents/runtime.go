package agents

import (
	"context"
	"time"
)

// StartRuntime is called once by bootstrap, never by route registration or a
// package initializer. The API role owns a service but does not run workers.
func (s *Service) StartRuntime(parent context.Context, workers bool) func() {
	s.mu.Lock()
	if s.workersStarted {
		s.mu.Unlock()
		return func() {}
	}
	s.workersStarted = true
	ctx, cancel := context.WithCancel(parent)
	s.workerCancel = cancel
	if workers && s.db != nil {
		s.startPeriodic(ctx, time.Minute, func(ctx context.Context) { s.recoverStaleFollowups(ctx); s.deliverOneFollowup(ctx) })
		s.startPeriodic(ctx, time.Minute, s.detectMissedLeads)
		s.startPeriodic(ctx, time.Minute, s.deliverOneOperatorNotification)
		if s.IntegrationEnabled() {
			s.startPeriodic(ctx, 30*time.Second, s.deliverOneIntegration)
		}
	}
	s.mu.Unlock()
	return func() {
		cancel()
		s.workerWG.Wait()
		if s.db != nil {
			_ = s.db.Close()
		}
	}
}

func (s *Service) startPeriodic(parent context.Context, interval time.Duration, cycle func(context.Context)) {
	s.workerWG.Add(1)
	go func() {
		defer s.workerWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if parent.Err() != nil {
				return
			}
			ctx, cancel := context.WithTimeout(parent, 30*time.Second)
			cycle(ctx)
			cancel()
			select {
			case <-parent.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
