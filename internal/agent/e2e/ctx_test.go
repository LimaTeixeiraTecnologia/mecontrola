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
	beforeCards         int
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

func (e *agentE2ECtx) countCards(userID uuid.UUID) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	err := e.db.QueryRowContext(
		ctx,
		"SELECT count(*) FROM mecontrola.cards WHERE user_id = $1 AND deleted_at IS NULL",
		userID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("contar cartoes: %w", err)
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

func (e *agentE2ECtx) cardNicknameByName(userID uuid.UUID, name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var nickname string
	err := e.db.QueryRowContext(
		ctx,
		`SELECT nickname
		   FROM mecontrola.cards
		  WHERE user_id = $1 AND lower(name) = lower($2) AND deleted_at IS NULL
		  ORDER BY created_at DESC LIMIT 1`,
		userID, name,
	).Scan(&nickname)
	if err != nil {
		return "", fmt.Errorf("consultar apelido do cartao: %w", err)
	}
	return nickname, nil
}

func (e *agentE2ECtx) cardDueDayByNickname(userID uuid.UUID, nickname string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var dueDay int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT due_day
		   FROM mecontrola.cards
		  WHERE user_id = $1 AND lower(nickname) = lower($2) AND deleted_at IS NULL
		  ORDER BY created_at DESC LIMIT 1`,
		userID, nickname,
	).Scan(&dueDay)
	if err != nil {
		return 0, fmt.Errorf("consultar vencimento do cartao: %w", err)
	}
	return dueDay, nil
}

func (e *agentE2ECtx) cardExistsByNickname(userID uuid.UUID, nickname string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT count(*)
		   FROM mecontrola.cards
		  WHERE user_id = $1 AND lower(nickname) = lower($2) AND deleted_at IS NULL`,
		userID, nickname,
	).Scan(&total)
	if err != nil {
		return false, fmt.Errorf("consultar existencia do cartao: %w", err)
	}
	return total > 0, nil
}

func (e *agentE2ECtx) allocationBasisPoints(userID uuid.UUID, competence, rootSlug string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var basisPoints int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT a.basis_points
		   FROM mecontrola.budgets_allocations a
		   JOIN mecontrola.budgets b ON b.id = a.budget_id
		  WHERE b.user_id = $1 AND b.competence = $2 AND a.root_slug = $3`,
		userID, competence, rootSlug,
	).Scan(&basisPoints)
	if err != nil {
		return 0, fmt.Errorf("consultar alocacao: %w", err)
	}
	return basisPoints, nil
}

func (e *agentE2ECtx) allocationBasisPointsSum(userID uuid.UUID, competence string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT COALESCE(sum(a.basis_points), 0)
		   FROM mecontrola.budgets_allocations a
		   JOIN mecontrola.budgets b ON b.id = a.budget_id
		  WHERE b.user_id = $1 AND b.competence = $2`,
		userID, competence,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("somar alocacoes: %w", err)
	}
	return total, nil
}

func (e *agentE2ECtx) seedActiveBudget(userID uuid.UUID, competence string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	budgetID := uuid.New()
	const totalCents = int64(1000000)
	if _, err := e.db.ExecContext(
		ctx,
		`INSERT INTO mecontrola.budgets (id, user_id, competence, total_cents, state, activated_at, auto_draft, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 2, $5, false, $5, $5)
		 ON CONFLICT (user_id, competence) DO UPDATE
		    SET total_cents = EXCLUDED.total_cents,
		        state = 2,
		        activated_at = EXCLUDED.activated_at,
		        auto_draft = false,
		        updated_at = EXCLUDED.updated_at`,
		budgetID, userID, competence, totalCents, now,
	); err != nil {
		return fmt.Errorf("seed budget: %w", err)
	}

	var resolvedID uuid.UUID
	if err := e.db.QueryRowContext(
		ctx,
		`SELECT id FROM mecontrola.budgets WHERE user_id = $1 AND competence = $2`,
		userID, competence,
	).Scan(&resolvedID); err != nil {
		return fmt.Errorf("resolver budget: %w", err)
	}

	allocations := []struct {
		slug        string
		basisPoints int
	}{
		{"expense.custo_fixo", 4000},
		{"expense.conhecimento", 1000},
		{"expense.prazeres", 2000},
		{"expense.metas", 2000},
		{"expense.liberdade_financeira", 1000},
	}
	for _, alloc := range allocations {
		planned := totalCents * int64(alloc.basisPoints) / 10000
		if _, err := e.db.ExecContext(
			ctx,
			`INSERT INTO mecontrola.budgets_allocations (budget_id, root_slug, basis_points, planned_cents)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (budget_id, root_slug) DO UPDATE
			    SET basis_points = EXCLUDED.basis_points,
			        planned_cents = EXCLUDED.planned_cents`,
			resolvedID, alloc.slug, alloc.basisPoints, planned,
		); err != nil {
			return fmt.Errorf("seed allocation %s: %w", alloc.slug, err)
		}
	}
	return nil
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
