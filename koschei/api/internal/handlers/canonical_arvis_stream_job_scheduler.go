package handlers

import (
	"context"
	"database/sql"
	"log"
	"time"

	"koschei/api/internal/jobs"
	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/services"
)

const (
	canonicalArvisStreamMode     = "background_arvis_stream"
	CanonicalArvisStreamHealthID = "arvis-stream-canonical-scheduler"
)

// canonicalArvisStreamJobScheduler turns claimed Pump/Raydium stream events
// into canonical investigation jobs. The job worker builds the same
// actor/behavior report as a customer investigation and only then may the
// service persist a source-bound signed final; this scheduler never grades.
type canonicalArvisStreamJobScheduler struct {
	DB              *sql.DB
	JobStore        *jobs.Store
	PollEvery       time.Duration
	MaxJobsPerCycle int
	StaleAfter      time.Duration
	Health          *runtimehealth.Registry
}

// StartCanonicalArvisStreamJobScheduler reports itself to runtime health as
// unconfigured while withheld, so operators can see why no stream event is
// being turned into a canonical investigation.
func StartCanonicalArvisStreamJobScheduler(ctx context.Context, db *sql.DB, store *jobs.Store, health *runtimehealth.Registry) func() {
	if !CanonicalInvestigationJobWorkerEnabled() || db == nil {
		health.Register(CanonicalArvisStreamHealthID, "worker", "solana-mainnet", false)
		return func() {}
	}
	if ready, reason := services.ArvisStreamCanonicalJobsReady(); !ready {
		health.Register(CanonicalArvisStreamHealthID, "worker", "solana-mainnet", false)
		log.Printf("canonical ARVIS stream scheduler withheld: %s", reason)
		return func() {}
	}
	if store == nil {
		store = jobs.NewStore(db)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	scheduler := &canonicalArvisStreamJobScheduler{
		DB: db, JobStore: store, Health: health,
		PollEvery:       time.Duration(canonicalPumpEnvInt("ARVIS_STREAM_CANONICAL_POLL_SECONDS", 300, 30, 3600)) * time.Second,
		MaxJobsPerCycle: canonicalPumpEnvInt("ARVIS_STREAM_CANONICAL_MAX_JOBS_PER_CYCLE", 1, 1, 10),
		StaleAfter:      time.Duration(canonicalPumpEnvInt("ARVIS_STREAM_CANONICAL_STALE_MINUTES", 30, 10, 720)) * time.Minute,
	}
	health.Register(CanonicalArvisStreamHealthID, "worker", "solana-mainnet", true)
	go scheduler.Start(workerCtx)
	log.Printf("canonical ARVIS stream scheduler started poll=%s max_jobs_per_cycle=%d stale_after=%s", scheduler.PollEvery, scheduler.MaxJobsPerCycle, scheduler.StaleAfter)
	return cancel
}

func (s *canonicalArvisStreamJobScheduler) Start(ctx context.Context) {
	if s == nil || s.DB == nil || s.JobStore == nil {
		return
	}
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			_, queued, err := s.RunOnce(ctx)
			if err != nil && ctx.Err() == nil {
				s.Health.Failure(CanonicalArvisStreamHealthID, err)
				log.Printf("canonical ARVIS stream scheduler cycle failed: %v", err)
			} else if err == nil {
				s.Health.Success(CanonicalArvisStreamHealthID, queued)
			}
			timer.Reset(s.PollEvery)
		}
	}
}

// RunOnce releases stale claims, claims new stream events and queues one
// canonical job per claimed event. A queueing failure closes the claim as a
// retryable failure instead of leaving it in processing.
func (s *canonicalArvisStreamJobScheduler) RunOnce(ctx context.Context) (released, queued int, err error) {
	if ready, _ := services.ArvisStreamCanonicalJobsReady(); !ready {
		return 0, 0, nil
	}
	released, err = services.ReleaseStaleArvisStreamCanonicalClaims(ctx, s.DB, s.StaleAfter, 20)
	if err != nil {
		return released, 0, err
	}
	events, err := services.ClaimArvisStreamCanonicalEvents(ctx, s.DB, s.MaxJobsPerCycle)
	if err != nil {
		return released, 0, err
	}
	for _, event := range events {
		dedupe := services.ArvisStreamCanonicalSource + "|" + event.ID
		payload := canonicalInvestigationJobPayload{
			Mint: event.Target, Network: event.Network, Mode: canonicalArvisStreamMode,
			RootTarget: event.Target, Source: services.ArvisStreamCanonicalSource, SourceEventID: event.ID,
			Depth: 0, MaxDepth: canonicalWorkerEnvInt("ACTOR_RECURSIVE_MAX_DEPTH", 1, 1, 3), DedupeKey: dedupe,
		}
		_, created, createErr := s.JobStore.CreateUniqueActive(ctx, jobs.CreateInput{
			Type: CanonicalInvestigationJobType, Network: event.Network, Target: event.Target, Request: payload,
		}, dedupe)
		if createErr != nil {
			if _, markErr := services.MarkArvisStreamCanonicalOutcome(ctx, s.DB, event.ID, createErr); markErr != nil {
				log.Printf("canonical ARVIS stream claim release failed event=%s err=%v", event.ID, markErr)
			}
			return released, queued, createErr
		}
		if created {
			queued++
		}
	}
	log.Printf("canonical ARVIS stream cycle: released_stale=%d claimed=%d queued=%d", released, len(events), queued)
	return released, queued, nil
}
