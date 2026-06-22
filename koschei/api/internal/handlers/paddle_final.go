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
	"strings"
	"time"

	"koschei/api/internal/services"
)

const paddleFinalRequestTimeout = 20 * time.Second

type paddleWebhookEnvelope struct {
	EventID      string          `json:"event_id"`
	EventType    string          `json:"event_type"`
	Type         string          `json:"type"`
	Notification string          `json:"notification_id"`
	OccurredAt   string          `json:"occurred_at"`
	Data         json.RawMessage `json:"data"`
}

func (h *Handler) CreateCheckoutFinal(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "PAYMENT_REQUIRED", "Authentication required")
		return
	}
	cfg := services.LoadPaddleConfigFromEnv()
	var req checkoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PACKAGE", "Invalid checkout request")
		return
	}
	planTier := normalizePlanTier(firstNonEmpty(req.Package, req.PlanTier, req.ProductID))
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" || planTier == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PACKAGE", "Valid Starter, Professional, or Enterprise package required")
		return
	}
	if !cfg.AutomationReady || !cfg.PlanReady(planTier) {
		writePaymentAudit(r, h, "payment_config_missing", "warning", map[string]any{"plan_tier": planTier, "paddle": cfg.PublicStatus()})
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "paddle_not_ready", "paddle": cfg.PublicStatus()})
		return
	}
	if err := ensurePaymentSchema(r.Context(), h.DB); err != nil {
		writePaymentAudit(r, h, "payment_schema_unavailable", "error", map[string]any{"provider": "paddle"})
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Payment schema unavailable")
		return
	}
	checkoutURL, transactionID, err := h.createPaddleCheckoutFinal(r.Context(), cfg, email, claims.Sub, planTier)
	if err != nil {
		writePaymentAudit(r, h, "paddle_checkout_failed", "error", map[string]any{"plan_tier": planTier, "environment": cfg.Environment, "error": err.Error()})
		if errors.Is(err, errPaddleCheckoutURLMissing) {
			writeAPIError(w, http.StatusBadGateway, "PADDLE_CHECKOUT_URL_MISSING", "Paddle checkout URL was not returned")
			return
		}
		writeAPIError(w, http.StatusBadGateway, "PADDLE_CHECKOUT_FAILED", "Paddle checkout failed")
		return
	}
	writePaymentAudit(r, h, "paddle_checkout_created", "info", map[string]any{"plan_tier": planTier, "environment": cfg.Environment, "transaction_id": transactionID})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "checkout_url": checkoutURL, "transaction_id": transactionID, "environment": cfg.Environment})
}

func (h *Handler) createPaddleCheckoutFinal(ctx context.Context, cfg services.PaddleConfig, customerEmail, authSubject, planTier string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, paddleFinalRequestTimeout)
	defer cancel()
	customerID, err := h.ensurePaddleCustomerFinal(ctx, cfg, customerEmail, planTier)
	if err != nil {
		return "", "", err
	}
	priceID := cfg.PriceID(planTier)
	payload := map[string]any{
		"collection_mode": "automatic",
		"customer_id":     customerID,
		"checkout":        map[string]any{"url": cfg.CheckoutURL},
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
			"user_email":          strings.ToLower(strings.TrimSpace(customerEmail)),
			"user_id":             strings.TrimSpace(authSubject),
		},
		"items": []map[string]any{paddleTransactionItem(priceID, planTier)},
	}
	var out paddleAPIResponse
	if err := paddleFinalRequest(ctx, cfg, http.MethodPost, "/transactions", payload, &out); err != nil {
		return "", "", err
	}
	checkoutURL := paddleCheckoutURL(out)
	transactionID := strings.TrimSpace(out.Data.ID)
	if transactionID == "" {
		return "", "", errors.New("Paddle response did not include transaction id")
	}
	if checkoutURL == "" {
		return "", transactionID, errPaddleCheckoutURLMissing
	}
	if err := h.upsertPaddlePendingOrder(ctx, transactionID, planTier, out); err != nil {
		return "", transactionID, err
	}
	return checkoutURL, transactionID, nil
}

func (h *Handler) ensurePaddleCustomerFinal(ctx context.Context, cfg services.PaddleConfig, email, planTier string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", errors.New("customer email is missing")
	}
	var existing string
	if h.DB != nil {
		err := h.DB.QueryRowContext(ctx, `SELECT paddle_customer_id FROM paddle_customers WHERE lower(email)=lower($1) ORDER BY created_at DESC LIMIT 1`, email).Scan(&existing)
		if err == nil && strings.TrimSpace(existing) != "" {
			return existing, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	payload := map[string]any{"email": email, "custom_data": map[string]string{"package_id": planTier, "plan_tier": planTier, "provider": "paddle", "source": "koschei_web3_hub"}}
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := paddleFinalRequest(ctx, cfg, http.MethodPost, "/customers", payload, &out); err != nil {
		return "", err
	}
	customerID := strings.TrimSpace(out.Data.ID)
	if customerID == "" {
		return "", errors.New("Paddle response did not include customer id")
	}
	if h.DB != nil {
		_, err := h.DB.ExecContext(ctx, `INSERT INTO paddle_customers (paddle_customer_id,email) VALUES ($1,lower($2)) ON CONFLICT (paddle_customer_id) DO UPDATE SET email=EXCLUDED.email`, customerID, email)
		if err != nil {
			return "", err
		}
	}
	return customerID, nil
}

func (h *Handler) upsertPaddlePendingOrder(ctx context.Context, transactionID, planTier string, response paddleAPIResponse) error {
	if h.DB == nil {
		return errors.New("database unavailable")
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = h.DB.ExecContext(ctx, `
		INSERT INTO orders (provider,provider_order_id,provider_payment_id,amount_try_cents,currency,status,raw_payload,created_at)
		VALUES ('paddle',$1,$1,$2,'USD','pending',$3::jsonb,now())
		ON CONFLICT (provider,provider_order_id) WHERE provider_order_id IS NOT NULL DO UPDATE SET
			provider_payment_id=EXCLUDED.provider_payment_id,
			status=CASE WHEN orders.status IN ('completed','paid') THEN orders.status ELSE EXCLUDED.status END,
			raw_payload=EXCLUDED.raw_payload`, transactionID, planMonthlyPriceCents(planTier), string(raw))
	return err
}

func (h *Handler) PaddleStatusFinal(w http.ResponseWriter, r *http.Request) {
	cfg := services.LoadPaddleConfigFromEnv()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "paddle": cfg.PublicStatus()})
}

func (h *Handler) PaddlePublicConfig(w http.ResponseWriter, r *http.Request) {
	cfg := services.LoadPaddleConfigFromEnv()
	if !cfg.CheckoutReady {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "paddle_checkout_not_ready", "paddle": cfg.PublicStatus()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"environment":  cfg.Environment,
		"client_token": cfg.ClientToken,
		"checkout_url": cfg.CheckoutURL,
		"success_url":  strings.TrimRight(cfg.PublicAppURL, "/") + "/dashboard?payment=paddle_success",
		"cancel_url":   strings.TrimRight(cfg.PublicAppURL, "/") + "/pricing?payment=paddle_cancelled",
	})
}

func (h *Handler) HandlePaddleWebhookFinal(w http.ResponseWriter, r *http.Request) {
	cfg := services.LoadPaddleConfigFromEnv()
	if !cfg.WebhookConfigured {
		writePaymentAudit(r, h, "payment_webhook_config_missing", "error", map[string]any{"provider": "paddle"})
		writeAPIError(w, http.StatusServiceUnavailable, "WEBHOOK_NOT_CONFIGURED", "Paddle webhook is not configured")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "WEBHOOK_INVALID", "Invalid webhook body")
		return
	}
	if !verifyPaddleWebhookRaw(cfg.WebhookSecret, r.Header.Get("Paddle-Signature"), raw, time.Now()) {
		writePaymentAudit(r, h, "payment_webhook_invalid", "warning", map[string]any{"provider": "paddle", "reason": "signature_invalid"})
		writeAPIError(w, http.StatusUnauthorized, "WEBHOOK_INVALID", "Invalid webhook")
		return
	}
	var event paddleWebhookEnvelope
	if err := json.Unmarshal(raw, &event); err != nil {
		writeAPIError(w, http.StatusBadRequest, "WEBHOOK_INVALID", "Invalid webhook JSON")
		return
	}
	eventType := strings.TrimSpace(firstNonEmpty(event.EventType, event.Type))
	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		digest := sha256.Sum256(raw)
		eventID = "sha256:" + hex.EncodeToString(digest[:])
	}
	claimed, err := h.claimPaddleWebhookEvent(r.Context(), eventID, eventType, event.OccurredAt, raw)
	if err != nil {
		writePaymentAudit(r, h, "payment_webhook_claim_failed", "error", map[string]any{"provider": "paddle", "event_type": eventType, "error": err.Error()})
		writeAPIError(w, http.StatusInternalServerError, "WEBHOOK_PROCESSING_FAILED", "Webhook processing failed")
		return
	}
	if !claimed {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "received": true, "duplicate": true, "event_id": eventID})
		return
	}
	writePaymentAudit(r, h, "payment_webhook_received", "info", map[string]any{"provider": "paddle", "event_type": eventType, "event_id": eventID})
	processErr := h.processPaddleWebhookFinal(r.Context(), eventType, event.Data)
	if processErr != nil {
		h.finishPaddleWebhookEvent(r.Context(), eventID, "failed", processErr.Error())
		writePaymentAudit(r, h, "payment_webhook_failed", "error", map[string]any{"provider": "paddle", "event_type": eventType, "event_id": eventID, "error": processErr.Error()})
		writeAPIError(w, http.StatusInternalServerError, "WEBHOOK_PROCESSING_FAILED", "Webhook processing failed")
		return
	}
	h.finishPaddleWebhookEvent(r.Context(), eventID, "processed", "")
	writePaymentAudit(r, h, "payment_webhook_processed", "info", map[string]any{"provider": "paddle", "event_type": eventType, "event_id": eventID})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "received": true, "event_id": eventID, "event_type": eventType})
}

func (h *Handler) processPaddleWebhookFinal(ctx context.Context, eventType string, data json.RawMessage) error {
	switch eventType {
	case "subscription.created", "subscription.activated", "subscription.updated", "subscription.resumed", "subscription.trialing", "subscription.past_due":
		return h.upsertPaddleSubscription(ctx, data, false)
	case "subscription.canceled", "subscription.cancelled", "subscription.paused":
		return h.upsertPaddleSubscription(ctx, data, true)
	case "transaction.completed", "transaction.paid", "payment.succeeded":
		return h.recordPaddleTransactionFinal(ctx, data, true)
	case "transaction.created", "transaction.ready", "transaction.updated", "transaction.billed", "transaction.canceled":
		return h.recordPaddleTransactionFinal(ctx, data, false)
	case "transaction.payment_failed", "transaction.past_due", "payment.failed":
		if err := h.recordPaddleTransactionFinal(ctx, data, false); err != nil {
			return err
		}
		var txData paddleTransactionData
		if err := json.Unmarshal(data, &txData); err != nil {
			return err
		}
		if txData.SubscriptionID != "" {
			_, err := h.DB.ExecContext(ctx, `UPDATE paddle_subscriptions SET status='past_due',updated_at=now() WHERE paddle_subscription_id=$1`, txData.SubscriptionID)
			return err
		}
		return nil
	default:
		return nil
	}
}

func (h *Handler) recordPaddleTransactionFinal(ctx context.Context, raw json.RawMessage, grantEntitlement bool) error {
	if err := ensurePaymentSchema(ctx, h.DB); err != nil {
		return err
	}
	var txData paddleTransactionData
	if err := json.Unmarshal(raw, &txData); err != nil {
		return err
	}
	txID := strings.TrimSpace(txData.ID)
	if txID == "" {
		return errors.New("Paddle transaction id is missing")
	}
	packageID := normalizePlanTier(firstNonEmpty(customDataString(txData.CustomData, "package"), customDataString(txData.CustomData, "package_id"), customDataString(txData.CustomData, "plan_tier")))
	email := strings.ToLower(strings.TrimSpace(firstNonEmpty(customDataString(txData.CustomData, "customer_email"), customDataString(txData.CustomData, "user_email"))))
	productID, priceID := transactionProductAndPrice(txData)
	if packageID == "" {
		packageID = tierFromPriceID(priceID)
	}
	if (email == "" || packageID == "" || productID == "") && txData.SubscriptionID != "" {
		var storedEmail, storedPlan, storedProduct string
		_ = h.DB.QueryRowContext(ctx, `SELECT lower(c.email),COALESCE(s.plan_tier,''),COALESCE(s.product_id,'') FROM paddle_subscriptions s JOIN paddle_customers c ON c.id=s.customer_id WHERE s.paddle_subscription_id=$1`, txData.SubscriptionID).Scan(&storedEmail, &storedPlan, &storedProduct)
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
	if email == "" && txData.CustomerID != "" {
		var storedEmail string
		_ = h.DB.QueryRowContext(ctx, `SELECT lower(email) FROM paddle_customers WHERE paddle_customer_id=$1`, txData.CustomerID).Scan(&storedEmail)
		if !strings.HasSuffix(storedEmail, "@paddle.local") {
			email = storedEmail
		}
	}
	if grantEntitlement && (email == "" || packageID == "") {
		return errors.New("completed Paddle transaction could not be mapped to customer and plan")
	}
	status := strings.ToLower(strings.TrimSpace(txData.Status))
	if status == "" {
		status = "updated"
	}
	amount, currency := paddleTransactionAmount(txData)
	purchasedAt := parsePaddleTime(firstNonEmpty(txData.BilledAt, txData.CreatedAt))
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "paddle-order:"+txID); err != nil {
		return err
	}
	var orderID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO orders(provider,provider_order_id,provider_payment_id,amount_try_cents,currency,status,raw_payload,purchased_at,created_at)
		VALUES('paddle',$1,$1,$2,$3,$4,$5::jsonb,$6,now())
		ON CONFLICT(provider,provider_order_id) WHERE provider_order_id IS NOT NULL DO UPDATE SET
			provider_payment_id=EXCLUDED.provider_payment_id,
			amount_try_cents=EXCLUDED.amount_try_cents,
			currency=EXCLUDED.currency,
			status=EXCLUDED.status,
			raw_payload=EXCLUDED.raw_payload,
			purchased_at=COALESCE(EXCLUDED.purchased_at,orders.purchased_at)
		RETURNING id::text`, txID, int(amount*100), currency, status, string(raw), purchasedAt).Scan(&orderID)
	if err != nil {
		return err
	}
	if grantEntitlement {
		if _, err := activatePackageEntitlementDetailedTx(ctx, tx, email, packageID, "paddle", txID, "", orderID, productID, nil); err != nil {
			return err
		}
	}
	if txData.SubscriptionID != "" && (status == "completed" || status == "paid") {
		_, _ = tx.ExecContext(ctx, `UPDATE paddle_subscriptions SET status='active',current_period_end=COALESCE(current_period_end,now()+interval '1 month'),updated_at=now() WHERE paddle_subscription_id=$1`, txData.SubscriptionID)
	}
	return tx.Commit()
}

func paddleFinalRequest(ctx context.Context, cfg services.PaddleConfig, method, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, cfg.APIBaseURL()+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Paddle-Version", "1")
	req.Header.Set("User-Agent", "Koschei-Web3/1.0")
	client := &http.Client{Timeout: paddleFinalRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	log.Printf("paddle api response: method=%s path=%s environment=%q status=%d", method, path, cfg.Environment, resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return paddleAPIError{StatusCode: resp.StatusCode, Message: paddleErrorMessage(resp.StatusCode, responseBody)}
	}
	if out != nil && len(responseBody) > 0 {
		return json.Unmarshal(responseBody, out)
	}
	return nil
}

func verifyPaddleWebhookRaw(secret, signatureHeader string, body []byte, now time.Time) bool {
	secret = strings.TrimSpace(secret)
	signatureHeader = strings.TrimSpace(signatureHeader)
	if secret == "" || signatureHeader == "" {
		return false
	}
	timestamp, signatures := parsePaddleSignature(signatureHeader)
	if timestamp == "" || len(signatures) == 0 {
		return false
	}
	ts, err := parsePaddleUnixTimestamp(timestamp)
	if err != nil {
		return false
	}
	skew := now.Sub(time.Unix(ts, 0))
	if skew > paddleWebhookMaxSkew || skew < -paddleWebhookMaxSkew {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + ":"))
	_, _ = mac.Write(body)
	expected := []byte(hex.EncodeToString(mac.Sum(nil)))
	for _, signature := range signatures {
		if subtle.ConstantTimeCompare(expected, []byte(strings.ToLower(strings.TrimSpace(signature)))) == 1 {
			return true
		}
	}
	return false
}

func parsePaddleUnixTimestamp(value string) (int64, error) {
	var ts int64
	_, err := fmt.Sscan(strings.TrimSpace(value), &ts)
	return ts, err
}

func (h *Handler) claimPaddleWebhookEvent(ctx context.Context, eventID, eventType, occurredAt string, payload []byte) (bool, error) {
	var claimed string
	err := h.DB.QueryRowContext(ctx, `
		INSERT INTO paddle_webhook_events(event_id,event_type,occurred_at,status,attempts,last_error,payload,created_at,updated_at)
		VALUES($1,$2,$3::timestamptz,'processing',1,'',$4::jsonb,now(),now())
		ON CONFLICT(event_id) DO UPDATE SET
			event_type=EXCLUDED.event_type,
			occurred_at=COALESCE(EXCLUDED.occurred_at,paddle_webhook_events.occurred_at),
			status='processing',attempts=paddle_webhook_events.attempts+1,last_error='',payload=EXCLUDED.payload,updated_at=now()
		WHERE paddle_webhook_events.status<>'processed' AND paddle_webhook_events.updated_at<now()-interval '5 seconds'
		RETURNING event_id`, eventID, eventType, nullablePaddleTime(occurredAt), string(payload)).Scan(&claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
func nullablePaddleTime(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t
	}
	return nil
}
func (h *Handler) finishPaddleWebhookEvent(ctx context.Context, eventID, status, lastError string) {
	processedAt := any(nil)
	if status == "processed" {
		processedAt = time.Now().UTC()
	}
	_, _ = h.DB.ExecContext(ctx, `UPDATE paddle_webhook_events SET status=$2,last_error=$3,processed_at=$4,updated_at=now() WHERE event_id=$1`, eventID, status, compactPaymentError(lastError), processedAt)
}
func compactPaymentError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 800 {
		return value[:800]
	}
	return value
}
