//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type budgetsE2ECtx struct {
	server         *httptest.Server
	db             *sqlx.DB
	lastResp       *http.Response
	lastBody       map[string]any
	lastBodyText   string
	lastBudgetID   string
	lastExpenseID  string
	lastCompetence string
	lastExternalID string
}

func newBudgetsE2ECtx(server *httptest.Server, db *sqlx.DB) *budgetsE2ECtx {
	return &budgetsE2ECtx{
		server: server,
		db:     db,
	}
}

func (e *budgetsE2ECtx) post(path string, body any) error {
	return e.doRequest(http.MethodPost, path, body, true)
}

func (e *budgetsE2ECtx) postWithoutAuth(path string, body any) error {
	return e.doRequest(http.MethodPost, path, body, false)
}

func (e *budgetsE2ECtx) patch(path string, body any) error {
	return e.doRequest(http.MethodPatch, path, body, true)
}

func (e *budgetsE2ECtx) delete(path string, body any) error {
	return e.doRequest(http.MethodDelete, path, body, true)
}

func (e *budgetsE2ECtx) get(path string) error {
	return e.doRequest(http.MethodGet, path, nil, true)
}

func (e *budgetsE2ECtx) getWithoutAuth(path string) error {
	return e.doRequest(http.MethodGet, path, nil, false)
}

func (e *budgetsE2ECtx) doRequest(method, path string, body any, withAuth bool) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("serializar body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, e.server.URL+path, reqBody)
	if err != nil {
		return fmt.Errorf("criar request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withAuth {
		req.Header.Set("X-User-ID", e2eUserID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("executar request: %w", err)
	}

	e.lastResp = resp
	e.lastBody, e.lastBodyText = parseResponseBody(resp)
	return nil
}

func parseResponseBody(resp *http.Response) (map[string]any, string) {
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var body map[string]any
	_ = json.Unmarshal(data, &body)

	return body, strings.TrimSpace(string(data))
}

func (e *budgetsE2ECtx) mustMarshalBody() string {
	if e.lastBodyText != "" {
		return e.lastBodyText
	}

	if e.lastBody == nil {
		return ""
	}

	data, _ := json.Marshal(e.lastBody)
	return string(bytes.TrimSpace(data))
}

func (e *budgetsE2ECtx) extractStringField(field string) string {
	if e.lastBody == nil {
		return ""
	}

	raw, ok := e.lastBody[field]
	if !ok {
		return ""
	}

	value, ok := raw.(string)
	if !ok {
		return ""
	}

	return value
}

func (e *budgetsE2ECtx) countBudgets(userID, competence string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int
	err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets WHERE user_id = $1 AND competence = $2`,
		userID, competence,
	).Scan(&n)
	return n, err
}

func (e *budgetsE2ECtx) budgetState(userID, competence string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var state int
	err := e.db.QueryRowContext(ctx,
		`SELECT state FROM mecontrola.budgets WHERE user_id = $1 AND competence = $2`,
		userID, competence,
	).Scan(&state)
	return state, err
}

func (e *budgetsE2ECtx) countExpenses(userID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int
	err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expenses WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&n)
	return n, err
}

func (e *budgetsE2ECtx) countOutboxByType(eventType string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int
	err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.outbox_events WHERE event_type = $1`,
		eventType,
	).Scan(&n)
	return n, err
}

func (e *budgetsE2ECtx) expenseDeletedAt(userID, source, extID string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var deletedAt *time.Time
	err := e.db.QueryRowContext(ctx,
		`SELECT deleted_at FROM mecontrola.budgets_expenses WHERE user_id = $1 AND source = $2 AND external_transaction_id = $3`,
		userID, source, extID,
	).Scan(&deletedAt)
	if err != nil {
		return false, err
	}
	return deletedAt != nil, nil
}

func (e *budgetsE2ECtx) countTombstones(userID, source, extID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int
	err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expenses WHERE user_id = $1 AND source = $2 AND external_transaction_id = $3 AND tombstone_version IS NOT NULL`,
		userID, source, extID,
	).Scan(&n)
	return n, err
}
