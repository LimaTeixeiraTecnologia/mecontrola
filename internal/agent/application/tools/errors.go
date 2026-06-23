package tools

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrCategoryAmbiguous         = errors.New("agent.intent_router: categoria ambigua")
	ErrCategoryNeedsConfirmation = errors.New("agent.intent_router: categoria precisa de confirmacao")
	ErrCategoryNotFound          = errors.New("agent.intent_router: categoria nao encontrada")
	ErrCategoryHintMissing       = errors.New("agent.intent_router: sem hint de categoria")
	ErrRecurringInvalidDay       = errors.New("agent.intent_router: dia da recorrencia invalido")
)

var (
	ErrAgentCardNotFound                 = errors.New("agent.intent_router: cartao nao encontrado")
	ErrAgentCardAmbiguous                = errors.New("agent.intent_router: cartao ambiguo")
	ErrCategoryPercentageUnknownCategory = errors.New("agent.intent_router: categoria de orcamento desconhecida")
	ErrCategoryPercentageNoBudget        = errors.New("agent.intent_router: orcamento ativo inexistente")
)

type CategoryAmbiguousError struct {
	Hint       string
	Candidates []string
}

func (e *CategoryAmbiguousError) Error() string {
	return fmt.Sprintf("%s: hint=%q candidatos=%s", ErrCategoryAmbiguous.Error(), e.Hint, strings.Join(e.Candidates, ", "))
}

func (e *CategoryAmbiguousError) Unwrap() error {
	return ErrCategoryAmbiguous
}

type CategoryNeedsConfirmationError struct {
	Hint       string
	Candidates []string
}

func (e *CategoryNeedsConfirmationError) Error() string {
	return fmt.Sprintf("%s: hint=%q candidatos=%s", ErrCategoryNeedsConfirmation.Error(), e.Hint, strings.Join(e.Candidates, ", "))
}

func (e *CategoryNeedsConfirmationError) Unwrap() error {
	return ErrCategoryNeedsConfirmation
}
