//go:build integration

package agents

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	agenttools "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/binding"
	agentpersistence "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

type gherkinFaultyLedger struct {
	agentsifaces.TransactionsLedger
	failuresLeft int
	forceErr     error
}

func (l *gherkinFaultyLedger) CreateTransaction(ctx context.Context, in agentsifaces.RawTransaction) (agentsifaces.EntryRef, error) {
	if l.failuresLeft > 0 {
		l.failuresLeft--
		return agentsifaces.EntryRef{}, l.forceErr
	}
	return l.TransactionsLedger.CreateTransaction(ctx, in)
}

type GherkinE2ESuite struct {
	suite.Suite
	ctx      context.Context
	db       *sqlx.DB
	cfg      *configs.Config
	provider llm.Provider

	categoriesModule *categories.CategoriesModule
	reader           agentsifaces.CategoriesReader
	txLedger         agentsifaces.TransactionsLedger
	ledgerRepo       agentusecases.WriteLedgerRepository
	idem             *agentusecases.IdempotentWrite

	pendingEngine workflow.Engine[workflows.PendingEntryState]
	pendingDef    workflow.Definition[workflows.PendingEntryState]

	registerAttempt *agentusecases.RegisterAttempt
	tools           []tool.ToolHandle
}

func TestGherkinE2ESuite(t *testing.T) {
	suite.Run(t, new(GherkinE2ESuite))
}

func (s *GherkinE2ESuite) SetupSuite() {
	s.ctx = context.Background()
	s.provider = buildRealLLMProvider(s.T())

	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	o11y := fake.NewProvider()
	passthrough := func(next http.Handler) http.Handler { return next }

	cfg, err := configs.LoadConfig("../../../..")
	s.Require().NoError(err, "carregar config")
	cfg.TransactionsConfig.Enabled = true
	s.cfg = cfg

	s.categoriesModule = categories.NewCategoriesModule(s.db, o11y, passthrough)

	cardModule, err := card.NewCardModule(s.ctx, s.cfg, o11y, s.db, passthrough, nil, nil)
	s.Require().NoError(err, "card module")

	txModule, err := transactions.NewTransactionsModule(s.cfg, o11y, s.db, cardModule, s.categoriesModule, passthrough)
	s.Require().NoError(err, "transactions module")

	s.reader = binding.NewCategoriesReaderAdapter(
		s.categoriesModule.SearchDictionaryUC,
		s.categoriesModule.ResolveCategoryForWriteUC,
		s.categoriesModule.ListCategoriesUC,
		o11y,
	)
	s.txLedger = binding.NewTransactionsLedgerAdapter(
		txModule.CreateTransactionUC,
		txModule.UpdateTransactionUC,
		txModule.DeleteTransactionUC,
		txModule.ListMonthlyEntriesUC,
		txModule.GetMonthlySummaryUC,
		txModule.GetTransactionUC,
		txModule.GetCardInvoiceUC,
		txModule.SearchTransactionsUC,
		txModule.CreateRecurringTemplateUC,
		o11y,
	)

	s.ledgerRepo = agentpersistence.NewWriteLedgerRepository(s.db, o11y)
	s.idem = agentusecases.NewIdempotentWrite(s.ledgerRepo, o11y)

	store := workflowpg.NewPostgresStore(o11y, s.db)
	s.pendingEngine = workflow.NewEngine[workflows.PendingEntryState](store, o11y)
	s.pendingDef = workflows.BuildPendingEntryWorkflow(s.txLedger, nil, s.reader, gherkinIdemAdapter{uc: s.idem})
	s.registerAttempt = agentusecases.NewRegisterAttempt(s.reader, s.txLedger, s.pendingEngine, s.pendingDef, o11y)

	s.tools = []tool.ToolHandle{
		agenttools.BuildRegisterExpenseTool(s.registerAttempt),
		agenttools.BuildRegisterIncomeTool(s.registerAttempt),
	}
}

type gherkinIdemAdapter struct {
	uc *agentusecases.IdempotentWrite
}

func (a gherkinIdemAdapter) Execute(
	ctx context.Context,
	userID uuid.UUID,
	wamid string,
	itemSeq int,
	operation string,
	resourceKind string,
	write workflows.IdempotentWriteFn,
	isDomainErr workflows.DomainErrorClassifier,
) (uuid.UUID, agent.ToolOutcome, error) {
	res, err := a.uc.Execute(ctx, userID, wamid, itemSeq, operation, resourceKind, agentusecases.WriteFn(write), isDomainErr)
	return res.ResourceID, res.Outcome, err
}

func (s *GherkinE2ESuite) newUser(t *testing.T) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)
	return userID
}

func (s *GherkinE2ESuite) authedCtx(userID uuid.UUID, wamid string) context.Context {
	ctx := auth.WithPrincipal(s.ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	return agent.WithToolInvocationContext(ctx, userID.String(), wamid, 0)
}

func (s *GherkinE2ESuite) confirmPending(userID uuid.UUID, resumeText, incomingMessageID string) (workflow.RunResult[workflows.PendingEntryState], error) {
	key := workflows.PendingEntryKey(userID.String(), "")
	patch, err := json.Marshal(map[string]string{"resumeText": resumeText, "incomingMessageId": incomingMessageID})
	s.Require().NoError(err)
	return s.pendingEngine.Resume(s.authedCtx(userID, incomingMessageID), s.pendingDef, key, patch)
}

func (s *GherkinE2ESuite) findTransaction(userID uuid.UUID) (direction int, amountCents int64, categoryPath, paymentMethod string, found bool) {
	var pm int
	err := s.db.QueryRowContext(s.ctx, `
		SELECT direction, amount_cents, category_path, payment_method
		  FROM mecontrola.transactions
		 WHERE user_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC
		 LIMIT 1`,
		userID,
	).Scan(&direction, &amountCents, &categoryPath, &pm)
	if err != nil {
		return 0, 0, "", "", false
	}
	return direction, amountCents, categoryPath, paymentMethodLabel(pm), true
}

func paymentMethodLabel(code int) string {
	switch code {
	case 1:
		return "pix"
	default:
		return ""
	}
}

func (s *GherkinE2ESuite) countTransactions(userID uuid.UUID) int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *GherkinE2ESuite) runAgent(ctx context.Context, tools []tool.ToolHandle, message string) (agent.Result, error) {
	a := BuildMeControlaAgent(s.provider, tools, nil, fake.NewProvider())
	execCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	return a.Execute(execCtx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: message},
		},
		MaxTokens: 1024,
	})
}

func (s *GherkinE2ESuite) TestG1_SalarioSemClarifyPersisteIncome() {
	userID := s.newUser(s.T())
	wamid := "wamid-g1-" + uuid.NewString()
	ctx := s.authedCtx(userID, wamid)

	result, err := s.runAgent(ctx, s.tools, "Recebi meu salário de R$ 13.874,40")
	s.Require().NoError(err)
	s.T().Logf("G1 resposta inicial: %s", result.Content)

	lower := strings.ToLower(result.Content)
	s.Require().Contains(lower, "confirma", "G1: deve suspender pedindo confirmação única, sem loop de clarify")
	s.Require().Contains(lower, "salário", "G1: resumo deve conter a folha Salário resolvida sem clarify")

	runResult, confirmErr := s.confirmPending(userID, "sim", wamid+"-confirm")
	s.Require().NoError(confirmErr)
	s.Equal(workflows.PendingStatusCompleted, runResult.State.Status)

	direction, amountCents, categoryPath, _, found := s.findTransaction(userID)
	s.Require().True(found, "G1: transação deve estar persistida após confirmação")
	s.Equal(1, direction, "G1 RF-03: direction=income (1) para salário")
	s.Equal(int64(1387440), amountCents, "G1 RF-03: valor deve ser 1387440 centavos")
	s.Contains(categoryPath, "Salário", "G1 RF-03: categoria deve estar sob a raiz Salário")
	s.Contains(categoryPath, "Salário > Salário", "G1 RF-03: deve resolver para a folha 'Salário > Salário', não Décimo Terceiro/outra")
}

func (s *GherkinE2ESuite) TestG2_DecimoTerceiroPreservado() {
	userID := s.newUser(s.T())
	wamid := "wamid-g2-" + uuid.NewString()
	ctx := s.authedCtx(userID, wamid)

	result, err := s.runAgent(ctx, s.tools, "recebi meu 13º salário de R$ 5.000,00")
	s.Require().NoError(err)
	s.T().Logf("G2 resposta inicial: %s", result.Content)

	runResult, confirmErr := s.confirmPending(userID, "sim", wamid+"-confirm")
	s.Require().NoError(confirmErr)
	s.Equal(workflows.PendingStatusCompleted, runResult.State.Status)

	direction, amountCents, categoryPath, _, found := s.findTransaction(userID)
	s.Require().True(found, "G2: transação deve estar persistida após confirmação")
	s.Equal(1, direction, "G2: direction=income para 13º salário")
	s.Equal(int64(500000), amountCents)
	s.Contains(categoryPath, "Décimo Terceiro", "G2 RF-05: 13º salário deve resolver para Décimo Terceiro, nunca para a folha base")
	s.NotContains(categoryPath, "Salário > Salário", "G2 RF-05: 13º NUNCA aponta para a folha de salário-base")
}

func (s *GherkinE2ESuite) TestG3_ConfirmacaoUnicaLivroNoPix() {
	userID := s.newUser(s.T())
	wamid := "wamid-g3-" + uuid.NewString()
	ctx := s.authedCtx(userID, wamid)

	result, err := s.runAgent(ctx, s.tools, "Comprei um livro de R$ 50,00 no pix")
	s.Require().NoError(err)
	s.T().Logf("G3 resposta inicial: %s", result.Content)

	lower := strings.ToLower(result.Content)
	s.Require().Equal(1, strings.Count(lower, "confirma"), "G3 RF-14/RF-15: um único resumo de confirmação, sem pergunta duplicada do LLM")
	s.Require().Contains(lower, "confirma", "G3: resumo deve pedir confirmação uma única vez")
	falseSuccessTerms := []string{"registrei", "anotei", "foi registrado", "registrado com sucesso"}
	for _, term := range falseSuccessTerms {
		s.Require().NotContains(lower, term, "G3 RF-14: LLM não deve confirmar sucesso antes do gate decidir")
	}

	runResult, confirmErr := s.confirmPending(userID, "sim", wamid+"-confirm")
	s.Require().NoError(confirmErr)
	s.Equal(workflows.PendingStatusCompleted, runResult.State.Status, "G3 RF-16: um único 'sim' deve efetivar a escrita, sem segunda pergunta")

	_, amountCents, categoryPath, paymentMethod, found := s.findTransaction(userID)
	s.Require().True(found)
	s.Equal(int64(5000), amountCents)
	s.Contains(categoryPath, "Livros e E-books", "G3: deve gravar em Conhecimento > Livros e E-books")
	s.Equal("pix", paymentMethod, "G3: forma de pagamento pix deve ser preservada")
}

func (s *GherkinE2ESuite) TestG4_CancelamentoDescartaSemGravar() {
	userID := s.newUser(s.T())
	wamid := "wamid-g4-" + uuid.NewString()
	ctx := s.authedCtx(userID, wamid)

	_, err := s.runAgent(ctx, s.tools, "Gastei R$ 40,00 no cinema no débito")
	s.Require().NoError(err)

	runResult, confirmErr := s.confirmPending(userID, "não", wamid+"-cancel")
	s.Require().NoError(confirmErr)
	s.Equal(workflows.PendingStatusCancelled, runResult.State.Status, "G4 RF-18: cancelamento explícito deve descartar sem gravar")

	s.Equal(0, s.countTransactions(userID), "G4: nenhuma transação deve ser persistida após cancelamento")
}

func (s *GherkinE2ESuite) TestG5_NumeroBRLUnicoNaoDisparaMultiplos() {
	userID := s.newUser(s.T())
	wamid := "wamid-g5-" + uuid.NewString()
	ctx := s.authedCtx(userID, wamid)

	result, err := s.runAgent(ctx, s.tools, "Recebi meu salário de R$ 13.874,40")
	s.Require().NoError(err)
	s.T().Logf("G5 resposta: %s", result.Content)

	s.Require().NotContains(result.Content, "um de cada vez",
		"G5 RF-19/RF-20: valor BRL com separador de milhar não deve disparar a mensagem de múltiplos lançamentos")
}

func (s *GherkinE2ESuite) TestG6_DoisLancamentosReaisDisparamOrientacao() {
	const iterations = 3
	const expectedGuidance = "Percebi mais de um lançamento na mesma mensagem. Por segurança, registro um de cada vez — me manda o primeiro (ex.: \"gastei 30 no ônibus\") que eu já cuido dele. 🙂"

	userID := s.newUser(s.T())

	for i := 1; i <= iterations; i++ {
		wamid := "wamid-g6-" + uuid.NewString()
		ctx := s.authedCtx(userID, wamid)

		result, err := s.runAgent(ctx, s.tools, "gastei 30 no ônibus e 15 no café")
		s.Require().NoError(err)
		s.T().Logf("G6 iteração %d resposta: %s", i, result.Content)

		s.Equal(expectedGuidance, result.Content,
			"G6 RF-21: a orientação de múltiplos lançamentos é interceptada deterministicamente por WithMultiItemGuard (internal/agents/application/agents/multi_item_guard.go) ANTES de qualquer chamada ao LLM — byte-exato garantido, sem variabilidade estatística")
	}

	s.Equal(0, s.countTransactions(userID), "G6: nenhuma transação deve ser criada quando múltiplos lançamentos são detectados")
}

func (s *GherkinE2ESuite) TestG7_FalhaTransitoriaRetentaEPersisteUmaVez() {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)

	faultyLedger := &gherkinFaultyLedger{
		TransactionsLedger: s.txLedger,
		failuresLeft:       1,
		forceErr:           &net.OpError{Op: "write", Err: errors.New("connection reset by peer")},
	}

	def := workflows.BuildPendingEntryWorkflow(faultyLedger, nil, s.reader, gherkinIdemAdapter{uc: s.idem})
	store := workflowpg.NewPostgresStore(fake.NewProvider(), s.db)
	engine := workflow.NewEngine[workflows.PendingEntryState](store, fake.NewProvider())
	registerAttempt := agentusecases.NewRegisterAttempt(s.reader, faultyLedger, engine, def, fake.NewProvider())

	tools := []tool.ToolHandle{
		agenttools.BuildRegisterExpenseTool(registerAttempt),
	}

	wamid := "wamid-g7-" + uuid.NewString()
	ctx := s.authedCtx(userID, wamid)

	_, err = s.runAgent(ctx, tools, "Gastei R$ 90,00 na farmácia no débito")
	s.Require().NoError(err)

	key := workflows.PendingEntryKey(userID.String(), "")
	patch, marshalErr := json.Marshal(map[string]string{"resumeText": "sim", "incomingMessageId": wamid + "-confirm"})
	s.Require().NoError(marshalErr)
	runResult, confirmErr := engine.Resume(ctx, def, key, patch)
	s.Require().NoError(confirmErr, "G7 RF-22: falha transitória deve ser retentada automaticamente dentro do mesmo turno")
	s.Equal(workflows.PendingStatusCompleted, runResult.State.Status, "G7: após retry bem-sucedido, usuário recebe confirmação de sucesso")

	s.Equal(1, s.countTransactions(userID), "G7 RF-22/RF-24: lançamento deve persistir exatamente uma vez, sem duplicar")
	s.Equal(0, faultyLedger.failuresLeft, "G7: a falha transitória injetada deve ter sido consumida pelo retry")
}

func (s *GherkinE2ESuite) TestG8_BRLCanonicoQuatroValores() {
	s.Equal("R$ 5.549,76", money.FromCents(554976).BRL())
	s.Equal("R$ 800.000,00", money.FromCents(80000000).BRL())
	s.Equal("R$ 50,00", money.FromCents(5000).BRL())
	s.Equal("R$ 50,50", money.FromCents(5050).BRL())
}

func (s *GherkinE2ESuite) TestG9_FormaDePagamentoObrigatoriaParaDespesaNaoParaReceita() {
	userID := s.newUser(s.T())
	wamid := "wamid-g9-expense-" + uuid.NewString()
	ctx := s.authedCtx(userID, wamid)

	result, err := s.runAgent(ctx, s.tools, "Gastei R$ 150,00 no supermercado ontem")
	s.Require().NoError(err)
	s.T().Logf("G9 despesa sem pagamento: %s", result.Content)

	lower := strings.ToLower(result.Content)
	s.Require().Contains(lower, "como você pagou", "G9 RF-30: despesa sem forma de pagamento deve perguntar com exemplos")
	s.Require().Contains(lower, "dinheiro", "G9 RF-30: prompt deve conter exemplos, incluindo dinheiro")
	s.Require().Contains(lower, "pix", "G9 RF-30: prompt deve conter exemplos, incluindo pix")
	s.Require().NotContains(lower, "confirma", "G9 RF-29: não deve assumir dinheiro e pular direto para confirmação")

	incomeUserID := s.newUser(s.T())
	incomeWamid := "wamid-g9-income-" + uuid.NewString()
	incomeCtx := s.authedCtx(incomeUserID, incomeWamid)

	incomeResult, incomeErr := s.runAgent(incomeCtx, s.tools, "Recebi 300 reais de um freelancer")
	s.Require().NoError(incomeErr)
	s.T().Logf("G9 receita: %s", incomeResult.Content)

	incomeLower := strings.ToLower(incomeResult.Content)
	s.Require().NotContains(incomeLower, "como você pagou", "G9 RF-32: receita não deve perguntar forma de pagamento")
}
