package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestIncidentAlertReferenceIntegrityPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var alertID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO security_alert_events
			(source,event_type,severity,target,title,message,dedupe_key,payload)
		VALUES ('global_radar_runtime','global_radar.coverage.attention','high','ethereum-mainnet',$1,$2,$3,'{}'::jsonb)
		RETURNING id::text
	`, "coverage ownership "+suffix, "coverage ownership "+suffix, "coverage-owner-test:"+suffix).Scan(&alertID); err != nil {
		t.Fatal(err)
	}

	h := &Handler{DB: db, DBRead: db}
	validTitle := "coverage incident " + suffix
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_incident_cases WHERE title=$1", validTitle)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_alert_events WHERE id=$1::uuid", alertID)
	})

	badReq := httptest.NewRequest(http.MethodPost, "/api/owner/incidents", strings.NewReader(
		`{"title":"invalid alert incident","severity":"high","alert_refs":["00000000-0000-0000-0000-000000000000"]}`,
	))
	badRes := httptest.NewRecorder()
	h.ownerIncidentCreate(badRes, badReq)
	if badRes.Code != http.StatusConflict {
		t.Fatalf("unresolved alert status=%d body=%s", badRes.Code, badRes.Body.String())
	}

	body := fmt.Sprintf(`{"title":%q,"severity":"high","network":"ethereum-mainnet","target":"head-ingest","alert_refs":[%q]}`, validTitle, alertID)
	createReq := httptest.NewRequest(http.MethodPost, "/api/owner/incidents", strings.NewReader(body))
	createRes := httptest.NewRecorder()
	h.ownerIncidentCreate(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRes.Code, createRes.Body.String())
	}

	var created struct {
		Incident ownerIncidentRecord `json:"incident"`
	}
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Incident.ID == "" || len(created.Incident.AlertRefs) != 1 || created.Incident.AlertRefs[0] != alertID {
		t.Fatalf("created incident alert linkage=%#v", created.Incident)
	}

	var source string
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(payload->>'alert_sources','')
		FROM security_incident_actions
		WHERE incident_id=$1::uuid AND action_type='created'
	`, created.Incident.ID).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, "security_alert_event:global_radar_runtime:global_radar.coverage.attention") {
		t.Fatalf("created audit missing resolved alert source: %q", source)
	}

	secondTitle := "coverage link incident " + suffix
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_incident_cases WHERE title=$1", secondTitle)
	})
	secondReq := httptest.NewRequest(http.MethodPost, "/api/owner/incidents", strings.NewReader(fmt.Sprintf(`{"title":%q,"severity":"medium"}`, secondTitle)))
	secondRes := httptest.NewRecorder()
	h.ownerIncidentCreate(secondRes, secondReq)
	if secondRes.Code != http.StatusCreated {
		t.Fatalf("second create status=%d body=%s", secondRes.Code, secondRes.Body.String())
	}
	var second struct {
		Incident ownerIncidentRecord `json:"incident"`
	}
	if err := json.Unmarshal(secondRes.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}

	badLinkReq := httptest.NewRequest(http.MethodPost, "/api/owner/incidents/"+second.Incident.ID, strings.NewReader(
		`{"action":"link_alert","ref":"00000000-0000-0000-0000-000000000000"}`,
	))
	badLinkRes := httptest.NewRecorder()
	h.ownerIncidentAction(badLinkRes, badLinkReq, second.Incident.ID)
	if badLinkRes.Code != http.StatusConflict {
		t.Fatalf("bad link status=%d body=%s", badLinkRes.Code, badLinkRes.Body.String())
	}

	linkReq := httptest.NewRequest(http.MethodPost, "/api/owner/incidents/"+second.Incident.ID, strings.NewReader(
		fmt.Sprintf(`{"action":"link_alert","summary":"own coverage episode","ref":%q}`, alertID),
	))
	linkRes := httptest.NewRecorder()
	h.ownerIncidentAction(linkRes, linkReq, second.Incident.ID)
	if linkRes.Code != http.StatusOK {
		t.Fatalf("link status=%d body=%s", linkRes.Code, linkRes.Body.String())
	}

	var linked bool
	if err := db.QueryRowContext(ctx, "SELECT alert_refs ? $2 FROM security_incident_cases WHERE id=$1::uuid", second.Incident.ID, alertID).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if !linked {
		t.Fatal("resolved alert was not linked to incident")
	}
}

func TestOwnerCoverageEpisodeOwnershipPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	componentID := "coverage-owner-" + suffix
	incidentTitle := "coverage ownership incident " + suffix
	var alertID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO security_alert_events
			(source,event_type,severity,target,title,message,dedupe_key,payload)
		VALUES ('global_radar_runtime','global_radar.coverage.attention','high','ethereum-mainnet',$1,$2,$3,'{}'::jsonb)
		RETURNING id::text
	`, componentID, componentID, "coverage-owner-view:"+suffix).Scan(&alertID); err != nil {
		t.Fatal(err)
	}
	var episodeID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO global_radar_coverage_episodes
			(component_id,network_id,status,coverage_status,coverage_reason,open_alert_id)
		VALUES ($1,'ethereum-mainnet','open','blind_spot','provider_head_is_behind_durable_cursor',$2::uuid)
		RETURNING id::text
	`, componentID, alertID).Scan(&episodeID); err != nil {
		t.Fatal(err)
	}

	h := &Handler{DB: db, DBRead: db}
	createReq := httptest.NewRequest(http.MethodPost, "/api/owner/incidents", strings.NewReader(
		fmt.Sprintf(`{"title":%q,"severity":"high","network":"ethereum-mainnet","target":%q,"alert_refs":[%q]}`, incidentTitle, componentID, alertID),
	))
	createRes := httptest.NewRecorder()
	h.ownerIncidentCreate(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("incident create status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	var created struct {
		Incident ownerIncidentRecord `json:"incident"`
	}
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_incident_cases WHERE id=$1::uuid", created.Incident.ID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM global_radar_coverage_episodes WHERE id=$1::uuid", episodeID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_alert_events WHERE id=$1::uuid", alertID)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/owner/radar/coverage-episodes?status=open&network=ethereum-mainnet&limit=20", nil)
	res := httptest.NewRecorder()
	h.OwnerRadarCoverageEpisodes(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("coverage episode status=%d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		CoverageEpisodes []ownerCoverageEpisodeRecord `json:"coverage_episodes"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range response.CoverageEpisodes {
		if item.ID != episodeID {
			continue
		}
		found = true
		if item.OpenAlertID != alertID || item.Status != "open" || item.CoverageStatus != "blind_spot" {
			t.Fatalf("unexpected episode: %#v", item)
		}
		if len(item.Incidents) != 1 || item.Incidents[0].ID != created.Incident.ID || item.Incidents[0].IncidentRef != created.Incident.IncidentRef {
			t.Fatalf("ownership linkage missing: %#v", item.Incidents)
		}
	}
	if !found {
		t.Fatalf("coverage episode %s missing from owner view", episodeID)
	}
}
