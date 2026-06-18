//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	budgetsconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	txconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/consumers"
)

type agentE2ECtx struct {
	t                   *testing.T
	server              *httptest.Server
	db                  *sqlx.DB
	gateway             *CapturingGateway
	recomputeConsumer   *txconsumers.MonthlySummaryRecomputeConsumer
	budgetsConsumer     *budgetsconsumers.TransactionCreatedConsumer
	budgetsDelConsumer  *budgetsconsumers.TransactionDeletedConsumer
	budgetsCardConsumer *budgetsconsumers.CardPurchaseCreatedConsumer
	telegramGateway     *CapturingTelegramGateway
	secret              string
	telegramSecret      string
	waNumber            string
	waFrom              string
	telegramChatID      int64
	telegramUserID      int64
	userID              uuid.UUID
	secondID            uuid.UUID
	lastStatus          int
	lastWAMID           string
	lastTelegramUpdate  int64
	lastRefMonth        string
	beforeUser          int
	beforeOther         int
	beforeCardPurchases int
	budgetsConsumerHits int
	budgetsDeletedHits  int
	budgetsCardHits     int
}

func newAgentE2ECtx(
	t *testing.T,
	server *httptest.Server,
	db *sqlx.DB,
	gateway *CapturingGateway,
	recomputeConsumer *txconsumers.MonthlySummaryRecomputeConsumer,
	budgetsConsumer *budgetsconsumers.TransactionCreatedConsumer,
	budgetsDelConsumer *budgetsconsumers.TransactionDeletedConsumer,
	budgetsCardConsumer *budgetsconsumers.CardPurchaseCreatedConsumer,
	secret, waNumber, waFrom string,
	userID uuid.UUID,
) *agentE2ECtx {
	return &agentE2ECtx{
		t:                   t,
		server:              server,
		db:                  db,
		gateway:             gateway,
		recomputeConsumer:   recomputeConsumer,
		budgetsConsumer:     budgetsConsumer,
		budgetsDelConsumer:  budgetsDelConsumer,
		budgetsCardConsumer: budgetsCardConsumer,
		secret:              secret,
		waNumber:            waNumber,
		waFrom:              waFrom,
		userID:              userID,
	}
}

func (e *agentE2ECtx) postWebhook(text, wamid string) error {
	body := e.buildPayload(e.waFrom, text, wamid)
	req, err := http.NewRequest(http.MethodPost, e.server.URL+"/api/v1/whatsapp/inbound", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("criar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", e.hmacSignature(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("executar request: %w", err)
	}
	defer resp.Body.Close()

	e.lastStatus = resp.StatusCode
	e.lastWAMID = wamid
	return nil
}

func (e *agentE2ECtx) postWebhookInvalidSignature(text, wamid string) error {
	body := e.buildPayload(e.waFrom, text, wamid)
	req, err := http.NewRequest(http.MethodPost, e.server.URL+"/api/v1/whatsapp/inbound", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("criar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("executar request: %w", err)
	}
	defer resp.Body.Close()

	e.lastStatus = resp.StatusCode
	e.lastWAMID = wamid
	return nil
}

func (e *agentE2ECtx) hmacSignature(body []byte) string {
	mac := hmac.New(sha256.New, []byte(e.secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (e *agentE2ECtx) buildPayload(from, text, wamid string) []byte {
	type textBody struct {
		Body string `json:"body"`
	}
	type message struct {
		From      string   `json:"from"`
		ID        string   `json:"id"`
		Timestamp string   `json:"timestamp"`
		Type      string   `json:"type"`
		Text      textBody `json:"text"`
	}
	type metadata struct {
		DisplayPhoneNumber string `json:"display_phone_number"`
		PhoneNumberID      string `json:"phone_number_id"`
	}
	type changeValue struct {
		MessagingProduct string    `json:"messaging_product"`
		Metadata         metadata  `json:"metadata"`
		Messages         []message `json:"messages"`
	}
	type change struct {
		Field string      `json:"field"`
		Value changeValue `json:"value"`
	}
	type entry struct {
		ID      string   `json:"id"`
		Changes []change `json:"changes"`
	}
	type webhookPayload struct {
		Object string  `json:"object"`
		Entry  []entry `json:"entry"`
	}
	wp := webhookPayload{
		Object: "whatsapp_business_account",
		Entry: []entry{{
			ID: "test-entry",
			Changes: []change{{
				Field: "messages",
				Value: changeValue{
					MessagingProduct: "whatsapp",
					Metadata:         metadata{DisplayPhoneNumber: "15550000001", PhoneNumberID: "test-phone-id"},
					Messages: []message{{
						From:      from,
						ID:        wamid,
						Timestamp: strconv.FormatInt(time.Now().UTC().Unix(), 10),
						Type:      "text",
						Text:      textBody{Body: text},
					}},
				},
			}},
		}},
	}
	raw, _ := json.Marshal(wp)
	return raw
}

func (e *agentE2ECtx) countTransactions(userID uuid.UUID) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	err := e.db.QueryRowContext(
		ctx,
		"SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL",
		userID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("contar transacoes: %w", err)
	}
	return total, nil
}

func (e *agentE2ECtx) countCardPurchases(userID uuid.UUID) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	err := e.db.QueryRowContext(
		ctx,
		"SELECT count(*) FROM mecontrola.transactions_card_purchases WHERE user_id = $1 AND deleted_at IS NULL",
		userID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("contar compras parceladas: %w", err)
	}
	return total, nil
}

func (e *agentE2ECtx) latestCardPurchase(userID uuid.UUID) (int64, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		totalAmountCents  int64
		installmentsTotal int
	)
	err := e.db.QueryRowContext(
		ctx,
		`SELECT total_amount_cents, installments_total
		   FROM mecontrola.transactions_card_purchases
		  WHERE user_id = $1 AND deleted_at IS NULL
		  ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&totalAmountCents, &installmentsTotal)
	if err != nil {
		return 0, 0, fmt.Errorf("consultar compra parcelada: %w", err)
	}
	return totalAmountCents, installmentsTotal, nil
}

func (e *agentE2ECtx) countRecurringTemplates(userID uuid.UUID) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	err := e.db.QueryRowContext(
		ctx,
		"SELECT count(*) FROM mecontrola.transactions_recurring_templates WHERE user_id = $1 AND deleted_at IS NULL",
		userID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("contar recorrencias: %w", err)
	}
	return total, nil
}

func (e *agentE2ECtx) latestRecurringTemplate(userID uuid.UUID) (int64, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		amountCents int64
		dayOfMonth  int
	)
	err := e.db.QueryRowContext(
		ctx,
		`SELECT amount_cents, day_of_month
		   FROM mecontrola.transactions_recurring_templates
		  WHERE user_id = $1 AND deleted_at IS NULL
		  ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&amountCents, &dayOfMonth)
	if err != nil {
		return 0, 0, fmt.Errorf("consultar recorrencia: %w", err)
	}
	return amountCents, dayOfMonth, nil
}

func (e *agentE2ECtx) latestTransactionAmountAndVersion(userID uuid.UUID) (int64, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		amountCents int64
		version     int64
	)
	err := e.db.QueryRowContext(
		ctx,
		`SELECT amount_cents, version
		   FROM mecontrola.transactions
		  WHERE user_id = $1 AND deleted_at IS NULL
		  ORDER BY occurred_at DESC, created_at DESC LIMIT 1`,
		userID,
	).Scan(&amountCents, &version)
	if err != nil {
		return 0, 0, fmt.Errorf("consultar ultima transacao: %w", err)
	}
	return amountCents, version, nil
}

func (e *agentE2ECtx) latestTransactionDeletedAt(userID uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var deleted bool
	err := e.db.QueryRowContext(
		ctx,
		`SELECT deleted_at IS NOT NULL
		   FROM mecontrola.transactions
		  WHERE user_id = $1
		  ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&deleted)
	if err != nil {
		return false, fmt.Errorf("consultar soft-delete: %w", err)
	}
	return deleted, nil
}
