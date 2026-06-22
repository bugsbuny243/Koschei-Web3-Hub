package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestVerifyPaddleWebhookRaw(t *testing.T) {
	secret := "pdl_ntfset_test_secret"
	body := []byte(`{"event_id":"evt_1","event_type":"transaction.completed","data":{"id":"txn_1"}}`)
	now := time.Unix(1782110000, 0)
	ts := "1782110000"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts + ":"))
	_, _ = mac.Write(body)
	header := "ts=" + ts + ";h1=" + hex.EncodeToString(mac.Sum(nil))
	if !verifyPaddleWebhookRaw(secret, header, body, now) {
		t.Fatal("valid Paddle signature was rejected")
	}
	if verifyPaddleWebhookRaw(secret, header, append(body, ' '), now) {
		t.Fatal("mutated body was accepted")
	}
	if verifyPaddleWebhookRaw(secret, header, body, now.Add(10*time.Minute)) {
		t.Fatal("stale signature was accepted")
	}
}

func TestParsePaddleUnixTimestamp(t *testing.T) {
	got, err := parsePaddleUnixTimestamp("1782110000")
	if err != nil || got != 1782110000 {
		t.Fatalf("unexpected timestamp: %d %v", got, err)
	}
}
