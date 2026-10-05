package cryptobrief

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// All credentials, identities and responses in this file are explicit fixtures.
// The custom transport never contacts Telegram or any external server.
func telegramSetupFixture() Config {
	return Config{Enabled: true, TelegramToken: "12345:fixture_setup_only", TelegramSecret: strings.Repeat("f", 32), TelegramUsername: "FixtureBot"}
}

func TestTelegramWebhookSetupProtocol(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	c := telegramSetupFixture()
	var methods []string
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		method := strings.TrimPrefix(r.URL.Path, "/bot"+c.TelegramToken+"/")
		methods = append(methods, method)
		if r.URL.Scheme != "https" || r.URL.Host != "api.telegram.org" || r.Method != "POST" {
			t.Fatal("setup request left the dedicated HTTPS provider")
		}
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 11*time.Second {
			t.Fatal("setup request is not bounded")
		}
		result := ""
		switch method {
		case "getMe":
			result = `{"id":12345,"is_bot":true,"username":"fixturebot"}`
		case "getWebhookInfo":
			if len(methods) == 2 {
				result = `{"url":""}`
			} else {
				result = `{"url":"` + TelegramWebhookURL + `","allowed_updates":["message"]}`
			}
		case "setWebhook":
			var payload struct {
				URL     string   `json:"url"`
				Secret  string   `json:"secret_token"`
				Allowed []string `json:"allowed_updates"`
				Drop    *bool    `json:"drop_pending_updates"`
			}
			if json.NewDecoder(r.Body).Decode(&payload) != nil || payload.URL != TelegramWebhookURL || payload.Secret != c.TelegramSecret || !reflect.DeepEqual(payload.Allowed, []string{"message"}) || payload.Drop == nil || *payload.Drop {
				t.Fatal("setup moved the webhook, lost authentication, or discarded updates")
			}
			result = "true"
		default:
			t.Fatal("unexpected setup method")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":` + result + `}`))}, nil
	})}
	if err := ConfigureTelegramWebhook(context.Background(), c, client); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(methods, []string{"getMe", "getWebhookInfo", "setWebhook", "getWebhookInfo"}) {
		t.Fatal("setup did not verify identity and read back registration")
	}
}

func TestTelegramWebhookSetupFailsClosed(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	for _, fixture := range []struct {
		name, identity, before, accepted, after string
		expected                                error
		calls                                   int
	}{
		{"different_username", `{"id":12345,"is_bot":true,"username":"OtherFixtureBot"}`, "", "", "", errTelegramIdentity, 1},
		{"not_a_bot", `{"id":12345,"is_bot":false,"username":"FixtureBot"}`, "", "", "", errTelegramIdentity, 1},
		{"missing_identity", `{"is_bot":true,"username":"FixtureBot"}`, "", "", "", errTelegramIdentity, 1},
		{"another_module", "", `{"url":"https://tradepigloball.co/webhooks/telegram"}`, "", "", errTelegramConflict, 2},
		{"registration_rejected", "", `{"url":""}`, "false", "", errTelegramUnverified, 3},
		{"foreign_readback", "", `{"url":""}`, "true", `{"url":"https://fixture.invalid/foreign","allowed_updates":["message"]}`, errTelegramUnverified, 4},
		{"missing_readback_fields", "", `{"url":"` + TelegramWebhookURL + `","allowed_updates":["message"]}`, "true", `{}`, errTelegramUnverified, 4},
		{"expanded_updates", "", `{"url":""}`, "true", `{"url":"` + TelegramWebhookURL + `","allowed_updates":["message","callback_query"]}`, errTelegramUnverified, 4},
		{"null_readback", "", `{"url":""}`, "true", `null`, errTelegramSetup, 4},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				var result string
				switch calls {
				case 1:
					result = fixture.identity
					if result == "" {
						result = `{"id":12345,"is_bot":true,"username":"FixtureBot"}`
					}
				case 2:
					result = fixture.before
				case 3:
					result = fixture.accepted
				case 4:
					result = fixture.after
				default:
					t.Fatal("unexpected retry or mutation")
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":` + result + `}`))}, nil
			})}
			if err := ConfigureTelegramWebhook(context.Background(), telegramSetupFixture(), client); !errors.Is(err, fixture.expected) || calls != fixture.calls {
				t.Fatalf("unsafe configuration accepted or changed: error=%v calls=%d", err, calls)
			}
		})
	}
}

func TestTelegramWebhookSetupDoesNotExposeProviderSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	c := telegramSetupFixture()
	for _, fixture := range []struct {
		status         int
		body           string
		transportError error
	}{
		{500, c.TelegramToken + c.TelegramSecret, nil},
		{200, strings.Repeat("x", 65537), nil},
		{200, `{"ok":false,"description":"` + c.TelegramToken + `"}`, nil},
		{0, "", errors.New("fixture HTTP error containing " + c.TelegramToken)},
	} {
		client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			if fixture.transportError != nil {
				return nil, fixture.transportError
			}
			return &http.Response{StatusCode: fixture.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fixture.body))}, nil
		})}
		if err := ConfigureTelegramWebhook(context.Background(), c, client); !errors.Is(err, errTelegramSetup) || strings.Contains(err.Error(), c.TelegramToken) || strings.Contains(err.Error(), c.TelegramSecret) {
			t.Fatal("provider error was accepted or leaked to operator telemetry")
		}
	}
	for _, secret := range []string{strings.Repeat("f", 31), strings.Repeat("f", 257), strings.Repeat("f", 32) + "+", strings.Repeat("f", 32) + "/"} {
		c.TelegramSecret = secret
		if c.Ready("telegram") {
			t.Fatal("provider-invalid webhook authentication was accepted")
		}
	}
}
