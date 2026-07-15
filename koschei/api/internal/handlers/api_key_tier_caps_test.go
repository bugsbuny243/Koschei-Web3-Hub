package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type apiKeyCapTestState struct {
	mu      sync.Mutex
	monthly int
	rpm     int
}

type apiKeyCapTestDriver struct {
	state *apiKeyCapTestState
}

func (d *apiKeyCapTestDriver) Open(string) (driver.Conn, error) {
	return &apiKeyCapTestConn{state: d.state}, nil
}

type apiKeyCapTestConn struct {
	state *apiKeyCapTestState
}

func (c *apiKeyCapTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (c *apiKeyCapTestConn) Close() error { return nil }
func (c *apiKeyCapTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *apiKeyCapTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "FROM verified_wallet_links"):
		// A failed token-access lookup must fall back to basic caps rather than
		// trusting caller-provided limits or failing key creation.
		return nil, errors.New("forced token-access lookup failure")
	case strings.Contains(query, "INSERT INTO api_keys"):
		if len(args) < 7 {
			return nil, errors.New("missing api key insert arguments")
		}
		monthly, ok := args[5].Value.(int64)
		if !ok {
			return nil, errors.New("monthly limit was not an integer")
		}
		rpm, ok := args[6].Value.(int64)
		if !ok {
			return nil, errors.New("rpm limit was not an integer")
		}
		c.state.mu.Lock()
		c.state.monthly = int(monthly)
		c.state.rpm = int(rpm)
		c.state.mu.Unlock()
		return &apiKeyCapTestRows{columns: []string{"id"}, values: [][]driver.Value{{"api-key-id"}}}, nil
	default:
		return nil, errors.New("unexpected query")
	}
}

func (c *apiKeyCapTestConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "INSERT INTO security_audit_events") {
		return driver.RowsAffected(1), nil
	}
	return nil, errors.New("unexpected exec")
}

type apiKeyCapTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *apiKeyCapTestRows) Columns() []string { return r.columns }
func (r *apiKeyCapTestRows) Close() error      { return nil }
func (r *apiKeyCapTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func TestCreateAPIKeyClampsUnrankedCallerToBasicCaps(t *testing.T) {
	state := &apiKeyCapTestState{}
	driverName := "api_key_tier_caps_test"
	sql.Register(driverName, &apiKeyCapTestDriver{state: state})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &Handler{DB: db}
	req := httptest.NewRequest(http.MethodPost, "/api/account/api-keys", bytes.NewBufferString(`{
		"name":"oversized",
		"monthly_limit":999999999,
		"rate_limit_per_minute":999999999
	}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), authContextKey, neonJWTClaims{
		Sub: "user-basic-fallback", Email: "basic@example.com",
	}))
	res := httptest.NewRecorder()

	h.CreateAPIKey(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}

	state.mu.Lock()
	persistedMonthly, persistedRPM := state.monthly, state.rpm
	state.mu.Unlock()
	if persistedMonthly != apiKeyCapsByTier["basic"].MaxMonthly {
		t.Fatalf("persisted monthly_limit=%d, want %d", persistedMonthly, apiKeyCapsByTier["basic"].MaxMonthly)
	}
	if persistedRPM != apiKeyCapsByTier["basic"].MaxRPM {
		t.Fatalf("persisted rate_limit_per_minute=%d, want %d", persistedRPM, apiKeyCapsByTier["basic"].MaxRPM)
	}

	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := int(payload["monthly_limit"].(float64)); got != persistedMonthly {
		t.Fatalf("response monthly_limit=%d, persisted=%d", got, persistedMonthly)
	}
	if got := int(payload["rate_limit_per_minute"].(float64)); got != persistedRPM {
		t.Fatalf("response rate_limit_per_minute=%d, persisted=%d", got, persistedRPM)
	}
	if payload["tier"] != "basic" {
		t.Fatalf("response tier=%v", payload["tier"])
	}
}

func TestAPIKeyDefaultsRemainSubjectToTierCaps(t *testing.T) {
	monthly, rpm := clampAPIKeyLimits(0, 0, apiKeyCapsByTier["basic"])
	if monthly != 1000 || rpm != 30 {
		t.Fatalf("basic defaults after clamp = %d/%d", monthly, rpm)
	}
}

func TestAPIKeyAuthAbsoluteCeilingProtectsExistingRows(t *testing.T) {
	principal := clampAPIPrincipalToAbsoluteCaps(apiPrincipal{
		MonthlyLimit:       999999999,
		RateLimitPerMinute: 999999999,
	})
	if principal.MonthlyLimit != 200000 || principal.RateLimitPerMinute != 600 {
		t.Fatalf("absolute clamp = %#v", principal)
	}
}
