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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	payload := map[string]any{"url": "https://tradepigloball.co/integrations/crypto-brief/telegram", "secret_token": c.TelegramSecret, "allowed_updates": []string{"message"}, "drop_pending_updates": false}
	data, e := json.Marshal(payload)
	if e != nil {
		fail()
	}
	req, e := outboundhttp.NewRequest(ctx, "POST", "https://api.telegram.org/bot"+c.TelegramToken+"/setWebhook", bytes.NewReader(data))
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
	var result struct {
		OK bool `json:"ok"`
	}
	if json.Unmarshal(body, &result) != nil || !result.OK {
		fail()
	}
	fmt.Println("Crypto Brief Telegram webhook configured. Test a consenting customer connection before marking delivery live.")
}
func fail() { fmt.Fprintln(os.Stderr, "Telegram webhook setup could not be verified."); os.Exit(1) }
