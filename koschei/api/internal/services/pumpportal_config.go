package services

import (
	"net/url"
	"os"
	"strings"
)

type PumpPortalConfig struct {
	Enabled         bool
	APIKey          string
	WalletPublicKey string
	DataWS          string
}

func LoadPumpPortalConfigFromEnv() PumpPortalConfig {
	dataWS := strings.TrimSpace(os.Getenv("PUMPPORTAL_DATA_WS"))
	if dataWS == "" {
		dataWS = "wss://pumpportal.fun/api/data"
	}
	return PumpPortalConfig{
		Enabled:         isTruthyEnv(os.Getenv("PUMPPORTAL_ENABLED")),
		APIKey:          strings.TrimSpace(os.Getenv("PUMPPORTAL_API_KEY")),
		WalletPublicKey: strings.TrimSpace(os.Getenv("PUMPPORTAL_WALLET_PUBLIC_KEY")),
		DataWS:          dataWS,
	}
}

func (c PumpPortalConfig) websocketURL() string {
	raw := strings.TrimSpace(c.DataWS)
	if raw == "" {
		raw = "wss://pumpportal.fun/api/data"
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set("api-key", c.APIKey)
	u.RawQuery = q.Encode()
	return u.String()
}

func (c PumpPortalConfig) redactedWebsocketHost() string {
	u, err := url.Parse(strings.TrimSpace(c.DataWS))
	if err != nil || u.Host == "" {
		return "pumpportal"
	}
	return u.Scheme + "://" + u.Host + u.Path
}

func isTruthyEnv(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
