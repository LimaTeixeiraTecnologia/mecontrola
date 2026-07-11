package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const onboardingInitialMessage = `🎉 Bem-vindo ao MeControla! 🎉

Estou aqui para te ajudar a organizar suas finanças e conquistar seus objetivos. 💪💰

Vamos começar? Qual é o seu principal objetivo financeiro para este mês?
(por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva; se quiser, já pode me contar o valor da meta, tipo "comprar uma casa, meta de R$ 400.000,00")`

type onboardingInitialGuard struct{}

func NewOnboardingInitialGuard() PreGuard {
	return &onboardingInitialGuard{}
}

func (g *onboardingInitialGuard) Name() string {
	return "onboarding_initial"
}

func (g *onboardingInitialGuard) Inspect(_ context.Context, in agent.Request) GuardDecision {
	if !isInitialOnboardingRequest(lastUserMessageContent(in.Messages)) {
		return GuardDecision{}
	}
	return GuardDecision{
		Handled: true,
		Result: agent.Result{
			Content:     onboardingInitialMessage,
			Mode:        agent.ExecutionModeSync,
			ToolOutcome: agent.ToolOutcomeClarify,
		},
	}
}

func isInitialOnboardingRequest(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" || normalized == "oi" || normalized == "olá" || normalized == "ola" {
		return false
	}
	if containsAnyText(normalized, "gastei", "paguei", "comprei", "recebi", "ganhei", "orçamento", "orcamento", "fatura", "💳", "cartao", "cartão") {
		return false
	}
	return strings.Contains(normalized, "mecontrola") &&
		containsAnyText(normalized, "começar", "comecar", "iniciar", "usar", "ativar", "onboarding")
}

func containsAnyText(s string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(s, value) {
			return true
		}
	}
	return false
}
