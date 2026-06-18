//go:build e2e

package e2e_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
)

type e2eCtx struct {
	server                   *httptest.Server
	mgr                      *sqlx.DB
	userID                   uuid.UUID
	lastResp                 *http.Response
	lastBody                 map[string]any
	lastBodyText             string
	categoryID               string
	txID                     string
	cardID                   string
	cardName                 string
	expectedDueDate          time.Time
	expectedDaysUntil        int
	invoiceDueAlertsJob      worker.Job
	channelGateway           *e2eChannelGateway
	billingWebhookSecret     string
	billingProductID         string
	billingOrderID           string
	billingSubscriptionID    string
	billingPeriodEnd         time.Time
	billingEventAt           time.Time
	billingPreviousPeriodEnd time.Time
}

func (e *e2eCtx) makeRequest(method, path string, payload any) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, e.server.URL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("X-User-ID", e.userID.String())
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if isMutatingMethod(method) {
		req.Header.Set("Idempotency-Key", uuid.NewString())
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	e.lastResp = resp
	e.lastBody, e.lastBodyText = parseResponseBody(resp)
	return nil
}

func (e *e2eCtx) makeSignedBillingWebhookRequest(payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal billing payload: %w", err)
	}

	mac := hmac.New(sha1.New, []byte(e.billingWebhookSecret))
	mac.Write(data)
	signature := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest(http.MethodPost, e.server.URL+"/api/v1/billing/webhooks/kiwify?signature="+signature, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("new billing request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do billing request: %w", err)
	}

	e.lastResp = resp
	e.lastBody, e.lastBodyText = parseResponseBody(resp)
	return nil
}

func parseResponseBody(resp *http.Response) (map[string]any, string) {
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return result, strings.TrimSpace(string(data))
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
