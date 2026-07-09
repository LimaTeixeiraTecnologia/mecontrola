package tools

import (
	"time"

	budgetsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

func resolveCompetenceReference(monthRefKind string, year, month int) (budgetsvo.Competence, budgetsvo.ClarifyReason, error) {
	if monthRefKind == "" {
		return budgetsvo.Competence{}, budgetsvo.ClarifyNone, nil
	}
	kind, err := budgetsvo.ParseMonthRefKind(monthRefKind)
	if err != nil {
		return budgetsvo.Competence{}, budgetsvo.ClarifyNone, err
	}
	ref := budgetsvo.MonthReference{Kind: kind, Year: year, Month: month}
	loc, locErr := time.LoadLocation("America/Sao_Paulo")
	if locErr != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	return budgetsvo.DecideCompetence(ref, now)
}

func competenceReferenceClarifyPrompt(reason budgetsvo.ClarifyReason) string {
	switch reason {
	case budgetsvo.ClarifyMissingYear:
		return "De qual ano é esse mês? Preciso do ano para consultar certinho."
	case budgetsvo.ClarifyUnrecognized:
		return "Não entendi de qual mês você está falando. Pode me dizer o mês (e o ano, se for diferente do atual)?"
	default:
		return "Pode me confirmar de qual mês você está falando?"
	}
}

func currentCompetenceFallback() string {
	loc, locErr := time.LoadLocation("America/Sao_Paulo")
	if locErr != nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01")
}

func budgetNotFoundOfferPrompt(rawCompetence string) string {
	monthLabel := rawCompetence
	if competence, err := budgetsvo.NewCompetence(rawCompetence); err == nil {
		monthLabel = budgetsvo.FormatCompetencePtBR(competence)
	}
	return "Você ainda não tem um orçamento para *" + monthLabel + "*. Posso te ajudar a criar um?"
}
