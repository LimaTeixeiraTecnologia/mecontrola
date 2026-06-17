//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/google/uuid"
)

type e2eCtx struct {
	server     *httptest.Server
	mgr        manager.Manager
	userID     uuid.UUID
	lastResp   *http.Response
	lastBody   map[string]any
	categoryID string
	txID       string
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
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	e.lastResp = resp
	e.lastBody = parseResponseBody(resp)
	return nil
}

func parseResponseBody(resp *http.Response) map[string]any {
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return result
}
