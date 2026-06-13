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

var errPaddleCheckoutURLMissing = errors.New("paddle_checkout_url_missing")

type checkoutRequest struct {
	Package       string `json:"package"`
	ProductID     string `json:"product_id"`
	CustomerEmail string `json:"customer_email"`
	Email         string `json:"email"`
	PlanTier      string `json:"plan_tier"`
}

type paddleAPIResponse struct {
	Data struct {
		ID          string `json:"id"`
		URL         string `json:"url"`
		CheckoutURL string `json:"checkout_url"`
		Checkout    struct {
			URL string `json:"url"`
		} `json:"checkout"`
	} `json:"data"`
	CheckoutURL string `json:"checkout_url"`
	URL         string `json:"url"`
	Checkout    struct {
		URL string `json:"url"`
	} `json:"checkout"`
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
	Items          []struct {
		Price struct {
			ID        string `json:"id"`
			ProductID string `json:"product_id"`
		} `json:"price"`
	} `json:"items"`
}

type billingPeriod struct {
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

func (h *Handler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "PAYMENT_REQUIRED", "Authentication required")
		return
	}
	var req checkoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PACKAGE", "Invalid checkout request")
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	planTier := normalizePlanTier(firstNonEmpty(req.Package, req.PlanTier, req.ProductID))
	priceID := paddleConfiguredPriceID(planTier)
	apiKeyConfigured := strings.TrimSpace(os.Getenv("PADDLE_API_KEY")) != ""
	appURLConfigured := publicAppURL() != ""
	log.Printf("paddle checkout request: package=%q environment=%q price_id_found=%t", planTier, paddleEnvironment(), priceID != "")
	if email == "" || planTier == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PACKAGE", "Valid Starter, Professional, or Enterprise package required")
		return
	}
	if priceID == "" || !apiKeyConfigured || !appURLConfigured {
		writeAPIError(w, http.StatusServiceUnavailable, "INTERNAL_ERROR", "Paddle payment configuration is unavailable")
		return
	}
	if err := ensurePaymentSchema(r.Context(), h.DB); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Payment schema unavailable")
		return
	}
	checkoutURL, err := h.CreateCheckoutSession(r.Context(), priceID, email, claims.Sub, planTier)
	if err != nil {
		if errors.Is(err, errPaddleCheckoutURLMissing) {
			writeAPIError(w, http.StatusBadGateway, "INTERNAL_ERROR", "Paddle checkout URL was not returned")
			return
		}
		writeAPIError(w, http.StatusBadGateway, "INTERNAL_ERROR", "Paddle checkout failed")
		return
	}
	writeAPIData(w, http.StatusOK, map[string]any{"checkout_url": checkoutURL})
}

func (h *Handler) CreateCheckoutSession(ctx context.Context, priceID, customerEmail, authSubject, planTier string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("PADDLE_API_KEY"))
	if apiKey == "" {
		return "", errors.New("PADDLE_API_KEY is missing")
	}
	planTier = normalizePlanTier(planTier)
	if planTier == "" {
		return "", errors.New("invalid plan tier")
	}
	appURL := publicAppURL()
	if appURL == "" {
		return "", errors.New("PUBLIC_APP_URL is missing")
	}
	successURL := appURL + "/dashboard"
	cancelURL := appURL + "/pricing"
	customerID, err := h.createPaddleCustomer(ctx, customerEmail, planTier)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"collection_mode": "automatic",
		"customer_id":     customerID,
		"custom_data": map[string]any{
			"auth_subject":        strings.TrimSpace(authSubject),
			"customer_email":      strings.ToLower(strings.TrimSpace(customerEmail)),
			"package":             planTier,
			"package_id":          planTier,
			"package_name":        packageName(planTier),
			"paddle_product_name": paddleProductName(planTier),
			"plan_tier":           planTier,
			"provider":            "paddle",
			"source":              "koschei_web3_hub",
			"success_url":         successURL,
			"cancel_url":          cancelURL,
			"user_email":          strings.ToLower(strings.TrimSpace(customerEmail)),
			"user_id":             strings.TrimSpace(authSubject),
		},
		"items": []map[string]any{paddleTransactionItem(priceID, planTier)},
	}
	var out paddleAPIResponse
	if err := paddleRequest(ctx, http.MethodPost, "/transactions", payload, &out); err != nil {
		return "", err
	}
	checkoutURL := paddleCheckoutURL(out)
	checkoutURLFound := checkoutURL != ""
	log.Printf("paddle checkout transaction response: package=%q environment=%q price_id_found=%t checkout_url_found=%t", planTier, paddleEnvironment(), priceID != "", checkoutURLFound)
	if !checkoutURLFound {
		return "", errPaddleCheckoutURLMissing
	}
	if h.DB != nil {
		raw, _ := json.Marshal(out)
		_, _ = h.DB.ExecContext(ctx, `
			INSERT INTO orders (provider, provider_order_id, provider_payment_id, amount_try_cents, currency, status, raw_payload, created_at)
			VALUES ('paddle', NULLIF($1,''), NULLIF($1,''), $2, 'USD', 'pending', $3::jsonb, now())`, out.Data.ID, planMonthlyPriceCents(planTier), string(raw))
	}
	return checkoutURL, nil
}

func (h *Handler) PaddleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"paddle_api_key": strings.TrimSpace(os.Getenv("PADDLE_API_KEY")) != "", "environment": paddleEnvironment(), "starter_price_id": paddleConfiguredPriceID("starter") != "", "professional_price_id": paddleConfiguredPriceID("professional") != "", "enterprise_price_id": paddleConfiguredPriceID("enterprise") != "", "public_app_url": publicAppURL() != ""})
}

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.ValidateWebhookSignature(r) {
		writeAPIError(w, http.StatusUnauthorized, "WEBHOOK_INVALID", "Invalid webhook")
		return
	}
	var event paddleEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeAPIError(w, http.StatusBadRequest, "WEBHOOK_INVALID", "Invalid webhook")
		return
	}
	eventType := event.EventType
	if eventType == "" {
		eventType = event.Type
	}
	var err error
	switch eventType {
	case "subscription.created", "subscription.activated", "subscription.updated", "subscription.resumed":
		err = h.upsertPaddleSubscription(r.Context(), event.Data, false)
	case "subscription.canceled", "subscription.cancelled", "subscription.paused":
		err = h.upsertPaddleSubscription(r.Context(), event.Data, true)
	case "payment.succeeded", "transaction.completed", "transaction.paid":
		err = h.markPaddlePaymentSucceeded(r.Context(), event.Data)
	case "transaction.updated":
		err = h.recordPaddleTransaction(r.Context(), event.Data, false)
	case "payment.failed", "transaction.payment_failed", "transaction.past_due":
		err = h.markPaddlePaymentFailed(r.Context(), event.Data)
	default:
		writeAPIError(w, http.StatusOK, "WEBHOOK_UNSUPPORTED", "Webhook event ignored")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Webhook processing failed")
		return
	}
	writeAPIData(w, http.StatusOK, map[string]any{"received": true})
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
		monthStart := time.Now().UTC().Format("2006-01-") + "01"
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
	payload := map[string]any{"email": strings.ToLower(strings.TrimSpace(email)), "custom_data": map[string]string{"package_id": planTier, "package_name": packageName(planTier), "paddle_product_name": paddleProductName(planTier), "plan_tier": planTier, "provider": "paddle", "source": "koschei_web3_hub"}}
	var out struct{ Data struct{ ID string `json:"id"` } `json:"data"` }
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
	req, err := http.NewRequestWithContext(ctx, method, paddleAPIBaseURL()+path, bytes.NewReader(body))
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

func (h *Handler) upsertPaddleSubscription(ctx context.Context, raw json.RawMessage, inactive bool) error {
	if err := ensurePaymentSchema(ctx, h.DB); err != nil {
		return err
	}
	var sub paddleSubscriptionData
	if err := json.Unmarshal(raw, &sub); err != nil {
		return err
	}
	if sub.ID == "" || sub.CustomerID == "" {
		return errors.New("missing subscription or customer id")
	}
	productID, planTier := subscriptionProductAndTier(sub)
	email := strings.ToLower(strings.TrimSpace(firstNonEmpty(customDataString(sub.CustomData, "customer_email"), customDataString(sub.CustomData, "user_email"))))
	status := strings.ToLower(strings.TrimSpace(sub.Status))
	if inactive {
		if status == "paused" {
			status = "paused"
		} else {
			status = "canceled"
		}
	}
	if status == "" {
		status = "active"
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
	customerEmail := email
	if customerEmail == "" {
		customerEmail = sub.CustomerID + "@paddle.local"
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO paddle_customers (paddle_customer_id,email) VALUES ($1,lower($2)) ON CONFLICT (paddle_customer_id) DO UPDATE SET email=EXCLUDED.email RETURNING id`, sub.CustomerID, customerEmail).Scan(&customerPK); err != nil {
		return err
	}
	var subscriptionPK int
	if err := tx.QueryRowContext(ctx, `INSERT INTO paddle_subscriptions (paddle_subscription_id,customer_id,product_id,plan_tier,status,current_period_end,cancel_at_period_end,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,now()) ON CONFLICT (paddle_subscription_id) DO UPDATE SET customer_id=EXCLUDED.customer_id,product_id=EXCLUDED.product_id,plan_tier=EXCLUDED.plan_tier,status=EXCLUDED.status,current_period_end=EXCLUDED.current_period_end,cancel_at_period_end=EXCLUDED.cancel_at_period_end,updated_at=now() RETURNING id`, sub.ID, customerPK, firstNonEmpty(productID, paddleConfiguredProductID(planTier), planTier), planTier, status, periodEnd, sub.ScheduledChange != nil && sub.ScheduledChange.Action == "cancel").Scan(&subscriptionPK); err != nil {
		return err
	}
	if (status == "active" || status == "trialing") && email != "" && planTier != "" {
		if _, err := activatePackageEntitlementDetailedTx(ctx, tx, email, planTier, "paddle", sub.ID, "", "", productID, periodEnd); err != nil {
			return err
		}
		if err := h.ensureSubscriptionAPIKey(ctx, tx, subscriptionPK, sub.ID, email, planTier); err != nil {
			return err
		}
	} else if email != "" {
		_, _ = tx.ExecContext(ctx, `UPDATE entitlements SET status=$2, expires_at=COALESCE(expires_at, now()), updated_at=now() WHERE lower(email)=lower($1) AND payment_provider='paddle' AND (external_payment_id=$3 OR status='active')`, email, status, sub.ID)
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

func (h *Handler) recordPaddleTransaction(ctx context.Context, raw json.RawMessage, grantEntitlement bool) error {
	if err := ensurePaymentSchema(ctx, h.DB); err != nil {
		return err
	}
	var txData paddleTransactionData
	if err := json.Unmarshal(raw, &txData); err != nil {
		return err
	}
	packageID := normalizePlanTier(firstNonEmpty(customDataString(txData.CustomData, "package"), customDataString(txData.CustomData, "package_id"), customDataString(txData.CustomData, "plan_tier")))
	email := strings.ToLower(strings.TrimSpace(firstNonEmpty(customDataString(txData.CustomData, "customer_email"), customDataString(txData.CustomData, "user_email"))))
	productID, priceID := transactionProductAndPrice(txData)
	if packageID == "" {
		packageID = tierFromPriceID(priceID)
	}
	if (email == "" || packageID == "") && txData.SubscriptionID != "" {
		var storedEmail, storedPlan, storedProduct string
		_ = h.DB.QueryRowContext(ctx, `SELECT lower(c.email), COALESCE(s.plan_tier, ''), COALESCE(s.product_id, '') FROM paddle_subscriptions s JOIN paddle_customers c ON c.id = s.customer_id WHERE s.paddle_subscription_id = $1`, txData.SubscriptionID).Scan(&storedEmail, &storedPlan, &storedProduct)
		if email == "" && !strings.HasSuffix(storedEmail, "@paddle.local") {
			email = storedEmail
		}
		if packageID == "" {
			packageID = normalizePlanTier(storedPlan)
		}
		if productID == "" {
			productID = storedProduct
		}
	}
	status := strings.ToLower(strings.TrimSpace(txData.Status))
	if status == "" {
		status = "updated"
	}
	amount, currency := paddleTransactionAmount(txData)
	purchasedAt := parsePaddleTime(firstNonEmpty(txData.BilledAt, txData.CreatedAt))
	rawPayload := string(raw)
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orderID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO orders (provider, provider_order_id, provider_payment_id, amount_try_cents, currency, status, raw_payload, purchased_at, created_at)
		VALUES ('paddle', NULLIF($1,''), NULLIF($2,''), $3, $4, $5, $6::jsonb, $7, now())
		RETURNING id::text`, txData.ID, txData.ID, int(amount*100), currency, status, rawPayload, purchasedAt).Scan(&orderID)
	if err != nil {
		return err
	}
	if grantEntitlement && email != "" && packageID != "" && txData.ID != "" {
		if _, err := activatePackageEntitlementDetailedTx(ctx, tx, email, packageID, "paddle", txData.ID, "", orderID, productID, nil); err != nil {
			return err
		}
	}
	if txData.SubscriptionID != "" && (status == "completed" || status == "paid") {
		_, _ = tx.ExecContext(ctx, `UPDATE paddle_subscriptions SET status='active', current_period_end=COALESCE(current_period_end, now() + interval '1 month'), updated_at=now() WHERE paddle_subscription_id=$1`, txData.SubscriptionID)
	}
	return tx.Commit()
}

func (h *Handler) markPaddlePaymentSucceeded(ctx context.Context, raw json.RawMessage) error {
	var txData paddleTransactionData
	if err := json.Unmarshal(raw, &txData); err != nil {
		return err
	}
	if txData.Status != "" && txData.Status != "completed" && txData.Status != "paid" {
		return h.recordPaddleTransaction(ctx, raw, false)
	}
	return h.recordPaddleTransaction(ctx, raw, true)
}

func (h *Handler) markPaddlePaymentFailed(ctx context.Context, raw json.RawMessage) error {
	if err := h.recordPaddleTransaction(ctx, raw, false); err != nil {
		return err
	}
	var txData paddleTransactionData
	if err := json.Unmarshal(raw, &txData); err != nil {
		return err
	}
	if txData.SubscriptionID == "" {
		return nil
	}
	_, err := h.DB.ExecContext(ctx, `UPDATE paddle_subscriptions SET status='past_due', updated_at=now() WHERE paddle_subscription_id=$1`, txData.SubscriptionID)
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
	env := strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("PADDLE_ENV"), os.Getenv("PADDLE_ENVIRONMENT"))))
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

func paddleCheckoutURL(out paddleAPIResponse) string {
	for _, value := range []string{out.Data.Checkout.URL, out.Data.CheckoutURL, out.Data.URL, out.Checkout.URL, out.CheckoutURL, out.URL} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func publicAppURL() string { return strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_APP_URL")), "/") }

func paddleProductName(planTier string) string {
	switch normalizePlanTier(planTier) {
	case "starter":
		return "Koschei Starter"
	case "professional":
		return "Koschei Professional"
	case "enterprise":
		return "Koschei Enterprise"
	default:
		return ""
	}
}

func paddleConfiguredPriceID(planTier string) string {
	switch normalizePlanTier(planTier) {
	case "starter":
		return strings.TrimSpace(os.Getenv("PADDLE_STARTER_PRICE_ID"))
	case "professional":
		return firstNonEmpty(strings.TrimSpace(os.Getenv("PADDLE_PROFESSIONAL_PRICE_ID")), strings.TrimSpace(os.Getenv("PADDLE_BUILDER_PRICE_ID")))
	case "enterprise":
		return firstNonEmpty(strings.TrimSpace(os.Getenv("PADDLE_ENTERPRISE_PRICE_ID")), strings.TrimSpace(os.Getenv("PADDLE_STUDIO_PRICE_ID")))
	default:
		return ""
	}
}

func paddleConfiguredProductID(planTier string) string {
	switch normalizePlanTier(planTier) {
	case "starter":
		return strings.TrimSpace(os.Getenv("PADDLE_STARTER_PRODUCT_ID"))
	case "professional":
		return firstNonEmpty(strings.TrimSpace(os.Getenv("PADDLE_PROFESSIONAL_PRODUCT_ID")), strings.TrimSpace(os.Getenv("PADDLE_BUILDER_PRODUCT_ID")))
	case "enterprise":
		return firstNonEmpty(strings.TrimSpace(os.Getenv("PADDLE_ENTERPRISE_PRODUCT_ID")), strings.TrimSpace(os.Getenv("PADDLE_STUDIO_PRODUCT_ID")))
	default:
		return ""
	}
}

func paddleTransactionItem(priceID, planTier string) map[string]any { return map[string]any{"price_id": priceID, "quantity": 1} }
func normalizePlanTier(planTier string) string { return normalizePackageID(planTier) }

func subscriptionProductAndTier(sub paddleSubscriptionData) (string, string) {
	var productID, priceID string
	for _, item := range sub.Items {
		if priceID == "" { priceID = strings.TrimSpace(item.Price.ID) }
		if productID == "" { productID = strings.TrimSpace(item.Price.ProductID) }
		if priceID != "" && productID != "" { break }
	}
	planTier := normalizePlanTier(customDataString(sub.CustomData, "plan_tier"))
	if planTier == "" { planTier = tierFromPriceID(priceID) }
	return firstNonEmpty(productID, priceID), planTier
}

func transactionProductAndPrice(txData paddleTransactionData) (string, string) {
	var productID, priceID string
	for _, item := range txData.Items {
		if productID == "" { productID = strings.TrimSpace(item.Price.ProductID) }
		if priceID == "" { priceID = strings.TrimSpace(item.Price.ID) }
	}
	return productID, priceID
}

func paddleTransactionAmount(txData paddleTransactionData) (float64, string) {
	currency := strings.ToUpper(strings.TrimSpace(stringFromNested(txData.Details, "totals", "currency_code")))
	if currency == "" { currency = strings.ToUpper(strings.TrimSpace(stringFromNested(txData.Details, "currency_code"))) }
	if currency == "" { currency = "USD" }
	amountText := firstNonEmpty(stringFromNested(txData.Details, "totals", "total"), stringFromNested(txData.Details, "totals", "subtotal"), stringFromNested(txData.Details, "total"))
	amountText = strings.TrimSpace(amountText)
	if amountText == "" { return 0, currency }
	amount, err := strconv.ParseFloat(amountText, 64)
	if err != nil { return 0, currency }
	if amount >= 100 && !strings.Contains(amountText, ".") { amount = amount / 100 }
	return amount, currency
}

func stringFromNested(data map[string]any, path ...string) string {
	var current any = data
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok || m == nil { return "" }
		current = m[key]
	}
	switch v := current.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	default:
		return ""
	}
}

func tierFromPriceID(priceID string) string {
	priceID = strings.TrimSpace(priceID)
	for _, tier := range []string{"starter", "professional", "enterprise"} {
		if priceID != "" && priceID == paddleConfiguredPriceID(tier) { return tier }
	}
	return ""
}

func customDataString(data map[string]any, key string) string {
	if data == nil { return "" }
	if v, ok := data[key].(string); ok { return strings.TrimSpace(v) }
	return ""
}

func parsePaddleTime(raw string) any {
	if strings.TrimSpace(raw) == "" { return nil }
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil { return t }
	return nil
}

func planMonthlyPriceCents(planTier string) int {
	switch normalizePlanTier(planTier) {
	case "starter":
		return 29900
	case "professional":
		return 99900
	case "enterprise":
		return 499900
	default:
		return 29900
	}
}

func planMonthlyQuota(planTier string) int {
	switch normalizePlanTier(planTier) {
	case "starter":
		return 1000
	case "professional":
		return 10000
	case "enterprise":
		return 100000
	default:
		return 1000
	}
}

func planRateLimit(planTier string) int {
	switch normalizePlanTier(planTier) {
	case "starter":
		return 100
	case "professional":
		return 500
	case "enterprise":
		return 2000
	default:
		return 100
	}
}

func safeError(err error) string {
	if err == nil { return "" }
	msg := err.Error()
	if len(msg) > 240 { return msg[:240] }
	return msg
}

func maxInt(a, b int) int { if a > b { return a }; return b }
