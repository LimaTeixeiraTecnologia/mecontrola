//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	onboardingvalueobjects "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/google/uuid"
)

type onboardingWorld struct {
	t                       *testing.T
	runtime                 *onboardingRuntime
	lastResp                *http.Response
	lastBody                map[string]any
	lastBodyText            string
	lastReply               string
	lastOutboxEventID       string
	currentTokenClear       string
	currentTokenID          string
	currentSubscriptionID   string
	currentUserID           uuid.UUID
	lastStatusCodes         []int
	lastActivationMobile    string
	lastActivationMessageID string
}

func newOnboardingWorld(t *testing.T, runtime *onboardingRuntime) *onboardingWorld {
	t.Helper()
	world := &onboardingWorld{
		t:       t,
		runtime: runtime,
	}
	if err := world.reset(); err != nil {
		t.Fatalf("reset world: %v", err)
	}
	return world
}

func (w *onboardingWorld) reset() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := w.runtime.deps.db.ExecContext(ctx, `
		TRUNCATE TABLE
			mecontrola.auth_events,
			mecontrola.user_identities,
			mecontrola.identity_entitlements,
			mecontrola.support_signals,
			mecontrola.consumer_lookup_attempts,
			mecontrola.channel_processed_messages,
			mecontrola.outbox_events,
			mecontrola.billing_subscriptions,
			mecontrola.onboarding_tokens,
			mecontrola.onboarding_activation_nomatch_throttle,
			mecontrola.onboarding_welcome_processed,
			mecontrola.users
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		return fmt.Errorf("truncate test tables: %w", err)
	}

	w.runtime.metaGateway.reset()
	w.runtime.outreachGateway.reset()
	w.runtime.emailSender.reset()
	w.runtime.failingHandler.reset()
	w.lastResp = nil
	w.lastBody = nil
	w.lastBodyText = ""
	w.lastReply = ""
	w.lastOutboxEventID = ""
	w.currentTokenClear = ""
	w.currentTokenID = ""
	w.currentSubscriptionID = ""
	w.currentUserID = uuid.Nil
	w.lastStatusCodes = nil
	w.lastActivationMobile = ""
	w.lastActivationMessageID = ""
	return nil
}

func (w *onboardingWorld) postJSON(path string, payload any, headers map[string]string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, w.runtime.server.URL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return w.do(req)
}

func (w *onboardingWorld) options(path string, headers map[string]string) error {
	req, err := http.NewRequest(http.MethodOptions, w.runtime.server.URL+path, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return w.do(req)
}

func (w *onboardingWorld) get(path string) error {
	req, err := http.NewRequest(http.MethodGet, w.runtime.server.URL+path, nil)
	if err != nil {
		return err
	}
	return w.do(req)
}

func (w *onboardingWorld) do(req *http.Request) error {
	resp, err := w.runtime.httpClient.Do(req)
	if err != nil {
		return err
	}
	w.lastResp = resp
	w.lastBody, w.lastBodyText = parseResponseBody(resp)
	return nil
}

func parseResponseBody(resp *http.Response) (map[string]any, string) {
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	body := make(map[string]any)
	_ = json.Unmarshal(raw, &body)
	return body, strings.TrimSpace(string(raw))
}

func (w *onboardingWorld) seedBillingSubscription(customerMobile, customerEmail string) (string, error) {
	subID := uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := w.runtime.deps.db.ExecContext(ctx, `
		INSERT INTO mecontrola.billing_subscriptions
			(id, funnel_token, user_id, kiwify_order_id, plan_code, status, period_start, period_end, last_event_at, customer_mobile_e164, customer_email, external_sale_id)
		VALUES
			($1, $2, NULL, $3, 'MONTHLY', 'ACTIVE', now() - interval '1 day', now() + interval '30 day', now(), $4, $5, $6)
	`,
		subID,
		"funnel-"+subID[:8],
		e2eOrderPrefix+subID[:8],
		nullIfEmpty(customerMobile),
		nullIfEmpty(customerEmail),
		"sale-"+subID[:8],
	)
	if err != nil {
		return "", err
	}
	return subID, nil
}

func (w *onboardingWorld) seedMagicToken(clearToken string, status string, opts tokenSeedOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	token, err := parseClearToken(clearToken)
	if err != nil {
		return err
	}
	ciphertext, err := w.runtime.deps.tokenCipher.Encrypt(ctx, clearToken)
	if err != nil {
		return err
	}

	tokenID := uuid.NewString()
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	if opts.expired {
		expiresAt = time.Now().UTC().Add(-24 * time.Hour)
	}

	_, err = w.runtime.deps.db.ExecContext(ctx, `
		INSERT INTO mecontrola.onboarding_tokens
			(id, token_hash, activation_token_ciphertext, status, plan_id, expires_at, created_at, paid_at, consumed_at, outreach_sent_at, subscription_id, customer_mobile_e164, customer_email, external_sale_id, consumed_by_user_id, consumed_by_mobile_e164, activation_path)
		VALUES
			($1, $2, $3, $4, $5, $6, now(), $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`,
		tokenID,
		token.Hash(),
		ciphertext,
		status,
		e2ePlanIDMonthly,
		expiresAt,
		opts.paidAt,
		opts.consumedAt,
		opts.outreachSentAt,
		nullIfEmpty(opts.subscriptionID),
		nullIfEmpty(opts.customerMobile),
		nullIfEmpty(opts.customerEmail),
		nullIfEmpty(opts.externalSaleID),
		nullIfEmpty(opts.consumedByUserID),
		nullIfEmpty(opts.consumedByMobile),
		nullIfEmpty(opts.activationPath),
	)
	if err != nil {
		return err
	}

	w.currentTokenClear = clearToken
	w.currentTokenID = tokenID
	w.currentSubscriptionID = opts.subscriptionID
	return nil
}

func (w *onboardingWorld) tokenRow() (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	token, err := parseClearToken(w.currentTokenClear)
	if err != nil {
		return nil, err
	}

	row := w.runtime.deps.db.QueryRowContext(ctx, `
		SELECT id::text, status, plan_id, customer_mobile_e164, customer_email, external_sale_id, subscription_id::text, consumed_by_user_id::text, activation_path
		  FROM mecontrola.onboarding_tokens
		 WHERE token_hash = $1
	`, token.Hash())

	var (
		id               string
		status           string
		planID           string
		customerMobile   sql.NullString
		customerEmail    sql.NullString
		externalSaleID   sql.NullString
		subscriptionID   sql.NullString
		consumedByUserID sql.NullString
		activationPath   sql.NullString
	)
	if err := row.Scan(&id, &status, &planID, &customerMobile, &customerEmail, &externalSaleID, &subscriptionID, &consumedByUserID, &activationPath); err != nil {
		return nil, err
	}

	return map[string]any{
		"id":                  id,
		"status":              status,
		"plan_id":             planID,
		"customer_mobile":     customerMobile.String,
		"customer_email":      customerEmail.String,
		"external_sale_id":    externalSaleID.String,
		"subscription_id":     subscriptionID.String,
		"consumed_by_user_id": consumedByUserID.String,
		"activation_path":     activationPath.String,
	}, nil
}

func (w *onboardingWorld) supportSignalCount(kind string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := w.runtime.deps.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mecontrola.support_signals WHERE kind = $1`, kind)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (w *onboardingWorld) countOutboxEvents(eventType string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := w.runtime.deps.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mecontrola.outbox_events WHERE event_type = $1`, eventType)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (w *onboardingWorld) latestOutboxStatus(eventType string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := w.runtime.deps.db.QueryRowContext(ctx, `
		SELECT status
		  FROM mecontrola.outbox_events
		 WHERE event_type = $1
		 ORDER BY created_at DESC
		 LIMIT 1
	`, eventType)
	var status int
	if err := row.Scan(&status); err != nil {
		return 0, err
	}
	return status, nil
}

func (w *onboardingWorld) latestOutboxDelivery(eventType string) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := w.runtime.deps.db.QueryRowContext(ctx, `
		SELECT attempts, COALESCE(last_error, '')
		  FROM mecontrola.outbox_events
		 WHERE event_type = $1
		 ORDER BY created_at DESC
		 LIMIT 1
	`, eventType)
	var attempts int
	var lastError string
	if err := row.Scan(&attempts, &lastError); err != nil {
		return 0, "", err
	}
	return attempts, lastError, nil
}

func (w *onboardingWorld) runDispatcher(registry outbox.Registry) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	job := newDispatcherJob(w.runtime.deps.db, w.runtime.deps.outboxCfg, registry, w.runtime.deps.o11y)
	for i := 0; i < 10; i++ {
		pending, err := w.pendingOutboxCount(ctx)
		if err != nil {
			return err
		}
		if pending == 0 {
			return nil
		}
		if err := job.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (w *onboardingWorld) pendingOutboxCount(ctx context.Context) (int, error) {
	var count int
	err := w.runtime.deps.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM mecontrola.outbox_events
		 WHERE status = 1 AND next_attempt_at <= now()
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("pending outbox count: %w", err)
	}
	return count, nil
}

func (w *onboardingWorld) extractTokenFromCheckoutURL() error {
	raw, ok := w.lastBody["checkout_url"].(string)
	if !ok || raw == "" {
		return fmt.Errorf("checkout_url ausente")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	token := parsed.Query().Get("sck")
	if token == "" {
		return fmt.Errorf("token sck ausente")
	}
	w.currentTokenClear = token
	return nil
}

func (w *onboardingWorld) tokenOutreachSentAt() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	token, err := parseClearToken(w.currentTokenClear)
	if err != nil {
		return false, err
	}
	row := w.runtime.deps.db.QueryRowContext(ctx, `
		SELECT outreach_sent_at IS NOT NULL
		  FROM mecontrola.onboarding_tokens
		 WHERE token_hash = $1
	`, token.Hash())
	var sent bool
	if err := row.Scan(&sent); err != nil {
		return false, err
	}
	return sent, nil
}

func (w *onboardingWorld) ensureCurrentUserByWhatsApp(mobileE164, email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := w.runtime.deps.identityGateway.UpsertUserByWhatsApp(ctx, mobileE164, email)
	if err != nil {
		return err
	}
	parsed, err := uuid.Parse(result.UserID)
	if err != nil {
		return err
	}
	w.currentUserID = parsed
	return nil
}

type tokenSeedOptions struct {
	subscriptionID   string
	customerMobile   string
	customerEmail    string
	externalSaleID   string
	consumedByUserID string
	consumedByMobile string
	activationPath   string
	paidAt           any
	consumedAt       any
	outreachSentAt   any
	expired          bool
}

func parseClearToken(clearToken string) (clearTokenValue, error) {
	token, err := onboardingvalueobjects.TokenFromClear(clearToken)
	if err != nil {
		return clearTokenValue{}, err
	}
	return clearTokenValue{hash: token.Hash()}, nil
}

type clearTokenValue struct {
	hash []byte
}

func (t clearTokenValue) Hash() []byte {
	return t.hash
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
