package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestPolarWebhookSubscriptionLifecyclePostgres17 drives signed Polar
// deliveries through the webhook handler against the real entitlement schema:
// checkout binding → Professional active → renewal → cancel → revoke. Every
// identity, product and secret here is a CI fixture, not a production value.
func TestPolarWebhookSubscriptionLifecyclePostgres17(t *testing.T) {
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

	const secret = "ci-fixture-polar-webhook-secret"
	const productID = "ci-fixture-polar-product-professional"
	t.Setenv("POLAR_ENVIRONMENT", "sandbox")
	t.Setenv("POLAR_WEBHOOK_SECRET", secret)
	t.Setenv("POLAR_PRODUCT_PROFESSIONAL_ID", productID)

	run := time.Now().UTC().Format("150405.000000")
	subject := "ci-polar-subject-" + run
	email := "ci-polar-" + run + "@example.invalid"
	subscriptionID := "ci-polar-sub-" + run
	strangerSubscriptionID := "ci-polar-sub-stranger-" + run
	if _, err := db.ExecContext(ctx, `
		INSERT INTO app_user_profiles(auth_subject,email,plan_id,status)
		VALUES($1,$2,'free','active')`, subject, email); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		_, _ = db.ExecContext(cleanup, `DELETE FROM billing_provider_events WHERE provider='polar' AND event_id LIKE $1`, "ci-polar-evt-"+run+"%")
		_, _ = db.ExecContext(cleanup, `DELETE FROM entitlements WHERE lower(email)=lower($1) OR external_payment_id IN ($2,$3)`, email, subscriptionID, strangerSubscriptionID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM app_user_profiles WHERE auth_subject=$1`, subject)
	})

	h := &Handler{DB: db, DBRead: db, EntitlementDB: db}
	binding := map[string]any{
		"koschei_auth_subject": subject,
		"koschei_email":        email,
		"koschei_plan":         "professional",
	}
	start := time.Now().UTC().Truncate(time.Second)
	firstPeriodEnd := start.Add(30 * 24 * time.Hour)
	renewedPeriodEnd := firstPeriodEnd.Add(30 * 24 * time.Hour)
	subscription := func(status string, periodEnd time.Time) map[string]any {
		return map[string]any{
			"id": subscriptionID, "status": status, "customer_id": "ci-polar-customer",
			"product_id": productID, "checkout_id": "ci-polar-checkout",
			"current_period_end": periodEnd.Format(time.RFC3339), "metadata": binding,
		}
	}

	deliverWith := func(eventID, eventType string, occurred time.Time, data map[string]any, signingSecret string) map[string]any {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"type": eventType, "timestamp": occurred.Format(time.RFC3339Nano), "data": data})
		if err != nil {
			t.Fatal(err)
		}
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(signingSecret))
		_, _ = mac.Write([]byte(eventID + "." + ts + "."))
		_, _ = mac.Write(raw)
		req := httptest.NewRequest(http.MethodPost, "/api/polar/webhook", bytes.NewReader(raw)).WithContext(ctx)
		req.Header.Set("webhook-id", eventID)
		req.Header.Set("webhook-timestamp", ts)
		req.Header.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		rec := httptest.NewRecorder()
		h.PolarWebhook(rec, req)
		out := map[string]any{"status": rec.Code}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		out["status"] = rec.Code
		return out
	}
	deliver := func(eventID, eventType string, occurred time.Time, data map[string]any) map[string]any {
		t.Helper()
		return deliverWith(eventID, eventType, occurred, data, secret)
	}
	evt := func(name string) string { return "ci-polar-evt-" + run + "-" + name }
	expectStatus := func(step string, got map[string]any, status int) {
		t.Helper()
		if got["status"] != status {
			t.Fatalf("%s: response %v, want HTTP %d", step, got, status)
		}
	}
	ledgered := func(eventID string) bool {
		t.Helper()
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM billing_provider_events WHERE provider='polar' AND event_id=$1)`, eventID).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		return exists
	}
	access := func() planAccessEvaluation {
		t.Helper()
		evaluation, err := h.evaluatePlanAccess(ctx, subject, email)
		if err != nil {
			t.Fatal(err)
		}
		return evaluation
	}
	// professionalGate reports the HTTP status of a Professional-only route,
	// the same gate that guards API key creation.
	professionalGate := func() int {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/account/api-keys", nil)
		req = req.WithContext(context.WithValue(ctx, authContextKey, neonJWTClaims{Sub: subject, Email: email}))
		rec := httptest.NewRecorder()
		h.RequirePlanTier("professional", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })(rec, req)
		return rec.Code
	}
	profilePlan := func() string {
		t.Helper()
		var plan string
		if err := db.QueryRowContext(ctx, `SELECT plan_id FROM app_user_profiles WHERE auth_subject=$1`, subject).Scan(&plan); err != nil {
			t.Fatal(err)
		}
		return plan
	}

	if access().Active || professionalGate() != http.StatusForbidden {
		t.Fatal("a free profile must not hold Professional access before any payment")
	}

	forged := deliverWith(evt("forged"), "subscription.active", start, subscription("active", firstPeriodEnd), "not-the-configured-secret")
	expectStatus("forged signature", forged, http.StatusUnauthorized)
	if ledgered(evt("forged")) || access().Active {
		t.Fatal("a delivery signed with another secret must not be recorded or grant access")
	}

	stranger := subscription("active", firstPeriodEnd)
	stranger["id"] = strangerSubscriptionID
	stranger["metadata"] = map[string]any{"koschei_auth_subject": "ci-polar-unknown-" + run, "koschei_email": "ci-polar-unknown-" + run + "@example.invalid", "koschei_plan": "professional"}
	expectStatus("unknown customer binding", deliver(evt("stranger"), "subscription.active", start, stranger), http.StatusConflict)
	otherProduct := subscription("active", firstPeriodEnd)
	otherProduct["id"] = strangerSubscriptionID
	otherProduct["product_id"] = "ci-fixture-polar-product-other"
	expectStatus("unconfigured product", deliver(evt("other-product"), "subscription.active", start, otherProduct), http.StatusConflict)
	if ledgered(evt("stranger")) || ledgered(evt("other-product")) {
		t.Fatal("rejected bindings must roll back so Polar redelivers them")
	}

	activated := deliver(evt("active"), "subscription.active", start, subscription("active", firstPeriodEnd))
	expectStatus("subscription.active", activated, http.StatusOK)
	if activated["entitlement_activated"] != true || activated["plan"] != "professional" {
		t.Fatalf("subscription.active did not activate Professional: %v", activated)
	}
	got := access()
	if !got.Active || got.Plan != "professional" || got.PaymentProvider != "polar" || got.ExpiresAt == nil || !got.ExpiresAt.Equal(firstPeriodEnd) {
		t.Fatalf("Professional access after activation = %+v, want active until %s", got, firstPeriodEnd)
	}
	if professionalGate() != http.StatusOK || profilePlan() != "professional" {
		t.Fatal("an active Polar subscription must open the Professional gate and profile plan")
	}

	replay := deliver(evt("active"), "subscription.active", start, subscription("active", firstPeriodEnd))
	expectStatus("replayed subscription.active", replay, http.StatusOK)
	if replay["duplicate"] != true {
		t.Fatalf("a redelivered event must be acknowledged as a duplicate: %v", replay)
	}
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM entitlements WHERE payment_provider='polar' AND external_payment_id=$1`, subscriptionID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("subscription %s holds %d entitlement rows, want 1", subscriptionID, rows)
	}

	firstOrder := deliver(evt("order-create"), "order.paid", start.Add(time.Second), map[string]any{
		"paid": true, "billing_reason": "subscription_create", "subscription_id": subscriptionID,
		"product_id": productID, "subscription": subscription("active", firstPeriodEnd),
	})
	expectStatus("first order.paid", firstOrder, http.StatusOK)
	if firstOrder["entitlement_changed"] != false {
		t.Fatalf("the first subscription order must not change the entitlement: %v", firstOrder)
	}

	renewal := deliver(evt("order-cycle"), "order.paid", start.Add(2*time.Second), map[string]any{
		"paid": true, "billing_reason": "subscription_cycle", "subscription_id": subscriptionID,
		"product_id": productID, "subscription": subscription("active", renewedPeriodEnd),
	})
	expectStatus("renewal order.paid", renewal, http.StatusOK)
	if renewal["entitlement_refreshed"] != true {
		t.Fatalf("a paid renewal must refresh the entitlement: %v", renewal)
	}
	if got := access(); !got.Active || got.ExpiresAt == nil || !got.ExpiresAt.Equal(renewedPeriodEnd) {
		t.Fatalf("access after renewal = %+v, want active until %s", got, renewedPeriodEnd)
	}

	canceled := deliver(evt("canceled"), "subscription.canceled", start.Add(3*time.Second), subscription("active", renewedPeriodEnd))
	expectStatus("subscription.canceled", canceled, http.StatusOK)
	if canceled["entitlement_changed"] != false || !access().Active {
		t.Fatalf("cancel at period end must keep the paid period: %v", canceled)
	}

	revoked := deliver(evt("revoked"), "subscription.revoked", start.Add(10*time.Second), subscription("revoked", renewedPeriodEnd))
	expectStatus("subscription.revoked", revoked, http.StatusOK)
	if revoked["entitlement_revoked"] != true || revoked["profile_plan"] != "free" {
		t.Fatalf("subscription.revoked did not end Professional access: %v", revoked)
	}
	if access().Active || professionalGate() != http.StatusForbidden || profilePlan() != "free" {
		t.Fatal("a revoked subscription must close the Professional gate and reset the profile plan")
	}

	late := deliver(evt("late-active"), "subscription.active", start.Add(5*time.Second), subscription("active", renewedPeriodEnd))
	expectStatus("late subscription.active", late, http.StatusOK)
	if late["activation_suppressed"] != true || access().Active {
		t.Fatalf("an activation older than the recorded revocation must not reopen access: %v", late)
	}

	// The owner control plane reads delivery evidence from the same ledger.
	t.Setenv("POLAR_ACCESS_TOKEN", "ci-fixture-polar-access-token")
	t.Setenv(commercialCheckoutEnv, "true")
	status := ownerPolarBillingStatus(ctx, db)
	if status["status"] != "live" || status["webhook_last_event_type"] != "subscription.active" {
		t.Fatalf("owner billing status = %v, want live with the latest verified delivery", status)
	}
	if total, _ := status["webhook_events_total"].(int64); total < 6 {
		t.Fatalf("owner billing status counted %d verified deliveries, want at least the 6 accepted here", total)
	}
}

func TestPolarBillingStateNamesTheFirstBlocker(t *testing.T) {
	at := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		configured, checkout, ledger bool
		events                       int64
		want                         string
	}{
		{false, true, true, 3, "incomplete"},
		{true, false, true, 3, "checkout_paused"},
		{true, true, false, 0, "ledger_unavailable"},
		{true, true, true, 0, "awaiting_first_webhook"},
		{true, true, true, 3, "live"},
	} {
		got, note := polarBillingState(tc.configured, tc.checkout, tc.ledger, tc.events, "subscription.active", at)
		if got != tc.want || note == "" {
			t.Fatalf("polarBillingState(%v,%v,%v,%d) = %q %q, want %q", tc.configured, tc.checkout, tc.ledger, tc.events, got, note, tc.want)
		}
	}
	if _, note := polarBillingState(true, true, true, 1, "order.paid", at); note != "last verified webhook order.paid · 2026-10-25T09:00:00Z" {
		t.Fatalf("live note = %q", note)
	}
}
