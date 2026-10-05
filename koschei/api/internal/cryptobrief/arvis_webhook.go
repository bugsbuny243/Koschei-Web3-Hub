package cryptobrief

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ARVISTelegramWebhookHTTP is the authenticated Telegram webhook for the ARVIS
// customer companion bot. Only private user messages are accepted.
func (s *Service) ARVISTelegramWebhookHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.Config.Ready("telegram") {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if !secretEqual(r.Header.Get("X-Telegram-Bot-Api-Secret-Token"), s.Config.TelegramSecret) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	var update struct {
		UpdateID int64 `json:"update_id"`
		Message  *struct {
			Date int64  `json:"date"`
			Text string `json:"text"`
			From struct {
				ID  int64 `json:"id"`
				Bot bool  `json:"is_bot"`
			} `json:"from"`
			Chat struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			} `json:"chat"`
		} `json:"message"`
	}
	decoder := json.NewDecoder(r.Body)
	if decoder.Decode(&update) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if update.UpdateID <= 0 || update.Message == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	message := update.Message
	messageTime := time.Unix(message.Date, 0)
	if message.Chat.Type != "private" || message.Chat.ID <= 0 || message.From.ID != message.Chat.ID || message.From.Bot || time.Since(messageTime) > 10*time.Minute || messageTime.After(time.Now().Add(time.Minute)) {
		w.WriteHeader(http.StatusOK)
		return
	}
	err := s.ARVISInbound(r.Context(), strconv.FormatInt(update.UpdateID, 10), strconv.FormatInt(message.Chat.ID, 10), message.Text)
	if errors.Is(err, ErrPairInvalid) {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
