package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

const (
	maxTelegramMessageRunes = 4096
	parseModeHTML           = "HTML"
)

var ErrSendUnavailable = errors.New("telegram.outbound: send message unavailable")

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

type Gateway struct {
	client    *httpclient.Client
	botToken  string
	o11y      observability.Observability
	sendTotal observability.Counter
	sendError observability.Counter
}

func NewGateway(client *httpclient.Client, botToken string, o11y observability.Observability) *Gateway {
	sendTotal := o11y.Metrics().Counter(
		"telegram_send_message_total",
		"Total de chamadas a sendMessage do Telegram por status",
		"1",
	)
	sendError := o11y.Metrics().Counter(
		"telegram_send_message_errors_total",
		"Total de falhas em sendMessage por reason",
		"1",
	)
	return &Gateway{
		client:    client,
		botToken:  botToken,
		o11y:      o11y,
		sendTotal: sendTotal,
		sendError: sendError,
	}
}

func (g *Gateway) SendTextMessage(ctx context.Context, chatID int64, text string) (retErr error) {
	ctx, span := g.o11y.Tracer().Start(ctx, "telegram.outbound.send_message")
	defer span.End()

	safe := htmlEscaper.Replace(truncateRunes(text, maxTelegramMessageRunes))
	if safe == "" {
		g.sendError.Add(ctx, 1, observability.String("reason", "empty_text"))
		return fmt.Errorf("telegram.outbound: text is empty: %w", ErrSendUnavailable)
	}

	body, err := json.Marshal(struct {
		ChatID    int64  `json:"chat_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}{ChatID: chatID, Text: safe, ParseMode: parseModeHTML})
	if err != nil {
		return fmt.Errorf("telegram.outbound: marshal body: %w", err)
	}

	path := "/bot" + g.botToken + "/sendMessage"
	resp, err := g.client.Post(ctx, path, bytes.NewReader(body),
		httpclient.WithHeader("Content-Type", "application/json"),
	)
	if err != nil {
		g.sendError.Add(ctx, 1, observability.String("reason", "transport"))
		span.RecordError(err)
		return fmt.Errorf("telegram.outbound: post: %w", err)
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr == nil {
			return
		}
		g.o11y.Logger().Warn(ctx, "telegram.outbound.send_message.close_failed",
			observability.Error(closeErr),
		)
		if retErr == nil {
			retErr = fmt.Errorf("telegram.outbound: close response body: %w", closeErr)
		}
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		g.sendTotal.Add(ctx, 1, observability.String("status", "ok"))
		return nil
	}

	g.sendTotal.Add(ctx, 1, observability.String("status", "error"))
	g.sendError.Add(ctx, 1, observability.String("reason", classifyStatus(resp.StatusCode)))
	g.o11y.Logger().Warn(ctx, "telegram.outbound.send_message.non_2xx",
		observability.Int("status_code", resp.StatusCode),
		observability.String("body_preview", previewBody(resp.Body)),
	)
	return fmt.Errorf("telegram.outbound: non-2xx status %d: %w", resp.StatusCode, ErrSendUnavailable)
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func classifyStatus(code int) string {
	switch {
	case code == http.StatusUnauthorized:
		return "unauthorized"
	case code == http.StatusForbidden:
		return "forbidden"
	case code == http.StatusTooManyRequests:
		return "rate_limited"
	case code >= 500:
		return "upstream_5xx"
	default:
		return "client_4xx"
	}
}

func previewBody(body io.Reader) string {
	const maxPreview = 256
	limited := io.LimitReader(body, maxPreview)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return ""
	}
	return string(buf)
}
