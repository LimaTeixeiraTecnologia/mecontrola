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
)

type categoriesE2ECtx struct {
	server            *httptest.Server
	db                *sqlx.DB
	lastResp          *http.Response
	lastBody          map[string]any
	lastBodyText      string
	lastETag          string
	lastCursor        string
	currentCategoryID string
	currentParentID   string
}

func newCategoriesE2ECtx(server *httptest.Server, db *sqlx.DB) *categoriesE2ECtx {
	return &categoriesE2ECtx{
		server: server,
		db:     db,
	}
}

func (e *categoriesE2ECtx) get(path string) error {
	return e.getWithHeaders(path, "", true)
}

func (e *categoriesE2ECtx) getWithETag(path, etag string) error {
	return e.getWithHeaders(path, etag, true)
}

func (e *categoriesE2ECtx) getWithoutAuth(path string) error {
	return e.getWithHeaders(path, "", false)
}

func (e *categoriesE2ECtx) getWithHeaders(path, etag string, withAuth bool) error {
	req, err := http.NewRequest(http.MethodGet, e.server.URL+path, nil)
	if err != nil {
		return fmt.Errorf("criar request: %w", err)
	}

	if withAuth {
		req.Header.Set("X-User-ID", e2eUserID)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("executar request: %w", err)
	}

	e.lastResp = resp
	e.lastETag = resp.Header.Get("ETag")
	e.lastBody, e.lastBodyText = parseResponseBody(resp)
	e.lastCursor = e.extractStringField("next_cursor")
	return nil
}

func parseResponseBody(resp *http.Response) (map[string]any, string) {
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var body map[string]any
	_ = json.Unmarshal(data, &body)

	return body, strings.TrimSpace(string(data))
}

func (e *categoriesE2ECtx) categoryIDBySlug(kind, slug string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id string
	err := e.db.QueryRowContext(
		ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind = $1 AND slug = $2 LIMIT 1`,
		kind,
		slug,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("buscar categoria %s/%s: %w", kind, slug, err)
	}

	return id, nil
}

func (e *categoriesE2ECtx) ensureDeprecatedCategory(kind, parentSlug, slug, name, allocationType string) (string, error) {
	parentID, err := e.categoryIDBySlug(kind, parentSlug)
	if err != nil {
		return "", err
	}

	categoryID := uuid.NewSHA1(uuid.Nil, []byte(kind+":"+slug)).String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = e.db.ExecContext(ctx, `
		INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type, deprecated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (kind, slug) DO UPDATE
		SET name = EXCLUDED.name,
			parent_id = EXCLUDED.parent_id,
			allocation_type = EXCLUDED.allocation_type,
			deprecated_at = now()
	`, categoryID, slug, name, kind, parentID, allocationType)
	if err != nil {
		return "", fmt.Errorf("criar categoria deprecated: %w", err)
	}

	return categoryID, nil
}

func (e *categoriesE2ECtx) ensureAmbiguousDictionaryEntries(term string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deliveryID, err := e.categoryIDBySlug("expense", "delivery")
	if err != nil {
		return err
	}

	restaurantsID, err := e.categoryIDBySlug("expense", "restaurantes")
	if err != nil {
		return err
	}

	entries := []struct {
		id         string
		categoryID string
	}{
		{uuid.NewSHA1(uuid.Nil, []byte(term+":delivery")).String(), deliveryID},
		{uuid.NewSHA1(uuid.Nil, []byte(term+":restaurantes")).String(), restaurantsID},
	}

	for _, entry := range entries {
		_, err = e.db.ExecContext(ctx, `
			INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous)
			VALUES ($1, $2, 'expense', $3, 'alias', 'high', true)
			ON CONFLICT (id) DO NOTHING
		`, entry.id, entry.categoryID, term)
		if err != nil {
			return fmt.Errorf("criar entrada ambigua: %w", err)
		}
	}

	return nil
}

func (e *categoriesE2ECtx) extractStringField(field string) string {
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

func (e *categoriesE2ECtx) bodyArray(field string) ([]any, error) {
	if e.lastBody == nil {
		return nil, fmt.Errorf("corpo json ausente")
	}

	raw, ok := e.lastBody[field]
	if !ok {
		return nil, fmt.Errorf("campo %q ausente", field)
	}

	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("campo %q nao e lista", field)
	}

	return items, nil
}

func (e *categoriesE2ECtx) arrayItemByStringField(field, expected string, items []any) (map[string]any, bool) {
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}

		value, ok := record[field].(string)
		if ok && value == expected {
			return record, true
		}
	}

	return nil, false
}

func (e *categoriesE2ECtx) mustMarshalBody() string {
	if e.lastBodyText != "" {
		return e.lastBodyText
	}

	if e.lastBody == nil {
		return ""
	}

	data, _ := json.Marshal(e.lastBody)
	return string(bytes.TrimSpace(data))
}
