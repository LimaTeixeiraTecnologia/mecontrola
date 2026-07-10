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
	o11y := fake.NewProvider()
	workflowStore := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.OnboardingState](workflowStore, o11y)

	threads := mempostgres.NewThreadRepository(s.db, o11y)
	messages := mempostgres.NewMessageRepository(s.db, o11y)
	workingMem := mempostgres.NewWorkingMemoryRepository(s.db, o11y)

	def := workflows.BuildOnboardingWorkflow(a, nil, nil, workingMem, threads, messages)
	resolver := usecases.NewResolveOnboardingOrAgent(engine, workflowStore, workingMem, def, o11y)

	return engine, resolver
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

	extract := struct {
		Goal      string  `json:"goal"`
		HasAmount bool    `json:"hasAmount"`
		AmountBRL float64 `json:"amountBRL"`
	}{Goal: "comprar um carro", HasAmount: true, AmountBRL: 50000}
	rawJSON, marshalErr := json.Marshal(extract)
	s.Require().NoError(marshalErr)

	postDeployAgent := agentmocks.NewAgent(s.T())
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

func (s *OnboardingWorkflowPostgresResumeIntegrationSuite) TestInteg_SemTTL_OnboardingPermaneceSuspendedEntrePassos_DecisaoDeDesign() {
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
	s.Equal("suspended", status, "onboarding não possui reaper/TTL wired em module.go por decisão de design: conversação multi-step de duração indeterminada, distinta das confirmações HITL de curta duração")
}

func agentResultRawJSON(rawJSON []byte) agent.Result {
	return agent.Result{RawJSON: rawJSON}
}
