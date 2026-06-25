package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/onboardingv2draft"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type OnboardingSnapshotCard struct {
	Name       string
	ClosingDay int
}

type OnboardingSnapshotSplit struct {
	Slug    string
	Percent int
}

type OnboardingSnapshot struct {
	InProgress      bool
	Phase           string
	IncomeCents     int64
	Objective       string
	Cards           []OnboardingSnapshotCard
	Splits          []OnboardingSnapshotSplit
	FirstTxRecorded bool
	WelcomeSent     bool
}

type OnboardingStateReader interface {
	Load(ctx context.Context, userID uuid.UUID) (OnboardingSnapshot, error)
}

type OnboardingToolResult struct {
	Reply            string
	Advance          bool
	Terminal         bool
	ObjectiveProfile string
}

type OnboardingToolDispatcher interface {
	Dispatch(ctx context.Context, userID uuid.UUID, channel string, call interfaces.ToolCall) (OnboardingToolResult, error)
}

type OnboardingPhaseSetter interface {
	SetPhase(ctx context.Context, userID uuid.UUID, phase string) error
}

type RunOnboardingTurnInput struct {
	UserID  uuid.UUID
	Channel string
	Text    string
}

type RunOnboardingTurnResult struct {
	Handled bool
	Reply   string
}

type OnboardingV2SessionGateway interface {
	Load(ctx context.Context, userID uuid.UUID, channel string) (onboardingv2draft.Draft, bool, error)
	Save(ctx context.Context, userID uuid.UUID, channel string, draft onboardingv2draft.Draft) error
	Clear(ctx context.Context, userID uuid.UUID, channel string) error
}

type OnboardingHistoryGatewayIface interface {
	LoadTurns(ctx context.Context, userID uuid.UUID) ([]onbentities.OnboardingTurn, error)
	AppendTurn(ctx context.Context, userID uuid.UUID, userMsg, assistantReply string) error
	MarkWelcomeSent(ctx context.Context, userID uuid.UUID) (alreadySent bool, err error)
}

type BudgetSplitSuggesterIface interface {
	Suggest(ctx context.Context, userID uuid.UUID, objectiveProfile, objective string, incomeCents int64) ([]onbusecases.SuggestBudgetSplitView, error)
}

type RunOnboardingTurn struct {
	interpreter    IntentInterpreter
	reader         OnboardingStateReader
	dispatcher     OnboardingToolDispatcher
	phases         OnboardingPhaseSetter
	v2session      OnboardingV2SessionGateway
	historyGateway OnboardingHistoryGatewayIface
	splitSuggester BudgetSplitSuggesterIface
	maxTokens      int
	o11y           observability.Observability
	turnsTotal     observability.Counter
}

func NewRunOnboardingTurn(
	interpreter IntentInterpreter,
	reader OnboardingStateReader,
	dispatcher OnboardingToolDispatcher,
	phases OnboardingPhaseSetter,
	maxTokens int,
	o11y observability.Observability,
	historyGateway OnboardingHistoryGatewayIface,
	splitSuggester BudgetSplitSuggesterIface,
	v2session OnboardingV2SessionGateway,
) (*RunOnboardingTurn, error) {
	if interpreter == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: interpreter is nil")
	}
	if reader == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: state reader is nil")
	}
	if dispatcher == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: tool dispatcher is nil")
	}
	if phases == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: phase setter is nil")
	}
	if o11y == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: observability is nil")
	}
	if v2session == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: v2session gateway is nil")
	}
	turnsTotal := o11y.Metrics().Counter(
		"agent_onboarding_turn_total",
		"Total de turnos de onboarding conversacional conduzidos pela IA por fase e outcome",
		"1",
	)
	return &RunOnboardingTurn{
		interpreter:    interpreter,
		reader:         reader,
		dispatcher:     dispatcher,
		phases:         phases,
		v2session:      v2session,
		historyGateway: historyGateway,
		splitSuggester: splitSuggester,
		maxTokens:      maxTokens,
		o11y:           o11y,
		turnsTotal:     turnsTotal,
	}, nil
}

func (uc *RunOnboardingTurn) Execute(ctx context.Context, in RunOnboardingTurnInput) (RunOnboardingTurnResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.run_onboarding_turn")
	defer span.End()

	text := strings.TrimSpace(in.Text)
	if in.UserID == uuid.Nil || text == "" {
		return RunOnboardingTurnResult{Handled: false}, nil
	}

	snapshot, err := uc.reader.Load(ctx, in.UserID)
	if err != nil {
		span.RecordError(err)
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: load context: %w", err)
	}

	isWelcomeSignal := text == services.OnboardingWelcomeSignal

	if !snapshot.InProgress {
		return RunOnboardingTurnResult{Handled: isWelcomeSignal}, nil
	}

	if isWelcomeSignal && strings.TrimSpace(snapshot.Phase) != "" {
		return RunOnboardingTurnResult{Handled: true}, nil
	}

	draft, _, err := uc.v2session.Load(ctx, in.UserID, in.Channel)
	if err != nil {
		span.RecordError(err)
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: load v2 draft: %w", err)
	}

	switch strings.TrimSpace(snapshot.Phase) {
	case "":
		return uc.emitWelcome(ctx, in.UserID)
	case OnbPhaseObjective:
		return uc.objectivePhase(ctx, in, snapshot, draft)
	case OnbPhaseBudget:
		return uc.budgetPhase(ctx, in, snapshot, draft)
	case OnbPhaseCards:
		return uc.cardsPhase(ctx, in, snapshot, draft)
	case OnbPhaseFinancialPlan:
		return uc.financialPlanPhase(ctx, in, snapshot, draft)
	case OnbPhaseFirstTx:
		return uc.firstTxPhase(ctx, in, snapshot)
	default:
		return uc.emitWelcome(ctx, in.UserID)
	}
}

func (uc *RunOnboardingTurn) emitWelcome(ctx context.Context, userID uuid.UUID) (RunOnboardingTurnResult, error) {
	if uc.historyGateway != nil {
		alreadySent, err := uc.historyGateway.MarkWelcomeSent(ctx, userID)
		if err != nil {
			return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: mark welcome sent: %w", err)
		}
		if alreadySent {
			uc.turnsTotal.Add(ctx, 1, observability.String("phase", "welcome"), observability.String("outcome", "dedup"))
			return RunOnboardingTurnResult{Handled: true}, nil
		}
	}
	if err := uc.phases.SetPhase(ctx, userID, OnbPhaseObjective); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase objective: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", "welcome"), observability.String("outcome", "emit"))
	return RunOnboardingTurnResult{Handled: true, Reply: uc.welcomeMessage(ctx)}, nil
}

func (uc *RunOnboardingTurn) welcomeMessage(ctx context.Context) string {
	resp, err := uc.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: onboardingWelcomeSystemPrompt,
		UserMessage:  onboardingWelcomeCue,
		FreeText:     true,
		MaxTokens:    uc.maxTokens,
	})
	if err != nil {
		return scriptWelcome
	}
	reply := sanitizeWhatsAppText(string(resp.RawJSON))
	if reply == "" {
		return scriptWelcome
	}
	return reply
}

func (uc *RunOnboardingTurn) objectivePhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, draft onboardingv2draft.Draft) (RunOnboardingTurnResult, error) {
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseObjective)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseObjective), observability.String("outcome", "stay"))
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	updated := draft.WithStep(onboardingv2draft.StepBudget)
	if out.ObjectiveProfile != "" {
		updated = updated.WithObjectiveProfile(out.ObjectiveProfile)
	}
	if saveErr := uc.v2session.Save(ctx, in.UserID, in.Channel, updated); saveErr != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: save draft objective: %w", saveErr)
	}
	if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseBudget); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase budget: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseObjective), observability.String("outcome", "advance"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, scriptBudget)}, nil
}

func (uc *RunOnboardingTurn) budgetPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, draft onboardingv2draft.Draft) (RunOnboardingTurnResult, error) {
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseBudget)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseBudget), observability.String("outcome", "stay"))
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	refreshed, err := uc.reader.Load(ctx, in.UserID)
	if err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: reload after budget: %w", err)
	}
	splits, previewMsg, splitErr := uc.buildSplitPreview(ctx, in.UserID, draft.ObjectiveProfile(), refreshed.Objective, refreshed.IncomeCents)
	if splitErr != nil {
		return RunOnboardingTurnResult{}, splitErr
	}
	updated := draft.WithIncome(refreshed.IncomeCents).WithAutoSplits(splits).WithStep(onboardingv2draft.StepCards)
	if saveErr := uc.v2session.Save(ctx, in.UserID, in.Channel, updated); saveErr != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: save draft budget: %w", saveErr)
	}
	if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseCards); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase cards: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseBudget), observability.String("outcome", "advance"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, previewMsg, scriptCards)}, nil
}

func (uc *RunOnboardingTurn) buildSplitPreview(ctx context.Context, userID uuid.UUID, objectiveProfile, objective string, incomeCents int64) ([]onboardingv2draft.SplitEntry, string, error) {
	if uc.splitSuggester == nil || incomeCents <= 0 {
		return nil, "", nil
	}
	views, err := uc.splitSuggester.Suggest(ctx, userID, objectiveProfile, objective, incomeCents)
	if err != nil {
		return nil, "", fmt.Errorf("agent.usecase.run_onboarding_turn: suggest split: %w", err)
	}
	entries := make([]onboardingv2draft.SplitEntry, len(views))
	for i, v := range views {
		entries[i] = onboardingv2draft.SplitEntry{RootSlug: v.RootSlug, AmountCents: v.PlannedCents}
	}
	return entries, buildAutoSplitPreview(entries), nil
}

func effectiveSplits(draft onboardingv2draft.Draft) []onboardingv2draft.SplitEntry {
	return draft.Splits()
}

func (uc *RunOnboardingTurn) cardsPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, draft onboardingv2draft.Draft) (RunOnboardingTurnResult, error) {
	splits := effectiveSplits(draft)
	if matchesOnboardingNegation(in.Text) {
		if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseFinancialPlan); err != nil {
			return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase financial_plan: %w", err)
		}
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseCards), observability.String("outcome", "negation"))
		return RunOnboardingTurnResult{Handled: true, Reply: buildFinancialPlanMessage(snapshot, splits)}, nil
	}
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseCards)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseCards), observability.String("outcome", "stay"))
		if strings.TrimSpace(out.Reply) == "" {
			return RunOnboardingTurnResult{Handled: true, Reply: scriptCardQuestion}, nil
		}
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseFinancialPlan); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase financial_plan: %w", err)
	}
	refreshed, err := uc.reader.Load(ctx, in.UserID)
	if err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: reload after cards: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseCards), observability.String("outcome", "advance"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, buildFinancialPlanMessage(refreshed, splits))}, nil
}

func (uc *RunOnboardingTurn) financialPlanPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, draft onboardingv2draft.Draft) (RunOnboardingTurnResult, error) {
	splits := effectiveSplits(draft)
	if !onboardingTextHasDigit(in.Text) && shouldAdvanceScriptedPhase(in.Text) {
		confirmCall := buildAutoSplitToolCall(splits)
		out, err := uc.dispatcher.Dispatch(ctx, in.UserID, in.Channel, confirmCall)
		if err != nil {
			return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: dispatch confirm splits: %w", err)
		}
		_ = uc.v2session.Clear(ctx, in.UserID, in.Channel)
		if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseFirstTx); err != nil {
			return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase first_tx: %w", err)
		}
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseFinancialPlan), observability.String("outcome", "confirm"))
		return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, scriptFirstTx)}, nil
	}
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseFinancialPlan)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseFinancialPlan), observability.String("outcome", "stay"))
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	_ = uc.v2session.Clear(ctx, in.UserID, in.Channel)
	if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseFirstTx); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase first_tx: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseFinancialPlan), observability.String("outcome", "adjust"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, scriptFirstTx)}, nil
}

func (uc *RunOnboardingTurn) firstTxPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot) (RunOnboardingTurnResult, error) {
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseFirstTx)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if out.Terminal {
		_ = uc.v2session.Clear(ctx, in.UserID, in.Channel)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseFirstTx), observability.String("outcome", outcomeForAdvance(out.Advance)))
	return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
}

func (uc *RunOnboardingTurn) runDataPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, phase string) (OnboardingToolResult, error) {
	schema, ok := onboardingPhaseSchema(phase)
	if !ok {
		return OnboardingToolResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: schema not found for phase %s", phase)
	}

	llmMessages := uc.loadHistory(ctx, in.UserID)

	resp, err := uc.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: onboardingDataPhasePrompt(phase, snapshot),
		UserMessage:  strings.TrimSpace(in.Text),
		Messages:     llmMessages,
		JSONSchema:   schema,
		MaxTokens:    uc.maxTokens,
	})
	if err != nil {
		return OnboardingToolResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: interpret %s: %w", phase, err)
	}

	structured, decodeErr := decodeOnboardingStructured(resp.RawJSON)
	if decodeErr != nil {
		uc.appendHistory(ctx, in.UserID, strings.TrimSpace(in.Text), "")
		return OnboardingToolResult{Reply: onboardingPhaseRetryReply(phase), Advance: false}, nil
	}

	if structured["action"] == "clarify" {
		reply := sanitizeWhatsAppText(stringFromMap(structured, "reply"))
		uc.appendHistory(ctx, in.UserID, strings.TrimSpace(in.Text), reply)
		return OnboardingToolResult{Reply: reply, Advance: false}, nil
	}

	actionName, _ := structured["action"].(string)
	uc.appendHistory(ctx, in.UserID, strings.TrimSpace(in.Text), "[structured_action:"+actionName+"]")

	if phase == OnbPhaseCards {
		return uc.dispatchCardActions(ctx, in.UserID, in.Channel, actionName, structured)
	}

	call := interfaces.ToolCall{
		FunctionName:  actionName,
		ArgumentsJSON: structured,
	}
	result, dispatchErr := uc.dispatcher.Dispatch(ctx, in.UserID, in.Channel, call)
	if dispatchErr != nil {
		return OnboardingToolResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: dispatch %s: %w", actionName, dispatchErr)
	}
	return result, nil
}

func (uc *RunOnboardingTurn) dispatchCardActions(ctx context.Context, userID uuid.UUID, channel, actionName string, structured map[string]any) (OnboardingToolResult, error) {
	rawCards, _ := structured["cards"].([]any)
	if len(rawCards) == 0 {
		return OnboardingToolResult{Reply: onboardingPhaseRetryReply(OnbPhaseCards), Advance: false}, nil
	}
	var replies []string
	var advance bool
	for _, raw := range rawCards {
		cardMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		call := interfaces.ToolCall{
			FunctionName:  actionName,
			ArgumentsJSON: cardMap,
		}
		result, dispatchErr := uc.dispatcher.Dispatch(ctx, userID, channel, call)
		if dispatchErr != nil {
			return OnboardingToolResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: dispatch card: %w", dispatchErr)
		}
		if r := strings.TrimSpace(result.Reply); r != "" {
			replies = append(replies, r)
		}
		if result.Advance {
			advance = true
		}
	}
	return OnboardingToolResult{Reply: strings.Join(replies, "\n\n"), Advance: advance}, nil
}

func decodeOnboardingStructured(raw []byte) (map[string]any, error) {
	cleaned := stripFences(raw)
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: empty structured response")
	}
	var out map[string]any
	if err := json.Unmarshal(cleaned, &out); err != nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: unmarshal structured: %w", err)
	}
	return out, nil
}

func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func onboardingPhaseRetryReply(phase string) string {
	switch phase {
	case OnbPhaseObjective:
		return "Não entendi seu objetivo. Pode me contar de novo com poucas palavras? 😊"
	case OnbPhaseBudget:
		return "Não entendi o valor. Quanto você recebe por mês? Me diga só o número. 💰"
	case OnbPhaseCards:
		return "Não entendi. Me diz o apelido do cartão e o dia de fechamento (ex: Nubank 15). 💳"
	case OnbPhaseFinancialPlan:
		return "Não entendi a distribuição. Me diz quanto quer pra cada categoria. 📊"
	case OnbPhaseFirstTx:
		return "Não entendi o lançamento. Me manda algo como 'gastei 35 no mercado'. 😊"
	default:
		return "Não entendi. Pode repetir? 😊"
	}
}

func (uc *RunOnboardingTurn) loadHistory(ctx context.Context, userID uuid.UUID) []interfaces.ConversationMessage {
	if uc.historyGateway == nil || userID == uuid.Nil {
		return nil
	}
	turns, err := uc.historyGateway.LoadTurns(ctx, userID)
	if err != nil || len(turns) == 0 {
		return nil
	}
	msgs := make([]interfaces.ConversationMessage, 0, len(turns))
	for _, t := range turns {
		msgs = append(msgs, interfaces.ConversationMessage{Role: t.Role, Content: t.Text})
	}
	return msgs
}

func (uc *RunOnboardingTurn) appendHistory(ctx context.Context, userID uuid.UUID, userMsg, assistantReply string) {
	if uc.historyGateway == nil || userID == uuid.Nil {
		return
	}
	_ = uc.historyGateway.AppendTurn(ctx, userID, userMsg, assistantReply)
}
