package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"koschei/api/internal/outboundhttp"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"koschei/api/internal/outbound"
)

var graphVersionPattern = regexp.MustCompile(`^v[0-9]+(?:\.[0-9]+)?$`)

func whatsappCredentialsEnabled() bool {
	return strings.TrimSpace(os.Getenv("WHATSAPP_ACCESS_TOKEN")) != "" &&
		graphVersionPattern.MatchString(strings.TrimSpace(os.Getenv("WHATSAPP_GRAPH_VERSION")))
}

func WhatsAppOutboundEnabled() bool {
	return whatsappCredentialsEnabled() && strings.TrimSpace(os.Getenv("WHATSAPP_PHONE_NUMBER_ID")) != ""
}

func WhatsAppAccountOutboundEnabled(phoneID string) bool {
	return whatsappCredentialsEnabled() && strings.TrimSpace(phoneID) != ""
}

func SendWhatsAppText(ctx context.Context, to, text string) error {
	return SendWhatsAppTextFrom(ctx, strings.TrimSpace(os.Getenv("WHATSAPP_PHONE_NUMBER_ID")), to, text)
}

func SendWhatsAppTextFrom(ctx context.Context, phoneID, to, text string) error {
	phoneID = strings.TrimSpace(phoneID)
	if !WhatsAppAccountOutboundEnabled(phoneID) {
		return fmt.Errorf("whatsapp outbound not configured")
	}
	token := strings.TrimSpace(os.Getenv("WHATSAPP_ACCESS_TOKEN"))
	version := strings.TrimSpace(os.Getenv("WHATSAPP_GRAPH_VERSION"))
	payload, err := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                strings.TrimSpace(to),
		"type":              "text",
		"text":              map[string]string{"body": text},
	})
	if err != nil {
		return err
	}
	endpoint := "https://graph.facebook.com/" + url.PathEscape(version) + "/" + url.PathEscape(phoneID) + "/messages"
	validated, err := outbound.ValidateFixedHTTPSHost(endpoint, "graph.facebook.com")
	if err != nil {
		return fmt.Errorf("validate WhatsApp endpoint: %w", err)
	}
	// #nosec G704 -- authority is fixed to graph.facebook.com; path segments are escaped and redirects are host-pinned below.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validated.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := outbound.HardenFixedHostClient(&http.Client{Timeout: 10 * time.Second}, "graph.facebook.com")
	// #nosec G704 -- request authority and every redirect are restricted to graph.facebook.com.
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("whatsapp send status %d", resp.StatusCode)
	}
	return nil
}

func SendTelegramText(ctx context.Context, chatID, text string) error {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if token == "" {
		return fmt.Errorf("telegram outbound not configured")
	}
	payload, err := json.Marshal(map[string]any{"chat_id": strings.TrimSpace(chatID), "text": text})
	if err != nil {
		return err
	}
	endpoint := "https://api.telegram.org/bot" + token + "/sendMessage"
	validated, err := outbound.ValidateFixedHTTPSHost(endpoint, "api.telegram.org")
	if err != nil {
		return fmt.Errorf("validate Telegram endpoint: %w", err)
	}
	// #nosec G704 -- authority is fixed to api.telegram.org and redirects are host-pinned below.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validated.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := outbound.HardenFixedHostClient(&http.Client{Timeout: 10 * time.Second}, "api.telegram.org")
	// #nosec G704 -- request authority and every redirect are restricted to api.telegram.org.
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram send status %d", resp.StatusCode)
	}
	return nil
}
