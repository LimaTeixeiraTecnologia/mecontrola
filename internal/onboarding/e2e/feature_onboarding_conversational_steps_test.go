//go:build e2e

package e2e_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	agentworkflow "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	identityinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	onbinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type e2eWelcomeMarker struct {
	uc *usecases.MarkWelcomeSent
}

func (b *e2eWelcomeMarker) Mark(ctx context.Context, userID uuid.UUID) (bool, error) {
	out, err := b.uc.Execute(ctx, usecases.MarkWelcomeSentInput{UserID: userID})
	if err != nil {
		return false, err
	}
	return out.AlreadySent, nil
}

type e2eObjectiveSaver struct {
	uc *usecases.SaveOnboardingObjective
}

func (b *e2eObjectiveSaver) Save(ctx context.Context, userID uuid.UUID, objective string) error {
	_, err := b.uc.Execute(ctx, usecases.SaveOnboardingObjectiveInput{UserID: userID, Objective: objective})
	return err
}

type e2eIncomeSaver struct {
	uc *usecases.SaveOnboardingIncome
}

func (b *e2eIncomeSaver) Save(ctx context.Context, userID uuid.UUID, incomeCents int64) error {
	_, err := b.uc.Execute(ctx, usecases.SaveOnboardingIncomeInput{UserID: userID, IncomeCents: incomeCents})
	return err
}

type e2eCardSaver struct {
	uc *usecases.SaveOnboardingCard
}

func (b *e2eCardSaver) Save(ctx context.Context, userID uuid.UUID, nickname string, dueDay int) error {
	_, err := b.uc.Execute(ctx, onbinput.SaveOnboardingCardInput{UserID: userID, Nickname: nickname, DueDay: dueDay})
	return err
}

type e2eSplitsSaver struct {
	uc *usecases.SaveOnboardingBudgetSplits
}

func (b *e2eSplitsSaver) Save(ctx context.Context, userID uuid.UUID, values map[string]int64) (bool, error) {
	slugToKind := map[string]valueobjects.CategoryKind{
		"fixed_cost":        valueobjects.CategoryKindFixedCost,
		"knowledge":         valueobjects.CategoryKindKnowledge,
		"pleasures":         valueobjects.CategoryKindPleasures,
		"goals":             valueobjects.CategoryKindGoals,
		"financial_freedom": valueobjects.CategoryKindFinancialFreedom,
	}
	items := make([]usecases.BudgetSplitItem, 0, len(values))
	for slug, amount := range values {
		kind, ok := slugToKind[slug]
		if !ok {
			continue
		}
		items = append(items, usecases.BudgetSplitItem{Kind: kind, AmountCents: amount})
	}
	out, err := b.uc.Execute(ctx, usecases.SaveOnboardingBudgetSplitsInput{UserID: userID, Allocations: items})
	if err != nil {
		return false, err
	}
	return out.Applied, nil
}

type e2ePhaseSetter struct {
	uc *usecases.SetOnboardingPhase
}

func (b *e2ePhaseSetter) Set(ctx context.Context, userID uuid.UUID, phase string) error {
	p, err := valueobjects.ParseOnboardingPhase(phase)
	if err != nil {
		return err
	}
	_, err = b.uc.Execute(ctx, usecases.SetOnboardingPhaseInput{UserID: userID, Phase: p})
	return err
}

type e2eSessionCompleter struct {
	uc *usecases.CompleteOnboardingSession
}

func (b *e2eSessionCompleter) Complete(ctx context.Context, userID uuid.UUID) error {
	_, err := b.uc.Execute(ctx, usecases.CompleteOnboardingSessionInput{UserID: userID})
	return err
}

type e2eStateChecker struct {
	uc *usecases.GetOnboardingContext
}

func (c *e2eStateChecker) IsOnboardingInProgress(ctx context.Context, userID uuid.UUID) (bool, error) {
	out, err := c.uc.Execute(ctx, usecases.GetOnboardingContextInput{UserID: userID})
	if err != nil {
		return false, err
	}
	if !out.Found {
		return false, nil
	}
	return out.CompletedAt == nil, nil
}

type e2eContextLoader struct {
	uc *usecases.GetOnboardingContext
}

func (b *e2eContextLoader) Load(ctx context.Context, userID uuid.UUID) (agentworkflow.OnboardingContext, error) {
	out, err := b.uc.Execute(ctx, usecases.GetOnboardingContextInput{UserID: userID})
	if err != nil {
		return agentworkflow.OnboardingContext{}, err
	}
	cards := make([]agentworkflow.OnboardingCardState, 0, len(out.Cards))
	for _, c := range out.Cards {
		cards = append(cards, agentworkflow.OnboardingCardState{Name: c.Name, DueDay: c.DueDay})
	}
	return agentworkflow.OnboardingContext{
		Objective:   out.Objective,
		IncomeCents: out.IncomeCents,
		Cards:       cards,
	}, nil
}

type e2eOnboardingInterpreter struct{}

func (d *e2eOnboardingInterpreter) RenderWelcome(_ context.Context) string {
	return "Bem-vindo! Vamos começar?"
}

func (d *e2eOnboardingInterpreter) RenderObjective(_ context.Context) string {
	return "Qual seu objetivo?"
}

func (d *e2eOnboardingInterpreter) RenderBudget(_ context.Context) string {
	return "Quanto voce recebe?"
}

func (d *e2eOnboardingInterpreter) RenderCards(_ context.Context, loop int) string {
	if loop == 0 {
		return "Usa cartao?"
	}
	return "Outro cartao?"
}

func (d *e2eOnboardingInterpreter) RenderCategories(_ context.Context) string {
	return "5 categorias. Faz sentido?"
}

func (d *e2eOnboardingInterpreter) RenderValues(_ context.Context, pending string) string {
	return fmt.Sprintf("Quanto para %s?", pending)
}

func (d *e2eOnboardingInterpreter) RenderSummary(_ context.Context, state agentworkflow.SummaryState) string {
	return fmt.Sprintf("Resumo: %s com renda %d. Esta tudo certo?", state.Objective, state.IncomeCents)
}

func (d *e2eOnboardingInterpreter) RenderRetry(_ context.Context, phase string) string {
	return fmt.Sprintf("Nao entendi (%s). Repita.", phase)
}

func (d *e2eOnboardingInterpreter) RenderDailyRedirect(_ context.Context, phase string) string {
	return fmt.Sprintf("Termine o setup primeiro (%s).", phase)
}

func (d *e2eOnboardingInterpreter) RenderConclusion(_ context.Context) string {
	return "Pronto!"
}

func (d *e2eOnboardingInterpreter) ParseObjective(_ context.Context, text string) (agentworkflow.ParsedObjective, error) {
	trimmed := strings.TrimSpace(text)
	if isDailyCommandTextE2E(trimmed) {
		return agentworkflow.ParsedObjective{DailyCommand: true}, nil
	}
	return agentworkflow.ParsedObjective{Objective: trimmed, Ambiguous: trimmed == ""}, nil
}

func (d *e2eOnboardingInterpreter) ParseBudget(_ context.Context, text string) (agentworkflow.ParsedBudget, error) {
	trimmed := strings.TrimSpace(text)
	if isDailyCommandTextE2E(trimmed) {
		return agentworkflow.ParsedBudget{DailyCommand: true}, nil
	}
	cents, ok := parseMoneyCentsE2E(trimmed)
	if !ok {
		return agentworkflow.ParsedBudget{Ambiguous: true}, nil
	}
	return agentworkflow.ParsedBudget{IncomeCents: cents}, nil
}

func (d *e2eOnboardingInterpreter) ParseCards(_ context.Context, text string, loop int) (agentworkflow.ParsedCards, error) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if isDailyCommandTextE2E(trimmed) {
		return agentworkflow.ParsedCards{DailyCommand: true}, nil
	}
	if isNegationE2E(lower) {
		return agentworkflow.ParsedCards{Skip: true}, nil
	}
	if loop == 0 && isConfirmationE2E(lower) {
		return agentworkflow.ParsedCards{AddAnother: true}, nil
	}
	nickname, dueDay, ok := extractCardE2E(trimmed)
	if !ok {
		return agentworkflow.ParsedCards{Ambiguous: true}, nil
	}
	return agentworkflow.ParsedCards{Nickname: nickname, DueDay: dueDay}, nil
}

func (d *e2eOnboardingInterpreter) ParseCategoriesConfirm(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (d *e2eOnboardingInterpreter) ParseValue(_ context.Context, text string) (int64, bool, error) {
	trimmed := strings.TrimSpace(text)
	cents, ok := parseMoneyCentsE2E(trimmed)
	if !ok {
		return 0, true, nil
	}
	return cents, false, nil
}

func (d *e2eOnboardingInterpreter) ParseSummary(_ context.Context, text string) (agentworkflow.ParsedSummary, error) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if isDailyCommandTextE2E(trimmed) {
		return agentworkflow.ParsedSummary{DailyCommand: true}, nil
	}
	if isConfirmationE2E(lower) {
		return agentworkflow.ParsedSummary{Confirm: true}, nil
	}
	if isNegationE2E(lower) || strings.Contains(lower, "corrigir") || strings.Contains(lower, "errado") {
		return parseCorrectionE2E(trimmed), nil
	}
	return agentworkflow.ParsedSummary{}, nil
}

func isDailyCommandTextE2E(text string) bool {
	lower := strings.ToLower(text)
	cues := []string{
		"gastei", "gasto", "comprei", "paguei", "recebi", "salário", "salario",
		"cartão", "cartao", "fatura", "como estou", "resumo", "orçamento", "orcamento",
	}
	for _, cue := range cues {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

func isConfirmationE2E(text string) bool {
	switch text {
	case "sim", "yes", "vamos", "começar", "bora", "ok", "certo", "quero", "acho que sim":
		return true
	default:
		return false
	}
}

func isNegationE2E(text string) bool {
	switch text {
	case "não", "nao", "no", "nunca", "não uso", "nao uso", "não tenho", "nao tenho":
		return true
	default:
		return false
	}
}

func parseMoneyCentsE2E(text string) (int64, bool) {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.ReplaceAll(cleaned, "R$", "")
	cleaned = strings.ReplaceAll(cleaned, "r$", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	if cleaned == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return int64(value * 100), true
}

var e2eCardDayRe = regexp.MustCompile(`(\d{1,2})`)

func extractCardE2E(text string) (string, int, bool) {
	trimmed := strings.TrimSpace(text)
	matches := e2eCardDayRe.FindStringSubmatch(trimmed)
	if len(matches) < 2 {
		return "", 0, false
	}
	day, err := strconv.Atoi(matches[1])
	if err != nil || day < 1 || day > 31 {
		return "", 0, false
	}
	nickname := strings.TrimSpace(e2eCardDayRe.ReplaceAllString(trimmed, ""))
	if nickname == "" {
		return "", 0, false
	}
	return nickname, day, true
}

func parseCorrectionE2E(text string) agentworkflow.ParsedSummary {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "orcamento"), strings.Contains(lower, "renda"):
		return agentworkflow.ParsedSummary{Correct: true, Target: agentworkflow.CorrectionTargetBudget, NewValue: extractMoneyTextE2E(text)}
	case strings.Contains(lower, "objetivo"):
		return agentworkflow.ParsedSummary{Correct: true, Target: agentworkflow.CorrectionTargetObjective, NewValue: extractObjectiveTextE2E(text)}
	default:
		return agentworkflow.ParsedSummary{Ambiguous: true}
	}
}

var e2eMoneyRe = regexp.MustCompile(`[Rr]\$\s*[\d.]+,?\d*`)
var e2eSimpleNumberRe = regexp.MustCompile(`\d+[,.]?\d*`)

func extractMoneyTextE2E(text string) string {
	if match := e2eMoneyRe.FindString(text); match != "" {
		return match
	}
	if match := e2eSimpleNumberRe.FindString(text); match != "" {
		return match
	}
	return text
}

func extractObjectiveTextE2E(text string) string {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "objetivo")
	if idx >= 0 {
		start := idx + len("objetivo")
		if start < len(text) {
			value := strings.TrimSpace(text[start:])
			value = strings.TrimPrefix(value, "para")
			value = strings.TrimPrefix(value, "eh")
			value = strings.TrimPrefix(value, "é")
			return strings.TrimSpace(value)
		}
	}
	return text
}

func registerOnboardingConversationalSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^que existe um usuario com sessao de onboarding iniciada$`, w.givenUserWithOnboardingSession)
	sc.Step(`^o usuario enviar "([^"]*)" com message_id "([^"]*)"$`, w.whenUserSendsMessage)
	sc.Step(`^o agente deve responder contendo "([^"]*)"$`, w.thenAgentReplyShouldContain)
	sc.Step(`^o estado da sessao de onboarding deve ser "([^"]*)"$`, w.thenOnboardingSessionActiveStateShouldBe)
	sc.Step(`^o objetivo persistido deve ser "([^"]*)"$`, w.thenPersistedObjectiveShouldBe)
	sc.Step(`^deve existir (\d+) evento\(s\) outbox do tipo "([^"]*)"$`, w.thenOutboxEventCountShouldBe)
	sc.Step(`^o runtime do agente for reiniciado$`, w.whenAgentRuntimeIsRestarted)
}

func (w *onboardingWorld) givenUserWithOnboardingSession() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mobile := fmt.Sprintf("+55119%08d", rand.Intn(100000000))
	result, err := w.runtime.deps.identityModule.UpsertUserUseCase.Execute(ctx, identityinput.UpsertUserByWhatsApp{
		WhatsAppNumber: mobile,
		Email:          mobile + "@test.mecontrola",
	})
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(result.ID)
	if err != nil {
		return err
	}
	w.currentUserID = userID

	_, err = w.runtime.deps.startBudgetConfiguration.Execute(ctx, usecases.StartBudgetConfigurationInput{
		UserID:  userID,
		Channel: entities.OnboardingChannelWhatsApp,
	})
	return err
}

func (w *onboardingWorld) whenUserSendsMessage(text, messageID string) error {
	res, ok := w.runtime.onboardingAgent.Handle(context.Background(), w.currentUserID, "whatsapp", "+5511999999999", text, messageID)
	if !ok {
		return fmt.Errorf("agente nao tratou a mensagem %q", text)
	}
	w.lastReply = res.Reply
	return nil
}

func (w *onboardingWorld) thenAgentReplyShouldContain(expected string) error {
	if !strings.Contains(w.lastReply, expected) {
		return fmt.Errorf("resposta esperada conter %q, recebida %q", expected, w.lastReply)
	}
	return nil
}

func (w *onboardingWorld) thenOnboardingSessionActiveStateShouldBe(expected string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var state string
	err := w.runtime.deps.db.QueryRowContext(ctx, `SELECT state FROM mecontrola.onboarding_sessions WHERE user_id = $1`, w.currentUserID).Scan(&state)
	if err != nil {
		return err
	}
	if state != expected {
		return fmt.Errorf("estado esperado %q, recebido %q", expected, state)
	}
	return nil
}

func (w *onboardingWorld) thenPersistedObjectiveShouldBe(expected string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var objective sql.NullString
	err := w.runtime.deps.db.QueryRowContext(ctx, `SELECT payload->>'objective' FROM mecontrola.onboarding_sessions WHERE user_id = $1`, w.currentUserID).Scan(&objective)
	if err != nil {
		return err
	}
	actual := ""
	if objective.Valid {
		actual = objective.String
	}
	if actual != expected {
		return fmt.Errorf("objetivo esperado %q, recebido %q", expected, actual)
	}
	return nil
}

func (w *onboardingWorld) whenAgentRuntimeIsRestarted() error {
	d := w.runtime.deps
	w.runtime.onboardingAgent = newE2EOnboardingAgent(
		d.o11y,
		d.db,
		d.getOnboardingContext,
		d.markWelcomeSent,
		d.saveOnboardingObjective,
		d.saveOnboardingIncome,
		d.saveOnboardingCard,
		d.saveOnboardingBudgetSplits,
		d.setOnboardingPhase,
		d.completeOnboardingSession,
	)
	return nil
}
