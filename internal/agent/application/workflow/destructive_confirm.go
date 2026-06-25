package workflow

import (
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const DestructiveConfirmWorkflowID = "destructive_confirm"

type DestructiveConfirmDeps struct {
	Authorize      steps.ConfirmAuthorizeFunc
	Replay         steps.ConfirmReplayFunc
	Policy         steps.ConfirmPolicyFunc
	AuditBegin     steps.ConfirmAuditBeginFunc
	OnSettle       steps.ConfirmOnSettleRegistered
	Searcher       tools.TransactionSearcher
	Targets        map[confirmation.OperationKind]steps.TargetResolver
	Executors      map[confirmation.OperationKind]steps.DestructiveExecutor
	TTL            time.Duration
	DenyReply      string
	ReplayReply    string
	AuditFailReply string
	RetryPolicy    platform.RetryPolicy
	MaxAttempts    int
	Observability  observability.Observability
}

func NewDestructiveConfirmDefinition(deps DestructiveConfirmDeps) platform.Definition[confirmation.ConfirmState] {
	root := platform.Sequence("destructive_confirm_seq",
		steps.NewConfirmAuthorize(deps.Authorize, deps.DenyReply),
		steps.NewConfirmReplay(deps.Replay),
		steps.NewConfirmPolicy(deps.Policy),
		platform.Retry(steps.NewConfirmAuditBegin(deps.AuditBegin, deps.OnSettle, deps.ReplayReply, deps.AuditFailReply), deps.RetryPolicy),
		platform.Retry(steps.NewResolveCandidates(steps.ResolveCandidatesDeps{Searcher: deps.Searcher}), deps.RetryPolicy),
		steps.NewSelectTargetWithObservability(deps.Observability),
		platform.Retry(steps.NewPrepareTarget(steps.PrepareTargetDeps{Targets: deps.Targets}), deps.RetryPolicy),
		steps.NewConfirmGateWithObservability(deps.TTL, deps.Observability),
		platform.Retry(steps.NewExecuteDestructive(steps.ExecuteDestructiveDeps{Executors: deps.Executors}), deps.RetryPolicy),
		steps.NewConfirmFormat(formatDestructiveReply),
	)
	if deps.MaxAttempts <= 0 {
		deps.MaxAttempts = 1
	}
	return platform.Definition[confirmation.ConfirmState]{
		ID:          DestructiveConfirmWorkflowID,
		Root:        root,
		Durable:     true,
		MaxAttempts: deps.MaxAttempts,
	}
}

func formatDestructiveReply(state confirmation.ConfirmState) string {
	switch state.OperationKind {
	case confirmation.OperationDeleteLast:
		return "✅ Último lançamento apagado com sucesso."
	case confirmation.OperationEditLast:
		return "✅ Último lançamento atualizado com sucesso."
	case confirmation.OperationDeleteCard:
		return "✅ Cartão removido com sucesso."
	case confirmation.OperationBudgetCommit:
		return "✅ Orçamento configurado e ativado com sucesso."
	case confirmation.OperationDeleteByRef:
		return fmt.Sprintf("🗑️ *Lançamento excluído!*\n%s — %s", tools.FormatBRL(state.TargetAmountCents), state.TargetDescription)
	case confirmation.OperationEditByRef:
		return fmt.Sprintf("✏️ *Lançamento atualizado!*\nDe %s para *%s* — %s", tools.FormatBRL(state.TargetAmountCents), tools.FormatBRL(state.NewAmountCents), state.TargetDescription)
	default:
		return "✅ Operação concluída com sucesso."
	}
}
