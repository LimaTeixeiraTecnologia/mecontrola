package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	devkitobs "github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

const (
	defaultBaseURL = "https://graph.facebook.com/v18.0"
	defaultTimeout = 10 * time.Second
)

type Config struct {
	PhoneNumberID string
	AccessToken   string
	BaseURL       string
	HTTPTimeout   time.Duration
}

type Client struct {
	httpClient    *httpclient.Client
	phoneNumberID string
	accessToken   string
}

func NewClient(o11y devkitobs.Observability, cfg Config) (*Client, error) {
	if o11y == nil {
		return nil, fmt.Errorf("onboarding/meta: observability é obrigatório")
	}
	if cfg.PhoneNumberID == "" {
		return nil, fmt.Errorf("onboarding/meta: phone_number_id é obrigatório")
	}
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("onboarding/meta: access_token é obrigatório")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	httpClient, err := httpclient.NewClient(
		o11y,
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("meta"),
		httpclient.WithTimeout(timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("onboarding/meta: criar http client: %w", err)
	}

	return &Client{
		httpClient:    httpClient,
		phoneNumberID: cfg.PhoneNumberID,
		accessToken:   cfg.AccessToken,
	}, nil
}

func (c *Client) SendTemplate(ctx context.Context, toE164, templateName, languageCode string, components []any) (string, error) {
	toE164 = strings.TrimPrefix(toE164, "+")

	if languageCode == "" {
		languageCode = "pt_BR"
	}

	payload := sendMessageRequest{
		MessagingProduct: "whatsapp",
		To:               toE164,
		Type:             "template",
		Template: &templatePayload{
			Name:       templateName,
			Language:   templateLanguage{Code: languageCode},
			Components: components,
		},
	}

	return c.doSend(ctx, payload)
}

func (c *Client) SendText(ctx context.Context, toE164, text string) (string, error) {
	toE164 = strings.TrimPrefix(toE164, "+")

	payload := sendMessageRequest{
		MessagingProduct: "whatsapp",
		To:               toE164,
		Type:             "text",
		Text:             &textPayload{Body: text},
	}

	return c.doSend(ctx, payload)
}

func (c *Client) doSend(ctx context.Context, payload sendMessageRequest) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("onboarding/meta: serializar payload: %w", err)
	}

	path := fmt.Sprintf("/%s/messages", c.phoneNumberID)

	resp, err := c.httpClient.Post(
		ctx,
		path,
		bytes.NewReader(body),
		httpclient.WithHeader("Authorization", "Bearer "+c.accessToken),
		httpclient.WithHeader("Content-Type", "application/json"),
		httpclient.WithoutRetry(),
	)
	if err != nil {
		return "", fmt.Errorf("onboarding/meta: enviar mensagem: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.WarnContext(ctx, "onboarding/meta: close response body", "error", closeErr)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("onboarding/meta: ler resposta: %w", err)
	}

	if err := c.checkStatus(resp.StatusCode, respBody); err != nil {
		return "", err
	}

	var result sendMessageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("onboarding/meta: deserializar resposta: %w", err)
	}

	if len(result.Messages) == 0 {
		return "", fmt.Errorf("onboarding/meta: resposta sem message id")
	}

	return result.Messages[0].ID, nil
}

func (c *Client) checkStatus(statusCode int, body []byte) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	var errResp errorResponse
	_ = json.Unmarshal(body, &errResp)

	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return fmt.Errorf("onboarding/meta: %w: status %d code %d: %s", ErrMetaAuth, statusCode, errResp.Error.Code, errResp.Error.Message)
	case statusCode >= 500:
		return fmt.Errorf("onboarding/meta: %w: status %d code %d: %s", ErrMetaServer, statusCode, errResp.Error.Code, errResp.Error.Message)
	case statusCode >= 400:
		return fmt.Errorf("onboarding/meta: %w: status %d code %d: %s", ErrMetaBadRequest, statusCode, errResp.Error.Code, errResp.Error.Message)
	default:
		return fmt.Errorf("onboarding/meta: status inesperado %d", statusCode)
	}
}
