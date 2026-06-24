package valueobjects

import "strings"

type ObjectiveProfile int

const (
	ProfileOrganizeSpending ObjectiveProfile = iota + 1
	ProfilePayoffDebt
	ProfileEmergencyFund
	ProfileInvest
	ProfileSpecificGoal
)

func (p ObjectiveProfile) String() string {
	switch p {
	case ProfileOrganizeSpending:
		return "organize_spending"
	case ProfilePayoffDebt:
		return "payoff_debt"
	case ProfileEmergencyFund:
		return "emergency_fund"
	case ProfileInvest:
		return "invest"
	case ProfileSpecificGoal:
		return "specific_goal"
	default:
		return "unknown"
	}
}

func ParseObjectiveProfile(raw string) (ObjectiveProfile, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "organize_spending":
		return ProfileOrganizeSpending, true
	case "payoff_debt":
		return ProfilePayoffDebt, true
	case "emergency_fund":
		return ProfileEmergencyFund, true
	case "invest":
		return ProfileInvest, true
	case "specific_goal":
		return ProfileSpecificGoal, true
	default:
		return 0, false
	}
}

func ResolveProfile(hint, objective string) ObjectiveProfile {
	if p, ok := ParseObjectiveProfile(hint); ok {
		return p
	}
	if p, ok := classifyByKeyword(objective); ok {
		return p
	}
	return ProfileOrganizeSpending
}

func classifyByKeyword(objective string) (ObjectiveProfile, bool) {
	lower := strings.ToLower(objective)

	debtKeywords := []string{
		"dívida", "divida", "dívidas", "dividas",
		"quitar", "pagar dívida", "pagar divida",
		"crédito", "credito", "empréstimo", "emprestimo",
		"débito", "debito",
	}
	for _, kw := range debtKeywords {
		if strings.Contains(lower, kw) {
			return ProfilePayoffDebt, true
		}
	}

	emergencyKeywords := []string{
		"reserva", "emergência", "emergencia",
		"fundo de emergência", "fundo de emergencia",
		"segurança financeira", "seguranca financeira",
		"proteção", "protecao",
	}
	for _, kw := range emergencyKeywords {
		if strings.Contains(lower, kw) {
			return ProfileEmergencyFund, true
		}
	}

	investKeywords := []string{
		"investir", "investimento", "patrimônio", "patrimonio",
		"aposentadoria", "independência financeira", "independencia financeira",
		"renda passiva", "aplicação", "aplicacao",
		"ações", "acoes", "fundo", "renda variável", "renda variavel",
	}
	for _, kw := range investKeywords {
		if strings.Contains(lower, kw) {
			return ProfileInvest, true
		}
	}

	goalKeywords := []string{
		"meta", "objetivo específico", "objetivo especifico",
		"viagem", "casa", "carro", "casamento", "faculdade",
		"comprar", "juntar", "poupar para",
	}
	for _, kw := range goalKeywords {
		if strings.Contains(lower, kw) {
			return ProfileSpecificGoal, true
		}
	}

	return 0, false
}

type SplitEntryBP struct {
	RootSlug    string
	BasisPoints int
}

func SplitTemplate(p ObjectiveProfile) []SplitEntryBP {
	switch p {
	case ProfilePayoffDebt:
		return []SplitEntryBP{
			{RootSlug: "custo_fixo", BasisPoints: 4500},
			{RootSlug: "conhecimento", BasisPoints: 500},
			{RootSlug: "prazeres", BasisPoints: 1000},
			{RootSlug: "metas", BasisPoints: 2500},
			{RootSlug: "liberdade_financeira", BasisPoints: 1500},
		}
	case ProfileEmergencyFund:
		return []SplitEntryBP{
			{RootSlug: "custo_fixo", BasisPoints: 4000},
			{RootSlug: "conhecimento", BasisPoints: 500},
			{RootSlug: "prazeres", BasisPoints: 1000},
			{RootSlug: "metas", BasisPoints: 1500},
			{RootSlug: "liberdade_financeira", BasisPoints: 3000},
		}
	case ProfileInvest:
		return []SplitEntryBP{
			{RootSlug: "custo_fixo", BasisPoints: 4000},
			{RootSlug: "conhecimento", BasisPoints: 1000},
			{RootSlug: "prazeres", BasisPoints: 1000},
			{RootSlug: "metas", BasisPoints: 1000},
			{RootSlug: "liberdade_financeira", BasisPoints: 3000},
		}
	case ProfileSpecificGoal:
		return []SplitEntryBP{
			{RootSlug: "custo_fixo", BasisPoints: 4000},
			{RootSlug: "conhecimento", BasisPoints: 500},
			{RootSlug: "prazeres", BasisPoints: 1000},
			{RootSlug: "metas", BasisPoints: 3000},
			{RootSlug: "liberdade_financeira", BasisPoints: 1500},
		}
	default:
		return []SplitEntryBP{
			{RootSlug: "custo_fixo", BasisPoints: 4000},
			{RootSlug: "conhecimento", BasisPoints: 1000},
			{RootSlug: "prazeres", BasisPoints: 1500},
			{RootSlug: "metas", BasisPoints: 2000},
			{RootSlug: "liberdade_financeira", BasisPoints: 1500},
		}
	}
}
