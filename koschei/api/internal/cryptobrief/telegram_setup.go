package cryptobrief

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/outboundhttp"
	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/workerwake"
)

const TelegramWebhookURL = "https://tradepigloball.co/integrations/arvis/telegram"
const legacyTelegramWebhookURL = "https://tradepigloball.co/integrations/crypto-brief/telegram"
const telegramWebhookHealthID = "arvis-telegram-webhook"

var (
	errTelegramSetup      = errors.New("telegram_webhook_setup_failed")
	errTelegramIdentity   = errors.New("telegram_bot_identity_mismatch")
	errTelegramConflict   = errors.New("telegram_webhook_owned_by_another_module")
	errTelegramUnverified = errors.New("telegram_webhook_not_verified")
)

// ConfigureTelegramWebhook binds only the configured dedicated ARVIS customer
// bot. Provider bodies and HTTP errors can contain credentials and must never be
// returned. Verification proves configuration, not receipt of a customer message.
func ConfigureTelegramWebhook(ctx context.Context, c Config, client *http.Client) error {
	if !c.Ready("telegram") {
		return ErrChannelUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var identity struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"is_bot"`
	}
	if err := telegramSetupCall(ctx, c, client, "getMe", map[string]any{}, &identity); err != nil {
		return err
	}
	if !identity.Bot || identity.ID <= 0 || !strings.EqualFold(identity.Username, c.TelegramUsername) {
		return errTelegramIdentity
	}
	var webhook struct {
		URL            string   `json:"url"`
		AllowedUpdates []string `json:"allowed_updates"`
	}
	if err := telegramSetupCall(ctx, c, client, "getWebhookInfo", map[string]any{}, &webhook); err != nil {
		return err
	}
	if webhook.URL != "" && webhook.URL != TelegramWebhookURL && webhook.URL != legacyTelegramWebhookURL {
		return errTelegramConflict
	}
	var accepted bool
	if err := telegramSetupCall(ctx, c, client, "setWebhook", map[string]any{
		"url": TelegramWebhookURL, "secret_token": c.TelegramSecret,
		"allowed_updates": []string{"message"}, "drop_pending_updates": false,
	}, &accepted); err != nil {
		return err
	}
	if !accepted {
		return errTelegramUnverified
	}
	webhook.URL = ""
	webhook.AllowedUpdates = nil
	if err := telegramSetupCall(ctx, c, client, "getWebhookInfo", map[string]any{}, &webhook); err != nil {
		return err
	}
	if webhook.URL != TelegramWebhookURL || len(webhook.AllowedUpdates) != 1 || webhook.AllowedUpdates[0] != "message" {
		return errTelegramUnverified
	}
	return nil
}

func telegramSetupCall(ctx context.Context, c Config, client *http.Client, method string, payload, result any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, err := json.Marshal(payload)
	if err != nil {
		return errTelegramSetup
	}
	req, err := outboundhttp.NewRequest(ctx, "POST", "https://api.telegram.org/bot"+c.TelegramToken+"/"+method, bytes.NewReader(data))
	if err != nil {
		return errTelegramSetup
	}
	req.Header.Set("Content-Type", "application/json")
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	response, err := outboundhttp.Do(client, req)
	if err != nil {
		return errTelegramSetup
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(body) > 65536 || response.StatusCode != http.StatusOK {
		return errTelegramSetup
	}
	var envelope struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(body, &envelope) != nil || !envelope.OK || len(envelope.Result) == 0 || string(envelope.Result) == "null" || json.Unmarshal(envelope.Result, result) != nil {
		return errTelegramSetup
	}
	return nil
}

func (s *Service) telegramDeliveryReady() bool {
	return s.Config.Ready("telegram") && (!s.telegramVerificationRequired || s.telegramVerified.Load())
}

func startTelegramWebhook(ctx context.Context, s *Service, health *runtimehealth.Registry) {
	configured := s.Config.Ready("telegram")
	health.RegisterPeriodic(telegramWebhookHealthID, "worker", "", configured, 2*time.Hour)
	if !configured {
		return
	}
	go func() {
		defer s.telegramVerified.Store(false)
		for ctx.Err() == nil {
			err := ConfigureTelegramWebhook(ctx, s.Config, s.Client)
			s.telegramVerified.Store(err == nil)
			workerwake.Signal(wakeID)
			wait := time.Hour
			if err != nil {
				health.Failure(telegramWebhookHealthID, err)
				log.Print("ARVIS Telegram webhook setup failed")
				wait = 15 * time.Minute
			} else {
				health.Success(telegramWebhookHealthID, 1)
				log.Print("ARVIS Telegram webhook configuration verified; customer result delivery still requires a real test")
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}
