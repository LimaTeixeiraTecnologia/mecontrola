//go:build integration

package workflows_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/binding"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	mempostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
)

type BudgetCreationWorkflowIntegrationSuite struct {
	suite.Suite
	ctx     context.Context
	db      *sqlx.DB
	cfg     *configs.Config
	planner agentsifaces.BudgetPlanner
}

func TestBudgetCreationWorkflowIntegrationSuite(t *testing.T) {
	suite.Run(t, new(BudgetCreationWorkflowIntegrationSuite))
}

func (s *BudgetCreationWorkflowIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	db, _ := testcontainer.Postgres(s.T())
	s.db = db

	cfg, err := configs.LoadConfig("../../../..")
	s.Require().NoError(err, "carregar config")
	s.cfg = cfg

	o11y := fake.NewProvider()
	passthrough := func(next http.Handler) http.Handler { return next }

	categoriesModule := categories.NewCategoriesModule(s.db, o11y, passthrough)
	budgetsModule, err := budgets.NewBudgetsModule(s.cfg, o11y, s.db, categoriesModule, passthrough, nil, nil)
	s.Require().NoError(err, "budgets module")

	s.planner = binding.NewBudgetPlannerAdapter(
		budgetsModule.CreateBudgetUC,
		budgetsModule.DeleteDraftBudgetUC,
		budgetsModule.ActivateBudgetUC,
		budgetsModule.CreateRecurrenceUC,
		budgetsModule.EditCategoryPercentageUC,
		budgetsModule.GetMonthlySummaryUC,
		budgetsModule.ListAlertsUC,
		budgetsModule.SuggestAllocationUC,
		o11y,
	)
}

func (s *BudgetCreationWorkflowIntegrationSuite) newUser() uuid.UUID {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)
	return userID
}

func (s *BudgetCreationWorkflowIntegrationSuite) countBudgets(userID uuid.UUID, competence string) int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.budgets WHERE user_id = $1 AND competence = $2`,
		userID, competence,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

const (
	budgetStateDraft  = 1
	budgetStateActive = 2
)

func (s *BudgetCreationWorkflowIntegrationSuite) budgetState(userID uuid.UUID, competence string) int {
	var state int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT state FROM mecontrola.budgets WHERE user_id = $1 AND competence = $2`,
		userID, competence,
	).Scan(&state)
	s.Require().NoError(err)
	return state
}

func (s *BudgetCreationWorkflowIntegrationSuite) buildEngineAndContinuer() (
	workflow.Engine[workflows.BudgetCreationState],
	workflow.Definition[workflows.BudgetCreationState],
	*agentusecases.BudgetCreationContinuer,
) {
	o11y := fake.NewProvider()

	store := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.BudgetCreationState](store, o11y)

	threads := mempostgres.NewThreadRepository(s.db, o11y)
	runs := agentpostgres.NewRunStore(s.db)

	a := agentmocks.NewAgent(s.T())

	def := workflows.BuildBudgetCreationWorkflow(a, s.planner)
	continuer := agentusecases.NewBudgetCreationContinuer(engine, def, threads, runs, o11y)

	return engine, def, continuer
}

func (s *BudgetCreationWorkflowIntegrationSuite) confirmState(userID uuid.UUID, competence string) workflows.BudgetCreationState {
	allocations := map[string]int{
		"expense.custo_fixo":           4000,
		"expense.conhecimento":         1000,
		"expense.prazeres":             1000,
		"expense.metas":                1000,
		"expense.liberdade_financeira": 3000,
	}
	return workflows.BudgetCreationState{
		Status:      workflows.BudgetCreationActive,
		UserID:      userID,
		Competence:  competence,
		TotalCents:  350000,
		Allocations: allocations,
		Awaiting:    workflows.AwaitingBudgetConfirm,
		MessageID:   "wamid-start-" + uuid.NewString(),
	}
}

func (s *BudgetCreationWorkflowIntegrationSuite) TestInteg_RetroativoConfirmadoCriaEAtiva() {
	userID := s.newUser()
	engine, def, continuer := s.buildEngineAndContinuer()
	competence := "2020-01"

	key := workflows.BudgetCreationKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.confirmState(userID, competence))
	s.Require().NoError(err)

	wamid := "wamid-integ-" + uuid.NewString()
	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), "sim", wamid)
	s.Require().NoError(contErr)
	s.True(handled)
	s.Contains(reply, "criado e ativado")
	s.Contains(reply, "janeiro de 2020", "RF-05: competência retroativa sem limite de antiguidade deve ser aceita e exibida por extenso")

	s.Equal(1, s.countBudgets(userID, competence))
	s.Equal(budgetStateActive, s.budgetState(userID, competence))
}

func (s *BudgetCreationWorkflowIntegrationSuite) TestInteg_Unicidade_NaoDuplicaOrcamentoExistente() {
	userID := s.newUser()
	competence := "2026-03"

	_, createErr := s.planner.CreateBudget(s.ctx, agentsifaces.DraftBudget{
		UserID:     userID,
		Competence: competence,
		TotalCents: 100000,
		Allocations: []agentsifaces.AllocationDraft{
			{RootSlug: "expense.custo_fixo", BasisPoints: 4000},
			{RootSlug: "expense.conhecimento", BasisPoints: 1000},
			{RootSlug: "expense.prazeres", BasisPoints: 1000},
			{RootSlug: "expense.metas", BasisPoints: 1000},
			{RootSlug: "expense.liberdade_financeira", BasisPoints: 3000},
		},
	})
	s.Require().NoError(createErr, "setup: orçamento pré-existente para a competência")

	engine, def, continuer := s.buildEngineAndContinuer()
	key := workflows.BudgetCreationKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.confirmState(userID, competence))
	s.Require().NoError(err)

	wamid := "wamid-integ-" + uuid.NewString()
	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), "sim", wamid)
	s.Require().NoError(contErr, "RF-11: conflito de unicidade é outcome de domínio, run conclui sem erro de infra")
	s.True(handled)
	s.Contains(reply, "Já existe um orçamento")

	s.Equal(1, s.countBudgets(userID, competence), "RF-11: nenhuma duplicata deve ser criada após o conflito")
}

func (s *BudgetCreationWorkflowIntegrationSuite) TestInteg_DraftDeMesFuturo_TratadoComoExistente() {
	userID := s.newUser()
	competence := "2027-12"

	_, createErr := s.planner.CreateBudget(s.ctx, agentsifaces.DraftBudget{
		UserID:     userID,
		Competence: competence,
		TotalCents: 50000,
		Allocations: []agentsifaces.AllocationDraft{
			{RootSlug: "expense.custo_fixo", BasisPoints: 4000},
			{RootSlug: "expense.conhecimento", BasisPoints: 1000},
			{RootSlug: "expense.prazeres", BasisPoints: 1000},
			{RootSlug: "expense.metas", BasisPoints: 1000},
			{RootSlug: "expense.liberdade_financeira", BasisPoints: 3000},
		},
	})
	s.Require().NoError(createErr, "setup: draft futuro pré-existente (state=1)")
	s.Equal(budgetStateDraft, s.budgetState(userID, competence))

	engine, def, continuer := s.buildEngineAndContinuer()
	key := workflows.BudgetCreationKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.confirmState(userID, competence))
	s.Require().NoError(err)

	wamid := "wamid-integ-" + uuid.NewString()
	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), "sim", wamid)
	s.Require().NoError(contErr)
	s.True(handled)
	s.Contains(reply, "Já existe um orçamento")

	s.Equal(1, s.countBudgets(userID, competence), "RF-12: draft futuro não deve ser duplicado")
	s.Equal(budgetStateDraft, s.budgetState(userID, competence), "RF-12: draft futuro não deve ser ativado por este fluxo")
}

func (s *BudgetCreationWorkflowIntegrationSuite) TestInteg_ConfirmacaoNegada_NaoCriaOrcamentoELimpaEstado() {
	userID := s.newUser()
	engine, def, continuer := s.buildEngineAndContinuer()
	competence := "2026-07"

	key := workflows.BudgetCreationKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.confirmState(userID, competence))
	s.Require().NoError(err)

	wamid := "wamid-integ-" + uuid.NewString()
	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), "não", wamid)
	s.Require().NoError(contErr)
	s.True(handled)
	s.Contains(reply, "cancelada")

	s.Equal(0, s.countBudgets(userID, competence), "RF-09: negação não deve criar orçamento")

	var status string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.BudgetCreationWorkflowID, key,
	).Scan(&status)
	s.Require().NoError(scanErr)
	s.Equal("succeeded", status, "RF-07: run nunca permanece suspended após cancelamento")
}

func (s *BudgetCreationWorkflowIntegrationSuite) TestInteg_TTLExpirado_CancelaSemCriarOrcamentoEEncerraRun() {
	userID := s.newUser()
	engine, def, continuer := s.buildEngineAndContinuer()
	competence := "2026-08"

	state := s.confirmState(userID, competence)
	state.SuspendedAt = time.Now().UTC().Add(-31 * time.Minute)

	key := workflows.BudgetCreationKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)

	wamid := "wamid-integ-ttl-" + uuid.NewString()
	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), "sim", wamid)
	s.Require().NoError(contErr)
	s.False(handled, "RF-07: TTL expirado devolve handled=false para o texto seguir ao ParseInbound")
	s.Empty(reply)

	s.Equal(0, s.countBudgets(userID, competence), "TTL expirado não deve criar orçamento")

	var status string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.BudgetCreationWorkflowID, key,
	).Scan(&status)
	s.Require().NoError(scanErr)
	s.Equal("succeeded", status, "RF-07: run nunca permanece suspended após expiração")
}

func (s *BudgetCreationWorkflowIntegrationSuite) TestInteg_ReaperEncerraRunOrfaoAlemDoStaleAfter() {
	userID := s.newUser()
	engine, def, _ := s.buildEngineAndContinuer()
	competence := "2026-09"

	key := workflows.BudgetCreationKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.confirmState(userID, competence))
	s.Require().NoError(err)

	_, execErr := s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.workflow_runs SET updated_at = now() - interval '40 minutes' WHERE workflow = $1 AND correlation_key = $2`,
		workflows.BudgetCreationWorkflowID, key,
	)
	s.Require().NoError(execErr)

	o11y := fake.NewProvider()
	store := workflowpg.NewPostgresStore(o11y, s.db)
	reaper := workflows.BuildBudgetCreationReaper(store, o11y)

	reaped, reapErr := reaper.Reap(s.ctx)
	s.Require().NoError(reapErr)
	s.GreaterOrEqual(reaped, int64(1))

	var status string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.BudgetCreationWorkflowID, key,
	).Scan(&status)
	s.Require().NoError(scanErr)
	s.Equal("failed", status, "reaper deve marcar run orfao como failed")

	s.Equal(0, s.countBudgets(userID, competence))
}
