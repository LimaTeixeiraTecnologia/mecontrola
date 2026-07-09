package workflows

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	CardCreateConfirmWorkflowID = "card-create-confirm"
	StepEvaluateCardCreateID    = "evaluate-card-create"
)

func CardCreateKey(resourceID string) string {
	return resourceID + ":card-create"
}

func BuildCardCreateConfirmWorkflow(idem IdempotentWriter, cards interfaces.CardManager) workflow.Definition[CardCreateState] {
	step := workflow.NewStepFunc(StepEvaluateCardCreateID, buildCardCreateEvalStep(idem, cards))
	return workflow.Definition[CardCreateState]{
		ID:          CardCreateConfirmWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

func buildCardCreateEvalStep(idem IdempotentWriter, cards interfaces.CardManager) func(context.Context, CardCreateState) (workflow.StepOutput[CardCreateState], error) {
	return func(ctx context.Context, state CardCreateState) (workflow.StepOutput[CardCreateState], error) {
		if state.ResumeText == "" {
			state.Awaiting = AwaitingConfirm
			state.ResponseText = buildCardCreateQuestion(state)
			return workflow.StepOutput[CardCreateState]{
				State:  state,
				Status: workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{
					Reason: workflow.SuspendAwaitingInput,
					Prompt: state.ResponseText,
				},
			}, nil
		}

		msg := PendingMessage{Text: state.ResumeText, MessageID: state.IncomingMessageID}
		action := DecideCardCreateConfirmation(state, msg, time.Now().UTC())

		switch action {
		case CardConfirmAccept:
			return executeCreateCard(ctx, state, idem, cards)
		case CardConfirmCancel:
			state.Status = CardCreateStatusCancelled
			state.Awaiting = AwaitingNone
			state.ResponseText = "🚫 Cadastro de cartão cancelado conforme solicitado."
			return workflow.StepOutput[CardCreateState]{State: state, Status: workflow.StepStatusCompleted}, nil
		case CardConfirmReprompt:
			state.ConfirmReprompt++
			state.ResumeText = ""
			state.ResponseText = "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar o cadastro do cartão."
			return workflow.StepOutput[CardCreateState]{
				State:  state,
				Status: workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{
					Reason: workflow.SuspendAwaitingInput,
					Prompt: state.ResponseText,
				},
			}, nil
		case CardConfirmExpire:
			state.Status = CardCreateStatusExpired
			state.Awaiting = AwaitingNone
			state.Expired = true
			state.ResponseText = ""
			return workflow.StepOutput[CardCreateState]{State: state, Status: workflow.StepStatusCompleted}, nil
		case CardConfirmReplay:
			state.Status = CardCreateStatusCompleted
			state.Awaiting = AwaitingNone
			return workflow.StepOutput[CardCreateState]{State: state, Status: workflow.StepStatusCompleted}, nil
		default:
			state.Status = CardCreateStatusCancelled
			state.Awaiting = AwaitingNone
			state.ResponseText = "🚫 Cadastro de cartão cancelado: resposta não reconhecida."
			return workflow.StepOutput[CardCreateState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}
	}
}

func buildCardCreateQuestion(state CardCreateState) string {
	base := fmt.Sprintf("⚠️ Confirma o cadastro do cartão *%s* (%s), vencimento dia %d?", state.Nickname, state.Bank, state.DueDay)
	if state.ClosingDayProvided {
		base = fmt.Sprintf("%s Fechamento dia %d.", base, state.ClosingDay)
	}
	return base + "\n\nResponda *sim* para confirmar ou *não* para cancelar."
}

func executeCreateCard(ctx context.Context, state CardCreateState, idem IdempotentWriter, cards interfaces.CardManager) (workflow.StepOutput[CardCreateState], error) {
	writeFn := IdempotentWriteFn(func(c context.Context) (uuid.UUID, bool, error) {
		ref, err := cards.CreateCard(c, interfaces.NewCard{
			UserID:             state.UserID,
			Nickname:           state.Nickname,
			Bank:               state.Bank,
			DueDay:             state.DueDay,
			ClosingDay:         state.ClosingDay,
			ClosingDayProvided: state.ClosingDayProvided,
		})
		if err != nil {
			return uuid.Nil, false, err
		}
		cardID, parseErr := uuid.Parse(ref.ID)
		if parseErr != nil {
			return uuid.Nil, false, fmt.Errorf("workflows.card_create_confirm: parse card id: %w", parseErr)
		}
		return cardID, false, nil
	})

	var (
		outcome agent.ToolOutcome
		idemErr error
	)
	for attempt := 1; attempt <= maxWriteAttempts; attempt++ {
		_, outcome, idemErr = idem.Execute(ctx, state.UserID, state.MessageID, 0, "create_card", "card", writeFn, isCardCreateDomainError)
		if idemErr == nil || isCardCreateDomainError(idemErr) || !IsTransient(idemErr) || attempt == maxWriteAttempts {
			break
		}
		select {
		case <-ctx.Done():
			state.ResponseText = "Não consegui cadastrar o cartão. Tente novamente em breve."
			return workflow.StepOutput[CardCreateState]{State: state, Status: workflow.StepStatusFailed}, ctx.Err()
		case <-time.After(backoffWithJitter(attempt)):
		}
	}

	if idemErr != nil {
		if isCardCreateDomainError(idemErr) {
			state.Status = CardCreateStatusCompleted
			state.Awaiting = AwaitingNone
			state.ResponseText = cardCreateDomainErrorMessage(idemErr)
			return workflow.StepOutput[CardCreateState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}
		state.ResponseText = "Não consegui cadastrar o cartão. Tente novamente em breve."
		return workflow.StepOutput[CardCreateState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("workflows.card_create_confirm: idempotent_write: %w", idemErr)
	}

	state.Status = CardCreateStatusCompleted
	state.Awaiting = AwaitingNone
	state.ProcessedMessageID = state.MessageID
	if outcome == agent.ToolOutcomeReplay {
		state.ResponseText = fmt.Sprintf("✅ Cartão *%s* já estava cadastrado.", state.Nickname)
	} else {
		state.ResponseText = fmt.Sprintf("✅ Cartão *%s* cadastrado com sucesso.", state.Nickname)
	}
	return workflow.StepOutput[CardCreateState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func isCardCreateDomainError(err error) bool {
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

func cardCreateDomainErrorMessage(err error) string {
	switch {
	case errors.Is(err, carddomain.ErrNicknameConflict):
		return "❌ Já existe um cartão com esse apelido. Escolha outro apelido para cadastrar."
	case errors.Is(err, carddomain.ErrInvalidNickname):
		return "❌ O apelido do cartão precisa ter entre 1 e 32 caracteres."
	case errors.Is(err, carddomain.ErrInvalidDueDay):
		return "❌ O dia de vencimento precisa estar entre 1 e 31."
	case errors.Is(err, carddomain.ErrInvalidClosingDay):
		return "❌ O dia de fechamento precisa estar entre 1 e 31."
	case errors.Is(err, carddomain.ErrInvalidBank):
		return "❌ Informe um banco válido para o cartão."
	default:
		return "❌ Não foi possível cadastrar o cartão."
	}
}
