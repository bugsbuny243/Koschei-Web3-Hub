package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const paddleWebhookMaxSkew = 5 * time.Minute

type checkoutRequest struct {
	Package       string `json:"package"`
	ProductID     string `json:"product_id"`
	CustomerEmail string `json:"customer_email"`
	Email         string `json:"email"`
	PlanTier      string `json:"plan_tier"`
}

type paddleAPIResponse struct {
	Data struct {
		ID       string `json:"id"`
		Checkout struct {
			URL string `json:"url"`
		} `json:"checkout"`
	} `json:"data"`
	Error any `json:"error"`
}

type paddleAPIError struct {
	StatusCode int
	Message    string
}

func (e paddleAPIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("Paddle API returned %d", e.StatusCode)
	}
	return e.Message
}

type paddleEvent struct {
	EventType string          `json:"event_type"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
}

type paddleSubscriptionData struct {
	ID                   string         `json:"id"`
	CustomerID           string         `json:"customer_id"`
	Status               string         `json:"status"`
	CurrentBillingPeriod *billingPeriod `json:"current_billing_period"`
	ScheduledChange      *struct {
		Action      string `json:"action"`
		EffectiveAt string `json:"effective_at"`
	} `json:"scheduled_change"`
	Items []struct {
		Price struct {
			ID        string `json:"id"`
			ProductID string `json:"product_id"`
		} `json:"price"`
	} `json:"items"`
	CustomData map[string]any `json:"custom_data"`
}

type paddleTransactionData struct {
	ID             string         `json:"id"`
	Status         string         `json:"status"`
	CustomerID     string         `json:"customer_id"`
	SubscriptionID string         `json:"subscription_id"`
	BilledAt       string         `json:"billed_at"`
	CreatedAt      string         `json:"created_at"`
	Details        map[string]any `json:"details"`
	CustomData     map[string]any `json:"custom_data"`
}

type billingPeriod struct {
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

func (h *Handler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req checkoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	planTier := normalizePlanTier(firstNonEmpty(req.Package, req.PlanTier, req.ProductID))
	priceID := paddleConfiguredPriceID(planTier)
	apiKeyConfigured := strings.TrimSpace(os.Getenv("PADDLE_API_KEY")) != ""
	log.Printf("paddle checkout request: package=%q price_id_configured=%t environment=%q", planTier, priceID != "", paddleEnvironment())
	if email == "" || planTier == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid_package_required", "message": "Choose a valid Paddle package."})
		return
	}
	if priceID == "" || !apiKeyConfigured {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "paddle_config_missing", "message": "Paddle global checkout is not configured yet."})
		return
	}
	checkoutURL, err := h.CreateCheckoutSession(r.Context(), priceID, email, claims.Sub, planTier)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "paddle_checkout_failed", "message": safeError(err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"checkout_url": checkoutURL})
}

func (h *Handler) CreateCheckoutSession(ctx context.Context, priceID, customerEmail, authSubject, planTier string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("PADDLE_API_KEY"))
	if apiKey == "" {
		return "", errors.New("PADDLE_API_KEY is not configured")
	}
	planTier = normalizePlanTier(planTier)
	if planTier == "" {
		return "", errors.New("invalid plan tier")
	}
	customerID, err := h.createPaddleCustomer(ctx, customerEmail, planTier)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"collection_mode": "automatic",
		"customer_id":     customerID,
		"custom_data": map[string]any{
			"auth_subject":   strings.TrimSpace(authSubject),
			"customer_email": strings.ToLower(strings.TrimSpace(customerEmail)),
			"package":        planTier,
			"package_id":     planTier,
			"package_name":   packageName(planTier),
			"plan_tier":      planTier,
			"provider":       "paddle",
			"source":         "koschei_web3_hub",
			"user_email":     strings.ToLower(strings.TrimSpace(customerEmail)),
			"user_id":        strings.TrimSpace(authSubject),
		},
		"items": []map[string]any{paddleTransactionItem(priceID, planTier)},
	}
	var out paddleAPIResponse
	if err := paddleRequest(ctx, http.MethodPost, "/transactions", payload, &out); err != nil {
		return "", err
	}
	checkoutURLFound := out.Data.Checkout.URL != ""
	log.Printf("paddle checkout transaction response: package=%q checkout_url_found=%t", planTier, checkoutURLFound)
	if !checkoutURLFound {
		return "", errors.New("Paddle response did not include checkout.url. Confirm the Paddle default payment link and approved checkout domain are configured.")
	}
	return out.Data.Checkout.URL, nil
}

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.ValidateWebhookSignature(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_signature"})
		return
	}
	var event paddleEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_webhook"})
		return
	}
	eventType := event.EventType
	if eventType == "" {
		eventType = event.Type
	}
	var err error
	switch eventType {
	case "subscription.created", "subscription.updated":
		err = h.upsertPaddleSubscription(r.Context(), event.Data, false)
	case "payment.succeeded", "transaction.completed", "transaction.paid":
		err = h.markPaddlePaymentSucceeded(r.Context(), event.Data)
	case "payment.failed", "transaction.payment_failed", "transaction.past_due":
		err = h.markPaddlePaymentFailed(r.Context(), event.Data)
	case "subscription.canceled", "subscription.cancelled":
		err = h.upsertPaddleSubscription(r.Context(), event.Data, true)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "webhook_processing_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) ValidateWebhookSignature(r *http.Request) bool {
	secret := strings.TrimSpace(os.Getenv("PADDLE_WEBHOOK_SECRET"))
	if secret == "" {
		return false
	}
	sigHeader := strings.TrimSpace(r.Header.Get("Paddle-Signature"))
	if sigHeader == "" {
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 2<<20))
	if err != nil {
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	timestamp, signatures := parsePaddleSignature(sigHeader)
	if timestamp == "" || len(signatures) == 0 {
		return false
	}
	unixTS, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if skew := time.Since(time.Unix(unixTS, 0)); skew > paddleWebhookMaxSkew || skew < -paddleWebhookMaxSkew {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + ":"))
	_, _ = mac.Write(body)
	expected := []byte(hex.EncodeToString(mac.Sum(nil)))
	for _, signature := range signatures {
		if subtle.ConstantTimeCompare(expected, []byte(signature)) == 1 {
			return true
		}
	}
	return false
}

func (h *Handler) CheckB2BQuota(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := apiPrincipalFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		limit := p.RateLimitPerMinute
		if limit <= 0 {
			limit = 100
		}
		if h.Limiter != nil && !h.Limiter.allow("b2b:"+p.KeyID+":"+r.URL.Path, limit, time.Minute) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limit_exceeded"})
			return
		}
		quota := p.MonthlyLimit
		if quota <= 0 {
			quota = 1000
		}
		var used int
		start := time.Now().UTC().Format("2006-01-02")
		monthStart := start[:8] + "01"
		_ = h.DB.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(GREATEST(credits_reserved, credits_charged)),0) FROM api_usage_events WHERE api_key_id=$1 AND created_at >= $2::date`, p.KeyID, monthStart).Scan(&used)
		if used >= quota {
			writeJSON(w, http.StatusPaymentRequired, map[string]any{"error": "monthly_quota_exceeded", "monthly_quota": quota, "used": used})
			return
		}
		next(w, r)
	}
}

func (h *Handler) B2BUsage(w http.ResponseWriter, r *http.Request) {
	p, ok := apiPrincipalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	monthStart := time.Now().UTC().Format("2006-01-") + "01"
	var used int
	_ = h.DB.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(credits_charged),0) FROM api_usage_events WHERE api_key_id=$1 AND created_at >= $2::date`, p.KeyID, monthStart).Scan(&used)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "api_key_id": p.KeyID, "email": p.Email, "rate_limit_per_minute": p.RateLimitPerMinute, "monthly_quota": p.MonthlyLimit, "monthly_used": used, "monthly_remaining": maxInt(p.MonthlyLimit-used, 0)})
}

func (h *Handler) createPaddleCustomer(ctx context.Context, email, planTier string) (string, error) {
	payload := map[string]any{"email": strings.ToLower(strings.TrimSpace(email)), "custom_data": map[string]string{"package_id": planTier, "package_name": packageName(planTier), "plan_tier": planTier, "provider": "paddle", "source": "koschei_web3_hub"}}
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := paddleRequest(ctx, http.MethodPost, "/customers", payload, &out); err != nil {
		return "", err
	}
	if out.Data.ID == "" {
		return "", errors.New("Paddle response did not include customer id")
	}
	return out.Data.ID, nil
}

func paddleRequest(ctx context.Context, method, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	base := paddleAPIBaseURL()
	req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(os.Getenv("PADDLE_API_KEY")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	keys := jsonTopLevelKeys(respBody)
	log.Printf("paddle api response: method=%s path=%s environment=%q status=%d top_level_keys=%v", method, path, paddleEnvironment(), resp.StatusCode, keys)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return paddleAPIError{StatusCode: resp.StatusCode, Message: paddleErrorMessage(resp.StatusCode, respBody)}
	}
	if out != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

func (h *Handler) upsertPaddleSubscription(ctx context.Context, raw json.RawMessage, canceled bool) error {
	var sub paddleSubscriptionData
	if err := json.Unmarshal(raw, &sub); err != nil {
		return err
	}
	if sub.ID == "" || sub.CustomerID == "" {
		return errors.New("missing subscription or customer id")
	}
	productID, planTier := subscriptionProductAndTier(sub)
	email := customDataString(sub.CustomData, "customer_email")
	if email == "" {
		email = sub.CustomerID + "@paddle.local"
	}
	status := sub.Status
	if canceled {
		status = "cancelled"
	}
	periodEnd := parsePaddleTime("")
	if sub.CurrentBillingPeriod != nil {
		periodEnd = parsePaddleTime(sub.CurrentBillingPeriod.EndsAt)
	}
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var customerPK int
	if err := tx.QueryRowContext(ctx, `INSERT INTO paddle_customers (paddle_customer_id,email) VALUES ($1,lower($2)) ON CONFLICT (paddle_customer_id) DO UPDATE SET email=EXCLUDED.email RETURNING id`, sub.CustomerID, email).Scan(&customerPK); err != nil {
		return err
	}
	var subscriptionPK int
	if err := tx.QueryRowContext(ctx, `INSERT INTO paddle_subscriptions (paddle_subscription_id,customer_id,product_id,plan_tier,status,current_period_end,cancel_at_period_end,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,now()) ON CONFLICT (paddle_subscription_id) DO UPDATE SET customer_id=EXCLUDED.customer_id,product_id=EXCLUDED.product_id,plan_tier=EXCLUDED.plan_tier,status=EXCLUDED.status,current_period_end=EXCLUDED.current_period_end,cancel_at_period_end=EXCLUDED.cancel_at_period_end,updated_at=now() RETURNING id`, sub.ID, customerPK, productID, planTier, status, periodEnd, sub.ScheduledChange != nil && sub.ScheduledChange.Action == "cancel").Scan(&subscriptionPK); err != nil {
		return err
	}
	if status == "active" || status == "trialing" {
		if err := h.ensureSubscriptionAPIKey(ctx, tx, subscriptionPK, sub.ID, email, planTier); err != nil {
			return err
		}
	} else if canceled {
		_, _ = tx.ExecContext(ctx, `UPDATE api_keys SET status='revoked', revoked_at=now() WHERE subscription_id=$1 AND status='active'`, subscriptionPK)
	}
	return tx.Commit()
}

func (h *Handler) ensureSubscriptionAPIKey(ctx context.Context, tx *sql.Tx, subscriptionPK int, paddleSubID, email, planTier string) error {
	var exists string
	err := tx.QueryRowContext(ctx, `SELECT id::text FROM api_keys WHERE subscription_id=$1 AND status='active' LIMIT 1`, subscriptionPK).Scan(&exists)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE api_keys SET monthly_limit=$1, monthly_quota=$1, rate_limit_per_minute=$2 WHERE id=$3`, planMonthlyQuota(planTier), planRateLimit(planTier), exists)
		return err
	}
	if err != sql.ErrNoRows {
		return err
	}
	raw, err := newRawAPIKey()
	if err != nil {
		return err
	}
	prefix := raw
	if len(prefix) > 18 {
		prefix = prefix[:18]
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO api_keys (auth_subject,email,name,key_prefix,key_hash,monthly_limit,rate_limit_per_minute,subscription_id,monthly_quota) VALUES ($1,lower($2),$3,$4,$5,$6,$7,$8,$6)`, "paddle:"+paddleSubID, email, "Paddle "+planTier+" API key", prefix, hashAPIKey(raw), planMonthlyQuota(planTier), planRateLimit(planTier), subscriptionPK)
	return err
}

func (h *Handler) markPaddlePaymentSucceeded(ctx context.Context, raw json.RawMessage) error {
	var txData paddleTransactionData
	if err := json.Unmarshal(raw, &txData); err != nil {
		return err
	}
	if txData.Status != "" && txData.Status != "completed" && txData.Status != "paid" {
		return nil
	}
	packageID := normalizePlanTier(firstNonEmpty(customDataString(txData.CustomData, "package"), customDataString(txData.CustomData, "package_id"), customDataString(txData.CustomData, "plan_tier")))
	email := strings.ToLower(strings.TrimSpace(firstNonEmpty(customDataString(txData.CustomData, "customer_email"), customDataString(txData.CustomData, "user_email"))))
	if (email == "" || packageID == "") && txData.SubscriptionID != "" {
		var storedEmail, storedPlan string
		_ = h.DB.QueryRowContext(ctx, `
			SELECT lower(c.email), COALESCE(s.plan_tier, '')
			FROM paddle_subscriptions s
			JOIN paddle_customers c ON c.id = s.customer_id
			WHERE s.paddle_subscription_id = $1`, txData.SubscriptionID).Scan(&storedEmail, &storedPlan)
		if email == "" {
			email = storedEmail
		}
		if packageID == "" {
			packageID = normalizePlanTier(storedPlan)
		}
	}
	externalPaymentID := strings.TrimSpace(txData.ID)
	if externalPaymentID == "" {
		externalPaymentID = strings.TrimSpace(txData.SubscriptionID)
	}
	if email != "" && packageID != "" && externalPaymentID != "" {
		if _, err := h.activatePackageEntitlement(ctx, email, packageID, "paddle", externalPaymentID); err != nil {
			return err
		}
	}
	if txData.SubscriptionID == "" {
		return nil
	}
	_, err := h.DB.ExecContext(ctx, `UPDATE paddle_subscriptions SET status='active', current_period_end=COALESCE(current_period_end, now() + interval '1 month'), updated_at=now() WHERE paddle_subscription_id=$1`, txData.SubscriptionID)
	return err
}

func (h *Handler) markPaddlePaymentFailed(ctx context.Context, raw json.RawMessage) error {
	var txData paddleTransactionData
	if err := json.Unmarshal(raw, &txData); err != nil {
		return err
	}
	if txData.SubscriptionID == "" {
		return nil
	}
	_, err := h.DB.ExecContext(ctx, `UPDATE paddle_subscriptions SET status='expired', updated_at=now() WHERE paddle_subscription_id=$1`, txData.SubscriptionID)
	return err
}

func parsePaddleSignature(header string) (string, []string) {
	var ts string
	var signatures []string
	for _, part := range strings.Split(header, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "ts":
			ts = kv[1]
		case "h1":
			signatures = append(signatures, kv[1])
		}
	}
	return ts, signatures
}

func paddleEnvironment() string {
	env := strings.ToLower(strings.TrimSpace(firstEnv("PADDLE_ENVIRONMENT", "PADDLE_ENV")))
	if env == "sandbox" {
		return "sandbox"
	}
	return "production"
}

func paddleAPIBaseURL() string {
	if paddleEnvironment() == "sandbox" {
		return "https://sandbox-api.paddle.com"
	}
	return "https://api.paddle.com"
}

func jsonTopLevelKeys(body []byte) []string {
	var raw map[string]json.RawMessage
	if len(body) == 0 || json.Unmarshal(body, &raw) != nil {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func paddleErrorMessage(statusCode int, body []byte) string {
	fallback := fmt.Sprintf("Paddle API returned %d.", statusCode)
	var raw map[string]any
	if len(body) == 0 || json.Unmarshal(body, &raw) != nil {
		return fallback
	}
	if msg := paddleMessageFromValue(raw["error"]); msg != "" {
		return msg
	}
	if msg := paddleMessageFromValue(raw["message"]); msg != "" {
		return msg
	}
	return fallback
}

func paddleMessageFromValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		for _, key := range []string{"detail", "message", "description", "type"} {
			if msg := paddleMessageFromValue(v[key]); msg != "" {
				return msg
			}
		}
	case []any:
		for _, item := range v {
			if msg := paddleMessageFromValue(item); msg != "" {
				return msg
			}
		}
	}
	return ""
}

func paddleConfiguredPriceID(planTier string) string {
	switch normalizePlanTier(planTier) {
	case "starter":
		return strings.TrimSpace(firstEnv("PADDLE_STARTER_PRICE_ID", "PADDLE_STARTER_PRODUCT_ID"))
	case "builder":
		return strings.TrimSpace(firstEnv("PADDLE_BUILDER_PRICE_ID", "PADDLE_PROFESSIONAL_PRODUCT_ID"))
	case "studio":
		return strings.TrimSpace(firstEnv("PADDLE_STUDIO_PRICE_ID", "PADDLE_ENTERPRISE_PRODUCT_ID"))
	default:
		return ""
	}
}

func paddleConfiguredProductID(planTier string) string {
	return paddleConfiguredPriceID(planTier)
}

func paddleTransactionItem(priceID, planTier string) map[string]any {
	return map[string]any{"price_id": priceID, "quantity": 1}
}

func normalizePlanTier(planTier string) string {
	switch strings.ToLower(strings.TrimSpace(planTier)) {
	case "starter":
		return "starter"
	case "builder", "pro", "professional":
		return "builder"
	case "studio", "enterprise":
		return "studio"
	default:
		return ""
	}
}

func subscriptionProductAndTier(sub paddleSubscriptionData) (string, string) {
	var productID string
	for _, item := range sub.Items {
		if item.Price.ProductID != "" {
			productID = item.Price.ProductID
			break
		}
		if item.Price.ID != "" {
			productID = item.Price.ID
		}
	}
	planTier := normalizePlanTier(customDataString(sub.CustomData, "plan_tier"))
	if planTier == "" {
		planTier = tierFromProductID(productID)
	}
	return productID, planTier
}

func tierFromProductID(productID string) string {
	for _, tier := range []string{"starter", "builder", "studio"} {
		if productID != "" && productID == paddleConfiguredProductID(tier) {
			return tier
		}
	}
	return ""
}

func customDataString(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	if v, ok := data[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func parsePaddleTime(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t
	}
	return nil
}

func planMonthlyPriceCents(planTier string) int {
	switch normalizePlanTier(planTier) {
	case "starter":
		return 29900
	case "builder":
		return 99900
	case "studio":
		return 499900
	default:
		return 29900
	}
}

func planMonthlyQuota(planTier string) int {
	switch normalizePlanTier(planTier) {
	case "starter":
		return 1000
	case "builder":
		return 10000
	case "studio":
		return 100000
	default:
		return 1000
	}
}

func planRateLimit(planTier string) int {
	switch normalizePlanTier(planTier) {
	case "starter":
		return 100
	case "builder":
		return 500
	case "studio":
		return 2000
	default:
		return 100
	}
}

func safeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 240 {
		return msg[:240]
	}
	return msg
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
