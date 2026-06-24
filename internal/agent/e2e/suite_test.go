//go:build e2e

package e2e_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	agentevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	budgetsconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	identityauth "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	identityuc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	tgdedup "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dedup/postgres"
	tgdispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dispatcher"
	tghandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/handlers"
	tgpayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/payload"
	tgsignature "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/signature"
	deduppostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/postgres"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wahandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
	wasignature "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
	txconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/consumers"
)

func TestE2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)

	o11y := noop.NewProvider()
	secret := "test-secret-e2e-2026"
	waNumber := "+5511988887777"
	waFrom := "5511988887777"
	telegramSecret := "test-telegram-secret-e2e-2026"
	telegramBotID := int64(987654321)
	telegramChatID := int64(555000111)
	telegramUserID := int64(555000111)

	cfg, err := configs.LoadConfig("../../..")
	if err != nil {
		t.Fatalf("carregar config: %v", err)
	}
	cfg.TransactionsConfig.Enabled = true

	ctx := context.Background()
	limiter := ratelimit.New(o11y)
	if startErr := limiter.Start(ctx); startErr != nil {
		t.Fatalf("iniciar limiter: %v", startErr)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(shutdownCtx)
	})

	userID := SeedActiveUserWA(t, db, waNumber)
	SeedTelegramIdentity(t, db, userID, telegramUserID)
	gateway := &CapturingGateway{}
	telegramGateway := &CapturingTelegramGateway{}

	router, downstream := buildServer(t, ctx, cfg, db, o11y, gateway, telegramGateway, limiter, secret, telegramSecret, telegramBotID, userID)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	suite := godog.TestSuite{
		Name: "agent-e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			sc.Before(func(c context.Context, _ *godog.Scenario) (context.Context, error) {
				gateway.Reset()
				telegramGateway.Reset()
				return c, nil
			})
			registerSteps(sc, newAgentE2ECtx(t, server, db, gateway, downstream.recompute, downstream.budgets, downstream.budgetsDeleted, downstream.budgetsCardPurchase, secret, waNumber, waFrom, userID).
				withTelegram(telegramGateway, telegramSecret, telegramChatID, telegramUserID))
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("cenarios e2e falharam")
	}
}

type e2eDownstream struct {
	recompute           *txconsumers.MonthlySummaryRecomputeConsumer
	budgets             *budgetsconsumers.TransactionCreatedConsumer
	budgetsDeleted      *budgetsconsumers.TransactionDeletedConsumer
	budgetsCardPurchase *budgetsconsumers.CardPurchaseCreatedConsumer
}

func buildServer(
	t *testing.T,
	ctx context.Context,
	cfg *configs.Config,
	db *sqlx.DB,
	o11y *noop.Provider,
	gateway *CapturingGateway,
	telegramGateway *CapturingTelegramGateway,
	limiter *ratelimit.Limiter,
	secret string,
	telegramSecret string,
	telegramBotID int64,
	userID uuid.UUID,
) (http.Handler, e2eDownstream) {
	t.Helper()
	authMW := func(h http.Handler) http.Handler { return h }

	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	cardModule, err := card.NewCardModule(ctx, cfg, o11y, db, authMW, nil, nil)
	if err != nil {
		t.Fatalf("card module: %v", err)
	}
	txModule, err := transactions.NewTransactionsModule(cfg, o11y, db, cardModule, catModule, authMW)
	if err != nil {
		t.Fatalf("transactions module: %v", err)
	}
	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, db, catModule, authMW, nil, nil)
	if err != nil {
		t.Fatalf("budgets module: %v", err)
	}

	logTx := usecases.NewRecordTransactionFromAgent(
		catModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(txModule.CreateTransactionUC),
		o11y,
	)
	expLogger := agentbinding.NewTransactionLoggerAdapter(logTx)

	logCardPurchase := usecases.NewRecordCardPurchaseFromAgent(
		catModule.SearchDictionaryUC,
		agentbinding.NewCardPurchaseCreatorAdapter(cardModule.ListCardsUC, txModule.CreateCardPurchaseUC),
		o11y,
	)
	createRecurring := usecases.NewCreateRecurringFromAgent(
		catModule.SearchDictionaryUC,
		agentbinding.NewRecurringTemplateCreatorAdapter(txModule.CreateRecurringTemplateUC),
		o11y,
	)

	authCtx := identityauth.WithPrincipal(ctx, identityauth.Principal{UserID: userID, Source: identityauth.SourceWhatsApp})
	seededCard, cardErr := cardModule.CreateCardUC.Execute(authCtx, cardinput.CreateCard{
		UserID:     userID,
		Name:       "Nubank",
		Nickname:   "nubank",
		ClosingDay: 10,
		DueDay:     intPtr(17),
		LimitCents: 500000,
	})
	if cardErr != nil {
		t.Fatalf("seed card: %v", cardErr)
	}
	if seededCard.ID == "" {
		t.Fatalf("seed card: id vazio")
	}

	expenseIntent, err := intent.NewRecordExpense(intent.RecordExpenseFields{AmountCents: 5000, Merchant: "mercado"})
	if err != nil {
		t.Fatalf("intent expense: %v", err)
	}
	incomeIntent, err := intent.NewRecordIncome(intent.RecordIncomeFields{AmountCents: 300000, Source: "salário"})
	if err != nil {
		t.Fatalf("intent income: %v", err)
	}
	unknownIntent, _ := intent.NewUnknown("ação não suportada")

	cardPurchaseIntent, err := intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{
		AmountCents:  120000,
		Merchant:     "supermercado",
		CardHint:     "nubank",
		Installments: 6,
	})
	if err != nil {
		t.Fatalf("intent card purchase: %v", err)
	}
	recurringIntent, err := intent.NewCreateRecurring(intent.CreateRecurringFields{
		AmountCents: 500000,
		Merchant:    "salário",
		Direction:   "income",
		Frequency:   "monthly",
		DayOfMonth:  5,
	})
	if err != nil {
		t.Fatalf("intent recurring: %v", err)
	}
	editIntent, err := intent.NewEditLastTransaction(8000)
	if err != nil {
		t.Fatalf("intent edit: %v", err)
	}
	deleteIntent := intent.NewDeleteLastTransaction()

	listTransactionsIntent, err := intent.NewListTransactions("")
	if err != nil {
		t.Fatalf("intent list transactions: %v", err)
	}
	monthlySummaryIntent, err := intent.NewMonthlySummary("")
	if err != nil {
		t.Fatalf("intent monthly summary: %v", err)
	}
	queryCategoryIntent, err := intent.NewQueryCategory("prazeres")
	if err != nil {
		t.Fatalf("intent query category: %v", err)
	}
	queryGoalIntent, err := intent.NewQueryGoal("reserva")
	if err != nil {
		t.Fatalf("intent query goal: %v", err)
	}
	queryCardIntent, err := intent.NewQueryCard("nubank")
	if err != nil {
		t.Fatalf("intent query card: %v", err)
	}
	howAmIDoingIntent := intent.NewHowAmIDoing()
	listRecurringIntent := intent.NewListRecurring()
	listCardsIntent := intent.NewListCards()

	cardNotFoundIntent, err := intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{
		AmountCents:  30000,
		Merchant:     "loja",
		CardHint:     "fantasma",
		Installments: 3,
	})
	if err != nil {
		t.Fatalf("intent card purchase not found: %v", err)
	}

	createCardIntent, err := intent.NewCreateCard(intent.CreateCardFields{
		Nickname:   "inter",
		Name:       "Inter Black",
		ClosingDay: 5,
		DueDay:     12,
		LimitCents: 800000,
	})
	if err != nil {
		t.Fatalf("intent create card: %v", err)
	}
	createCardC6Intent, err := intent.NewCreateCard(intent.CreateCardFields{
		Nickname:   "c6",
		Name:       "C6 Carbon",
		ClosingDay: 8,
		DueDay:     15,
		LimitCents: 600000,
	})
	if err != nil {
		t.Fatalf("intent create card c6: %v", err)
	}
	createCardWillIntent, err := intent.NewCreateCard(intent.CreateCardFields{
		Nickname:   "will",
		Name:       "Will Bank",
		ClosingDay: 3,
		DueDay:     10,
		LimitCents: 400000,
	})
	if err != nil {
		t.Fatalf("intent create card will: %v", err)
	}
	countCardsIntent := intent.NewCountCards()

	updateCardNicknameIntent, err := intent.NewUpdateCard(intent.UpdateCardFields{
		CardName: "nubank",
		Nickname: strPtr("roxinho"),
	})
	if err != nil {
		t.Fatalf("intent update card nickname: %v", err)
	}
	updateCardDueDayIntent, err := intent.NewUpdateCard(intent.UpdateCardFields{
		CardName: "roxinho",
		DueDay:   intPtr(25),
	})
	if err != nil {
		t.Fatalf("intent update card due day: %v", err)
	}
	updateCardNotFoundIntent, err := intent.NewUpdateCard(intent.UpdateCardFields{
		CardName: "fantasma",
		Nickname: strPtr("qualquer"),
	})
	if err != nil {
		t.Fatalf("intent update card not found: %v", err)
	}
	deleteCardIntent, err := intent.NewDeleteCard("roxinho")
	if err != nil {
		t.Fatalf("intent delete card: %v", err)
	}
	deleteCardNotFoundIntent, err := intent.NewDeleteCard("fantasma")
	if err != nil {
		t.Fatalf("intent delete card not found: %v", err)
	}
	editCategoryPercentageIntent, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{
		CategoryName: "prazeres",
		Percentage:   30,
	})
	if err != nil {
		t.Fatalf("intent edit category percentage: %v", err)
	}
	editCategoryUnknownIntent, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{
		CategoryName: "viagens",
		Percentage:   20,
	})
	if err != nil {
		t.Fatalf("intent edit category unknown: %v", err)
	}

	stubP := NewStubParser(map[string]intent.Intent{
		"gastei 50 no mercado":                            expenseIntent,
		"recebi salário de 3000":                          incomeIntent,
		"ação não suportada":                              unknownIntent,
		"parcelei 1200 em 6x no nubank":                   cardPurchaseIntent,
		"todo mês recebo 5000 no dia 5":                   recurringIntent,
		"na verdade foi 80":                               editIntent,
		"apaga o último":                                  deleteIntent,
		"lista meus lançamentos":                          listTransactionsIntent,
		"resumo do mês":                                   monthlySummaryIntent,
		"quanto gastei em prazeres?":                      queryCategoryIntent,
		"como está minha meta?":                           queryGoalIntent,
		"qual a fatura do nubank?":                        queryCardIntent,
		"como estou indo?":                                howAmIDoingIntent,
		"quais minhas recorrências?":                      listRecurringIntent,
		"meus cartões":                                    listCardsIntent,
		"comprei 300 no cartão fantasma":                  cardNotFoundIntent,
		"cadastra meu cartão inter":                       createCardIntent,
		"cadastra meu cartão c6":                          createCardC6Intent,
		"cadastra meu cartão will":                        createCardWillIntent,
		"quantos cartões eu tenho?":                       countCardsIntent,
		"muda o apelido do cartão nubank pra roxinho":     updateCardNicknameIntent,
		"troca o vencimento do cartão roxinho pro dia 25": updateCardDueDayIntent,
		"muda o apelido do cartão fantasma pra qualquer":  updateCardNotFoundIntent,
		"apaga o cartão roxinho":                          deleteCardIntent,
		"apaga o cartão fantasma":                         deleteCardNotFoundIntent,
		"coloca 30% em prazeres":                          editCategoryPercentageIntent,
		"coloca 20% em viagens":                           editCategoryUnknownIntent,
	}, nil)

	publisher := outbox.NewPostgresPublisher(
		outbox.NewPostgresStorage(db),
		configs.OutboxConfig{RetryMaxAttempts: 3},
	)

	intentRouter, err := appservices.NewIntentRouter(o11y, appservices.IntentRouterDeps{
		Parser:                   stubP,
		Fallback:                 &StubFallback{},
		WhatsAppGateway:          gateway,
		TelegramGateway:          telegramGateway,
		ExpenseRecorder:          expLogger,
		CardPurchaseLog:          agentbinding.NewCardPurchaseLoggerAdapter(logCardPurchase),
		RecurringCreator:         agentbinding.NewRecurringCreatorAdapter(createRecurring),
		TransactionLister:        agentbinding.NewTransactionListerAdapter(txModule.ListTransactionsUC),
		LastEditor:               agentbinding.NewLastTransactionEditorAdapter(txModule.GetTransactionUC, txModule.UpdateTransactionUC),
		LastDeleter:              agentbinding.NewLastTransactionDeleterAdapter(txModule.DeleteTransactionUC),
		MonthlySummary:           budgetsModule.GetMonthlySummaryUC,
		CardLister:               cardModule.ListCardsUC,
		CardInvoice:              cardModule.InvoiceForUC,
		CardCreator:              agentbinding.NewCardCreatorAdapter(cardModule.CreateCardUC),
		CardCounter:              agentbinding.NewCardCounterAdapter(cardModule.CountCardsUC),
		CardUpdater:              agentbinding.NewCardUpdaterAdapter(cardModule.ListCardsUC, cardModule.UpdateCardUC),
		CardDeleter:              agentbinding.NewCardDeleterAdapter(cardModule.ListCardsUC, cardModule.SoftDeleteCardUC),
		CategoryPercentageEditor: agentbinding.NewCategoryPercentageEditorAdapter(budgetsModule.EditCategoryPercentageUC),
		RecurringLister:          agentbinding.NewRecurringListerAdapter(txModule.ListRecurringTemplatesUC),
		EventPublisher:           agentevents.NewIntentEventPublisher(publisher, o11y),
		Location:                 time.UTC,
	})
	if err != nil {
		t.Fatalf("intent router: %v", err)
	}

	agentRoute := func(c context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
		principal, ok := identityauth.FromContext(c)
		if !ok {
			return wadispatcher.OutcomeAgent
		}
		_ = intentRouter.RouteWhatsApp(c, appservices.Principal{UserID: principal.UserID}, appservices.InboundMessage{
			Text:       msg.Text,
			WhatsAppTo: msg.From,
			MessageID:  msg.WAMID,
		})
		return wadispatcher.OutcomeAgent
	}

	onboardingRoute := func(_ context.Context, _ wapayload.Message) wadispatcher.RouteOutcome {
		return wadispatcher.OutcomeOnboarding
	}

	factory := identityrepos.NewRepositoryFactory(o11y)
	establishUoW := uow.NewUnitOfWork(db)
	establishUC := identityuc.NewEstablishPrincipal(establishUoW, factory, publisher, o11y)
	dedupRepo := deduppostgres.NewMessageRepository(o11y, db)

	disp := wadispatcher.New(
		dedupRepo,
		establishUC,
		limiter,
		publisher,
		onboardingRoute,
		agentRoute,
		o11y,
	)

	verifyHandler := wahandlers.NewVerifyHandler(cfg.WhatsAppConfig.VerifyToken)
	inboundHandler := wahandlers.NewInboundHandler(disp, o11y)

	tgResolveUoW := uow.NewUnitOfWork(db)
	tgResolveUC := identityuc.NewResolvePrincipalByIdentity(tgResolveUoW, factory, o11y)
	tgDedupRepo := tgdedup.NewUpdateRepository(o11y, db)

	tgAgentRoute := func(c context.Context, msg tgpayload.Message) tgdispatcher.RouteOutcome {
		principal, ok := identityauth.FromContext(c)
		if !ok {
			return tgdispatcher.OutcomeAgent
		}
		_ = intentRouter.RouteTelegram(c, appservices.Principal{UserID: principal.UserID}, appservices.InboundMessage{
			Text:       msg.Text,
			TelegramTo: msg.ChatID,
			MessageID:  strconv.FormatInt(msg.MessageID, 10),
		})
		return tgdispatcher.OutcomeAgent
	}
	tgOnboardingRoute := func(_ context.Context, _ tgpayload.Message) tgdispatcher.RouteOutcome {
		return tgdispatcher.OutcomeFallback
	}

	tgDisp := tgdispatcher.New(
		telegramBotID,
		tgDedupRepo,
		tgResolveUC,
		limiter,
		publisher,
		tgOnboardingRoute,
		tgAgentRoute,
		o11y,
	)
	tgInboundHandler := tghandlers.NewInboundHandler(tgDisp, o11y)

	r := chi.NewRouter()
	r.Route("/api/v1/whatsapp", func(sub chi.Router) {
		sub.Get("/verify", verifyHandler.Handle)
		sub.With(wasignature.Compose(secret, "", nil)).Post("/inbound", inboundHandler.Handle)
	})
	r.With(tgsignature.SecretToken(telegramSecret, "")).
		Post("/api/v1/channels/telegram/webhook", tgInboundHandler.Handle)
	budgetsCardPurchase := budgetsconsumers.NewCardPurchaseCreatedConsumer(budgetsModule.UpsertExpenseUC, o11y)

	return r, e2eDownstream{
		recompute:           txModule.MonthlySummaryRecomputeConsumer,
		budgets:             budgetsModule.TransactionCreatedConsumer,
		budgetsDeleted:      budgetsModule.TransactionDeletedConsumer,
		budgetsCardPurchase: budgetsCardPurchase,
	}
}
