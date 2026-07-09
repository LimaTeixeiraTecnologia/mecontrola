//go:build integration

package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/scorers"
	agenttools "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/formatting"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type harnessStore struct {
	mu   sync.RWMutex
	data map[string]workflow.Snapshot
}

func newHarnessStore() *harnessStore {
	return &harnessStore{data: make(map[string]workflow.Snapshot)}
}

func (s *harnessStore) storeKey(wid, ck string) string { return wid + "::" + ck }

func (s *harnessStore) Insert(_ context.Context, snap workflow.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.storeKey(snap.Workflow, snap.CorrelationKey)
	if ex, ok := s.data[k]; ok {
		if ex.Status == workflow.RunStatusRunning || ex.Status == workflow.RunStatusSuspended {
			return workflow.ErrRunAlreadyExists
		}
	}
	s.data[k] = snap
	return nil
}

func (s *harnessStore) Load(_ context.Context, wid, key string) (workflow.Snapshot, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.data[s.storeKey(wid, key)]
	return snap, ok, nil
}

func (s *harnessStore) LoadLatest(_ context.Context, wid, key string) (workflow.Snapshot, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.data[s.storeKey(wid, key)]
	return snap, ok, nil
}

func (s *harnessStore) Save(_ context.Context, snap workflow.Snapshot, expected int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.storeKey(snap.Workflow, snap.CorrelationKey)
	if ex, ok := s.data[k]; ok && ex.Version != expected {
		return workflow.ErrVersionConflict
	}
	s.data[k] = snap
	return nil
}

func (s *harnessStore) AppendStep(_ context.Context, _ workflow.StepRecord) error { return nil }

func (s *harnessStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

func (s *harnessStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]workflow.Snapshot, error) {
	return nil, nil
}

func buildRealLLMProvider(t *testing.T) llm.Provider {
	t.Helper()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" || os.Getenv("RUN_REAL_LLM") != "1" {
		t.Skip("RUN_REAL_LLM=1 e OPENROUTER_API_KEY obrigatórios")
	}
	baseURL := "https://openrouter.ai"
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter"),
		httpclient.WithTimeout(30*time.Second),
	)
	require.NoError(t, err)
	model := os.Getenv("AGENT_HARNESS_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini"
	}
	return llm.NewOpenRouterProvider(client, llm.Config{
		Model:          model,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		HTTPReferer:    "https://github.com/LimaTeixeiraTecnologia/mecontrola",
		XTitle:         "mecontrola-integration-test",
		MaxTokens:      1536,
		Temperature:    0,
		RequestTimeout: 30 * time.Second,
	}, fake.NewProvider())
}

func buildFakeRegisterExpenseTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"wamid":         map[string]any{"type": "string"},
				"itemSeq":       map[string]any{"type": "integer"},
				"userId":        map[string]any{"type": "string"},
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
			},
			"required":             []string{"wamid", "itemSeq", "userId", "amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resourceId": map[string]any{"type": "string"},
				"kind":       map[string]any{"type": "string"},
				"isReplay":   map[string]any{"type": "boolean"},
			},
			"required":             []string{"resourceId", "kind", "isReplay"},
			"additionalProperties": false,
		},
	}
	type input struct {
		Wamid         string `json:"wamid"`
		ItemSeq       int    `json:"itemSeq"`
		UserID        string `json:"userId"`
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
	}
	type output struct {
		ResourceID string `json:"resourceId"`
		Kind       string `json:"kind"`
		IsReplay   bool   `json:"isReplay"`
	}
	return tool.NewTool[input, output]("register_expense", "Registra um lançamento de despesa no ledger financeiro do usuário.", in, out,
		func(_ context.Context, in input) (output, error) {
			return output{
				ResourceID: uuid.New().String(),
				Kind:       "transaction",
				IsReplay:   false,
			}, nil
		},
	)
}

func buildFakeQueryMonthTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "query_month_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"userId":   map[string]any{"type": "string"},
				"refMonth": map[string]any{"type": "string"},
				"cursor":   map[string]any{"type": "string"},
				"limit":    map[string]any{"type": "integer"},
			},
			"required":             []string{"userId"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "query_month_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"refMonth":     map[string]any{"type": "string"},
				"incomeCents":  map[string]any{"type": "integer"},
				"outcomeCents": map[string]any{"type": "integer"},
				"totalCents":   map[string]any{"type": "integer"},
				"entries":      map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			},
			"required":             []string{"refMonth", "incomeCents", "outcomeCents", "totalCents", "entries"},
			"additionalProperties": false,
		},
	}
	type input struct {
		UserID   string `json:"userId"`
		RefMonth string `json:"refMonth,omitempty"`
	}
	type output struct {
		RefMonth     string `json:"refMonth"`
		IncomeCents  int64  `json:"incomeCents"`
		OutcomeCents int64  `json:"outcomeCents"`
		TotalCents   int64  `json:"totalCents"`
		Entries      []any  `json:"entries"`
	}
	return tool.NewTool[input, output]("query_month", "Consulta o resumo e os lançamentos do mês financeiro do usuário.", in, out,
		func(_ context.Context, in input) (output, error) {
			refMonth := in.RefMonth
			if refMonth == "" {
				refMonth = time.Now().Format("2006-01")
			}
			return output{
				RefMonth:     refMonth,
				IncomeCents:  500000,
				OutcomeCents: 320000,
				TotalCents:   180000,
				Entries:      []any{},
			}, nil
		},
	)
}

func buildFailingRegisterExpenseTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"wamid":         map[string]any{"type": "string"},
				"itemSeq":       map[string]any{"type": "integer"},
				"userId":        map[string]any{"type": "string"},
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
			},
			"required":             []string{"wamid", "itemSeq", "userId", "amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	type input struct {
		Wamid string `json:"wamid"`
	}
	return tool.NewTool[input, map[string]any]("register_expense", "Registra um lançamento de despesa no ledger financeiro do usuário.", in, llm.Schema{},
		func(_ context.Context, _ input) (map[string]any, error) {
			return nil, fmt.Errorf("falha de persistência: banco de dados indisponível")
		},
	)
}

func buildFailingRegisterExpenseToolCA03() tool.ToolHandle {
	inSchema := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"wamid":         map[string]any{"type": "string"},
				"itemSeq":       map[string]any{"type": "integer"},
				"userId":        map[string]any{"type": "string"},
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
			},
			"required":             []string{"wamid", "itemSeq", "userId", "amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	outSchema := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	type input struct {
		Wamid         string `json:"wamid"`
		ItemSeq       int    `json:"itemSeq"`
		UserID        string `json:"userId"`
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
	}
	return tool.NewTool[input, map[string]any](
		"register_expense",
		"Registra um lançamento de despesa no ledger financeiro do usuário.",
		inSchema,
		outSchema,
		func(_ context.Context, _ input) (map[string]any, error) {
			return nil, errors.New("persistence failure: connection refused")
		},
	)
}

func buildPendingClarifyTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":     map[string]any{"type": "integer"},
				"description":     map[string]any{"type": "string"},
				"paymentMethod":   map[string]any{"type": "string"},
				"occurredAt":      map[string]any{"type": "string"},
				"categoryId":      map[string]any{"type": "string"},
				"subcategoryId":   map[string]any{"type": "string"},
				"categoryVersion": map[string]any{"type": "integer"},
				"cardId":          map[string]any{"type": "string"},
				"installments":    map[string]any{"type": "integer"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resourceId": map[string]any{"type": "string"},
				"kind":       map[string]any{"type": "string"},
				"isReplay":   map[string]any{"type": "boolean"},
				"outcome":    map[string]any{"type": "string"},
			},
			"required":             []string{"resourceId", "kind", "isReplay", "outcome"},
			"additionalProperties": false,
		},
	}
	type input struct {
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
	}
	type output struct {
		ResourceID string `json:"resourceId"`
		Kind       string `json:"kind"`
		IsReplay   bool   `json:"isReplay"`
		Outcome    string `json:"outcome"`
	}
	return tool.NewTool[input, output]("register_expense", "Registra um lançamento de despesa no ledger financeiro do usuário.", in, out,
		func(_ context.Context, _ input) (output, error) {
			return output{
				ResourceID: "",
				Kind:       "pending",
				IsReplay:   false,
				Outcome:    "clarify",
			}, nil
		},
	)
}

func chainGatedRegistrar(cat agentsifaces.CategoriesReader, ledger agentsifaces.TransactionsLedger, obs *fake.Provider) (*agentusecases.RegisterAttempt, *harnessStore) {
	store := newHarnessStore()
	engine := workflow.NewEngine[workflows.PendingEntryState](store, obs)
	def := workflows.BuildPendingEntryWorkflow(ledger, nil, cat, nil)
	return agentusecases.NewRegisterAttempt(cat, ledger, engine, def, obs), store
}

func chainLoadPending(t *testing.T, store *harnessStore, userID uuid.UUID) workflows.PendingEntryState {
	t.Helper()
	key := workflows.PendingEntryKey(userID.String(), "")
	snap, found, err := store.Load(context.Background(), workflows.PendingEntryWorkflowID, key)
	require.NoError(t, err)
	require.True(t, found, "register deve abrir pendência durável (gate de confirmação), sem escrita síncrona")
	st, decErr := workflow.NewCodec[workflows.PendingEntryState]().Decode(snap.State)
	require.NoError(t, decErr)
	return st
}

type MecontrolaAgentIntegrationSuite struct {
	suite.Suite
	provider llm.Provider
}

func TestMecontrolaAgentIntegrationSuite(t *testing.T) {
	suite.Run(t, new(MecontrolaAgentIntegrationSuite))
}

func (s *MecontrolaAgentIntegrationSuite) SetupSuite() {
	s.provider = buildRealLLMProvider(s.T())
}

func (s *MecontrolaAgentIntegrationSuite) TestToolCalling() {
	type args struct {
		content   string
		maxTokens int
	}
	type dependencies struct {
		tools []tool.ToolHandle
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result agent.Result, err error)
	}{
		{
			name: "register_expense via linguagem natural",
			args: args{
				content:   "meu userId é " + uuid.New().String() + " e o wamid é wamid-test-001. gastei 50 reais no almoço hoje. paymentMethod: debit",
				maxTokens: 512,
			},
			dependencies: dependencies{tools: []tool.ToolHandle{buildFakeRegisterExpenseTool(), buildFakeQueryMonthTool()}},
			expect: func(result agent.Result, err error) {
				s.Require().NoError(err)
				s.Require().NotEmpty(result.Content)
				s.T().Logf("resposta do agente: %s", result.Content)
			},
		},
		{
			name: "query_month via linguagem natural",
			args: args{
				content:   "meu userId é " + uuid.New().String() + ". quanto gastei esse mês?",
				maxTokens: 512,
			},
			dependencies: dependencies{tools: []tool.ToolHandle{buildFakeRegisterExpenseTool(), buildFakeQueryMonthTool()}},
			expect: func(result agent.Result, err error) {
				s.Require().NoError(err)
				s.Require().NotEmpty(result.Content)
				s.T().Logf("resposta do agente: %s", result.Content)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			obs := fake.NewProvider()
			a := BuildMeControlaAgent(s.provider, scenario.dependencies.tools, nil, obs)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			result, err := a.Execute(ctx, agent.Request{
				AgentID:   MecontrolaAgentID,
				Messages:  []llm.Message{{Role: "user", Content: scenario.args.content}},
				MaxTokens: scenario.args.maxTokens,
			})
			scenario.expect(result, err)
		})
	}
}

func (s *MecontrolaAgentIntegrationSuite) TestScorerCategorizationLLMJudged() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sc := scorers.NewCategorizationScorer(s.provider)

	s.Require().Equal(scorer.ScorerKindLLMJudged, sc.Kind())

	sample := scorer.RunSample{
		Input:  "gastei no mercado",
		Output: "✅ Registrei R$ 100,00 na categoria Alimentação. Lançamento confirmado para o mês atual.",
	}

	result, err := sc.Score(ctx, sample)

	s.Require().NoError(err)
	s.Require().GreaterOrEqual(result.Score, 0.0)
	s.Require().LessOrEqual(result.Score, 1.0)
	s.Require().NotEmpty(result.Reason)
	s.T().Logf("score=%.2f reason=%s", result.Score, result.Reason)
}

func (s *MecontrolaAgentIntegrationSuite) TestOnboardingSummaryUsesWhatsAppFormattingAndEmojis() {
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(s.provider, nil, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Explique de forma didática as 5 categorias do orçamento 30/20/20/10/20 do MeControla, usando negrito compatível com WhatsApp para os nomes das categorias. Depois gere um resumo de onboarding com renda mensal de R$8.000,00, sem objetivo financeiro definido, cartão de crédito não informado, distribuição: Conhecimento 20%, Custo Fixo 30%, Liberdade Financeira 20%, Metas 10%, Prazeres 20%. Termine com uma única pergunta de confirmação para ativar o orçamento."},
		},
	})

	s.Require().NoError(err)
	s.Require().NotEmpty(result.Content)
	s.Require().False(result.TruncatedByLength)

	normalized := formatting.NormalizeOutboundText(result.Content)

	s.Require().Contains(normalized, "*Custo Fixo")
	s.Require().NotContains(normalized, "**")
	s.Require().Contains(normalized, "📊")
	s.Require().True(strings.Contains(normalized, "✅") || strings.Contains(normalized, "🎯"))
	s.Require().Contains(normalized, "Liberdade Financeira")
	s.Require().Contains(normalized, "Você confirma")
	s.T().Logf("resposta onboarding raw: %s", result.Content)
	s.T().Logf("resposta onboarding normalized: %s", normalized)
}

func (s *MecontrolaAgentIntegrationSuite) TestToolErrorProducesHonestResponse() {
	obs := fake.NewProvider()
	userID := uuid.New().String()

	tools := []tool.ToolHandle{buildFailingRegisterExpenseTool()}

	a := BuildMeControlaAgent(s.provider, tools, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "meu userId é " + userID + " e o wamid é wamid-fail-001, itemSeq 1. gastei 30 reais no cafe. paymentMethod: debit"},
		},
		MaxTokens: 512,
	})

	s.Require().NoError(err)
	s.Require().NotEmpty(result.Content, "resposta nao pode ser vazia: tool com erro deve gerar resposta honesta")

	lower := strings.ToLower(result.Content)
	s.T().Logf("resposta do agente com tool em falha: %s", result.Content)

	negativas := []string{"registrei com sucesso", "foi registrado com sucesso", "registrado com sucesso", "despesa registrada com sucesso"}
	for _, n := range negativas {
		s.Require().NotContains(lower, n, "agente nao deve confirmar sucesso quando tool falhou")
	}
}

func (s *MecontrolaAgentIntegrationSuite) TestCardPurchaseChainResolveClassifyRegister() {
	scenarios := []struct {
		name             string
		message          string
		wamid            string
		wantInstallments int
	}{
		{
			name:             "parcelada em 3x roteia via register_expense credit_card",
			message:          "Comprei um notebook de 3000 reais em 3x no cartão Nubank. Registre essa compra.",
			wamid:            "wamid-chain-parcelada",
			wantInstallments: 3,
		},
		{
			name:             "à vista roteia via register_expense credit_card com installments=1",
			message:          "Comprei um teclado de 300 reais à vista no cartão Nubank. Registre essa compra.",
			wamid:            "wamid-chain-avista",
			wantInstallments: 1,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			t := s.T()
			obs := fake.NewProvider()

			userID := uuid.New()
			cardID := uuid.New()
			rootID := uuid.New()
			leafID := uuid.New()

			var resolveCalled, searchCalled bool

			cardMock := imocks.NewCardManager(t)
			cardMock.EXPECT().ResolveCardByNickname(mock.Anything, mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.Card, error) {
					resolveCalled = true
					return agentsifaces.Card{ID: cardID.String(), UserID: userID.String(), Nickname: "Nubank", Bank: "Nubank", DueDay: 1}, nil
				}).Maybe()

			catMock := imocks.NewCategoriesReader(t)
			catMock.EXPECT().SearchDictionary(mock.Anything, mock.Anything, "expense").
				RunAndReturn(func(_ context.Context, _, _ string) (agentsifaces.CategorySearchResult, error) {
					searchCalled = true
					return agentsifaces.CategorySearchResult{
						Outcome: agentsifaces.ClassifyOutcomeMatched,
						Version: 1,
						Candidates: []agentsifaces.CategoryCandidate{{
							CategoryID:     leafID,
							RootCategoryID: rootID,
							Path:           "Custo Fixo > Eletrônicos",
							Score:          0.95,
							Confidence:     "high",
							MatchQuality:   "exact",
							SignalType:     "canonical_name",
							MatchedTerm:    "eletrônicos",
						}},
					}, nil
				}).Maybe()
			catMock.EXPECT().ResolveForWrite(mock.Anything, mock.Anything).
				Return(agentsifaces.CategoryWriteDecision{
					RootCategoryID:   rootID,
					SubcategoryID:    leafID,
					Path:             "Custo Fixo > Eletrônicos",
					RootSlug:         "custo-fixo",
					SubcategorySlug:  "eletronicos",
					EditorialVersion: 1,
				}, nil).Maybe()

			ledgerMock := imocks.NewTransactionsLedger(t)

			registrar, store := chainGatedRegistrar(catMock, ledgerMock, obs)

			tools := []tool.ToolHandle{
				agenttools.BuildResolveCardTool(cardMock),
				agenttools.BuildRegisterExpenseTool(registrar),
			}

			a := BuildMeControlaAgent(s.provider, tools, nil, obs)

			ctx := agent.WithToolInvocationContext(context.Background(), userID.String(), sc.wamid, 0)
			ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
			defer cancel()

			result, err := a.Execute(ctx, agent.Request{
				AgentID: MecontrolaAgentID,
				Messages: []llm.Message{
					{Role: "user", Content: sc.message},
				},
				MaxTokens: 1024,
			})
			require.NoError(t, err)
			t.Logf("resposta do agente: %s", result.Content)

			require.True(t, resolveCalled, "agente deve consultar os cartões do usuário via resolve_card")
			require.True(t, searchCalled, "a categoria deve ser classificada deterministicamente pela ferramenta, não pelo LLM")

			st := chainLoadPending(t, store, userID)
			require.Equal(t, workflows.PendingOpRegisterExpense, st.OperationKind)
			require.Equal(t, "credit_card", st.PaymentMethod, "compra no crédito deve rotear via register_expense com paymentMethod=credit_card")
			require.NotNil(t, st.CardID, "register_expense deve receber o cardId resolvido pelo resolve_card")
			require.Equal(t, cardID, *st.CardID, "cardId deve ser o resolvido pelo resolve_card")
			require.Equal(t, sc.wantInstallments, st.Installments, "installments deve refletir o parcelamento informado")
			require.Len(t, st.Candidates, 1)
			require.Equal(t, rootID, st.Candidates[0].RootCategoryID, "raiz classificada deterministicamente")
			require.Equal(t, leafID, st.Candidates[0].SubcategoryID, "folha classificada deterministicamente")
		})
	}
}

func (s *MecontrolaAgentIntegrationSuite) TestRegisterOpensConfirmationGate() {
	t := s.T()
	obs := fake.NewProvider()
	userID := uuid.New()
	rootID := uuid.New()
	leafID := uuid.New()

	catMock := imocks.NewCategoriesReader(t)
	catMock.EXPECT().SearchDictionary(mock.Anything, mock.Anything, "expense").
		Return(agentsifaces.CategorySearchResult{
			Outcome: agentsifaces.ClassifyOutcomeMatched,
			Version: 9,
			Candidates: []agentsifaces.CategoryCandidate{{
				CategoryID:     leafID,
				RootCategoryID: rootID,
				Path:           "Custo Fixo > Supermercado",
				Score:          0.95,
				Confidence:     "high",
				MatchQuality:   "exact",
				SignalType:     "canonical_name",
				MatchedTerm:    "supermercado",
			}},
		}, nil).Maybe()
	catMock.EXPECT().ResolveForWrite(mock.Anything, mock.Anything).
		Return(agentsifaces.CategoryWriteDecision{
			RootCategoryID:   rootID,
			SubcategoryID:    leafID,
			Path:             "Custo Fixo > Supermercado",
			RootSlug:         "custo-fixo",
			SubcategorySlug:  "supermercado",
			EditorialVersion: 9,
		}, nil).Maybe()

	ledgerMock := imocks.NewTransactionsLedger(t)

	registrar, store := chainGatedRegistrar(catMock, ledgerMock, obs)

	tools := []tool.ToolHandle{
		agenttools.BuildRegisterExpenseTool(registrar),
		agenttools.BuildClassifyCategoryTool(catMock),
	}
	a := BuildMeControlaAgent(s.provider, tools, nil, obs)

	ctx := agent.WithToolInvocationContext(context.Background(), userID.String(), "wamid-gate-chain", 0)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 150,00 no mercado hoje, no pix. Registre esse lançamento."},
		},
		MaxTokens: 1024,
	})
	require.NoError(t, err)
	t.Logf("resposta do agente: %s", result.Content)

	lower := strings.ToLower(result.Content)
	require.Contains(t, lower, "confirma", "o agente deve relayar a pergunta de confirmação ao usuário, não improvisar erro")
	require.NotContains(t, lower, "dificuldade", "não deve reportar falha quando o gate apenas abriu a confirmação")

	st := chainLoadPending(t, store, userID)
	require.Equal(t, workflows.PendingOpRegisterExpense, st.OperationKind)
	require.Equal(t, int64(15000), st.AmountCents, "valor original deve ser preservado no gate")
	require.Equal(t, "pix", st.PaymentMethod, "forma de pagamento original deve ser preservada")
	require.Equal(t, workflows.PendingStatusActive, st.Status, "O-07/RF-38: escrita não persiste antes da confirmação")
}

func (s *MecontrolaAgentIntegrationSuite) TestQueryCardInvoiceChainC4() {
	t := s.T()
	const iterations = 10
	const minPass = 8

	passes := 0
	for i := 1; i <= iterations; i++ {
		obs := fake.NewProvider()
		userID := uuid.New()
		cardID := uuid.New()

		var capturedCardID uuid.UUID

		cardMock := imocks.NewCardManager(t)
		cardMock.EXPECT().ResolveCardByNickname(mock.Anything, mock.Anything, mock.AnythingOfType("string")).
			RunAndReturn(func(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.Card, error) {
				return agentsifaces.Card{ID: cardID.String(), UserID: userID.String(), Nickname: "Nubank", Bank: "Nubank", DueDay: 10}, nil
			}).Maybe()

		ledgerMock := imocks.NewTransactionsLedger(t)
		ledgerMock.EXPECT().GetCardInvoice(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
			RunAndReturn(func(_ context.Context, cid uuid.UUID, _ string) (agentsifaces.CardInvoice, error) {
				capturedCardID = cid
				return agentsifaces.CardInvoice{
					ID:              uuid.New(),
					UserID:          userID,
					CardID:          cardID,
					RefMonth:        "2026-07",
					ClosingAt:       time.Now().Add(10 * 24 * time.Hour),
					DueAt:           time.Now().Add(17 * 24 * time.Hour),
					ItemsTotalCents: 45000,
					Items:           []agentsifaces.CardInvoiceItem{},
				}, nil
			}).Maybe()

		tools := []tool.ToolHandle{
			agenttools.BuildResolveCardTool(cardMock),
			agenttools.BuildQueryCardInvoiceTool(ledgerMock),
		}

		a := BuildMeControlaAgent(s.provider, tools, nil, obs)
		ctx := agent.WithToolInvocationContext(context.Background(), userID.String(), "wamid-c4-chain", 0)
		ctx, cancel := context.WithTimeout(ctx, 120*time.Second)

		result, err := a.Execute(ctx, agent.Request{
			AgentID: MecontrolaAgentID,
			Messages: []llm.Message{
				{Role: "user", Content: "quanto está minha fatura do cartão nubank?"},
			},
			MaxTokens: 1024,
		})
		cancel()
		require.NoError(t, err)

		lower := strings.ToLower(result.Content)
		require.NotContainsf(t, lower, "nubank-", "anti-alucinação: resposta não deve conter cardId bruto fabricado")

		if capturedCardID == cardID {
			passes++
		} else {
			t.Logf("C4 iteração %d não roteou resolve_card→query_card_invoice: %s", i, result.Content)
		}
	}

	t.Logf("C4 gate de confiabilidade: %d/%d roteou resolve_card→query_card_invoice", passes, iterations)
	require.GreaterOrEqualf(t, passes, minPass,
		"RF-05/RF-32a: C4 deve rotear resolve_card→query_card_invoice em pelo menos %d de %d execuções (cardId sempre de resolve_card)", minPass, iterations)
}

func (s *MecontrolaAgentIntegrationSuite) TestLastTransactionChainC5() {
	t := s.T()
	obs := fake.NewProvider()
	userID := uuid.New()
	knownTxID := "txn-known-" + uuid.New().String()

	var capturedTxID string

	ledgerMock := imocks.NewTransactionsLedger(t)
	ledgerMock.EXPECT().GetMonthlySummary(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
		Return(agentsifaces.MonthlySummary{
			RefMonth:     "2026-07",
			IncomeCents:  500000,
			OutcomeCents: 32000,
			TotalCents:   468000,
		}, nil).Maybe()
	ledgerMock.EXPECT().ListMonthlyEntries(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string"), mock.Anything, mock.Anything).
		Return([]agentsifaces.MonthlyEntry{
			{
				Kind:        agentsifaces.EntryKindTransaction,
				ID:          knownTxID,
				RefMonth:    "2026-07",
				AmountCents: 32000,
				Direction:   "outcome",
				Description: "Supermercado Extra",
				CreatedAt:   time.Now().Add(-2 * time.Hour),
			},
		}, nil).Maybe()
	ledgerMock.EXPECT().GetTransaction(mock.Anything, mock.AnythingOfType("string")).
		RunAndReturn(func(_ context.Context, txID string) (agentsifaces.Entry, error) {
			capturedTxID = txID
			return agentsifaces.Entry{
				Kind:                    agentsifaces.EntryKindTransaction,
				ID:                      txID,
				UserID:                  userID.String(),
				Direction:               "outcome",
				PaymentMethod:           "debit",
				AmountCents:             32000,
				Description:             "Supermercado Extra",
				CategoryNameSnapshot:    "Custo Fixo",
				SubcategoryNameSnapshot: "Supermercado",
				RefMonth:                "2026-07",
				OccurredAt:              time.Now().Add(-2 * time.Hour),
			}, nil
		}).Maybe()

	tools := []tool.ToolHandle{
		agenttools.BuildQueryMonthTool(ledgerMock),
		agenttools.BuildGetTransactionTool(ledgerMock),
	}

	a := BuildMeControlaAgent(s.provider, tools, nil, obs)
	ctx := agent.WithToolInvocationContext(context.Background(), userID.String(), "wamid-c5-chain", 0)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "qual foi a minha última transação?"},
		},
		MaxTokens: 1024,
	})
	require.NoError(t, err)
	t.Logf("C5 resposta: %s", result.Content)

	require.Equal(t, knownTxID, capturedTxID, "C5: get_transaction deve ser chamado com o ID retornado por query_month")

	lower := strings.ToLower(result.Content)
	require.True(t,
		strings.Contains(lower, "custo fixo") && strings.Contains(lower, "supermercado"),
		"RF-06/D-11: resposta deve exibir categoryNameSnapshot E subcategoryNameSnapshot via get_transaction (o formato literal '>' é best-effort de apresentação, ver D-11); resposta=%s", result.Content,
	)
}

func (s *MecontrolaAgentIntegrationSuite) TestCA03HonestConfirmationToolErrorNeverSuccessNorEmpty() {
	t := s.T()
	obs := fake.NewProvider()
	userID := uuid.New().String()

	tools := []tool.ToolHandle{buildFailingRegisterExpenseToolCA03()}

	a := BuildMeControlaAgent(s.provider, tools, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: "meu userId é " + userID + " e o wamid é wamid-ca03-001, itemSeq 1. gastei 80 reais no mercado hoje. paymentMethod: debit",
			},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err, "CA-03: agent.Execute must not return error even when tool fails")
	require.NotEmpty(t, result.Content, "CA-03: content must never be empty when tool errors")

	lower := strings.ToLower(result.Content)
	t.Logf("CA-03 honest reply: %s", result.Content)

	falseSuccessTerms := []string{
		"registrei com sucesso",
		"foi registrado com sucesso",
		"registrado com sucesso",
		"despesa registrada com sucesso",
		"lançamento registrado",
		"lançamento foi registrado",
	}
	for _, term := range falseSuccessTerms {
		require.NotContains(t, lower, term,
			"CA-03: agent must never confirm success when persistence failed (found: %q)", term)
	}

	require.Equal(t, agent.ToolOutcomeUsecaseError, result.ToolOutcome,
		"CA-03: a failing write tool must deterministically yield ToolOutcomeUsecaseError regardless of LLM text")
}

func (s *MecontrolaAgentIntegrationSuite) TestPendingEntryCA01ClarifyAsksOneQuestion() {
	t := s.T()
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 150,00 no mercado hoje no pix"},
		},
		MaxTokens: 256,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("CA-01 resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)

	questionMarks := strings.Count(result.Content, "?")
	require.LessOrEqual(t, questionMarks, 1, "CA-01: agente deve fazer no maximo uma pergunta")

	falseSucessTerms := []string{"registrei", "anotei", "salvo", "foi registrada", "registrada com sucesso"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "CA-01 M-03=0: agente nao deve confirmar sucesso sem write real")
	}

	require.NotContains(t, lower, "r$ 150", "CA-01 M-02: agente nao deve repetir valor no slot de categoria")
	require.NotContains(t, lower, "pix", "CA-01 M-02: agente nao deve repetir pagamento no slot de categoria")
}

func (s *MecontrolaAgentIntegrationSuite) TestPendingEntryCA06LedgerErrorHonestResponse() {
	t := s.T()
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{buildFailingRegisterExpenseTool()}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 80,00 na farmácia hoje no pix"},
		},
		MaxTokens: 256,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("CA-06 G7-15 resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)
	falseSucessTerms := []string{"registrei", "registrada com sucesso", "anotei", "foi registrado"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "CA-06 M-03=0: agente nao deve confirmar sucesso com erro de ledger")
	}
}

func (s *MecontrolaAgentIntegrationSuite) TestPendingEntryNoInfraInResponse() {
	t := s.T()
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 200,00 no supermercado ontem no débito"},
		},
		MaxTokens: 256,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("NoInfra resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)
	infraTerms := []string{"workflow", "pendência interna", "correlação", "thread_id", "resource_id", "correlation_key"}
	for _, term := range infraTerms {
		require.NotContains(t, lower, term, "agente nao deve mencionar infraestrutura ao usuario")
	}
}

func (s *MecontrolaAgentIntegrationSuite) TestPendingEntryCA12DoubleAsteriskProibido() {
	t := s.T()
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 320,00 no supermercado hoje no pix"},
		},
		MaxTokens: 256,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("CA-12 WhatsApp formatting resposta: %s", result.Content)

	require.NotContains(t, result.Content, "**", "CA-12: agente nao deve usar duplo asterisco (WhatsApp incompativel)")
}

func (s *MecontrolaAgentIntegrationSuite) TestPendingEntryCA04MultipleCandidatesListaLegivel() {
	t := s.T()
	obs := fake.NewProvider()

	in := llm.Schema{
		Name:   "classify_category_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"term": map[string]any{"type": "string"},
				"kind": map[string]any{"type": "string"},
			},
			"required":             []string{"term", "kind"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "classify_category_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"writeDecision": map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
				"path":          map[string]any{"type": "string"},
				"candidates":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			},
			"required":             []string{"writeDecision"},
			"additionalProperties": false,
		},
	}
	type classifyInput struct {
		Term string `json:"term"`
		Kind string `json:"kind"`
	}
	type candidate struct {
		Path string `json:"path"`
	}
	type classifyOutput struct {
		WriteDecision string      `json:"writeDecision"`
		CategoryID    string      `json:"categoryId"`
		SubcategoryID string      `json:"subcategoryId"`
		Path          string      `json:"path"`
		Candidates    []candidate `json:"candidates"`
	}
	classifyTool := tool.NewTool[classifyInput, classifyOutput]("classify_category", "Classifica termo em categoria.", in, out,
		func(_ context.Context, _ classifyInput) (classifyOutput, error) {
			return classifyOutput{
				WriteDecision: "blocked",
				Candidates: []candidate{
					{Path: "Custo Fixo > Plano de Saúde"},
					{Path: "Custo Fixo > Consultas e Exames"},
					{Path: "Custo Fixo > Terapia e Saúde Mental"},
				},
			}, nil
		},
	)

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{buildPendingClarifyTool(), classifyTool}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Paguei R$ 350,00 de plano de saúde hoje no boleto"},
			{Role: "assistant", Content: "Qual é a categoria para esse lançamento?"},
			{Role: "user", Content: "saúde"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("CA-04 multiplos candidatos resposta: %s", result.Content)

	require.NotContains(t, result.Content, "**", "CA-04: sem duplo asterisco")

	hasNumbers := strings.Contains(result.Content, "1.") || strings.Contains(result.Content, "1)")
	hasSaudeReference := strings.Contains(strings.ToLower(result.Content), "saúde") || strings.Contains(strings.ToLower(result.Content), "saude")
	require.True(t, hasNumbers || hasSaudeReference, "CA-04: agente deve mostrar opcoes de categoria de forma legivel")
}

func (s *MecontrolaAgentIntegrationSuite) TestR2DataRelativaOntem() {
	t := s.T()
	obs := fake.NewProvider()

	type capturedInput struct {
		PaymentMethod string `json:"paymentMethod"`
		OccurredAt    string `json:"occurredAt"`
	}
	var captured capturedInput

	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}

	handle := tool.NewTool[capturedInput, map[string]any]("register_expense", "Registra um lançamento de despesa.", in, out,
		func(_ context.Context, inp capturedInput) (map[string]any, error) {
			captured = inp
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Ontem fui na feira e gastei cinquenta reais em pastel no dinheiro"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("R2 resposta: %s | paymentMethod=%s occurredAt=%s", result.Content, captured.PaymentMethod, captured.OccurredAt)

	require.NotEmpty(t, captured.PaymentMethod, "R2 M-01: paymentMethod deve ser extraído")
	require.NotContains(t, result.Content, "**", "R2 RF-23: sem duplo asterisco")
}

func (s *MecontrolaAgentIntegrationSuite) TestR5ReceitaDecimoTerceiro() {
	t := s.T()
	obs := fake.NewProvider()

	var incomeToolCalled bool
	in := llm.Schema{
		Name:   "register_income_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents": map[string]any{"type": "integer"},
				"description": map[string]any{"type": "string"},
				"occurredAt":  map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_income_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}
	type incomeInput struct {
		AmountCents int64  `json:"amountCents"`
		Description string `json:"description"`
	}
	handle := tool.NewTool[incomeInput, map[string]any]("register_income", "Registra um lançamento de receita.", in, out,
		func(_ context.Context, _ incomeInput) (map[string]any, error) {
			incomeToolCalled = true
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Recebi meu décimo terceiro de R$ 10.000,00"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("R5 resposta: %s | incomeToolCalled=%v", result.Content, incomeToolCalled)

	require.True(t, incomeToolCalled, "R5 M-01: deve chamar register_income para receita")
	require.NotContains(t, result.Content, "**", "R5 RF-23: sem duplo asterisco")
}

func (s *MecontrolaAgentIntegrationSuite) TestR6ReceitaFreelancer() {
	t := s.T()
	obs := fake.NewProvider()

	var incomeToolCalled bool
	var capturedAmount int64
	in := llm.Schema{
		Name:   "register_income_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents": map[string]any{"type": "integer"},
				"description": map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_income_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}
	type incomeInput struct {
		AmountCents int64  `json:"amountCents"`
		Description string `json:"description"`
	}
	handle := tool.NewTool[incomeInput, map[string]any]("register_income", "Registra um lançamento de receita.", in, out,
		func(_ context.Context, inp incomeInput) (map[string]any, error) {
			incomeToolCalled = true
			capturedAmount = inp.AmountCents
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Recebi duzentos reais de um freelancer"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("R6 resposta: %s | incomeToolCalled=%v amount=%d", result.Content, incomeToolCalled, capturedAmount)

	require.True(t, incomeToolCalled, "R6 M-01: deve chamar register_income para receita variável")
	if capturedAmount > 0 {
		require.Equal(t, int64(20000), capturedAmount, "R6 M-01: valor deve ser R$ 200,00 (20000 centavos)")
	}
}

func (s *MecontrolaAgentIntegrationSuite) TestR7CategoriaIncertaPedeEsclarecimento() {
	t := s.T()
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei 35 reais com algo pro trabalho"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("R7 resposta: %s", result.Content)

	require.NotContains(t, result.Content, "**", "R7 RF-23: sem duplo asterisco")

	lower := strings.ToLower(result.Content)
	falseSucessTerms := []string{"registrei", "anotei", "foi registrado", "registrado com sucesso"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "R7 M-03=0: agente nao deve confirmar sucesso sem write real")
	}
}

func (s *MecontrolaAgentIntegrationSuite) TestDiaDaSemanaTerca() {
	t := s.T()
	obs := fake.NewProvider()

	var capturedOccurredAt string
	type expInput struct {
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
		OccurredAt    string `json:"occurredAt"`
	}
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}
	handle := tool.NewTool[expInput, map[string]any]("register_expense", "Registra um lançamento de despesa.", in, out,
		func(_ context.Context, inp expInput) (map[string]any, error) {
			capturedOccurredAt = inp.OccurredAt
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Na terça fui no mercado e gastei R$ 80,00 no pix"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("DiasSemana terça resposta: %s | occurredAt=%s", result.Content, capturedOccurredAt)

	require.NotEmpty(t, capturedOccurredAt, "RF-07: agente deve repassar o texto de data (dia da semana) ao tool")
	require.NotContains(t, result.Content, "**", "RF-23: sem duplo asterisco")
}

func (s *MecontrolaAgentIntegrationSuite) TestSemanaMesPassadoRejeita() {
	t := s.T()
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Semana passada gastei R$ 120,00 no supermercado no pix"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("SemanaMesPassado resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)
	falseSucessTerms := []string{"registrei", "anotei", "foi registrado", "registrado com sucesso"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "RF-08 M-03=0: agente nao deve registrar com data vaga 'semana passada'")
	}
}

func (s *MecontrolaAgentIntegrationSuite) TestMultiItemRF16UmPorVez() {
	t := s.T()
	obs := fake.NewProvider()

	var toolCallCount int
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}
	type expInput struct {
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
	}
	handle := tool.NewTool[expInput, map[string]any]("register_expense", "Registra um lançamento de despesa.", in, out,
		func(_ context.Context, _ expInput) (map[string]any, error) {
			toolCallCount++
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Hoje gastei 30 reais no ônibus e 15 no café"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("RF-16 multi-item resposta: %s | toolCalls=%d", result.Content, toolCallCount)

	lower := strings.ToLower(result.Content)
	falseSuccessTerms := []string{"registrei", "anotei", "foi registrado", "registrado com sucesso", "feito!", "pronto! seu gasto"}
	for _, term := range falseSuccessTerms {
		require.NotContains(t, lower, term, "RF-16 M-05=0: agente nao deve afirmar sucesso de escrita em mensagem multi-item")
	}
	require.NotContains(t, result.Content, "**", "RF-23: sem duplo asterisco")
	t.Logf("RF-16 M-05=0: nenhuma escrita persistida confirmada (tool retornou clarify em todas as %d chamadas)", toolCallCount)
}

func (s *MecontrolaAgentIntegrationSuite) TestCancelamentoNaoEscreve() {
	t := s.T()
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(s.provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 50,00 no mercado hoje no pix"},
			{Role: "assistant", Content: "Em qual categoria você enquadra esse gasto?"},
			{Role: "user", Content: "cancela"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("cancelamento resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)
	falseSucessTerms := []string{"registrei", "anotei", "foi registrado", "registrado com sucesso"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "M-05=0: cancelamento nao deve resultar em escrita")
	}
}
