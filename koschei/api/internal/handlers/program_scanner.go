package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const upgradeableLoaderID = "BPFLoaderUpgradeab1e11111111111111111111111"

type programScanRequest struct {
	ProgramID string `json:"program_id"`
	Address   string `json:"address"`
	Network   string `json:"network"`
}

type programScanResponse struct {
	ProgramID        string   `json:"program_id"`
	Network          string   `json:"network"`
	Score            int      `json:"score"`
	RiskLevel        string   `json:"risk_level"`
	Executable       bool     `json:"executable"`
	Owner            string   `json:"owner"`
	Upgradeable      bool     `json:"upgradeable"`
	UpgradeAuthority string   `json:"upgrade_authority,omitempty"`
	ProgramData      string   `json:"program_data,omitempty"`
	BalanceSOL       string   `json:"balance_sol"`
	RecentTxCount    int      `json:"recent_tx_count"`
	Findings         []string `json:"findings"`
	Recommendations  []string `json:"recommendations"`
	Disclaimer       string   `json:"disclaimer"`
}

type solanaAccount struct {
	Lamports   int64         `json:"lamports"`
	Owner      string        `json:"owner"`
	Executable bool          `json:"executable"`
	Data       []interface{} `json:"data"`
}

func (h *Handler) ProgramScan(w http.ResponseWriter, r *http.Request) {
	var req programScanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	programID := strings.TrimSpace(req.ProgramID)
	if programID == "" {
		programID = strings.TrimSpace(req.Address)
	}
	if programID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "program_id required"})
		return
	}
	if req.Network == "" {
		req.Network = "solana-devnet"
	}

	client := &http.Client{Timeout: 12 * time.Second}
	rpcURL := solanaRPCURL(req.Network, os.Getenv("ALCHEMY_API_KEY"))
	account, err := fetchSolanaAccount(client, rpcURL, programID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "RPC request failed"})
		return
	}
	if account == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "program not found on this network"})
		return
	}

	findings := []string{}
	recommendations := []string{}
	score := 100
	if !account.Executable {
		score -= 70
		findings = append(findings, "The supplied address is not marked executable.")
		recommendations = append(recommendations, "Verify the program ID using the project's official documentation.")
	} else {
		findings = append(findings, "The account is an executable Solana program.")
	}

	upgradeable := account.Owner == upgradeableLoaderID
	programData := ""
	upgradeAuthority := ""
	if upgradeable {
		score -= 20
		findings = append(findings, "The program uses the upgradeable BPF loader.")
		programData = programDataAddress(accountData(account))
		if programData != "" {
			programDataAccount, fetchErr := fetchSolanaAccount(client, rpcURL, programData)
			if fetchErr == nil && programDataAccount != nil {
				upgradeAuthority = programUpgradeAuthority(accountData(programDataAccount))
			}
		}
		if upgradeAuthority != "" {
			score -= 20
			findings = append(findings, "An active upgrade authority can replace the deployed program.")
			recommendations = append(recommendations, "Confirm the upgrade authority is controlled by an appropriate multisig or governance process.")
		} else {
			findings = append(findings, "No active upgrade authority was detected; the program appears immutable.")
		}
	} else if account.Executable {
		findings = append(findings, "The program is not owned by the upgradeable BPF loader.")
	}

	recentTxCount := fetchRecentSignatureCount(client, rpcURL, programID)
	if recentTxCount == 0 {
		score -= 10
		findings = append(findings, "No recent transactions were returned for this program.")
		recommendations = append(recommendations, "Review deployment history, source code, and independent audits before interacting.")
	} else {
		findings = append(findings, fmt.Sprintf("The RPC returned %d recent transactions involving this program.", recentTxCount))
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Continue to verify official addresses, source code, audits, and governance changes.")
	}
	if score < 0 {
		score = 0
	}

	riskLevel := "low"
	if score < 50 {
		riskLevel = "high"
	} else if score < 80 {
		riskLevel = "medium"
	}

	writeJSON(w, http.StatusOK, programScanResponse{
		ProgramID:        programID,
		Network:          req.Network,
		Score:            score,
		RiskLevel:        riskLevel,
		Executable:       account.Executable,
		Owner:            account.Owner,
		Upgradeable:      upgradeable,
		UpgradeAuthority: upgradeAuthority,
		ProgramData:      programData,
		BalanceSOL:       fmt.Sprintf("%.6f SOL", float64(account.Lamports)/1e9),
		RecentTxCount:    recentTxCount,
		Findings:         findings,
		Recommendations:  recommendations,
		Disclaimer:       "Preliminary on-chain analysis only; this is not a substitute for a professional security audit.",
	})
}

func fetchSolanaAccount(client *http.Client, rpcURL, address string) (*solanaAccount, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "getAccountInfo",
		"params": []interface{}{address, map[string]string{"encoding": "base64"}},
	})
	resp, err := client.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("RPC returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Result struct {
			Value *solanaAccount `json:"value"`
		} `json:"result"`
		Error interface{} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("RPC returned an error")
	}
	return result.Result.Value, nil
}

func fetchRecentSignatureCount(client *http.Client, rpcURL, address string) int {
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 2, "method": "getSignaturesForAddress",
		"params": []interface{}{address, map[string]int{"limit": 100}},
	})
	resp, err := client.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var result struct {
		Result []json.RawMessage `json:"result"`
	}
	if data, err := io.ReadAll(resp.Body); err == nil {
		_ = json.Unmarshal(data, &result)
	}
	return len(result.Result)
}

func accountData(account *solanaAccount) []byte {
	if account == nil || len(account.Data) == 0 {
		return nil
	}
	encoded, ok := account.Data[0].(string)
	if !ok {
		return nil
	}
	data, _ := base64.StdEncoding.DecodeString(encoded)
	return data
}

func programDataAddress(data []byte) string {
	if len(data) < 36 || binary.LittleEndian.Uint32(data[:4]) != 2 {
		return ""
	}
	return base58Encode(data[4:36])
}

func programUpgradeAuthority(data []byte) string {
	if len(data) < 13 || binary.LittleEndian.Uint32(data[:4]) != 3 || data[12] == 0 {
		return ""
	}
	if len(data) < 45 {
		return ""
	}
	return base58Encode(data[13:45])
}

func base58Encode(input []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	if len(input) == 0 {
		return ""
	}
	zeros := 0
	for zeros < len(input) && input[zeros] == 0 {
		zeros++
	}
	digits := []byte{0}
	for _, b := range input[zeros:] {
		carry := int(b)
		for i := 0; i < len(digits); i++ {
			carry += int(digits[i]) << 8
			digits[i] = byte(carry % 58)
			carry /= 58
		}
		for carry > 0 {
			digits = append(digits, byte(carry%58))
			carry /= 58
		}
	}
	var out strings.Builder
	for i := 0; i < zeros; i++ {
		out.WriteByte(alphabet[0])
	}
	if zeros < len(input) {
		for i := len(digits) - 1; i >= 0; i-- {
			out.WriteByte(alphabet[digits[i]])
		}
	}
	return out.String()
}
