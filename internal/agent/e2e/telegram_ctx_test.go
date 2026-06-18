//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/signature"
)

func (e *agentE2ECtx) withTelegram(
	gateway *CapturingTelegramGateway,
	telegramSecret string,
	telegramChatID, telegramUserID int64,
) *agentE2ECtx {
	e.telegramGateway = gateway
	e.telegramSecret = telegramSecret
	e.telegramChatID = telegramChatID
	e.telegramUserID = telegramUserID
	return e
}

func (e *agentE2ECtx) buildTelegramPayload(text string, updateID int64) []byte {
	type tgUser struct {
		ID           int64  `json:"id"`
		IsBot        bool   `json:"is_bot"`
		LanguageCode string `json:"language_code"`
	}
	type tgChat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	}
	type tgMessage struct {
		MessageID int64  `json:"message_id"`
		From      tgUser `json:"from"`
		Chat      tgChat `json:"chat"`
		Date      int64  `json:"date"`
		Text      string `json:"text"`
	}
	type tgUpdate struct {
		UpdateID int64     `json:"update_id"`
		Message  tgMessage `json:"message"`
	}
	upd := tgUpdate{
		UpdateID: updateID,
		Message: tgMessage{
			MessageID: updateID,
			From:      tgUser{ID: e.telegramUserID, IsBot: false, LanguageCode: "pt-br"},
			Chat:      tgChat{ID: e.telegramChatID, Type: "private"},
			Date:      time.Now().UTC().Unix(),
			Text:      text,
		},
	}
	raw, _ := json.Marshal(upd)
	return raw
}

func (e *agentE2ECtx) postTelegramWebhook(text string, updateID int64) error {
	body := e.buildTelegramPayload(text, updateID)
	req, err := http.NewRequest(http.MethodPost, e.server.URL+"/api/v1/channels/telegram/webhook", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("criar request telegram: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(signature.HeaderSecretToken, e.telegramSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("executar request telegram: %w", err)
	}
	defer resp.Body.Close()

	e.lastStatus = resp.StatusCode
	e.lastTelegramUpdate = updateID
	return nil
}
