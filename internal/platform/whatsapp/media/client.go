package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	defaultDownloadBufferHint int64 = 64 << 10
)

type Config struct {
	AccessToken string
	BaseURL     string
	HTTPTimeout time.Duration
}

type Client struct {
	httpClient  *httpclient.Client
	accessToken string
}

func NewClient(o11y devkitobs.Observability, cfg Config) (*Client, error) {
	if o11y == nil {
		return nil, fmt.Errorf("whatsapp/media: observability é obrigatório")
	}
	if strings.TrimSpace(cfg.AccessToken) == "" {
		return nil, fmt.Errorf("whatsapp/media: access_token é obrigatório")
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
		httpclient.WithTarget("whatsapp_media"),
		httpclient.WithTimeout(timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("whatsapp/media: criar http client: %w", err)
	}

	return &Client{
		httpClient:  httpClient,
		accessToken: cfg.AccessToken,
	}, nil
}

func (c *Client) Resolve(ctx context.Context, mediaID string) (MediaDescriptor, error) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return MediaDescriptor{}, ErrMediaIDEmpty
	}

	resp, err := c.httpClient.Get(
		ctx,
		"/"+mediaID,
		httpclient.WithHeader("Authorization", "Bearer "+c.accessToken),
		httpclient.WithoutRetry(),
	)
	if err != nil {
		return MediaDescriptor{}, fmt.Errorf("whatsapp/media: resolver media id: %w", err)
	}
	defer closeBody(ctx, resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MediaDescriptor{}, fmt.Errorf("whatsapp/media: ler resposta de resolve: %w", err)
	}

	if err := checkStatus(resp.StatusCode, body); err != nil {
		return MediaDescriptor{}, err
	}

	var parsed resolveMediaResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return MediaDescriptor{}, fmt.Errorf("whatsapp/media: deserializar resposta de resolve: %w", err)
	}

	if strings.TrimSpace(parsed.URL) == "" {
		return MediaDescriptor{}, ErrMediaURLMissing
	}

	return MediaDescriptor{
		URL:      parsed.URL,
		MimeType: parsed.MimeType,
		SHA256:   parsed.SHA256,
		Size:     parsed.FileSize,
	}, nil
}

func (c *Client) Download(ctx context.Context, url string, maxBytes int64) (DownloadedMedia, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return DownloadedMedia{}, ErrDownloadURLEmpty
	}
	if maxBytes <= 0 {
		return DownloadedMedia{}, fmt.Errorf("whatsapp/media: maxBytes deve ser positivo, recebido %d", maxBytes)
	}

	resp, err := c.httpClient.Get(
		ctx,
		url,
		httpclient.WithHeader("Authorization", "Bearer "+c.accessToken),
		httpclient.WithoutRetry(),
	)
	if err != nil {
		return DownloadedMedia{}, fmt.Errorf("whatsapp/media: baixar midia: %w", err)
	}
	defer closeBody(ctx, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return DownloadedMedia{}, checkStatus(resp.StatusCode, body)
	}

	if resp.ContentLength > maxBytes {
		return DownloadedMedia{}, ErrDownloadTooLarge
	}

	buf := bytes.NewBuffer(make([]byte, 0, downloadBufferHint(resp.ContentLength, maxBytes)))
	if _, err := buf.ReadFrom(io.LimitReader(resp.Body, maxBytes+1)); err != nil {
		return DownloadedMedia{}, fmt.Errorf("whatsapp/media: ler bytes do download: %w", err)
	}
	data := buf.Bytes()

	if int64(len(data)) > maxBytes {
		return DownloadedMedia{}, ErrDownloadTooLarge
	}

	sum := sha256.Sum256(data)
	downloaded := DownloadedMedia{
		Bytes:    data,
		MimeType: resp.Header.Get("Content-Type"),
		SHA256:   hex.EncodeToString(sum[:]),
		Size:     int64(len(data)),
	}

	return downloaded, nil
}

func downloadBufferHint(contentLength, maxBytes int64) int64 {
	if contentLength > 0 && contentLength <= maxBytes {
		return contentLength
	}
	if maxBytes < defaultDownloadBufferHint {
		return maxBytes
	}
	return defaultDownloadBufferHint
}

func VerifyChecksum(descriptor MediaDescriptor, downloaded DownloadedMedia) error {
	expected := strings.ToLower(strings.TrimSpace(descriptor.SHA256))
	actual := strings.ToLower(strings.TrimSpace(downloaded.SHA256))
	if expected == "" || expected != actual {
		return ErrSHA256Mismatch
	}
	return nil
}

func checkStatus(statusCode int, body []byte) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	var errResp mediaErrorResponse
	_ = json.Unmarshal(body, &errResp)

	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return fmt.Errorf("%w: status %d code %d: %s", ErrMediaAuth, statusCode, errResp.Error.Code, errResp.Error.Message)
	case statusCode >= 500:
		return fmt.Errorf("%w: status %d code %d: %s", ErrMediaServer, statusCode, errResp.Error.Code, errResp.Error.Message)
	case statusCode >= 400:
		return fmt.Errorf("%w: status %d code %d: %s", ErrMediaBadRequest, statusCode, errResp.Error.Code, errResp.Error.Message)
	default:
		return fmt.Errorf("whatsapp/media: status inesperado %d", statusCode)
	}
}

func closeBody(ctx context.Context, body io.ReadCloser) {
	if closeErr := body.Close(); closeErr != nil {
		slog.WarnContext(ctx, "whatsapp/media: close response body", "error", closeErr)
	}
}
