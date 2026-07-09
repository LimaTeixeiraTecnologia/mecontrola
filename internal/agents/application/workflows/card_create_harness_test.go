//go:build integration

package workflows_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	mempostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

type harnessStubPublisher struct{}

func (p *harnessStubPublisher) Publish(_ context.Context, _ outbox.Event) error { return nil }

type harnessCapturingGateway struct {
	mu       sync.Mutex
	messages []string
}

func (g *harnessCapturingGateway) SendTextMessage(_ context.Context, _, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.messages = append(g.messages, text)
	return nil
}

func (g *harnessCapturingGateway) last() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.messages) == 0 {
		return ""
	}
	return g.messages[len(g.messages)-1]
}

func (g *harnessCapturingGateway) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.messages = nil
}

type harnessInboundEvent struct {
	eventType string
	payload   any
}

func (e *harnessInboundEvent) GetEventType() string { return e.eventType }
func (e *harnessInboundEvent) GetPayload() any      { return e.payload }

type harnessInboundPayload struct {
	UserID    string `json:"user_id"`
	Peer      string `json:"peer"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
}

func buildHarnessEnvelope(p harnessInboundPayload) outbox.Envelope {
	raw, _ := json.Marshal(p)
	return outbox.Envelope{ID: uuid.NewString(), Payload: raw}
}

type CardCreateHarnessSuite struct {
	suite.Suite
	ctx        context.Context
	db         *sqlx.DB
	cfg        *configs.Config
	model      string
	gateway    *harnessCapturingGateway
	handler    events.Handler
	workingMem memory.WorkingMemory
}

func TestCardCreateHarnessSuite(t *testing.T) {
	suite.Run(t, new(CardCreateHarnessSuite))
}

func (s *CardCreateHarnessSuite) SetupSuite() {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" || os.Getenv("RUN_REAL_LLM") != "1" {
		s.T().Skip("RUN_REAL_LLM=1 e OPENROUTER_API_KEY obrigatórios")
	}

	s.ctx = context.Background()
	s.model = os.Getenv("AGENT_HARNESS_MODEL")
	if s.model == "" {
		s.model = "openai/gpt-4o-mini"
	}

	db, _ := testcontainer.Postgres(s.T())
	s.db = db

	cfg, err := configs.LoadConfig("../../../..")
	s.Require().NoError(err, "carregar config")
	cfg.TransactionsConfig.Enabled = true
	s.cfg = cfg

	o11y := fake.NewProvider()
	passthrough := func(next http.Handler) http.Handler { return next }

	categoriesModule := categories.NewCategoriesModule(s.db, o11y, passthrough)
	cardModule, err := card.NewCardModule(s.ctx, s.cfg, o11y, s.db, passthrough, nil, nil)
	s.Require().NoError(err, "card module")
	budgetsModule, err := budgets.NewBudgetsModule(s.cfg, o11y, s.db, categoriesModule, passthrough, nil, nil)
	s.Require().NoError(err, "budgets module")
	txModule, err := transactions.NewTransactionsModule(s.cfg, o11y, s.db, cardModule, categoriesModule, passthrough)
	s.Require().NoError(err, "transactions module")

	s.gateway = &harnessCapturingGateway{}

	deps := agents.Deps{
		DB:              s.db,
		O11y:            o11y,
		OutboxPublisher: &harnessStubPublisher{},
		LLM: agents.LLMConfig{
			Model:       s.model,
			EmbedModel:  "openai/text-embedding-3-small",
			APIKey:      apiKey,
			BaseURL:     "https://openrouter.ai",
			MaxTokens:   1536,
			Temperature: 0,
		},
		CategoriesModule:   categoriesModule,
		CardModule:         cardModule,
		BudgetsModule:      budgetsModule,
		TransactionsModule: txModule,
		WhatsAppGateway:    s.gateway,
	}

	module, err := agents.NewModule(deps)
	s.Require().NoError(err, "agents.NewModule deve subir com deps reais")

	var found bool
	for _, h := range module.EventHandlers {
		if h.EventType == agents.EventTypeWhatsAppInbound {
			s.handler = h.Handler
			found = true
		}
	}
	s.Require().True(found, "consumer de inbound do WhatsApp deve estar registrado")

	s.workingMem = mempostgres.NewWorkingMemoryRepository(s.db, o11y)
}

func (s *CardCreateHarnessSuite) newUser() uuid.UUID {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)

	err = s.workingMem.Upsert(s.ctx, userID.String(), "## Objetivo Financeiro\n\neconomizar para reserva de emergência")
	s.Require().NoError(err, "onboarding deve estar marcado como concluído para o harness exercitar o agente financeiro")

	return userID
}

func (s *CardCreateHarnessSuite) countActiveCards(userID uuid.UUID) int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.cards WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *CardCreateHarnessSuite) send(userID uuid.UUID, peer, text string) string {
	s.gateway.reset()
	wamid := "wamid-harness-" + uuid.NewString()
	env := buildHarnessEnvelope(harnessInboundPayload{
		UserID:    userID.String(),
		Peer:      peer,
		Text:      text,
		MessageID: wamid,
	})
	err := s.handler.Handle(s.ctx, &harnessInboundEvent{eventType: agents.EventTypeWhatsAppInbound, payload: env})
	s.Require().NoError(err)
	reply := s.gateway.last()
	s.T().Logf("modelo=%q envio=%q resposta=%q", s.model, text, reply)
	return reply
}

func (s *CardCreateHarnessSuite) TestGate_CardCreateGherkinScenarios() {
	s.T().Logf("nota de escopo do gate LLM (limiar 0.90): os cenários de TTL-expirado (15min) e idempotência-replay (mesmo wamid) são intencionalmente omitidos deste denominador — são inviáveis de forçar via conversa real e já estão cobertos deterministicamente por testes de integração e de decisão (workflow.Decide*/executeCreateCard)")

	type scenario struct {
		name string
		run  func() bool
	}

	scenarios := []scenario{
		{
			name: "fluxo_feliz_banco_reconhecido",
			run: func() bool {
				userID := s.newUser()
				peer := "peer-" + uuid.NewString()
				reply := s.send(userID, peer, "cadastrar cartão, apelido Nu, banco Nubank, vencimento dia 10")
				lower := strings.ToLower(reply)
				if !strings.Contains(lower, "confirma") {
					return false
				}
				confirmReply := s.send(userID, peer, "sim")
				confirmLower := strings.ToLower(confirmReply)
				if !strings.Contains(confirmLower, "cadastr") {
					return false
				}
				return s.countActiveCards(userID) == 1
			},
		},
		{
			name: "banco_nao_reconhecido_pergunta_fechamento",
			run: func() bool {
				userID := s.newUser()
				peer := "peer-" + uuid.NewString()
				reply := s.send(userID, peer, "cadastrar cartão, apelido XP, banco XP Desconhecida, vencimento dia 1")
				lower := strings.ToLower(reply)
				if !strings.Contains(lower, "fechamento") {
					return false
				}
				closingReply := s.send(userID, peer, "dia 20")
				closingLower := strings.ToLower(closingReply)
				if !strings.Contains(closingLower, "confirma") {
					return false
				}
				confirmReply := s.send(userID, peer, "sim")
				if !strings.Contains(strings.ToLower(confirmReply), "cadastr") {
					return false
				}
				return s.countActiveCards(userID) == 1
			},
		},
		{
			name: "slot_filling_pergunta_apenas_dado_faltante",
			run: func() bool {
				userID := s.newUser()
				peer := "peer-" + uuid.NewString()
				reply := s.send(userID, peer, "cadastrar cartão, apelido Nu, banco Nubank")
				lower := strings.ToLower(reply)
				return strings.Contains(lower, "vencimento") || strings.Contains(lower, "dia")
			},
		},
		{
			name: "confirmacao_negada_nao_cria_cartao",
			run: func() bool {
				userID := s.newUser()
				peer := "peer-" + uuid.NewString()
				s.send(userID, peer, "cadastrar cartão, apelido Nu, banco Nubank, vencimento dia 5")
				s.send(userID, peer, "não")
				return s.countActiveCards(userID) == 0
			},
		},
		{
			name: "ambiguidade_reprompt_depois_cancela",
			run: func() bool {
				userID := s.newUser()
				peer := "peer-" + uuid.NewString()
				s.send(userID, peer, "cadastrar cartão, apelido Nu, banco Nubank, vencimento dia 5")
				first := s.send(userID, peer, "talvez")
				if !strings.Contains(strings.ToLower(first), "sim") {
					return false
				}
				s.send(userID, peer, "quem sabe")
				return s.countActiveCards(userID) == 0
			},
		},
		{
			name: "apelido_duplicado_sem_criar_segundo",
			run: func() bool {
				userID := s.newUser()
				peer := "peer-" + uuid.NewString()
				s.send(userID, peer, "cadastrar cartão, apelido Duplicado, banco Nubank, vencimento dia 5")
				for i := 0; i < 4 && s.countActiveCards(userID) == 0; i++ {
					s.send(userID, peer, "sim")
				}
				if s.countActiveCards(userID) != 1 {
					return false
				}
				s.send(userID, peer, "cadastrar cartão, apelido Duplicado, banco Nubank, vencimento dia 8")
				duplicateRejected := false
				for i := 0; i < 4 && !duplicateRejected && s.countActiveCards(userID) == 1; i++ {
					reply := s.send(userID, peer, "sim")
					if strings.Contains(strings.ToLower(reply), "apelido") {
						duplicateRejected = true
					}
				}
				return duplicateRejected && s.countActiveCards(userID) == 1
			},
		},
		{
			name: "dia_invalido_nao_cria_cartao",
			run: func() bool {
				userID := s.newUser()
				peer := "peer-" + uuid.NewString()
				reply := s.send(userID, peer, "cadastrar cartão, apelido Nu, banco Nubank, vencimento dia 32")
				lower := strings.ToLower(reply)
				falseSuccess := strings.Contains(lower, "cadastrei") || strings.Contains(lower, "cadastrado com sucesso")
				mentionsDayCorrection := strings.Contains(lower, "31") || strings.Contains(lower, "vencimento") || strings.Contains(lower, "dia válido") || strings.Contains(lower, "dia valido")
				s.T().Logf("dia_invalido resposta menciona correção de dia=%v", mentionsDayCorrection)
				return !falseSuccess && s.countActiveCards(userID) == 0
			},
		},
		{
			name: "regressao_sem_tool_call_nunca_afirma_cadastro",
			run: func() bool {
				userID := s.newUser()
				peer := "peer-" + uuid.NewString()
				reply := s.send(userID, peer, "Quero cadastrar um cartão, XP, banco XP Desconhecida, vencimento dia 1")
				lower := strings.ToLower(reply)
				falseAssertion := strings.Contains(lower, "cadastrei o cartão") ||
					strings.Contains(lower, "cadastrado com sucesso") ||
					strings.Contains(lower, "não consegui cadastrar")
				if falseAssertion {
					return false
				}
				engagedCardFlow := strings.Contains(lower, "cartão") ||
					strings.Contains(lower, "apelido") ||
					strings.Contains(lower, "fechamento") ||
					strings.Contains(lower, "confirma") ||
					strings.Contains(lower, "vencimento")
				return engagedCardFlow && strings.TrimSpace(reply) != "" && s.countActiveCards(userID) == 0
			},
		},
	}

	hits := 0
	total := len(scenarios)

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			ok := sc.run()
			if ok {
				hits++
			}
			s.T().Logf("caso=%q modelo=%q ok=%v", sc.name, s.model, ok)
		})
	}

	ratio := float64(hits) / float64(total)
	s.T().Logf("gate real-LLM card_create modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, total, ratio)
	require.GreaterOrEqual(s.T(), ratio, 0.90, "gate de merge RF-22: ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}
