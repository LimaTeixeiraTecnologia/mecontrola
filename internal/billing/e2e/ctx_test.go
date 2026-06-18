//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
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

	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
)

type billingE2ECtx struct {
	server         *httptest.Server
	db             *sqlx.DB
	module         billing.BillingModule
	webhookSecret  string
	productMonthly string

	orderID     string
	kiwifySubID string

	billingEventAt       time.Time
	billingPeriodEnd     time.Time
	billingPrevPeriodEnd time.Time

	lastResp     *http.Response
	lastBody     map[string]any
	lastBodyText string

	capturedSubID     string
	lastConsumerErr   error
	outboxCountBefore int
}

func newBillingE2ECtx(
	server *httptest.Server,
	db *sqlx.DB,
	module billing.BillingModule,
	webhookSecret string,
	productMonthly string,
) *billingE2ECtx {
	return &billingE2ECtx{
		server:         server,
		db:             db,
		module:         module,
		webhookSecret:  webhookSecret,
		productMonthly: productMonthly,
	}
}

func (e *billingE2ECtx) freshOrderIDs() {
	nano := time.Now().UnixNano()
	e.orderID = fmt.Sprintf("order-e2e-%d", nano)
	e.kiwifySubID = fmt.Sprintf("sub-e2e-%d", nano)
	e.billingEventAt = time.Time{}
	e.billingPeriodEnd = time.Time{}
	e.billingPrevPeriodEnd = time.Time{}
	e.capturedSubID = ""
}

func (e *billingE2ECtx) makeWebhookRequest(payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	mac := hmac.New(sha1.New, []byte(e.webhookSecret))
	mac.Write(data)
	signature := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest(http.MethodPost,
		e.server.URL+"/api/v1/billing/webhooks/kiwify?signature="+signature,
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	e.lastResp = resp
	e.lastBody, e.lastBodyText = parseBody(resp)
	return nil
}

func (e *billingE2ECtx) makeWebhookRequestRaw(body []byte, contentType, signature string) error {
	url := e.server.URL + "/api/v1/billing/webhooks/kiwify"
	if signature != "" {
		url += "?signature=" + signature
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	e.lastResp = resp
	e.lastBody, e.lastBodyText = parseBody(resp)
	return nil
}

func (e *billingE2ECtx) buildWebhookPayload(eventType string, now time.Time) map[string]any {
	return map[string]any{
		"order_id":           e.orderID,
		"order_ref":          "e2e-ref",
		"order_status":       "paid",
		"webhook_event_type": eventType,
		"subscription_id":    e.kiwifySubID,
		"Product": map[string]any{
			"product_id":   e.productMonthly,
			"product_name": "E2E Billing Product",
		},
		"Customer": map[string]any{
			"email":  "e2e@example.com",
			"mobile": "+5511900000000",
			"CPF":    "00000000000",
		},
		"Subscription": map[string]any{
			"status":       "active",
			"start_date":   now.Add(-24 * time.Hour).Format(time.RFC3339),
			"next_payment": now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
		},
		"TrackingParameters": map[string]any{
			"sck": "e2e-funnel-token",
			"s1":  nil,
			"src": nil,
		},
		"approved_date": now.Format("2006-01-02 15:04:05"),
		"updated_at":    now.Format("2006-01-02 15:04:05"),
		"created_at":    now.Format("2006-01-02 15:04:05"),
	}
}

func (e *billingE2ECtx) buildWebhookPayloadNoTracking(eventType string, now time.Time) map[string]any {
	p := e.buildWebhookPayload(eventType, now)
	p["TrackingParameters"] = map[string]any{
		"sck": "",
		"s1":  "",
		"src": "",
	}
	return p
}

func (e *billingE2ECtx) nextEventTime() time.Time {
	if e.billingEventAt.IsZero() {
		e.billingEventAt = time.Now().UTC()
	} else {
		e.billingEventAt = e.billingEventAt.Add(time.Minute)
	}
	return e.billingEventAt
}

func (e *billingE2ECtx) lookupSubscription() (status string, periodEnd time.Time, graceEnd time.Time, subID string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := e.db.QueryRowContext(ctx, `
        SELECT id, status, period_end, COALESCE(grace_end, '0001-01-01'::timestamptz)
          FROM billing_subscriptions
         WHERE kiwify_order_id = $1
    `, e.orderID)
	if scanErr := row.Scan(&subID, &status, &periodEnd, &graceEnd); scanErr != nil {
		err = fmt.Errorf("lookup subscription: %w", scanErr)
	}
	return
}

func (e *billingE2ECtx) countOutboxEvents(subID, eventType string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	row := e.db.QueryRowContext(ctx, `
        SELECT COUNT(*)
          FROM outbox_events
         WHERE aggregate_id = $1
           AND event_type   = $2
    `, subID, eventType)
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count outbox events: %w", err)
	}
	return count, nil
}

func (e *billingE2ECtx) loadOutboxPayload(subID, eventType string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var raw []byte
	row := e.db.QueryRowContext(ctx, `
        SELECT payload
          FROM outbox_events
         WHERE aggregate_id = $1
           AND event_type   = $2
         ORDER BY occurred_at DESC
         LIMIT 1
    `, subID, eventType)
	if err := row.Scan(&raw); err != nil {
		return nil, fmt.Errorf("load outbox payload: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal outbox payload: %w", err)
	}
	return result, nil
}

func (e *billingE2ECtx) loadOutboxAggregateType(subID, eventType string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var aggType string
	row := e.db.QueryRowContext(ctx, `
        SELECT aggregate_type
          FROM outbox_events
         WHERE aggregate_id = $1
           AND event_type   = $2
         ORDER BY occurred_at DESC
         LIMIT 1
    `, subID, eventType)
	if err := row.Scan(&aggType); err != nil {
		return "", fmt.Errorf("load aggregate_type: %w", err)
	}
	return aggType, nil
}

func parseBody(resp *http.Response) (map[string]any, string) {
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return result, strings.TrimSpace(string(data))
}
