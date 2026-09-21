package handlers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"koschei/api/internal/services"
)

// This driver exercises the real HTTP -> store -> database/sql call path,
// including cancellation and concurrent cache misses, without a live provider.
type publicRadarTestQuery func(context.Context, string, []driver.NamedValue) (driver.Rows, error)
type publicRadarTestConnector struct{ query publicRadarTestQuery }
type publicRadarTestConn struct{ query publicRadarTestQuery }
type publicRadarTestDriver struct{}
type publicRadarTestRows struct {
	values  [][]driver.Value
	columns int
}

func (c publicRadarTestConnector) Connect(context.Context) (driver.Conn, error) {
	return &publicRadarTestConn{c.query}, nil
}
func (c publicRadarTestConnector) Driver() driver.Driver { return publicRadarTestDriver{} }
func (publicRadarTestDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use connector")
}
func (*publicRadarTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (*publicRadarTestConn) Close() error { return nil }
func (*publicRadarTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}
func (c *publicRadarTestConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.query(ctx, query, args)
}
func (r *publicRadarTestRows) Columns() []string {
	out := make([]string, r.columns)
	for i := range out {
		out[i] = fmt.Sprintf("column_%d", i)
	}
	return out
}
func (*publicRadarTestRows) Close() error { return nil }
func (r *publicRadarTestRows) Next(dest []driver.Value) error {
	if len(r.values) == 0 {
		return io.EOF
	}
	copy(dest, r.values[0])
	r.values = r.values[1:]
	return nil
}

func publicRadarFixtureRows(query string) (driver.Rows, error) {
	now := time.Now().UTC()
	switch {
	case strings.Contains(query, "FROM security_radar_verdicts v"):
		if !strings.Contains(query, "interval '24 hours'") {
			return nil, errors.New("unexpected historical verdict fallback")
		}
		return &publicRadarTestRows{columns: 24}, nil
	case strings.Contains(query, "AS raw_stream_events_estimate"):
		return &publicRadarTestRows{columns: 2, values: [][]driver.Value{{now, int64(2500000)}}}, nil
	case strings.Contains(query, "FROM arvis_stream_processing"):
		return &publicRadarTestRows{columns: 7, values: [][]driver.Value{{int64(0), int64(1), int64(0), int64(0), int64(1), int64(0), now}}}, nil
	case strings.Contains(query, "LEFT JOIN LATERAL"):
		return &publicRadarTestRows{columns: 3, values: [][]driver.Value{{services.ModulePumpSybilRadar, now, now}, {services.ModuleRaydiumPoolGuardian, nil, nil}}}, nil
	default:
		return nil, fmt.Errorf("unexpected public telemetry query: %.80s", query)
	}
}

func TestPublicRadarLiveFeedRecentOnlyAndCachedBoundedTelemetry(t *testing.T) {
	var verdictReads, telemetryReads atomic.Int64
	db := sql.OpenDB(publicRadarTestConnector{query: func(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
		if strings.Contains(query, "FROM security_radar_verdicts v") {
			verdictReads.Add(1)
		} else {
			telemetryReads.Add(1)
		}
		if strings.Contains(query, "count(*) FROM security_radar_stream_events") {
			t.Error("public poll performed lifetime journal count")
		}
		return publicRadarFixtureRows(query)
	}})
	defer db.Close()
	h := &Handler{DB: db}
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		h.PublicRadarLiveFeed(recorder, httptest.NewRequest(http.MethodGet, "/api/public/soc/feed", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		pipeline := body["pipeline"].(map[string]any)
		if pipeline["raw_stream_events"] != nil || pipeline["recognized_events"] != nil {
			t.Fatalf("unknown inventory must not become exact counts: %#v", pipeline)
		}
		if pipeline["raw_stream_events_estimate"] != float64(2500000) || pipeline["estimate_source"] != "postgres_statistics" {
			t.Fatalf("estimate provenance missing: %#v", pipeline)
		}
		if pipeline["telemetry_status"] != "available" || body["pipeline_status"] != "healthy" {
			t.Fatalf("unexpected telemetry: %#v", pipeline)
		}
	}
	if verdictReads.Load() != 2 || telemetryReads.Load() != 3 {
		t.Fatalf("verdict reads=%d telemetry reads=%d; wanted 2 and 3", verdictReads.Load(), telemetryReads.Load())
	}
}

func TestPublicRadarLiveFeedSlowTelemetryIsBoundedAndUnknown(t *testing.T) {
	db := sql.OpenDB(publicRadarTestConnector{query: func(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
		if strings.Contains(query, "FROM security_radar_verdicts v") {
			return publicRadarFixtureRows(query)
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}})
	defer db.Close()
	h := &Handler{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	recorder := httptest.NewRecorder()
	h.PublicRadarLiveFeed(recorder, httptest.NewRequest(http.MethodGet, "/api/public/soc/feed", nil).WithContext(ctx))
	if time.Since(start) > time.Second {
		t.Fatal("optional telemetry blocked public verdict response")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("valid verdict read should survive optional telemetry loss: %s", recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	pipeline := body["pipeline"].(map[string]any)
	if body["pipeline_status"] != "unknown" || pipeline["telemetry_status"] != "unavailable" || pipeline["processing_completed"] != nil {
		t.Fatalf("telemetry failure was hidden: %#v", pipeline)
	}
}

func TestPublicRadarLiveFeedVerdictFailureRemainsUnavailable(t *testing.T) {
	db := sql.OpenDB(publicRadarTestConnector{query: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
		return nil, context.DeadlineExceeded
	}})
	defer db.Close()
	recorder := httptest.NewRecorder()
	(&Handler{DB: db}).PublicRadarLiveFeed(recorder, httptest.NewRequest(http.MethodGet, "/api/public/soc/feed", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"ok":false`) {
		t.Fatalf("verdict failure must fail closed: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicRadarTelemetryCoalescesConcurrentPolls(t *testing.T) {
	var queries atomic.Int64
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	db := sql.OpenDB(publicRadarTestConnector{query: func(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
		queries.Add(1)
		if strings.Contains(query, "AS raw_stream_events_estimate") {
			once.Do(func() { close(started) })
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return publicRadarFixtureRows(query)
	}})
	defer db.Close()
	h := &Handler{DB: db}
	var wg sync.WaitGroup
	wg.Add(8)
	for i := 0; i < 8; i++ {
		go func() {
			defer wg.Done()
			if got := h.publicRadarLiveTelemetry(context.Background(), db); got["telemetry_status"] != "available" {
				t.Errorf("unexpected snapshot: %#v", got)
			}
		}()
	}
	<-started
	close(release)
	wg.Wait()
	if queries.Load() != 3 {
		t.Fatalf("concurrent polls caused %d telemetry queries; want one 3-query refresh", queries.Load())
	}
}
