package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Alchemy RPC client
func callAlchemy(method string, params []interface{}) (map[string]interface{}, error) {
	apiKey := os.Getenv("ALCHEMY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ALCHEMY_API_KEY not set")
	}

	url := fmt.Sprintf("https://solana-mainnet.g.alchemy.com/v2/%s", apiKey)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	body, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	return result, nil
}

// Get account info
func GetAccountInfo(address string) (map[string]interface{}, error) {
	return callAlchemy("getAccountInfo", []interface{}{
		address,
		map[string]string{"encoding": "jsonParsed"},
	})
}

// Get transaction
func GetTransaction(signature string) (map[string]interface{}, error) {
	return callAlchemy("getTransaction", []interface{}{
		signature,
		map[string]string{"encoding": "jsonParsed"},
	})
}