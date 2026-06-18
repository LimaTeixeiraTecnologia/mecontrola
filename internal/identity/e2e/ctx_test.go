//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type e2eCtx struct {
	httpServer   *httptest.Server
	mgr          *sqlx.DB
	userID       uuid.UUID
	lastResp     *http.Response
	lastBody     map[string]any
	lastBodyText string

	identityModule                  identity.IdentityModule
	identityScenarioStart           time.Time
	identityUserID                  string
	identityLinkedUserID            uuid.UUID
	identitySecondUserID            uuid.UUID
	identityResolvedChan            string
	identityLinkErr                 error
	identitySubID                   string
	identityLastAuthEnvID           string
	identityLastAuthEnv             outbox.Envelope
	identityResolvePrincipalErr     error
	identityResolvedPrincipalUserID uuid.UUID
	identityOldAuthEventID          string
	identityRecentAuthEventID       string
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

	req, err := http.NewRequest(method, e.httpServer.URL+path, bodyReader)
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
