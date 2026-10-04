// Configure a dedicated Crypto Brief bot from deployment-managed environment secrets.
// Run only in that trusted runtime; this command never prints credentials or provider bodies.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/cryptobrief"
	"koschei/api/internal/outboundhttp"
)

func main() {
	c := cryptobrief.ConfigFromEnv()
	if !c.Ready("telegram") {
		fmt.Fprintln(os.Stderr, "Dedicated Crypto Brief Telegram configuration is incomplete.")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	webhookURL := "https://tradepigloball.co/integrations/crypto-brief/telegram"
	var identity struct {
		Username string `json:"username"`
		Bot      bool   `json:"is_bot"`
	}
	call(ctx, c, "getMe", map[string]any{}, &identity)
	if !identity.Bot || !strings.EqualFold(identity.Username, c.TelegramUsername) {
		fmt.Fprintln(os.Stderr, "Configured public username does not match the dedicated bot.")
		os.Exit(1)
	}
	var webhook struct {
		URL string `json:"url"`
	}
	call(ctx, c, "getWebhookInfo", map[string]any{}, &webhook)
	if webhook.URL != "" && webhook.URL != webhookURL {
		fmt.Fprintln(os.Stderr, "This bot already owns another webhook. Use a dedicated Crypto Brief bot.")
		os.Exit(1)
	}
	call(ctx, c, "setWebhook", map[string]any{"url": webhookURL, "secret_token": c.TelegramSecret, "allowed_updates": []string{"message"}, "drop_pending_updates": false}, nil)
	fmt.Println("Crypto Brief Telegram webhook configured. Test a consenting customer connection before marking delivery live.")
}
func call(ctx context.Context, c cryptobrief.Config, method string, payload any, result any) {
	data, e := json.Marshal(payload)
	if e != nil {
		fail()
	}
	req, e := outboundhttp.NewRequest(ctx, "POST", "https://api.telegram.org/bot"+c.TelegramToken+"/"+method, bytes.NewReader(data))
	if e != nil {
		fail()
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := outboundhttp.Do(&http.Client{Timeout: 12 * time.Second}, req)
	if e != nil {
		fail()
	}
	defer res.Body.Close()
	body, e := io.ReadAll(io.LimitReader(res.Body, 65537))
	if e != nil || len(body) > 65536 || res.StatusCode != 200 {
		fail()
	}
	var response struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(body, &response) != nil || !response.OK {
		fail()
	}
	if result != nil && json.Unmarshal(response.Result, result) != nil {
		fail()
	}
}
func fail() { fmt.Fprintln(os.Stderr, "Telegram webhook setup could not be verified."); os.Exit(1) }
