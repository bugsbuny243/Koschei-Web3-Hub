package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// legacyArvisStreamPendingTargetsQuery is the original single-pass query,
// kept only as an oracle: the indexed per-bucket query must select the same
// events in the same order.
const legacyArvisStreamPendingTargetsQuery = `
	WITH eligible AS (
		SELECT s.id::text,
		       s.created_at,
		       CASE
		         WHEN p.status='failed' THEN 'failed'
		         WHEN p.status='exhausted' THEN 'exhausted'
		         WHEN p.stream_event_id IS NULL AND s.created_at >= now() - interval '2 minutes' THEN 'fresh'
		         ELSE 'backlog'
		       END AS bucket,
		       COALESCE(p.updated_at,s.created_at) AS queue_time
		FROM security_radar_stream_events s
		LEFT JOIN arvis_stream_processing p ON p.stream_event_id=s.id
		WHERE COALESCE(s.target,'') <> ''
		  AND COALESCE(s.target_type,'')='token'
		  AND COALESCE(s.module_id,'') IN ($2,$3)
		  AND (
			p.stream_event_id IS NULL
			OR (p.status='failed' AND p.attempts < 3 AND p.updated_at < now() - interval '30 seconds')
			OR (p.status='exhausted' AND p.attempts < 5 AND p.updated_at < now() - interval '30 minutes')
		  )
	), ranked AS (
		SELECT eligible.*,
		       row_number() OVER (
		         PARTITION BY bucket
		         ORDER BY
		           CASE WHEN bucket='fresh' THEN created_at END DESC,
		           CASE WHEN bucket<>'fresh' THEN queue_time END ASC,
		           created_at ASC
		       ) AS bucket_rank
		FROM eligible
	), prioritized AS (
		SELECT ranked.*,
		       CASE
		         WHEN bucket='failed' AND bucket_rank <= GREATEST(1,$1/5) THEN 0
		         WHEN bucket='fresh' AND bucket_rank <= GREATEST(1,($1*3)/10) THEN 1
		         WHEN bucket='backlog' AND bucket_rank <= GREATEST(1,($1*4)/10) THEN 2
		         WHEN bucket='exhausted' AND bucket_rank <= GREATEST(1,$1/10) THEN 3
		         ELSE 10
		       END AS queue_priority
		FROM ranked
	)
	SELECT id
	FROM prioritized
	ORDER BY
	  queue_priority,
	  CASE WHEN bucket='fresh' THEN created_at END DESC,
	  CASE WHEN bucket<>'fresh' THEN queue_time END ASC,
	  created_at ASC
	LIMIT $1
`

func TestArvisStreamPendingTargetsMatchesLegacyQueryPostgres17(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	// Registered first so it runs last, after the fixture cleanup below.
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	run := time.Now().UTC().Format("150405.000000")
	ids := []string{}
	n := 0
	// insert adds a stream event created age ago and, when status is set, a
	// processing claim last updated claimAge ago.
	insert := func(module, targetType, target string, age time.Duration, status string, attempts int, claimAge time.Duration) {
		t.Helper()
		n++
		var id string
		if err := db.QueryRowContext(ctx, `
			INSERT INTO security_radar_stream_events
			(provider,stream_mode,network,module_id,event_type,target,target_type,signature,program_id,evidence_quality,created_at)
			VALUES('ci_fixture','test','solana-mainnet',$1,'test_pending_targets',NULLIF($2,''),$3,$4,'ci-program','fixture_only',now()-make_interval(secs => $5))
			RETURNING id::text
		`, module, target, targetType, fmt.Sprintf("ci-pending-sig-%s-%d", run, n), age.Seconds()).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		if status != "" {
			if _, err := db.ExecContext(ctx, `
				INSERT INTO arvis_stream_processing(stream_event_id,target,signature,status,attempts,last_error,created_at,updated_at)
				VALUES($1::uuid,$2,'ci',$3,$4,'',now()-make_interval(secs => $5),now()-make_interval(secs => $5))
			`, id, target, status, attempts, claimAge.Seconds()); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		for _, id := range ids {
			_, _ = db.ExecContext(cleanup, `DELETE FROM arvis_stream_processing WHERE stream_event_id=$1::uuid`, id)
			_, _ = db.ExecContext(cleanup, `DELETE FROM security_radar_stream_events WHERE id=$1::uuid`, id)
		}
	})

	mint := "ci-pending-mint-" + run
	pump, raydium := ModulePumpSybilRadar, ModuleRaydiumPoolGuardian
	// Every bucket holds more events than the small limits below, so a wrong
	// per-bucket order or cap would change which events are selected.
	// Fresh (unclaimed, under two minutes old), both modules.
	for i, age := range []time.Duration{10, 15, 20, 25, 30, 35, 40} {
		module := pump
		if i%3 == 2 {
			module = raydium
		}
		insert(module, "token", mint, age*time.Second, "", 0, 0)
	}
	// Backlog (unclaimed, older), both modules.
	for i := 1; i <= 8; i++ {
		module := pump
		if i%3 == 0 {
			module = raydium
		}
		insert(module, "token", mint, time.Duration(i)*time.Hour, "", 0, 0)
	}
	// Failed claims due for retry, and ones that are not.
	for i := 1; i <= 5; i++ {
		module := pump
		if i%2 == 0 {
			module = raydium
		}
		insert(module, "token", mint, time.Duration(10+i)*time.Hour, "failed", 1+i%2, time.Duration(i)*time.Hour)
	}
	insert(pump, "token", mint, 16*time.Hour, "failed", 1, 5*time.Second)
	insert(pump, "token", mint, 17*time.Hour, "failed", 3, time.Hour)
	// Exhausted claims due for retry, and ones that are not.
	for i := 1; i <= 4; i++ {
		module := pump
		if i%2 == 0 {
			module = raydium
		}
		insert(module, "token", mint, time.Duration(20+i)*time.Hour, "exhausted", i, time.Duration(i)*time.Hour)
	}
	insert(pump, "token", mint, 25*time.Hour, "exhausted", 2, 10*time.Minute)
	insert(pump, "token", mint, 26*time.Hour, "exhausted", 5, 2*time.Hour)
	// Never eligible: other claim states, other modules, targets or types.
	insert(pump, "token", mint, 27*time.Hour, "processing", 1, time.Hour)
	insert(pump, "token", mint, 28*time.Hour, "completed", 1, time.Hour)
	insert(pump, "token", mint, 29*time.Hour, "insufficient_evidence", 1, time.Hour)
	insert("other_module", "token", mint, 30*time.Hour, "", 0, 0)
	insert(pump, "wallet", mint, 31*time.Hour, "", 0, 0)
	insert(pump, "token", "", 32*time.Hour, "", 0, 0)

	for limit := 1; limit <= 30; limit++ {
		rows, err := db.QueryContext(ctx, legacyArvisStreamPendingTargetsQuery, limit, pump, raydium)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			want = append(want, id)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		targets, err := arvisStreamPendingTargets(ctx, db, limit)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, 0, len(targets))
		for _, target := range targets {
			got = append(got, target.ID)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("limit %d: indexed query selected %v, legacy query %v", limit, got, want)
		}
		if limit == 30 && len(got) < 24 {
			t.Fatalf("expected all 24 eligible fixtures at limit 30, got %d", len(got))
		}
	}
}
