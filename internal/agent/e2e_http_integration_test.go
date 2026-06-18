//go:build integration

package agent

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/testsupport"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	budgetuc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	carduc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	catuc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	identityauth "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	identityuc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	deduppostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/postgres"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wahandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
	wasignature "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

type E2EHTTPSuite struct {
	suite.Suite
	ctx      context.Context
	mgr      *sqlx.DB
	o11y     *noop.Provider
	cfg      *configs.Config
	gateway  *testsupport.CapturingGateway
	server   *httptest.Server
	limiter  *ratelimit.Limiter
	secret   string
	waNumber string
	waFrom   string
	userID   uuid.UUID
}

func TestE2EHTTPSuite(t *testing.T) {
	suite.Run(t, new(E2EHTTPSuite))
}

func (s *E2EHTTPSuite) SetupSuite() {
	mgr, _ := testcontainer.Postgres(s.T())
	s.mgr = mgr
	s.o11y = noop.NewProvider()
	s.ctx = context.Background()
	s.secret = "test-secret-e2e-2026"
	s.waNumber = "+5511988887777"
	s.waFrom = "5511988887777"

	cfg, err := configs.LoadConfig("../..")
	s.Require().NoError(err)
	cfg.TransactionsConfig.Enabled = true
	s.cfg = cfg

	s.userID = testsupport.SeedActiveUserWA(s.T(), mgr, s.waNumber)
	s.gateway = &testsupport.CapturingGateway{}

	s.limiter = ratelimit.New(s.o11y)
	s.Require().NoError(s.limiter.Start(s.ctx))

	r := s.buildRouter()
	s.server = httptest.NewServer(r)
}

func (s *E2EHTTPSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
	if s.limiter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = s.limiter.Shutdown(ctx)
	}
}

func (s *E2EHTTPSuite) SetupTest() {
	s.gateway.Reset()
}

func (s *E2EHTTPSuite) buildRouter() http.Handler {
	authMW := func(h http.Handler) http.Handler { return h }

	catModule := categories.NewCategoriesModule(s.mgr, s.o11y, authMW)
	cardModule, err := card.NewCardModule(s.ctx, s.cfg, s.o11y, s.mgr, authMW, nil, nil)
	s.Require().NoError(err)
	txModule, err := transactions.NewTransactionsModule(s.cfg, s.o11y, s.mgr, cardModule, catModule, authMW)
	s.Require().NoError(err)

	logTx := usecases.NewLogTransactionFromAgent(
		catModule.SearchDictionaryUC,
		newTransactionCreatorAdapter(txModule.CreateTransactionUC),
		s.o11y,
	)
	expLogger := &transactionLoggerAdapter{uc: logTx}

	expenseIntent, err := intent.NewLogExpense(intent.LogExpenseFields{AmountCents: 5000, Merchant: "mercado"})
	s.Require().NoError(err)
	incomeIntent, err := intent.NewLogIncome(intent.LogIncomeFields{AmountCents: 300000, Source: "salário"})
	s.Require().NoError(err)
	unknownIntent, _ := intent.NewUnknown("ação não suportada")

	stubP := testsupport.NewStubParser(map[string]intent.Intent{
		"gastei 50 no mercado":   expenseIntent,
		"recebi salário de 3000": incomeIntent,
		"ação não suportada":     unknownIntent,
	}, nil)

	intentRouter, err := appservices.NewIntentRouter(s.o11y, appservices.IntentRouterDeps{
		Parser:          stubP,
		Fallback:        &testsupport.StubFallback{},
		WhatsAppGateway: s.gateway,
		ExpenseLogger:   expLogger,
	})
	s.Require().NoError(err)

	agentRoute := func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
		principal, ok := identityauth.FromContext(ctx)
		if !ok {
			return wadispatcher.OutcomeAgent
		}
		_ = intentRouter.RouteWhatsApp(ctx, appservices.Principal{UserID: principal.UserID}, appservices.InboundMessage{
			Text:       msg.Text,
			WhatsAppTo: msg.From,
			MessageID:  msg.WAMID,
		})
		return wadispatcher.OutcomeAgent
	}

	onboardingRoute := func(_ context.Context, _ wapayload.Message) wadispatcher.RouteOutcome {
		return wadispatcher.OutcomeOnboarding
	}

	factory := identityrepos.NewRepositoryFactory(s.o11y)
	establishUoW := uow.NewUnitOfWork(s.mgr)
	publisher := outbox.NewPostgresPublisher(
		outbox.NewPostgresStorage(s.mgr),
		configs.OutboxConfig{RetryMaxAttempts: 3},
	)
	establishUC := identityuc.NewEstablishPrincipal(establishUoW, factory, publisher, s.o11y)
	dedupRepo := deduppostgres.NewMessageRepository(s.o11y, s.mgr)

	disp := wadispatcher.New(
		dedupRepo,
		establishUC,
		s.limiter,
		publisher,
		onboardingRoute,
		agentRoute,
		s.o11y,
	)

	verifyHandler := wahandlers.NewVerifyHandler(s.cfg.WhatsAppConfig.VerifyToken)
	inboundHandler := wahandlers.NewInboundHandler(disp, s.o11y)

	r := chi.NewRouter()
	r.Route("/api/v1/whatsapp", func(sub chi.Router) {
		sub.Get("/verify", verifyHandler.Handle)
		sub.With(wasignature.Compose(s.secret, "", nil)).Post("/inbound", inboundHandler.Handle)
	})
	return r
}

func (s *E2EHTTPSuite) TestI1_CreateExpense_PersistsRow() {
	before := s.countTransactions(s.userID)
	status := s.postWebhook("gastei 50 no mercado", "wamid.e2e.expense."+uuid.New().String())
	s.Equal(http.StatusOK, status)
	s.Equal(before+1, s.countTransactions(s.userID))
	reply, ok := s.gateway.LastReply()
	s.True(ok)
	s.Equal(s.waNumber, reply.To)
}

func (s *E2EHTTPSuite) TestI1_CreateIncome_PersistsRow() {
	before := s.countTransactions(s.userID)
	status := s.postWebhook("recebi salário de 3000", "wamid.e2e.income."+uuid.New().String())
	s.Equal(http.StatusOK, status)
	s.Equal(before+1, s.countTransactions(s.userID))
	reply, ok := s.gateway.LastReply()
	s.True(ok)
	s.NotEmpty(reply.Text)
}

func (s *E2EHTTPSuite) TestI2_UnknownIntent_NoWriteHonestRefusal() {
	before := s.countTransactions(s.userID)
	status := s.postWebhook("ação não suportada", "wamid.e2e.unknown."+uuid.New().String())
	s.Equal(http.StatusOK, status)
	s.Equal(before, s.countTransactions(s.userID))
	reply, ok := s.gateway.LastReply()
	s.True(ok)
	lower := strings.ToLower(reply.Text)
	s.NotContains(lower, "registrei")
	s.NotContains(lower, "anotei")
}

func (s *E2EHTTPSuite) TestI4_UserIsolation_OtherUserSeesZeroRows() {
	otherID := testsupport.SeedActiveUserWA(s.T(), s.mgr, "+5511977776666")
	beforeOther := s.countTransactions(otherID)
	status := s.postWebhook("gastei 50 no mercado", "wamid.e2e.isolation."+uuid.New().String())
	s.Equal(http.StatusOK, status)
	s.Equal(beforeOther, s.countTransactions(otherID), "user B deve ter zero rows novas após ação de user A")
}

func (s *E2EHTTPSuite) TestI5_Idempotency_SameWAMIDDeduplicates() {
	wamid := "wamid.e2e.idempotency." + uuid.New().String()
	before := s.countTransactions(s.userID)
	status1 := s.postWebhook("gastei 50 no mercado", wamid)
	s.Equal(http.StatusOK, status1)
	afterFirst := s.countTransactions(s.userID)
	s.Equal(before+1, afterFirst)
	status2 := s.postWebhook("gastei 50 no mercado", wamid)
	s.Equal(http.StatusOK, status2)
	s.Equal(afterFirst, s.countTransactions(s.userID))
}

func (s *E2EHTTPSuite) countTransactions(userID uuid.UUID) int {
	var total int
	s.Require().NoError(
		s.mgr.QueryRowContext(s.ctx,
			"SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL",
			userID,
		).Scan(&total),
	)
	return total
}

func (s *E2EHTTPSuite) postWebhook(text, wamid string) int {
	body := s.buildPayload(s.waFrom, text, wamid)
	req, err := http.NewRequest(http.MethodPost, s.server.URL+"/api/v1/whatsapp/inbound", bytes.NewReader(body))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", s.hmacSignature(body))
	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func (s *E2EHTTPSuite) hmacSignature(body []byte) string {
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (s *E2EHTTPSuite) buildPayload(from, text, wamid string) []byte {
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
	raw, err := json.Marshal(wp)
	s.Require().NoError(err)
	return raw
}

func TestI3_FailFast_OpenRouterTransactionsDisabled(t *testing.T) {
	builder := &agentModuleBuilder{
		cfg: &configs.Config{
			AgentConfig: configs.AgentConfig{
				Mode: ModeOpenRouter,
			},
		},
		o11y: noop.NewProvider(),
		categoriesModule: &categories.CategoriesModule{
			ListCategoriesUC:   new(catuc.ListCategories),
			GetCategoryUC:      new(catuc.GetCategory),
			ListDictionaryUC:   new(catuc.ListDictionary),
			SearchDictionaryUC: new(catuc.SearchDictionary),
		},
		cardModule: card.CardModule{
			ListCardsUC:  new(carduc.ListCards),
			CreateCardUC: new(carduc.CreateCard),
		},
		budgetsModule: &budgets.BudgetsModule{
			ListAlertsUC: new(budgetuc.ListAlerts),
		},
		transactionsModule: transactions.TransactionsModule{},
		whatsAppGateway:    &testsupport.CapturingGateway{},
	}
	_, err := builder.buildLLMModule()
	require.Error(t, err)
	require.Contains(t, err.Error(), "transactions")
}
