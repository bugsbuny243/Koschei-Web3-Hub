package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/router"
)

type Request struct {
	Prompt string `json:"prompt"`
}

type Response struct {
	Action     string `json:"action"` // "create", "update", "delete"
	File       string `json:"file"`
	Code       string `json:"code"`
	CommitMsg  string `json:"commit_msg"`
	IssueTitle string `json:"issue_title,omitempty"`
	IssueBody  string `json:"issue_body,omitempty"`
}

// CallAI sends prompts through Koschei's single AI router. OpenAI is tried first; Together is used only as fallback.
func CallAI(prompt string) (*Response, error) {
	ai, err := router.Chat(context.Background(), router.ChatRequest{
		System:      "You are Koschei AI. Return only the requested JSON object.",
		Prompt:      buildSystemPrompt(prompt),
		MaxTokens:   2048,
		Temperature: 0.2,
		Timeout:     30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(ai.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var res Response
	if err := json.Unmarshal([]byte(content), &res); err != nil {
		return nil, err
	}
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
