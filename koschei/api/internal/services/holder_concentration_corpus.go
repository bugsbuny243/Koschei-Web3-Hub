package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"time"
)

const holderConcentrationStatsKey = "owner_resolved_top_share_v1"

type HolderConcentrationContext struct {
	Available    bool      `json:"available"`
	TopSharePct float64   `json:"top_share_pct"`
	TopPercentile float64 `json:"top_percentile"`
	SampleCount  int64     `json:"sample_count"`
	CalculatedAt time.Time `json:"calculated_at,omitempty"`
	Method       string    `json:"method"`
}

// RefreshHolderConcentrationCorpusStats is a bounded background aggregation.
// Request handlers read only the single aggregate row and never scan the corpus.
func RefreshHolderConcentrationCorpusStats(ctx context.Context, db *sql.DB) error {
	if db == nil { return nil }
	stepCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	rows, err := db.QueryContext(stepCtx, `
		SELECT width_bucket(
			LEAST(100::numeric, GREATEST(0::numeric,
				NULLIF(source_payload#>>'{holder_intelligence,top_owner_percentage}','')::numeric)),
			0, 100.0001, 101
		) - 1 AS bucket,
		count(*)
		FROM dossier_source_snapshots
		WHERE (source_payload#>>'{holder_intelligence,available}')::boolean IS TRUE
		  AND (source_payload#>>'{holder_intelligence,owner_aggregation_applied}')::boolean IS TRUE
		  AND NULLIF(source_payload#>>'{holder_intelligence,top_owner_percentage}','') IS NOT NULL
		GROUP BY 1
		ORDER BY 1`)
	if err != nil { return err }
	defer rows.Close()
	buckets := make([]int64, 101)
	var sampleCount int64
	for rows.Next() {
		var bucket int
		var count int64
		if err := rows.Scan(&bucket, &count); err != nil { return err }
		if bucket >= 0 && bucket < len(buckets) { buckets[bucket] = count; sampleCount += count }
	}
	if err := rows.Err(); err != nil { return err }
	raw, err := json.Marshal(buckets)
	if err != nil { return err }
	_, err = db.ExecContext(stepCtx, `
		INSERT INTO holder_concentration_corpus_stats
		(stats_key,sample_count,bucket_width,bucket_counts,calculated_at,source_window_start,source_window_end)
		VALUES ($1,$2,1,$3::jsonb,now(),
			(SELECT min(produced_at) FROM dossier_source_snapshots),
			(SELECT max(produced_at) FROM dossier_source_snapshots))
		ON CONFLICT (stats_key) DO UPDATE SET
			sample_count=EXCLUDED.sample_count,
			bucket_width=EXCLUDED.bucket_width,
			bucket_counts=EXCLUDED.bucket_counts,
			calculated_at=EXCLUDED.calculated_at,
			source_window_start=EXCLUDED.source_window_start,
			source_window_end=EXCLUDED.source_window_end`, holderConcentrationStatsKey, sampleCount, string(raw))
	return err
}

func LoadHolderConcentrationContext(ctx context.Context, db *sql.DB, share float64) HolderConcentrationContext {
	out := HolderConcentrationContext{TopSharePct: share, Method: "nightly_bucketed_owner_resolved_corpus_v1"}
	if db == nil || share < 0 || share > 100 { return out }
	var sampleCount int64
	var raw []byte
	var calculated time.Time
	if err := db.QueryRowContext(ctx, `SELECT sample_count,bucket_counts,calculated_at FROM holder_concentration_corpus_stats WHERE stats_key=$1`, holderConcentrationStatsKey).Scan(&sampleCount,&raw,&calculated); err != nil || sampleCount <= 0 { return out }
	var buckets []int64
	if json.Unmarshal(raw,&buckets) != nil || len(buckets) == 0 { return out }
	index := int(math.Floor(share)); if index < 0 { index=0 }; if index >= len(buckets) { index=len(buckets)-1 }
	var atOrAbove int64
	for i:=index;i<len(buckets);i++ { atOrAbove += buckets[i] }
	out.Available=true;out.SampleCount=sampleCount;out.CalculatedAt=calculated.UTC();out.TopPercentile=math.Round(float64(atOrAbove)/float64(sampleCount)*10000)/100
	return out
}
