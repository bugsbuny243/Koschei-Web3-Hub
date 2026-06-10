package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type Request struct {
	Prompt string `json:"prompt"`
}

type Response struct {
	Action     string   `json:"action"` // "create", "update", "delete"
	File       string   `json:"file"`
	Code       string   `json:"code"`
	CommitMsg  string   `json:"commit_msg"`
	IssueTitle string   `json:"issue_title,omitempty"`
	IssueBody  string   `json:"issue_body,omitempty"`
}

// CallAI sends prompt to Together AI and gets structured response
func CallAI(prompt string) (*Response, error) {
	apiKey := os.Getenv("TOGETHER_API_KEY")
	model := os.Getenv("TOGETHER_MODEL")

	payload := map[string]interface{}{
		"model":      model,
		"prompt":     buildSystemPrompt(prompt),
		"max_tokens": 2048,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.together.xyz/inference", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	output := result["output"].(map[string]interface{})["choices"].([]interface{})[0].(map[string]interface{})["text"].(string)

	var res Response
	json.Unmarshal([]byte(output), &res)
	return &res, nil
}

func buildSystemPrompt(userPrompt string) string {
	return fmt.Sprintf(`
Sen Koschei AI'sin. Projenin beynisin. Kullanıcı sana bir görev verdiğinde:
1. Ne yapılması gerektiğini anla
2. Hangi dosyaların etkileneceğini tahmin et
3. JSON formatında yanıt ver:

{
  "action": "create|update|delete",
  "file": "dosya/yolu.go",
  "code": "kod içeriği",
  "commit_msg": "feat: yeni özellik eklendi",
  "issue_title": "Opsiyonel hata başlığı",
  "issue_body": "Hatanın detayı"
}

Kurallar:
- Yalnızca bir dosya işle
- Kod sentaks hatasız olmalı
- Türkçe açıklama kullanma, commit mesajı İngilizce olsun
- Asla mevcut kodu silme, yorum satırı kullan

Kullanıcı dedi ki:
"%s"
`, userPrompt)
}
