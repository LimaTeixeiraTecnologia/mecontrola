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

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
)

type cardE2ECtx struct {
	server                *httptest.Server
	db                    *sqlx.DB
	userID                uuid.UUID
	lastResp              *http.Response
	lastBody              map[string]any
	lastBodyText          string
	cardID                string
	cardName              string
	lastCursor            string
	listItems             []map[string]any
	capturedIdemKey       string
	capturedIdemPayload   map[string]any
	firstResponseBody     string
	channelGateway        *cardE2EChannelGateway
	eventHandlers         map[string]platformevents.Handler
	invoiceDueJob         worker.Job
	expectedDueDate       time.Time
	expectedDaysUntil     int
	lastOnboardingEventID string
	lastInvoiceDueEventID string
}

func newCardE2ECtx(runtime *cardE2ERuntime, db *sqlx.DB) *cardE2ECtx {
	return &cardE2ECtx{
		server:         runtime.server,
		db:             db,
		userID:         uuid.MustParse(e2eUserID),
		channelGateway: runtime.channelGateway,
		eventHandlers:  runtime.eventHandlers,
		invoiceDueJob:  runtime.invoiceDueJob,
	}
}

func (e *cardE2ECtx) makeRequest(method, path string, payload any) error {
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
	e.lastBody, e.lastBodyText = parseCardResponseBody(resp)
	return nil
}

func (e *cardE2ECtx) makeRequestWithKey(method, path string, payload any, key string) error {
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
		req.Header.Set("Idempotency-Key", key)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	e.lastResp = resp
	e.lastBody, e.lastBodyText = parseCardResponseBody(resp)
	e.firstResponseBody = e.lastBodyText
	return nil
}

func (e *cardE2ECtx) createCardViaHTTP(name string, closingDay, dueDay int, limitCents int64) error {
	payload := map[string]any{
		"name":        name,
		"nickname":    e.uniqueNickname(name),
		"closing_day": closingDay,
		"due_day":     dueDay,
		"limit_cents": limitCents,
	}
	if err := e.makeRequest(http.MethodPost, "/api/v1/cards/", payload); err != nil {
		return err
	}
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}
	if e.lastResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("status esperado 201, recebido %d, corpo: %s", e.lastResp.StatusCode, e.lastBodyText)
	}
	id, ok := e.lastBody["id"].(string)
	if !ok || id == "" {
		return fmt.Errorf("resposta de criação sem id válido: %s", e.lastBodyText)
	}
	e.cardID = id
	e.cardName = name
	return nil
}

func (e *cardE2ECtx) uniqueCardName(prefix string) string {
	return fmt.Sprintf("%s %d", prefix, time.Now().UnixNano())
}

func (e *cardE2ECtx) uniqueNickname(prefix string) string {
	base := strings.ToLower(strings.ReplaceAll(prefix, " ", "-"))
	if len(base) > 20 {
		base = base[:20]
	}
	return fmt.Sprintf("%s-%s", base, uuid.NewString()[:8])
}

func (e *cardE2ECtx) cardExistsWithDetails(nome string, fechamento, vencimento int, limite int64) error {
	return e.createCardViaHTTP(nome, fechamento, vencimento, limite)
}

func (e *cardE2ECtx) runInvoiceDueAlertsJob() error {
	if e.invoiceDueJob == nil {
		return fmt.Errorf("job de alertas de fatura não disponível")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.invoiceDueJob.Run(ctx)
}

func parseCardResponseBody(resp *http.Response) (map[string]any, string) {
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
