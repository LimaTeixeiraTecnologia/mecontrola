//go:build e2e

package transactions_e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
	txconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/consumers"
)

type txE2ECtx struct {
	server              *httptest.Server
	db                  *sqlx.DB
	userID              uuid.UUID
	lastResp            *http.Response
	lastBody            map[string]any
	lastBodyText        string
	capturedTxID        string
	capturedCPID        string
	capturedRTID        string
	capturedAggregateID string
	capturedVersion     int64
	cardID              string
	lastJobErr          error
	recomputeConsumer   *txconsumers.MonthlySummaryRecomputeConsumer
	recurringJob        worker.Job
	reconcilerJob       worker.Job
}

func (e *txE2ECtx) makeRequest(method, path string, payload any) error {
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

func (e *txE2ECtx) makeUnauthenticatedRequest(method, path string, payload any) error {
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
		return fmt.Errorf("new request unauthenticated: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do unauthenticated request: %w", err)
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
	return result, string(data)
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
