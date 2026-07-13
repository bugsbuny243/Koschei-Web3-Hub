package services

import (
	"context"
	"database/sql"
	"log"
)

func StartActorDefenseCorrelator(ctx context.Context, db *sql.DB) func() {
	if db == nil {
		return func() {}
	}
	workerCtx, cancel := context.WithCancel(ctx)
	worker := NewActorDefenseCorrelator(db)
	go worker.Start(workerCtx)
	log.Printf("actor defense correlator started: interval=%s rpc_calls=false", worker.PollEvery)
	return cancel
}
