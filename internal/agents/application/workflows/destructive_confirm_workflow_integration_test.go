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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type DestructiveConfirmWorkflowIntegrationSuite struct {
	suite.Suite
	ctx   context.Context
	db    *sqlx.DB
	cfg   *configs.Config
	cards agentsifaces.CardManager
}

func TestDestructiveConfirmWorkflowIntegrationSuite(t *testing.T) {
	suite.Run(t, new(DestructiveConfirmWorkflowIntegrationSuite))
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	db, _ := testcontainer.Postgres(s.T())
	s.db = db

	cfg, err := configs.LoadConfig("../../../..")
	s.Require().NoError(err, "carregar config")
	cfg.TransactionsConfig.Enabled = true
	s.cfg = cfg

	o11y := fake.NewProvider()
	passthrough := func(next http.Handler) http.Handler { return next }

	cardModule, err := card.NewCardModule(s.ctx, s.cfg, o11y, s.db, passthrough, nil, nil)
	s.Require().NoError(err, "card module")

	s.cards = binding.NewCardManagerAdapter(
		cardModule.CreateCardUC,
		cardModule.ListCardsUC,
		cardModule.GetCardUC,
		cardModule.ResolveCardByNicknameUC,
		cardModule.CountCardsUC,
		cardModule.BestPurchaseDayUC,
		cardModule.UpdateCardUC,
		cardModule.SoftDeleteCardUC,
		nil,
		cardModule.IsBankRecognizedUC,
		o11y,
	)
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) newUser() uuid.UUID {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)
	return userID
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) newCard(userID uuid.UUID, nickname string) uuid.UUID {
	ref, err := s.cards.CreateCard(s.ctx, agentsifaces.NewCard{
		UserID:             userID,
		Nickname:           nickname,
		Bank:               "Nubank",
		DueDay:             10,
		ClosingDayProvided: false,
	})
	s.Require().NoError(err, "setup: cartão a ser removido")
	id, parseErr := uuid.Parse(ref.ID)
	s.Require().NoError(parseErr)
	return id
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) countActiveCard(cardID uuid.UUID) int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.cards WHERE id = $1 AND deleted_at IS NULL`,
		cardID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) buildEngineAndContinuer() (
	workflow.Engine[workflows.ConfirmState],
	workflow.Definition[workflows.ConfirmState],
	*agentusecases.DestructiveConfirmContinuer,
) {
	o11y := fake.NewProvider()

	store := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.ConfirmState](store, o11y)

	def := workflows.BuildDestructiveConfirmWorkflow(nil, s.cards, nil, nil)
	continuer := agentusecases.NewDestructiveConfirmContinuer(engine, def, o11y)

	return engine, def, continuer
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) pendingDeleteCardState(userID, cardID uuid.UUID) workflows.ConfirmState {
	return workflows.ConfirmState{
		Awaiting:    workflows.AwaitingConfirm,
		Operation:   workflows.OpDeleteCard,
		TargetRef:   cardID.String(),
		TargetKind:  "card",
		ImpactNote:  "Remoção permanente do cartão.",
		UserID:      userID,
		MessageID:   "wamid-start-" + uuid.NewString(),
		SuspendedAt: time.Now().UTC(),
	}
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) TestInteg_RetomadaPosDeploy_ConfirmaExclusao() {
	userID := s.newUser()
	cardID := s.newCard(userID, "Retomada-"+uuid.NewString()[:8])

	o11y := fake.NewProvider()
	store := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.ConfirmState](store, o11y)
	def := workflows.BuildDestructiveConfirmWorkflow(nil, s.cards, nil, nil)

	key := workflows.DestructiveConfirmKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.pendingDeleteCardState(userID, cardID))
	s.Require().NoError(err)

	postDeployStore := workflowpg.NewPostgresStore(fake.NewProvider(), s.db)
	postDeployEngine := workflow.NewEngine[workflows.ConfirmState](postDeployStore, fake.NewProvider())
	postDeployContinuer := agentusecases.NewDestructiveConfirmContinuer(postDeployEngine, def, fake.NewProvider())

	handled, reply, contErr := postDeployContinuer.Continue(s.ctx, userID.String(), "sim")
	s.Require().NoError(contErr)
	s.True(handled, "RF-45: novo Engine/Store apontando para o mesmo Postgres deve retomar o run suspenso")
	s.Contains(reply, "removido")

	s.Equal(0, s.countActiveCard(cardID), "confirmação pós-deploy deve efetivar a exclusão")
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) TestInteg_TTLExpirado_NaoExecutaOperacao() {
	userID := s.newUser()
	cardID := s.newCard(userID, "TTL-"+uuid.NewString()[:8])
	engine, def, continuer := s.buildEngineAndContinuer()

	state := s.pendingDeleteCardState(userID, cardID)
	state.SuspendedAt = time.Now().UTC().Add(-6 * time.Minute)

	key := workflows.DestructiveConfirmKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)

	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), "sim")
	s.Require().NoError(contErr)
	s.False(handled, "RF-45: TTL expirado devolve handled=false para o texto seguir ao ParseInbound")
	s.Empty(reply)

	s.Equal(1, s.countActiveCard(cardID), "confirmação expirada não deve excluir o cartão")

	var status string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.DestructiveConfirmWorkflowID, key,
	).Scan(&status)
	s.Require().NoError(scanErr)
	s.Equal("succeeded", status, "RF-46: run nunca permanece suspended após expiração")
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) TestInteg_Cancelamento_NaoExecutaOperacaoRunEncerra() {
	userID := s.newUser()
	cardID := s.newCard(userID, "Cancel-"+uuid.NewString()[:8])
	engine, def, continuer := s.buildEngineAndContinuer()

	key := workflows.DestructiveConfirmKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.pendingDeleteCardState(userID, cardID))
	s.Require().NoError(err)

	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), "não")
	s.Require().NoError(contErr)
	s.True(handled)
	s.Contains(reply, "cancelada")

	s.Equal(1, s.countActiveCard(cardID), "RF-45: cancelamento não deve excluir o cartão")

	var status string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.DestructiveConfirmWorkflowID, key,
	).Scan(&status)
	s.Require().NoError(scanErr)
	s.Equal("succeeded", status, "RF-46: run nunca permanece suspended após cancelamento")
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) TestInteg_MensagemRepetidaAposConclusao_NaoReexecutaOperacao() {
	userID := s.newUser()
	cardID := s.newCard(userID, "Repeticao-"+uuid.NewString()[:8])
	engine, def, continuer := s.buildEngineAndContinuer()

	key := workflows.DestructiveConfirmKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.pendingDeleteCardState(userID, cardID))
	s.Require().NoError(err)

	handled1, reply1, err1 := continuer.Continue(s.ctx, userID.String(), "sim")
	s.Require().NoError(err1)
	s.True(handled1)
	s.Contains(reply1, "removido")
	s.Equal(0, s.countActiveCard(cardID))

	handled2, reply2, err2 := continuer.Continue(s.ctx, userID.String(), "sim")
	s.Require().NoError(err2)
	s.False(handled2, "RF-45/RF-14: mensagem repetida sem run suspenso deve ser no-op determinístico")
	s.Empty(reply2)

	s.Equal(0, s.countActiveCard(cardID), "repetição pós-conclusão não pode reexecutar a exclusão")
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) TestInteg_ConcorrenciaConfirmacaoSimultanea_UmaUnicaExclusao() {
	userID := s.newUser()
	cardID := s.newCard(userID, "Concorrente-"+uuid.NewString()[:8])
	engine, def, _ := s.buildEngineAndContinuer()

	key := workflows.DestructiveConfirmKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.pendingDeleteCardState(userID, cardID))
	s.Require().NoError(err)

	const goroutines = 5
	type result struct {
		handled bool
		err     error
	}
	results := make(chan result, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			o11y := fake.NewProvider()
			store := workflowpg.NewPostgresStore(o11y, s.db)
			localEngine := workflow.NewEngine[workflows.ConfirmState](store, o11y)
			localContinuer := agentusecases.NewDestructiveConfirmContinuer(localEngine, def, o11y)
			handled, _, contErr := localContinuer.Continue(s.ctx, userID.String(), "sim")
			results <- result{handled: handled, err: contErr}
		}()
	}

	handledCount := 0
	for i := 0; i < goroutines; i++ {
		r := <-results
		if r.err != nil {
			s.Contains(r.err.Error(), "version conflict", "RF-45: única classe de erro aceitável em concorrência é o CAS otimista do kernel — perdedor nunca reexecuta")
			continue
		}
		if r.handled {
			handledCount++
		}
	}

	s.GreaterOrEqual(handledCount, 1, "ao menos uma goroutine deve concluir a confirmação")
	s.Equal(0, s.countActiveCard(cardID), "concorrência sobre a mesma confirmação deve produzir exatamente 1 exclusão")
}

func (s *DestructiveConfirmWorkflowIntegrationSuite) TestInteg_Reaper_RunOrfaoEncerraFailedNuncaSuspended() {
	userID := s.newUser()
	cardID := s.newCard(userID, "Orfao-"+uuid.NewString()[:8])
	engine, def, _ := s.buildEngineAndContinuer()

	key := workflows.DestructiveConfirmKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.pendingDeleteCardState(userID, cardID))
	s.Require().NoError(err)

	_, execErr := s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.workflow_runs SET updated_at = now() - interval '15 minutes' WHERE workflow = $1 AND correlation_key = $2`,
		workflows.DestructiveConfirmWorkflowID, key,
	)
	s.Require().NoError(execErr)

	o11y := fake.NewProvider()
	store := workflowpg.NewPostgresStore(o11y, s.db)
	reaper := workflow.NewStaleSuspendedReaper(store, workflows.DestructiveConfirmWorkflowID, 10*time.Minute, 100, o11y)

	reaped, reapErr := reaper.Reap(s.ctx)
	s.Require().NoError(reapErr)
	s.GreaterOrEqual(reaped, int64(1))

	var status string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.DestructiveConfirmWorkflowID, key,
	).Scan(&status)
	s.Require().NoError(scanErr)
	s.Equal("failed", status, "RF-46: reaper deve marcar run órfão como failed, nunca permanecer suspended")

	s.Equal(1, s.countActiveCard(cardID), "run órfão reapeado não deve efetivar a exclusão")
}
