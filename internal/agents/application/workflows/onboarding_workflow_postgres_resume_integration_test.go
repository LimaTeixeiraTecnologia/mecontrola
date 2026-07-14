//go:build integration

package workflows_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	mempostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type OnboardingWorkflowPostgresResumeIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestOnboardingWorkflowPostgresResumeIntegrationSuite(t *testing.T) {
	suite.Run(t, new(OnboardingWorkflowPostgresResumeIntegrationSuite))
}

func (s *OnboardingWorkflowPostgresResumeIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
}

func (s *OnboardingWorkflowPostgresResumeIntegrationSuite) newUser() uuid.UUID {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)
	return userID
}

func (s *OnboardingWorkflowPostgresResumeIntegrationSuite) buildResolver(a *agentmocks.Agent) (
	workflow.Engine[workflows.OnboardingState],
	*usecases.ResolveOnboardingOrAgent,
) {
	return s.buildResolverWithBudgets(a, nil)
}

func (s *OnboardingWorkflowPostgresResumeIntegrationSuite) buildResolverWithBudgets(a *agentmocks.Agent, budgets interfaces.BudgetPlanner) (
	workflow.Engine[workflows.OnboardingState],
	*usecases.ResolveOnboardingOrAgent,
) {
	o11y := fake.NewProvider()
	workflowStore := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.OnboardingState](workflowStore, o11y)

	threads := mempostgres.NewThreadRepository(s.db, o11y)
	messages := mempostgres.NewMessageRepository(s.db, o11y)
	workingMem := mempostgres.NewWorkingMemoryRepository(s.db, o11y)

	def := workflows.BuildOnboardingWorkflow(a, nil, budgets, workingMem, threads, messages, nil)
	resolver := usecases.NewResolveOnboardingOrAgent(engine, workflowStore, workingMem, def, o11y)

	return engine, resolver
}

func (s *OnboardingWorkflowPostgresResumeIntegrationSuite) newBudgetPlannerMock(income int64) *interfacemocks.BudgetPlanner {
	m := interfacemocks.NewBudgetPlanner(s.T())
	suggestion := []interfaces.AllocationCents{
		{RootSlug: "expense.custo_fixo", BasisPoints: 4000, PlannedCents: income * 4000 / 10000},
		{RootSlug: "expense.conhecimento", BasisPoints: 1000, PlannedCents: income * 1000 / 10000},
		{RootSlug: "expense.prazeres", BasisPoints: 1000, PlannedCents: income * 1000 / 10000},
		{RootSlug: "expense.metas", BasisPoints: 1000, PlannedCents: income * 1000 / 10000},
		{RootSlug: "expense.liberdade_financeira", BasisPoints: 3000, PlannedCents: income * 3000 / 10000},
	}
	m.EXPECT().
		SuggestAllocation(mock.Anything, income, mock.Anything).
		Return(suggestion, nil).
		Maybe()
	m.EXPECT().
		GetMonthlySummary(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
		Return(interfaces.BudgetSummary{}, interfaces.ErrBudgetNotFound).
		Maybe()
	m.EXPECT().
		CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
		Return(interfaces.BudgetRef{}, nil).
		Maybe()
	return m
}

func (s *OnboardingWorkflowPostgresResumeIntegrationSuite) TestInteg_RetomadaPosDeploy_MergePatchAntesDoParse() {
	userID := s.newUser()
	peer := "peer-onboarding-" + uuid.NewString()

	startAgent := agentmocks.NewAgent(s.T())
	_, startResolver := s.buildResolver(startAgent)

	startResult, err := startResolver.Execute(s.ctx, userID.String(), peer, "")
	s.Require().NoError(err)
	s.True(startResult.Handled)
	s.NotEmpty(startResult.Message, "RF-15: estado de espera persistido antes de pedir a primeira pergunta")

	postDeployAgent := agentmocks.NewAgent(s.T())
	extract := struct {
		Goal      string  `json:"goal"`
		HasAmount bool    `json:"hasAmount"`
		AmountBRL float64 `json:"amountBRL"`
	}{Goal: "comprar um carro", HasAmount: true, AmountBRL: 50000}
	rawJSON, marshalErr := json.Marshal(extract)
	s.Require().NoError(marshalErr)
	postDeployAgent.EXPECT().
		Execute(mock.Anything, mock.Anything).
		Return(agentResultRawJSON(rawJSON), nil).
		Once()
	_, postDeployResolver := s.buildResolver(postDeployAgent)

	resumeResult, resumeErr := postDeployResolver.Execute(s.ctx, userID.String(), peer, "quero comprar um carro, meta de R$ 50.000,00")
	s.Require().NoError(resumeErr)
	s.True(resumeResult.Handled, "RF-45: novo Engine/Store apontando para o mesmo Postgres deve retomar o run suspenso")
	s.False(resumeResult.Done, "onboarding tem múltiplos passos; primeira retomada não conclui o fluxo")

	var status, state string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status, state::text FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.OnboardingWorkflowID, userID.String(),
	).Scan(&status, &state)
	s.Require().NoError(scanErr)
	s.Equal("suspended", status, "onboarding é multi-step: run permanece suspended aguardando o próximo slot, nunca fica orphan sem prompt")
	s.Contains(state, "comprar um carro", "RF-15: merge-patch aplicado sobre o snapshot preserva o estado rico acumulado (goal extraído)")
}

func (s *OnboardingWorkflowPostgresResumeIntegrationSuite) TestInteg_DentroDoTTL_OnboardingPermaneceSuspendedEntrePassos() {
	userID := s.newUser()
	peer := "peer-onboarding-ttl-" + uuid.NewString()

	startAgent := agentmocks.NewAgent(s.T())
	_, resolver := s.buildResolver(startAgent)

	_, err := resolver.Execute(s.ctx, userID.String(), peer, "")
	s.Require().NoError(err)

	_, execErr := s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.workflow_runs SET updated_at = now() - interval '3 hours' WHERE workflow = $1 AND correlation_key = $2`,
		workflows.OnboardingWorkflowID, userID.String(),
	)
	s.Require().NoError(execErr)

	var status string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.OnboardingWorkflowID, userID.String(),
	).Scan(&status)
	s.Require().NoError(scanErr)
	s.Equal("suspended", status, "onboarding tem reaper dedicado com TTL de 7 dias (workflows.OnboardingStaleAfter, D-12/ADR-005); 3 horas de inatividade não expira o run, distinto das confirmações HITL de curta duração")
}

func (s *OnboardingWorkflowPostgresResumeIntegrationSuite) TestInteg_ReviewAwaitPersonalize_SuspendeERetomaComMergePatch() {
	userID := s.newUser()
	peer := "peer-onboarding-personalize-" + uuid.NewString()
	const income int64 = 1350000

	startAgent := agentmocks.NewAgent(s.T())
	budgets := s.newBudgetPlannerMock(income)
	_, startResolver := s.buildResolverWithBudgets(startAgent, budgets)

	startResult, err := startResolver.Execute(s.ctx, userID.String(), peer, "")
	s.Require().NoError(err)
	s.True(startResult.Handled)

	goalAgent := agentmocks.NewAgent(s.T())
	goalExtract := struct {
		Goal      string  `json:"goal"`
		HasAmount bool    `json:"hasAmount"`
		AmountBRL float64 `json:"amountBRL"`
	}{Goal: "juntar reserva de emergencia", HasAmount: true, AmountBRL: 10000}
	goalRawJSON, marshalErr := json.Marshal(goalExtract)
	s.Require().NoError(marshalErr)
	goalAgent.EXPECT().
		Execute(mock.Anything, mock.Anything).
		Return(agentResultRawJSON(goalRawJSON), nil).
		Once()
	_, goalResolver := s.buildResolverWithBudgets(goalAgent, budgets)

	goalResult, goalErr := goalResolver.Execute(s.ctx, userID.String(), peer, "quero juntar uma reserva de emergencia")
	s.Require().NoError(goalErr)
	s.True(goalResult.Handled)
	s.False(goalResult.Done)

	budgetAgent := agentmocks.NewAgent(s.T())
	budgetExtract := struct {
		AmountBRL float64 `json:"amountBRL"`
	}{AmountBRL: 13500}
	budgetRawJSON, budgetMarshalErr := json.Marshal(budgetExtract)
	s.Require().NoError(budgetMarshalErr)
	budgetAgent.EXPECT().
		Execute(mock.Anything, mock.Anything).
		Return(agentResultRawJSON(budgetRawJSON), nil).
		Once()
	_, budgetResolver := s.buildResolverWithBudgets(budgetAgent, budgets)

	budgetResult, budgetErr := budgetResolver.Execute(s.ctx, userID.String(), peer, "R$ 13.500,00")
	s.Require().NoError(budgetErr)
	s.True(budgetResult.Handled)
	s.False(budgetResult.Done)

	personalizeIntentAgent := agentmocks.NewAgent(s.T())
	intentExtract := struct {
		Action    string `json:"action"`
		MixedUnit bool   `json:"mixed_unit"`
	}{Action: "personalize", MixedUnit: false}
	intentRawJSON, intentMarshalErr := json.Marshal(intentExtract)
	s.Require().NoError(intentMarshalErr)
	personalizeIntentAgent.EXPECT().
		Execute(mock.Anything, mock.Anything).
		Return(agentResultRawJSON(intentRawJSON), nil).
		Once()
	_, personalizeIntentResolver := s.buildResolverWithBudgets(personalizeIntentAgent, budgets)

	personalizeResult, personalizeErr := personalizeIntentResolver.Execute(s.ctx, userID.String(), peer, "não, prefiro escolher eu mesmo")
	s.Require().NoError(personalizeErr)
	s.True(personalizeResult.Handled, "RF-15: novo Engine/Store apontando para o mesmo Postgres deve retomar o run suspenso")
	s.False(personalizeResult.Done, "sub-estado reviewAwaitPersonalize suspende aguardando os valores por categoria")

	var statusAfterPersonalize, stateAfterPersonalize string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status, state::text FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.OnboardingWorkflowID, userID.String(),
	).Scan(&statusAfterPersonalize, &stateAfterPersonalize)
	s.Require().NoError(scanErr)
	s.Equal("suspended", statusAfterPersonalize, "sub-estado reviewAwaitPersonalize permanece suspended aguardando os valores por categoria")
	s.Contains(stateAfterPersonalize, `"reviewAwait": 3`, "snapshot persiste o sub-estado reviewAwaitPersonalize (valor enumerado 3, ver reviewAwaitKind) antes de pedir os valores")
	s.Contains(stateAfterPersonalize, "juntar reserva de emergencia", "merge-patch preserva o Goal extraido em passo anterior")
	s.Contains(stateAfterPersonalize, `"monthlyBudgetCents": 1350000`, "merge-patch preserva o MonthlyBudgetCents extraido em passo anterior")

	valuesAgent := agentmocks.NewAgent(s.T())
	valuesExtract := struct {
		Action              string  `json:"action"`
		CustoFixo           float64 `json:"custo_fixo"`
		Conhecimento        float64 `json:"conhecimento"`
		Prazeres            float64 `json:"prazeres"`
		Metas               float64 `json:"metas"`
		LiberdadeFinanceira float64 `json:"liberdade_financeira"`
	}{Action: "reais", CustoFixo: 5400, Conhecimento: 1350, Prazeres: 1350, Metas: 1350, LiberdadeFinanceira: 4050}
	valuesRawJSON, valuesMarshalErr := json.Marshal(valuesExtract)
	s.Require().NoError(valuesMarshalErr)
	valuesAgent.EXPECT().
		Execute(mock.Anything, mock.Anything).
		Return(agentResultRawJSON(valuesRawJSON), nil).
		Once()
	_, valuesResolver := s.buildResolverWithBudgets(valuesAgent, budgets)

	valuesResult, valuesErr := valuesResolver.Execute(s.ctx, userID.String(), peer,
		"Custo Fixo R$ 5.400, Conhecimento R$ 1.350, Prazeres R$ 1.350, Metas R$ 1.350, Liberdade Financeira R$ 4.050")
	s.Require().NoError(valuesErr)
	s.True(valuesResult.Handled)
	s.False(valuesResult.Done, "onboarding segue para reviewAwaitConfirm (resumo) antes de concluir")

	var statusAfterValues, stateAfterValues string
	scanErr = s.db.QueryRowContext(s.ctx,
		`SELECT status, state::text FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.OnboardingWorkflowID, userID.String(),
	).Scan(&statusAfterValues, &stateAfterValues)
	s.Require().NoError(scanErr)
	s.Equal("suspended", statusAfterValues, "apos ativar a distribuicao personalizada, o run suspende no proximo slot (reviewAwaitConfirm)")
	s.NotContains(stateAfterValues, `"reviewAwait": 3`, "run avancou e deixou o sub-estado reviewAwaitPersonalize")
	s.Contains(stateAfterValues, `"reviewAwait": 2`, "run avancou para reviewAwaitConfirm (valor enumerado 2) apos ativar a distribuicao")
	s.Contains(stateAfterValues, `"monthlyBudgetCents": 1350000`, "merge-patch continua preservando o estado acumulado apos a segunda retomada")
}

func agentResultRawJSON(rawJSON []byte) agent.Result {
	return agent.Result{RawJSON: rawJSON}
}
