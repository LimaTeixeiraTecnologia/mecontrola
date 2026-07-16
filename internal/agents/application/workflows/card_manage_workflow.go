package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	CardManageWorkflowID  = "card-manage"
	stepCardManageID      = "card-manage"
	CardManageStaleAfter  = 35 * time.Minute
	CardManageReaperBatch = 100
)

func CardManageKey(resourceID, threadID string) string {
	return CorrelationKey(resourceID, threadID, CardManageWorkflowID)
}

func BuildCardManageWorkflow(cards interfaces.CardManager, idem IdempotentWriter) workflow.Definition[CardManageState] {
	step := workflow.NewStepFunc(stepCardManageID, buildCardManageStep(cards, idem))
	return workflow.Definition[CardManageState]{
		ID:          CardManageWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

func BuildCardManageReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, CardManageWorkflowID, CardManageStaleAfter, CardManageReaperBatch, o11y)
}

type cardManageExecFn func(ctx context.Context, state CardManageState, cards interfaces.CardManager, idem IdempotentWriter) (workflow.StepOutput[CardManageState], error)

func cardManageExecMap() map[CardManageOperationKind]cardManageExecFn {
	return map[CardManageOperationKind]cardManageExecFn{
		CardManageOpCreate: executeCardManageCreate,
		CardManageOpEdit:   executeCardManageEdit,
	}
}

func buildCardManageStep(cards interfaces.CardManager, idem IdempotentWriter) func(context.Context, CardManageState) (workflow.StepOutput[CardManageState], error) {
	execMap := cardManageExecMap()
	return func(ctx context.Context, state CardManageState) (workflow.StepOutput[CardManageState], error) {
		if state.ResumeText == "" {
			return cardManageAskConfirmation(ctx, state, cards)
		}

		msg := PendingMessage{Text: state.ResumeText, MessageID: state.IncomingMessageID}
		action := DecideCardManageConfirmation(state, msg, time.Now().UTC())

		switch action {
		case CardManageActionAccept:
			fn, ok := execMap[state.Operation]
			if !ok {
				state.Status = CardManageCancelled
				state.ResponseText = "🚫 Não foi possível identificar a operação de cartão solicitada."
				return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
			}
			return fn(ctx, state, cards, idem)
		case CardManageActionCancel:
			state.Status = CardManageCancelled
			state.ResponseText = "🚫 Operação de cartão cancelada conforme solicitado."
			return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		case CardManageActionReprompt:
			state.ConfirmReprompt++
			state.ResumeText = ""
			state.ResponseText = "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar."
			return workflow.StepOutput[CardManageState]{
				State:  state,
				Status: workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{
					Reason: workflow.SuspendAwaitingInput,
					Prompt: state.ResponseText,
				},
			}, nil
		case CardManageActionExpire:
			state.Status = CardManageExpired
			state.Expired = true
			state.ResponseText = ""
			return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		case CardManageActionReplay:
			state.Status = CardManageCompleted
			return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		default:
			state.Status = CardManageCancelled
			state.ResponseText = "🚫 Operação de cartão cancelada: resposta não reconhecida."
			return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}
	}
}

func cardManageAskConfirmation(ctx context.Context, state CardManageState, cards interfaces.CardManager) (workflow.StepOutput[CardManageState], error) {
	if state.Operation == CardManageOpEdit && !state.PreviousFetched {
		cardID, err := uuid.Parse(state.CardID)
		if err != nil {
			state.Status = CardManageCancelled
			state.ResponseText = "❌ Identificador de cartão inválido."
			return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}
		current, getErr := cards.GetCard(ctx, cardID, state.UserID)
		if getErr != nil {
			if errors.Is(getErr, interfaces.ErrCardNotFound) {
				state.Status = CardManageCancelled
				state.ResponseText = "❌ Não encontrei esse cartão."
				return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
			}
			return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.card_manage.edit: get_card: %w", getErr)
		}
		state.PreviousFetched = true
		state.PreviousNickname = current.Nickname
		state.PreviousBank = current.Bank
		state.PreviousDueDay = current.DueDay
	}

	state.ResponseText = cardManageConfirmQuestion(state)
	if state.SuspendedAt.IsZero() {
		state.SuspendedAt = time.Now().UTC()
	}
	return workflow.StepOutput[CardManageState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: state.ResponseText,
		},
	}, nil
}

func cardManageConfirmQuestion(state CardManageState) string {
	if state.Operation == CardManageOpEdit {
		nickname := state.PreviousNickname
		if state.NicknameProvided {
			nickname = state.Nickname
		}
		bank := state.PreviousBank
		if state.BankProvided {
			bank = state.Bank
		}
		dueDay := state.PreviousDueDay
		if state.DueDayProvided {
			dueDay = state.DueDay
		}
		base := fmt.Sprintf(
			"⚠️ Confirma a atualização do 💳 *%s* (%s), vencimento dia %d, para *%s* (%s), vencimento dia %d?",
			state.PreviousNickname, state.PreviousBank, state.PreviousDueDay,
			nickname, bank, dueDay,
		)
		return base + "\n\nResponda *sim* para confirmar ou *não* para cancelar."
	}

	base := fmt.Sprintf("⚠️ Confirma o cadastro do 💳 *%s* (%s), vencimento dia %d?", state.Nickname, state.Bank, state.DueDay)
	if state.ClosingDayProvided {
		base = fmt.Sprintf("%s Fechamento dia %d.", base, state.ClosingDay)
	}
	return base + "\n\nResponda *sim* para confirmar ou *não* para cancelar."
}

func executeCardManageCreate(ctx context.Context, state CardManageState, cards interfaces.CardManager, idem IdempotentWriter) (workflow.StepOutput[CardManageState], error) {
	writeFn := IdempotentWriteFn(func(c context.Context) (uuid.UUID, bool, error) {
		ref, createErr := cards.CreateCard(c, interfaces.NewCard{
			UserID:             state.UserID,
			Nickname:           state.Nickname,
			Bank:               state.Bank,
			DueDay:             state.DueDay,
			ClosingDay:         state.ClosingDay,
			ClosingDayProvided: state.ClosingDayProvided,
		})
		if createErr != nil {
			return uuid.Nil, false, createErr
		}
		cardID, parseErr := uuid.Parse(ref.ID)
		if parseErr != nil {
			return uuid.Nil, false, fmt.Errorf("agents.card_manage.create: parse card id: %w", parseErr)
		}
		return cardID, false, nil
	})

	var (
		resourceID uuid.UUID
		outcome    agent.ToolOutcome
		idemErr    error
	)
	for attempt := 1; attempt <= maxWriteAttempts; attempt++ {
		resourceID, outcome, idemErr = idem.Execute(ctx, state.UserID, state.MessageID, 0, state.Operation.String(), "card", writeFn, isCardManageDomainError)
		if idemErr == nil || isCardManageDomainError(idemErr) || !IsTransient(idemErr) || attempt == maxWriteAttempts {
			break
		}
		select {
		case <-ctx.Done():
			state.ResponseText = "Não consegui cadastrar o cartão. Tente novamente em breve."
			return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusFailed}, ctx.Err()
		case <-time.After(backoffWithJitter(attempt)):
		}
	}
	if idemErr != nil {
		if isCardManageDomainError(idemErr) {
			state.Status = CardManageCompleted
			state.ResponseText = cardManageDomainErrorMessage(idemErr)
			return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}
		state.ResponseText = "Não consegui cadastrar o cartão. Tente novamente em breve."
		return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.card_manage.create: idempotent_write: %w", idemErr)
	}

	state.Status = CardManageCompleted
	state.ProcessedMessageID = state.MessageID
	state.CardID = resourceID.String()
	seed := messages.NewMotivationSeed(state.MessageID)
	if outcome == agent.ToolOutcomeReplay {
		state.ResponseText = fmt.Sprintf("✅ 💳 *%s* já estava cadastrado.", state.Nickname)
	} else {
		state.ResponseText = fmt.Sprintf("✅ 💳 *%s* cadastrado com sucesso.\n\n%s", state.Nickname, messages.CardManageMotivation(seed))
	}
	return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func executeCardManageEdit(ctx context.Context, state CardManageState, cards interfaces.CardManager, _ IdempotentWriter) (workflow.StepOutput[CardManageState], error) {
	cardID, err := uuid.Parse(state.CardID)
	if err != nil {
		state.Status = CardManageCancelled
		state.ResponseText = "❌ Identificador de cartão inválido."
		return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}

	update := interfaces.CardUpdate{ID: cardID, UserID: state.UserID}
	if state.NicknameProvided {
		nickname := state.Nickname
		update.Nickname = &nickname
	}
	if state.BankProvided {
		bank := state.Bank
		update.Bank = &bank
	}
	if state.DueDayProvided {
		dueDay := state.DueDay
		update.DueDay = &dueDay
	}

	if _, updateErr := cards.UpdateCard(ctx, update); updateErr != nil {
		if isCardManageDomainError(updateErr) {
			state.Status = CardManageCompleted
			state.ResponseText = cardManageDomainErrorMessage(updateErr)
			return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}
		state.ResponseText = "Não consegui atualizar o cartão. Tente novamente em breve."
		return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.card_manage.edit: update_card: %w", updateErr)
	}

	state.Status = CardManageCompleted
	state.ProcessedMessageID = state.MessageID
	state.ResponseText = "✅ 💳 atualizado com sucesso."
	return workflow.StepOutput[CardManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func isCardManageDomainError(err error) bool {
	domainErrs := []error{
		carddomain.ErrNicknameConflict,
		carddomain.ErrInvalidNickname,
		carddomain.ErrInvalidDueDay,
		carddomain.ErrInvalidClosingDay,
		carddomain.ErrInvalidBank,
	}
	for _, de := range domainErrs {
		if errors.Is(err, de) {
			return true
		}
	}
	return false
}

func cardManageDomainErrorMessage(err error) string {
	switch {
	case errors.Is(err, carddomain.ErrNicknameConflict):
		return "❌ Já existe um cartão com esse apelido. Escolha outro apelido."
	case errors.Is(err, carddomain.ErrInvalidNickname):
		return "❌ O apelido do cartão precisa ter entre 1 e 32 caracteres."
	case errors.Is(err, carddomain.ErrInvalidDueDay):
		return "❌ O dia de vencimento precisa estar entre 1 e 31."
	case errors.Is(err, carddomain.ErrInvalidClosingDay):
		return "❌ O dia de fechamento precisa estar entre 1 e 31."
	case errors.Is(err, carddomain.ErrInvalidBank):
		return "❌ Informe um banco válido para o cartão."
	default:
		return "❌ Não foi possível processar a operação de cartão."
	}
}

func ContinueCardManage(
	ctx context.Context,
	engine workflow.Engine[CardManageState],
	def workflow.Definition[CardManageState],
	key string,
	userMessage string,
) (bool, string, error) {
	resumeBytes, err := json.Marshal(map[string]string{"resumeText": userMessage})
	if err != nil {
		return false, "", fmt.Errorf("workflows.card_manage: marshal resume: %w", err)
	}

	result, resumeErr := engine.Resume(ctx, def, key, resumeBytes)
	if result.Status == 0 && resumeErr == nil {
		return false, "", nil
	}

	if resumeErr != nil {
		return true, result.State.ResponseText, fmt.Errorf("workflows.card_manage: resume: %w", resumeErr)
	}

	if result.State.Expired {
		return false, "", nil
	}

	return true, result.State.ResponseText, nil
}
