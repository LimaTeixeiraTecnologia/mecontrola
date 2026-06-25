//go:build e2e && integration

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

func TestJourney_RealConversation_PersistsAcrossTables_E2E(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para a conversa real")
	}

	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	cfg, err := configs.LoadConfig("../../..")
	require.NoError(t, err)
	cfg.TransactionsConfig.Enabled = true

	authMW := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	cardModule, err := card.NewCardModule(ctx, cfg, o11y, db, authMW, nil, nil)
	require.NoError(t, err)
	txModule, err := transactions.NewTransactionsModule(cfg, o11y, db, cardModule, catModule, authMW)
	require.NoError(t, err)

	logTx := usecases.NewRecordTransactionFromAgent(
		catModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(txModule.CreateTransactionUC),
		o11y,
	)
	logCardPurchase := usecases.NewRecordCardPurchaseFromAgent(
		catModule.SearchDictionaryUC,
		agentbinding.NewCardPurchaseCreatorAdapter(cardModule.ListCardsUC, txModule.CreateCardPurchaseUC),
		o11y,
	)
	expLogger := agentbinding.NewTransactionLoggerAdapter(logTx)

	transactionLister := agentbinding.NewTransactionListerAdapter(txModule.ListTransactionsUC)
	transactionSearcher := agentbinding.NewTransactionSearcherAdapter(txModule.SearchTransactionsUC)
	lastEditor := agentbinding.NewLastTransactionEditorAdapter(txModule.GetTransactionUC, txModule.UpdateTransactionUC)
	lastDeleter := agentbinding.NewLastTransactionDeleterAdapter(txModule.DeleteTransactionUC)
	cardDeleter := agentbinding.NewCardDeleterAdapter(cardModule.ListCardsUC, cardModule.SoftDeleteCardUC)
	kernelDeps, _, err := buildConfirmKernelDeps(
		o11y, db, cfg,
		transactionLister, transactionSearcher, lastEditor, lastDeleter,
		cardModule.ListCardsUC, cardDeleter,
		agentbinding.NewKernelCategoryResolver(catModule.SearchDictionaryUC),
		agentbinding.NewKernelPersistFunc(expLogger, agentbinding.NewCardPurchaseLoggerAdapter(logCardPurchase)),
	)
	require.NoError(t, err)

	gateway := &CapturingGateway{}
	router, err := appservices.NewIntentRouter(o11y, appservices.IntentRouterDeps{
		Parser:              &parserAdapter{uc: realParserRobust(t)},
		ExpenseRecorder:     expLogger,
		TransactionLister:   transactionLister,
		TransactionSearcher: transactionSearcher,
		Fallback:            &StubFallback{},
		WhatsAppGateway:     gateway,
		Location:            time.UTC,
		Kernel:              kernelDeps,
	})
	require.NoError(t, err)

	waNumber := "+5511955554444"
	userID := SeedActiveUserWA(t, db, waNumber)
	principal := appservices.Principal{UserID: userID}

	turns := []string{
		"gastei 58 no ifood",
		"recebi meu salário de 4000",
		"meus lançamentos",
	}

	t.Log("==================== CONVERSA REAL (WhatsApp -> agent -> OpenRouter) ====================")
	for i, text := range turns {
		wamid := fmt.Sprintf("wamid.journey.%d", i+1)
		router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{Text: text, WhatsAppTo: waNumber, MessageID: wamid})
		last, ok := gateway.LastReply()
		require.True(t, ok, "turno %d sem resposta", i+1)
		t.Logf("\n👤 USUÁRIO: %s\n🤖 AGENTE: %s", text, last.Text)
	}

	t.Log("\n==================== EVIDÊNCIA DE PERSISTÊNCIA (DDL real, schema mecontrola) ====================")

	dumpRows(t, db, "(T) transactions",
		`SELECT direction, payment_method, amount_cents, description, category_name_snapshot, ref_month, version
		   FROM mecontrola.transactions WHERE user_id=$1 AND deleted_at IS NULL ORDER BY created_at`, userID)

	dumpRows(t, db, "(K) workflow_runs (kernel de escrita)",
		`SELECT workflow, status, suspend_reason, cursor FROM mecontrola.workflow_runs ORDER BY created_at`)

	dumpRows(t, db, "(K) workflow_steps",
		`SELECT s.step_id, s.status, s.seq FROM mecontrola.workflow_steps s ORDER BY s.started_at`)

	dumpRows(t, db, "(P) outbox_events",
		`SELECT event_type, status FROM mecontrola.outbox_events WHERE aggregate_user_id=$1 ORDER BY occurred_at`, userID)

	var txCount, expense, income int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*),
		        count(*) FILTER (WHERE direction=2),
		        count(*) FILTER (WHERE direction=1)
		   FROM mecontrola.transactions WHERE user_id=$1 AND deleted_at IS NULL`, userID).Scan(&txCount, &expense, &income))
	require.Equal(t, 2, txCount, "esperado 2 transações persistidas (despesa + receita)")
	require.Equal(t, 1, expense, "1 despesa")
	require.Equal(t, 1, income, "1 receita")

	var events int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*) FROM mecontrola.outbox_events WHERE aggregate_user_id=$1 AND event_type='transactions.transaction.created.v1'`,
		userID).Scan(&events))
	require.Equal(t, 2, events, "2 eventos transaction.created publicados no outbox")

	t.Log("\n✅ Conversa real persistiu em transactions (T) + workflow_runs/steps (K) + outbox_events (P) [agent_decisions exige o runtime completo server/worker]")
}

func dumpRows(t *testing.T, db *sqlx.DB, label, query string, args ...any) {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	require.NoError(t, err)
	t.Logf("\n--- %s ---\n%v", label, cols)
	n := 0
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		require.NoError(t, rows.Scan(ptrs...))
		rendered := make([]string, len(vals))
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				rendered[i] = string(b)
			} else {
				rendered[i] = fmt.Sprintf("%v", v)
			}
		}
		t.Logf("  %v", rendered)
		n++
	}
	require.NoError(t, rows.Err())
	t.Logf("  (%d linha(s))", n)
}
