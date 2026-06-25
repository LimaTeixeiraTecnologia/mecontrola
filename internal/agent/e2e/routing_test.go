//go:build integration

package e2e_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

func TestAgentRouter_RealLLM_PersistsTransactions_Integration(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a prova real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	mgr, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	cfg, err := configs.LoadConfig("../../..")
	require.NoError(t, err)

	authMW := func(h http.Handler) http.Handler { return h }
	categoriesModule := categories.NewCategoriesModule(mgr, o11y, authMW)
	cardModule, err := card.NewCardModule(ctx, cfg, o11y, mgr, authMW, nil, nil)
	require.NoError(t, err)
	txModule, err := transactions.NewTransactionsModule(cfg, o11y, mgr, cardModule, categoriesModule, authMW)
	require.NoError(t, err)

	logTx := usecases.NewRecordTransactionFromAgent(
		categoriesModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(txModule.CreateTransactionUC),
		o11y,
	)

	gateway := &CapturingGateway{}
	router, err := appservices.NewIntentRouter(o11y, appservices.IntentRouterDeps{
		Parser:          &parserAdapter{uc: realParser(t)},
		ExpenseRecorder: agentbinding.NewTransactionLoggerAdapter(logTx),
		Fallback:        &StubFallback{},
		WhatsAppGateway: gateway,
		Location:        time.UTC,
	})
	require.NoError(t, err)

	userID := uuid.New()
	principal := appservices.Principal{UserID: userID}

	expense := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "ifood 58 reais", WhatsAppTo: "+5511900000099", MessageID: "wamid.router.exp.1",
	})
	require.Equal(t, tools.OutcomeRouted, expense.Outcome, "gasto deve ser roteado e persistido")

	income := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "recebi meu salário de 16.400", WhatsAppTo: "+5511900000099", MessageID: "wamid.router.inc.1",
	})
	require.Equal(t, tools.OutcomeRouted, income.Outcome, "salário deve ser roteado e persistido")

	all := gateway.All()
	require.GreaterOrEqual(t, len(all), 2)
	require.Contains(t, all[0].Text, "Transação realizada", "gasto deve confirmar persistência")
	require.Contains(t, all[1].Text, "Recebimento registrado", "salário deve confirmar persistência")

	db := mgr
	var total int
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1", userID,
	).Scan(&total))
	require.Equal(t, 2, total, "router real deve ter persistido 2 lançamentos (gasto + salário)")

	var sumCents int64
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT coalesce(sum(amount_cents),0) FROM mecontrola.transactions WHERE user_id = $1", userID,
	).Scan(&sumCents))
	require.Equal(t, int64(5800+1640000), sumCents, "soma persistida deve bater")

	t.Logf("[router real] persistiu 2 lançamentos, soma=%d centavos; replies=%v", sumCents, all)
}

func TestAgentRouter_NewCapabilities_Integration(t *testing.T) {
	mgr, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	cfg, err := configs.LoadConfig("../../..")
	require.NoError(t, err)

	authMW := func(h http.Handler) http.Handler { return h }
	categoriesModule := categories.NewCategoriesModule(mgr, o11y, authMW)
	cardModule, err := card.NewCardModule(ctx, cfg, o11y, mgr, authMW, nil, nil)
	require.NoError(t, err)
	txModule, err := transactions.NewTransactionsModule(cfg, o11y, mgr, cardModule, categoriesModule, authMW)
	require.NoError(t, err)

	userID := uuid.New()
	principal := appservices.Principal{UserID: userID}
	authCtx := auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})

	seedDB := mgr
	_, err = seedDB.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+5511900000099",
	)
	require.NoError(t, err)

	seededCard, err := cardModule.CreateCardUC.Execute(authCtx, cardinput.CreateCard{
		UserID:     userID,
		Name:       "Nubank",
		Nickname:   "nubank",
		ClosingDay: 10,
		DueDay:     intPtr(17),
		LimitCents: 500000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, seededCard.ID)

	logCardPurchase := usecases.NewRecordCardPurchaseFromAgent(
		categoriesModule.SearchDictionaryUC,
		agentbinding.NewCardPurchaseCreatorAdapter(cardModule.ListCardsUC, txModule.CreateCardPurchaseUC),
		o11y,
	)
	createRecurring := usecases.NewCreateRecurringFromAgent(
		categoriesModule.SearchDictionaryUC,
		agentbinding.NewRecurringTemplateCreatorAdapter(txModule.CreateRecurringTemplateUC),
		o11y,
	)
	logTx := usecases.NewRecordTransactionFromAgent(
		categoriesModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(txModule.CreateTransactionUC),
		o11y,
	)

	cardPurchaseIntent, err := intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{AmountCents: 120000, Merchant: "supermercado", CardHint: "nubank", Installments: 6})
	require.NoError(t, err)
	recurringIntent, err := intent.NewCreateRecurring(intent.CreateRecurringFields{AmountCents: 500000, Merchant: "salário", Direction: "income", Frequency: "monthly", DayOfMonth: 5})
	require.NoError(t, err)
	expenseIntent, err := intent.NewRecordExpense(intent.RecordExpenseFields{AmountCents: 5800, Merchant: "mercado"})
	require.NoError(t, err)
	listTxIntent, err := intent.NewListTransactions("")
	require.NoError(t, err)
	editIntent, err := intent.NewEditLastTransaction(8000)
	require.NoError(t, err)
	deleteIntent := intent.NewDeleteLastTransaction()
	listRecurringIntent := intent.NewListRecurring()

	stubParser := NewStubParser(map[string]intent.Intent{
		"comprei 1200 em 6x no nubank":    cardPurchaseIntent,
		"todo mês recebo 5000 de salário": recurringIntent,
		"gastei 58 no ifood":              expenseIntent,
		"mostra minhas transações":        listTxIntent,
		"na verdade foram 80 reais":       editIntent,
		"apaga o último lançamento":       deleteIntent,
		"quais minhas recorrências?":      listRecurringIntent,
	}, nil)

	lister := agentbinding.NewTransactionListerAdapter(txModule.ListTransactionsUC)
	searcher := agentbinding.NewTransactionSearcherAdapter(txModule.SearchTransactionsUC)
	lastEditor := agentbinding.NewLastTransactionEditorAdapter(txModule.GetTransactionUC, txModule.UpdateTransactionUC)
	lastDeleter := agentbinding.NewLastTransactionDeleterAdapter(txModule.DeleteTransactionUC)
	cardLister := cardModule.ListCardsUC
	cardDeleter := agentbinding.NewCardDeleterAdapter(cardModule.ListCardsUC, cardModule.SoftDeleteCardUC)

	kernelCategoryResolver := agentbinding.NewKernelCategoryResolver(categoriesModule.SearchDictionaryUC)
	kernelPersistFn := agentbinding.NewKernelPersistFunc(
		agentbinding.NewTransactionLoggerAdapter(logTx),
		agentbinding.NewCardPurchaseLoggerAdapter(logCardPurchase),
	)
	kernelDeps, _, err := buildConfirmKernelDeps(o11y, mgr, cfg, lister, searcher, lastEditor, lastDeleter, cardLister, cardDeleter, kernelCategoryResolver, kernelPersistFn)
	require.NoError(t, err)

	gateway := &CapturingGateway{}
	router, err := appservices.NewIntentRouter(o11y, appservices.IntentRouterDeps{
		Parser:            stubParser,
		ExpenseRecorder:   agentbinding.NewTransactionLoggerAdapter(logTx),
		CardPurchaseLog:   agentbinding.NewCardPurchaseLoggerAdapter(logCardPurchase),
		TransactionLister: lister,
		LastDeleter:       lastDeleter,
		LastEditor:        lastEditor,
		RecurringCreator:  agentbinding.NewRecurringCreatorAdapter(createRecurring),
		RecurringLister:   agentbinding.NewRecurringListerAdapter(txModule.ListRecurringTemplatesUC),
		Fallback:          &StubFallback{},
		WhatsAppGateway:   gateway,
		Location:          time.UTC,
		Kernel:            kernelDeps,
	})
	require.NoError(t, err)

	db := mgr

	cardPurchase := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "comprei 1200 em 6x no nubank", WhatsAppTo: "+5511900000099", MessageID: "wamid.cp.1",
	})
	require.Equal(t, tools.OutcomeRouted, cardPurchase.Outcome, "compra parcelada deve persistir; reply=%s", cardPurchase.Reply)
	var cpCount int
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT count(*) FROM mecontrola.transactions_card_purchases WHERE user_id = $1", userID,
	).Scan(&cpCount))
	require.Equal(t, 1, cpCount, "deve haver 1 compra parcelada persistida")

	recurring := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "todo mês recebo 5000 de salário", WhatsAppTo: "+5511900000099", MessageID: "wamid.rt.1",
	})
	require.Equal(t, tools.OutcomeRouted, recurring.Outcome, "recorrência deve persistir; reply=%s", recurring.Reply)
	var rtCount int
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT count(*) FROM mecontrola.transactions_recurring_templates WHERE user_id = $1", userID,
	).Scan(&rtCount))
	require.Equal(t, 1, rtCount, "deve haver 1 template recorrente persistido")

	expense := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "gastei 58 no ifood", WhatsAppTo: "+5511900000099", MessageID: "wamid.exp.2",
	})
	require.Equal(t, tools.OutcomeRouted, expense.Outcome, "gasto deve persistir; reply=%s", expense.Reply)

	listed := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "mostra minhas transações", WhatsAppTo: "+5511900000099", MessageID: "wamid.list.1",
	})
	require.Equal(t, tools.OutcomeRouted, listed.Outcome, "listagem deve funcionar; reply=%s", listed.Reply)
	require.Contains(t, listed.Reply, "Lançamentos")

	edited := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "na verdade foram 80 reais", WhatsAppTo: "+5511900000099", MessageID: "wamid.edit.1",
	})
	require.Equal(t, tools.OutcomeClarify, edited.Outcome, "edição deve pedir confirmação; reply=%s", edited.Reply)
	require.Contains(t, edited.Reply, "atualizar")

	editConfirm := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "sim", WhatsAppTo: "+5511900000099", MessageID: "wamid.edit.2",
	})
	require.Equal(t, tools.OutcomeRouted, editConfirm.Outcome, "confirmação de edição deve executar; reply=%s", editConfirm.Reply)
	require.Contains(t, editConfirm.Reply, "atualizado")
	var editedCents int64
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT amount_cents FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL ORDER BY occurred_at DESC, created_at DESC LIMIT 1", userID,
	).Scan(&editedCents))
	require.Equal(t, int64(8000), editedCents, "último lançamento deve ter sido atualizado para 80,00")

	deleted := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "apaga o último lançamento", WhatsAppTo: "+5511900000099", MessageID: "wamid.del.1",
	})
	require.Equal(t, tools.OutcomeClarify, deleted.Outcome, "exclusão deve pedir confirmação; reply=%s", deleted.Reply)
	require.Contains(t, deleted.Reply, "apagar")

	deleteConfirm := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "sim", WhatsAppTo: "+5511900000099", MessageID: "wamid.del.2",
	})
	require.Equal(t, tools.OutcomeRouted, deleteConfirm.Outcome, "confirmação de exclusão deve executar; reply=%s", deleteConfirm.Reply)
	require.Contains(t, deleteConfirm.Reply, "apagado")
	var activeCount int
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL", userID,
	).Scan(&activeCount))
	require.Equal(t, 0, activeCount, "o único lançamento simples deve ter sido soft-deletado")

	listedRecurring := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text: "quais minhas recorrências?", WhatsAppTo: "+5511900000099", MessageID: "wamid.listrt.1",
	})
	require.Equal(t, tools.OutcomeRouted, listedRecurring.Outcome, "listagem de recorrências deve funcionar; reply=%s", listedRecurring.Reply)
	require.Contains(t, listedRecurring.Reply, "Recorrências")

	t.Logf("[router real new] card_purchase=%d recurring=%d; replies=%v", cpCount, rtCount, gateway.All())
}
