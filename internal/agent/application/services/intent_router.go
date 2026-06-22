package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

const (
	ChannelWhatsApp = "whatsapp"
	ChannelTelegram = "telegram"
)

const (
	OutcomeRouted          = "routed"
	OutcomeFallback        = "fallback"
	OutcomeParseError      = "parse_error"
	OutcomeUsecaseError    = "usecase_error"
	OutcomeMissingResolver = "missing_resolver"
	OutcomeReplyFailed     = "reply_failed"
	OutcomeEmptyText       = "empty_text"
	OutcomeAuthzDenied     = "authz_denied"
	OutcomeClarify         = "clarify"
)

const (
	maxReadRetryAttempts = 3
	readRetryBaseBackoff = 50 * time.Millisecond
)

const authzDeniedText = "Não consegui concluir essa ação agora. Tente de novo em instantes 🙏"

var (
	ErrIntentParserNil    = errors.New("agent.intent_router: intent parser is nil")
	ErrObservabilityNil   = errors.New("agent.intent_router: observability is nil")
	ErrFallbackNil        = errors.New("agent.intent_router: fallback is nil")
	ErrWhatsAppGatewayNil = errors.New("agent.intent_router: whatsapp gateway is nil")
)

var (
	ErrCategoryAmbiguous   = errors.New("agent.intent_router: categoria ambigua")
	ErrCategoryNotFound    = errors.New("agent.intent_router: categoria nao encontrada")
	ErrCategoryHintMissing = errors.New("agent.intent_router: sem hint de categoria")
	ErrRecurringInvalidDay = errors.New("agent.intent_router: dia da recorrencia invalido")
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

const (
	defaultListCardsLimit   = 200
	fallbackMissingText     = "Não recebi nenhuma mensagem. Me conta o que você precisa nas suas finanças 😊"
	fallbackParseError      = "Não entendi direito. Pode reformular? Posso te ajudar com cartões, orçamento e lançamentos."
	fallbackUsecaseError    = "Tive uma instabilidade para consultar isso agora. Tente de novo em instantes 🙏"
	registerUnavailableText = "Ainda não consigo registrar lançamentos por aqui. Já já isso fica disponível pra você 🙏"
	noTransactionsText      = "Não encontrei nenhum lançamento recente seu para mexer. Quer registrar um agora? 😊"
	budgetCancelledText     = "Ok, cancelei a configuração do orçamento. Quando quiser, é só chamar de novo. 😊"
	categoryNoHintText      = "Pra registrar certinho, me diz em qual categoria você quer anotar isso? 🙂"
	recurringInvalidDayText = "Pra criar uma recorrência, o dia do mês precisa estar entre 1 e 28. Me confirma o dia certo? 🙂"
)

func formatCategoryAmbiguous(candidates []string) string {
	var sb strings.Builder
	sb.WriteString("Encontrei mais de uma categoria parecida. Qual delas você quer usar?")
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		sb.WriteString("\n• ")
		sb.WriteString(trimmed)
	}
	sb.WriteString("\nÉ só me dizer o nome. 🙂")
	return sb.String()
}

func formatCategoryNotFound(hint string) string {
	trimmed := strings.TrimSpace(hint)
	if trimmed == "" {
		return "Não encontrei uma categoria pra isso. Pode reformular ou me dizer a categoria? 🙂"
	}
	return fmt.Sprintf("Não encontrei a categoria %q. Pode reformular ou me dizer outra categoria? 🙂", trimmed)
}

var budgetCancelCues = []string{
	"cancelar", "cancela", "deixa pra lá", "deixa pra la", "esquece", "parar",
}

func matchesBudgetCancel(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	for _, cue := range budgetCancelCues {
		if normalized == cue || strings.Contains(normalized, cue) {
			return true
		}
	}
	return false
}

type ParsedIntent struct {
	Intent       intent.Intent
	Raw          []byte
	DirectReply  string
	LLMModel     string
	PromptSHA256 string
}

type IntentParser interface {
	Parse(ctx context.Context, userID uuid.UUID, text string) (ParsedIntent, error)
}

type MonthlySummaryReader interface {
	Execute(ctx context.Context, userID string, competence string) (budgetsoutput.MonthlySummaryOutput, error)
}

type CardLister interface {
	Execute(ctx context.Context, in cardinput.ListCards) (cardoutput.CardList, error)
}

type CardInvoiceReader interface {
	Execute(ctx context.Context, in cardinput.InvoiceFor) (cardoutput.Invoice, error)
}

type CardCreator interface {
	Execute(ctx context.Context, userID uuid.UUID, in intent.Intent) (CardCreatorResult, error)
}

type CardCreatorResult struct {
	Nickname   string
	Name       string
	ClosingDay int
	DueDay     int
	LimitCents int64
}

type CardCounter interface {
	Execute(ctx context.Context, userID uuid.UUID) (int64, error)
}

type Fallback interface {
	Reply(ctx context.Context, userID uuid.UUID, channel, text string) (string, error)
}

type WhatsAppOutbound interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type TelegramOutbound interface {
	SendTextMessage(ctx context.Context, chatID int64, text string) error
}

type Principal struct {
	UserID uuid.UUID
}

type InboundMessage struct {
	Text       string
	WhatsAppTo string
	TelegramTo int64
	MessageID  string
}

type RouteResult struct {
	Reply   string
	Outcome string
	Kind    intent.Kind
}

type BudgetConfigurator interface {
	Start(ctx context.Context, userID uuid.UUID, channel string) (string, error)
}

type BudgetConversationResult struct {
	Draft    budgetdraft.Draft
	Complete bool
	Reply    string
}

type BudgetConversation interface {
	Configure(ctx context.Context, text string, draft budgetdraft.Draft) (BudgetConversationResult, error)
}

type BudgetConfigCommitter interface {
	Commit(ctx context.Context, userID uuid.UUID, draft budgetdraft.Draft) (string, error)
}

type BudgetSessionGateway interface {
	Load(ctx context.Context, userID uuid.UUID, channel string) (budgetdraft.Draft, bool, error)
	Save(ctx context.Context, userID uuid.UUID, channel string, draft budgetdraft.Draft) error
	Clear(ctx context.Context, userID uuid.UUID, channel string) error
}

type OnboardingConversation struct {
	Handled bool
	Reply   string
}

type OnboardingContinuation interface {
	Continue(ctx context.Context, userID uuid.UUID, channel, peer, text, messageID string) (OnboardingConversation, error)
}

type OnboardingTurnResult struct {
	Handled bool
	Reply   string
}

type OnboardingTurnRunner interface {
	Run(ctx context.Context, userID uuid.UUID, channel, text string) (OnboardingTurnResult, error)
}

type ExpenseLogger interface {
	Execute(ctx context.Context, in ExpenseLoggerInput) (ExpenseLoggerResult, error)
}

type CardPurchaseLogger interface {
	Execute(ctx context.Context, in CardPurchaseLoggerInput) (CardPurchaseLoggerResult, error)
}

type CardPurchaseLoggerInput struct {
	UserID string
	Intent intent.Intent
}

type CardPurchaseLoggerResult struct {
	Persisted    bool
	CardFound    bool
	CardName     string
	AmountCents  int64
	Installments int
	CategoryPath string
}

type TransactionView struct {
	ID          string
	Direction   string
	AmountCents int64
	Description string
	OccurredAt  time.Time
	CreatedAt   time.Time
	Version     int64
}

type TransactionLister interface {
	Execute(ctx context.Context, in TransactionListInput) (TransactionListResult, error)
}

type TransactionListInput struct {
	UserID   string
	RefMonth string
}

type TransactionListResult struct {
	RefMonth     string
	Transactions []TransactionView
}

type LastTransactionDeleter interface {
	Execute(ctx context.Context, userID, txID string, version int64) error
}

type LastTransactionEditor interface {
	Execute(ctx context.Context, in EditTransactionInput) (EditTransactionResult, error)
}

type EditTransactionInput struct {
	UserID    string
	Current   TransactionView
	NewAmount int64
}

type EditTransactionResult struct {
	Persisted   bool
	OldAmount   int64
	NewAmount   int64
	Description string
}

type RecurringCreator interface {
	Execute(ctx context.Context, in RecurringCreatorInput) (RecurringCreatorResult, error)
}

type RecurringCreatorInput struct {
	UserID string
	Intent intent.Intent
}

type RecurringCreatorResult struct {
	Persisted    bool
	Direction    string
	AmountCents  int64
	Frequency    string
	DayOfMonth   int
	CategoryPath string
	Description  string
}

type RecurringView struct {
	Direction   string
	AmountCents int64
	Description string
	Frequency   string
	DayOfMonth  int
}

type RecurringLister interface {
	Execute(ctx context.Context, userID string) ([]RecurringView, error)
}

type ExpenseLoggerInput struct {
	UserID string
	Intent intent.Intent
}

type ExpenseLoggerResult struct {
	Persisted      bool
	SubcategoryID  string
	RootCategoryID string
	AmountCents    int64
	Competence     string
	CategoryPath   string
	OccurredAt     time.Time
}

type IntentRouter struct {
	onboarding      *OnboardingAgent
	daily           *DailyLedgerAgent
	whatsAppGateway WhatsAppOutbound
	telegramGateway TelegramOutbound
	eventPublisher  interfaces.IntentEventPublisher
	o11y            observability.Observability
	routedTotal     observability.Counter
}

type IntentRouterDeps struct {
	Parser            IntentParser
	MonthlySummary    MonthlySummaryReader
	CardLister        CardLister
	CardInvoice       CardInvoiceReader
	CardCreator       CardCreator
	CardCounter       CardCounter
	ExpenseLogger     ExpenseLogger
	CardPurchaseLog   CardPurchaseLogger
	TransactionLister TransactionLister
	LastDeleter       LastTransactionDeleter
	LastEditor        LastTransactionEditor
	RecurringCreator  RecurringCreator
	RecurringLister   RecurringLister
	BudgetConfig      BudgetConfigurator
	BudgetConvo       BudgetConversation
	BudgetCommitter   BudgetConfigCommitter
	BudgetSession     BudgetSessionGateway
	Onboarding        OnboardingContinuation
	OnboardingRunner  OnboardingTurnRunner
	Fallback          Fallback
	WhatsAppGateway   WhatsAppOutbound
	TelegramGateway   TelegramOutbound
	EventPublisher    interfaces.IntentEventPublisher
	Decision          DecisionAuditDeps
	Redactor          DecisionRedactor
	Location          *time.Location
}

type DecisionRedactor interface {
	Clean(raw string) (string, error)
}

func NewIntentRouter(o11y observability.Observability, deps IntentRouterDeps) (*IntentRouter, error) {
	if o11y == nil {
		return nil, ErrObservabilityNil
	}
	if deps.Parser == nil {
		return nil, ErrIntentParserNil
	}
	if deps.Fallback == nil {
		return nil, ErrFallbackNil
	}
	if deps.WhatsAppGateway == nil {
		return nil, ErrWhatsAppGatewayNil
	}
	loc := deps.Location
	if loc == nil {
		loc = time.UTC
	}
	routedTotal := o11y.Metrics().Counter(
		"agent_intent_routed_total",
		"Total de intents roteados pelo IntentRouter por kind, channel e outcome",
		"1",
	)
	authzDeniedTotal := o11y.Metrics().Counter(
		"agent_authz_denied_total",
		"Total de dispatches de escrita negados pela guarda de autorizacao por kind",
		"1",
	)
	daily := newDailyLedgerAgent(o11y, routedTotal, authzDeniedTotal, loc, deps)
	warnMissingToolBindings(o11y, deps)
	return &IntentRouter{
		onboarding:      newOnboardingAgent(o11y, routedTotal, deps),
		daily:           daily,
		whatsAppGateway: deps.WhatsAppGateway,
		telegramGateway: deps.TelegramGateway,
		eventPublisher:  deps.EventPublisher,
		o11y:            o11y,
		routedTotal:     routedTotal,
	}, nil
}

func warnMissingToolBindings(o11y observability.Observability, deps IntentRouterDeps) {
	registry, err := tools.DefaultRegistry()
	if err != nil {
		o11y.Logger().Warn(context.Background(), "agent.intent_router.tool_registry_unavailable",
			observability.Error(err),
		)
		return
	}
	bindings := map[string]bool{
		"record_transaction": deps.ExpenseLogger != nil,
		"monthly_summary":    deps.MonthlySummary != nil,
		"list_cards":         deps.CardLister != nil,
		"create_card":        deps.CardCreator != nil,
		"count_cards":        deps.CardCounter != nil,
		"configure_budget":   deps.BudgetConfig != nil,
	}
	for _, spec := range registry.Specs() {
		present, tracked := bindings[spec.Name]
		if !tracked {
			continue
		}
		if !present {
			o11y.Logger().Warn(context.Background(), "agent.intent_router.tool_binding_ausente",
				observability.String("tool", spec.Name),
				observability.String("kind", spec.IntentKind.String()),
			)
		}
	}
}

func (r *IntentRouter) RouteWhatsApp(ctx context.Context, principal Principal, msg InboundMessage) RouteResult {
	startedAt := time.Now().UTC()
	result := r.route(ctx, principal, ChannelWhatsApp, msg.WhatsAppTo, msg.Text, msg.MessageID)
	defer r.publishEvent(ctx, principal, ChannelWhatsApp, result, startedAt)
	if result.Reply == "" {
		return result
	}
	if err := r.whatsAppGateway.SendTextMessage(ctx, msg.WhatsAppTo, result.Reply); err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.whatsapp_send_failed",
			observability.String("kind", result.Kind.String()),
			observability.Error(err),
		)
		r.record(ctx, result.Kind.String(), ChannelWhatsApp, OutcomeReplyFailed)
	}
	return result
}

func (r *IntentRouter) RouteTelegram(ctx context.Context, principal Principal, msg InboundMessage) RouteResult {
	startedAt := time.Now().UTC()
	result := r.route(ctx, principal, ChannelTelegram, "", msg.Text, msg.MessageID)
	defer r.publishEvent(ctx, principal, ChannelTelegram, result, startedAt)
	if result.Reply == "" {
		return result
	}
	if r.telegramGateway == nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.telegram_gateway_missing")
		r.record(ctx, result.Kind.String(), ChannelTelegram, OutcomeReplyFailed)
		return result
	}
	if err := r.telegramGateway.SendTextMessage(ctx, msg.TelegramTo, result.Reply); err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.telegram_send_failed",
			observability.String("kind", result.Kind.String()),
			observability.Error(err),
		)
		r.record(ctx, result.Kind.String(), ChannelTelegram, OutcomeReplyFailed)
	}
	return result
}

func (r *IntentRouter) publishEvent(ctx context.Context, principal Principal, channel string, result RouteResult, startedAt time.Time) {
	if r.eventPublisher == nil {
		return
	}
	ev := interfaces.IntentEvent{
		EventID:    uuid.New(),
		UserID:     principal.UserID,
		Channel:    channel,
		Outcome:    result.Outcome,
		LatencyMS:  time.Since(startedAt).Milliseconds(),
		OccurredAt: time.Now().UTC(),
	}
	if span := r.o11y.Tracer().SpanFromContext(ctx); span != nil {
		ev.TraceID = span.TraceID()
	}
	if result.Outcome == OutcomeRouted && result.Kind != intent.KindUnknown {
		ev.Module = result.Kind.String()
	}
	var pubErr error
	if result.Outcome == OutcomeRouted {
		pubErr = r.eventPublisher.PublishExecuted(ctx, ev)
	} else {
		pubErr = r.eventPublisher.PublishRejected(ctx, ev)
	}
	if pubErr != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.publish_failed",
			observability.String("event_id", ev.EventID.String()),
			observability.String("channel", channel),
			observability.Error(pubErr),
		)
	}
}

func (r *IntentRouter) route(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.intent_router.route")
	defer span.End()

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		r.record(ctx, intent.KindUnknown.String(), channel, OutcomeEmptyText)
		return RouteResult{Reply: fallbackMissingText, Outcome: OutcomeEmptyText, Kind: intent.KindUnknown}
	}

	if result, ok := r.onboarding.Handle(ctx, principal.UserID, channel, peer, trimmed, messageID); ok {
		return result
	}

	return r.daily.Handle(ctx, principal, channel, peer, trimmed, messageID)
}

func (r *IntentRouter) authorizeWrite(ctx context.Context, principal Principal, effectiveUserID uuid.UUID, kind intent.Kind, channel string) bool {
	return r.daily.authorizeWrite(ctx, principal, effectiveUserID, kind, channel)
}

func isWriteKind(kind intent.Kind) bool {
	switch kind {
	case intent.KindLogExpense,
		intent.KindLogIncome,
		intent.KindLogCardPurchase,
		intent.KindCreateCard,
		intent.KindConfigureBudget:
		return true
	default:
		return false
	}
}

func withReadRetry[T any](ctx context.Context, op func(context.Context) (T, error)) (T, error) {
	var (
		out T
		err error
	)
	for attempt := 1; attempt <= maxReadRetryAttempts; attempt++ {
		out, err = op(ctx)
		if err == nil || !isTransientReadError(err) {
			return out, err
		}
		if attempt == maxReadRetryAttempts {
			break
		}
		backoff := time.Duration(attempt) * readRetryBaseBackoff
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return out, errors.Join(err, ctx.Err())
		case <-timer.C:
		}
	}
	return out, err
}

func isTransientReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

func pickMostRecent(items []TransactionView) (TransactionView, bool, error) {
	if len(items) == 0 {
		return TransactionView{}, false, nil
	}
	latest := items[0]
	for _, item := range items[1:] {
		if moreRecent(item, latest) {
			latest = item
		}
	}
	return latest, true, nil
}

func moreRecent(candidate, current TransactionView) bool {
	if candidate.CreatedAt.Equal(current.CreatedAt) {
		return candidate.ID > current.ID
	}
	return candidate.CreatedAt.After(current.CreatedAt)
}

func (r *IntentRouter) record(ctx context.Context, kind, channel, outcome string) {
	r.routedTotal.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome),
	)
}

func resolveCardByName(list cardoutput.CardList, name string) (cardoutput.Card, bool) {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return cardoutput.Card{}, false
	}
	for _, item := range list.Items {
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return item, true
		}
		if strings.EqualFold(strings.TrimSpace(item.Nickname), name) {
			return item, true
		}
	}
	for _, item := range list.Items {
		if strings.Contains(strings.ToLower(item.Name), target) {
			return item, true
		}
		if strings.Contains(strings.ToLower(item.Nickname), target) {
			return item, true
		}
	}
	return cardoutput.Card{}, false
}

func formatBRL(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	reais := cents / 100
	centavos := cents % 100
	reaisStr := formatThousands(reais)
	sign := ""
	if negative {
		sign = "-"
	}
	return fmt.Sprintf("R$ %s%s,%02d", sign, reaisStr, centavos)
}

func formatThousands(value int64) string {
	raw := fmt.Sprintf("%d", value)
	if len(raw) <= 3 {
		return raw
	}
	var out strings.Builder
	prefix := len(raw) % 3
	if prefix > 0 {
		out.WriteString(raw[:prefix])
		if len(raw) > prefix {
			out.WriteString(".")
		}
	}
	for idx := prefix; idx < len(raw); idx += 3 {
		out.WriteString(raw[idx : idx+3])
		if idx+3 < len(raw) {
			out.WriteString(".")
		}
	}
	return out.String()
}

func formatPersistedExpense(amountCents int64, merchant, categoryPath string) string {
	var sb strings.Builder
	sb.WriteString("💸 *Transação realizada!*\n*")
	sb.WriteString(formatBRL(amountCents))
	sb.WriteString("*")
	if strings.TrimSpace(merchant) != "" {
		sb.WriteString(" em *")
		sb.WriteString(merchant)
		sb.WriteString("*")
	}
	if strings.TrimSpace(categoryPath) != "" {
		sb.WriteString("\n📂 ")
		sb.WriteString(categoryPath)
	}
	sb.WriteString("\n🔔 *Atualizando seu orçamento automaticamente...*")
	return sb.String()
}

func formatPersistedIncome(amountCents int64, source, categoryPath string) string {
	var sb strings.Builder
	sb.WriteString("💰 *Recebimento registrado!*\n*")
	sb.WriteString(formatBRL(amountCents))
	sb.WriteString("*")
	if strings.TrimSpace(source) != "" {
		sb.WriteString(" de *")
		sb.WriteString(source)
		sb.WriteString("*")
	}
	if strings.TrimSpace(categoryPath) != "" {
		sb.WriteString("\n📂 ")
		sb.WriteString(categoryPath)
	}
	sb.WriteString("\n✅ Anotei na sua conta.")
	return sb.String()
}

func registerFailedText(amountCents int64, merchant string) string {
	var sb strings.Builder
	sb.WriteString("😕 Não consegui registrar ")
	sb.WriteString(formatBRL(amountCents))
	if strings.TrimSpace(merchant) != "" {
		sb.WriteString(" em ")
		sb.WriteString(merchant)
	}
	sb.WriteString(" agora. Pode tentar de novo em instantes? Se quiser, me diga a categoria pra eu organizar certinho.")
	return sb.String()
}

func formatMonthlySummary(summary budgetsoutput.MonthlySummaryOutput) string {
	var sb strings.Builder
	sb.WriteString("📊 *Resumo de ")
	sb.WriteString(summary.Competence)
	sb.WriteString("*\n")
	sb.WriteString("• Gasto total: ")
	sb.WriteString(formatBRL(summary.TotalSpentCents))
	if summary.TotalPlannedCents != nil {
		sb.WriteString(" / planejado ")
		sb.WriteString(formatBRL(*summary.TotalPlannedCents))
	}
	sb.WriteString("\n")
	for _, allocation := range summary.Allocations {
		if allocation.SpentCents == 0 && (allocation.PlannedCents == nil || *allocation.PlannedCents == 0) {
			continue
		}
		sb.WriteString("• ")
		sb.WriteString(rootSlugLabel(allocation.RootSlug))
		sb.WriteString(": ")
		sb.WriteString(formatBRL(allocation.SpentCents))
		if allocation.PlannedCents != nil {
			sb.WriteString(" / ")
			sb.WriteString(formatBRL(*allocation.PlannedCents))
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatCategoryAllocation(summary budgetsoutput.MonthlySummaryOutput, categoryName string) string {
	target := strings.ToLower(strings.TrimSpace(categoryName))
	for _, allocation := range summary.Allocations {
		if strings.ToLower(allocation.RootSlug) == target {
			var sb strings.Builder
			sb.WriteString("📊 *")
			sb.WriteString(rootSlugLabel(allocation.RootSlug))
			sb.WriteString("* (")
			sb.WriteString(summary.Competence)
			sb.WriteString("): ")
			sb.WriteString(formatBRL(allocation.SpentCents))
			if allocation.PlannedCents != nil {
				sb.WriteString(" de ")
				sb.WriteString(formatBRL(*allocation.PlannedCents))
				sb.WriteString(" planejados")
				if allocation.PercentageSpent != nil {
					_, _ = fmt.Fprintf(&sb, " (%.0f%% da meta)", *allocation.PercentageSpent)
				}
			}
			sb.WriteString(".")
			return sb.String()
		}
	}
	return fmt.Sprintf("Não encontrei dados para a categoria %q em %s.", categoryName, summary.Competence)
}

func rootSlugLabel(slug string) string {
	switch slug {
	case "expense.custo_fixo":
		return "Custo Fixo"
	case "expense.conhecimento":
		return "Conhecimento"
	case "expense.prazeres":
		return "Prazeres"
	case "expense.metas":
		return "Metas"
	case "expense.liberdade_financeira":
		return "Liberdade Financeira"
	default:
		return slug
	}
}

func formatGoalUnavailable(goalName string) string {
	if strings.TrimSpace(goalName) == "" {
		return "Consultar metas ainda não está disponível, mas anotei seu pedido."
	}
	return fmt.Sprintf("Consultar a meta %q ainda não está disponível, mas anotei seu pedido.", goalName)
}

const rootSlugMetas = "expense.metas"

func formatGoalProgress(summary budgetsoutput.MonthlySummaryOutput, goalName string) string {
	for _, allocation := range summary.Allocations {
		if allocation.RootSlug == rootSlugMetas {
			var sb strings.Builder
			sb.WriteString("🎯 ")
			if strings.TrimSpace(goalName) != "" {
				sb.WriteString("*")
				sb.WriteString(goalName)
				sb.WriteString("*: ")
			}
			sb.WriteString("você já guardou ")
			sb.WriteString(formatBRL(allocation.SpentCents))
			if allocation.PlannedCents != nil && *allocation.PlannedCents > 0 {
				sb.WriteString(" de ")
				sb.WriteString(formatBRL(*allocation.PlannedCents))
				sb.WriteString(" previstos em Metas")
				if allocation.PercentageSpent != nil {
					_, _ = fmt.Fprintf(&sb, " (%.0f%%)", *allocation.PercentageSpent)
				}
			} else {
				sb.WriteString(" em Metas")
			}
			sb.WriteString(".")
			return sb.String()
		}
	}
	if strings.TrimSpace(goalName) != "" {
		return fmt.Sprintf("Não encontrei dados de metas para %q em %s.", goalName, summary.Competence)
	}
	return fmt.Sprintf("Não encontrei dados de metas em %s.", summary.Competence)
}

func formatCardList(list cardoutput.CardList) string {
	if len(list.Items) == 0 {
		return "💳 Você ainda não tem cartões cadastrados. Quer cadastrar um agora?"
	}
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "💳 *Seus cartões* (%d)\n", len(list.Items))
	for _, card := range list.Items {
		name := strings.TrimSpace(card.Nickname)
		if name == "" {
			name = strings.TrimSpace(card.Name)
		}
		if name == "" {
			name = "Cartão sem nome"
		}
		sb.WriteString("• *")
		sb.WriteString(name)
		sb.WriteString("*")
		if card.LimitCents > 0 {
			sb.WriteString(" — limite ")
			sb.WriteString(formatBRL(card.LimitCents))
		}
		_, _ = fmt.Fprintf(&sb, " (fecha dia %d, vence dia %d)", card.ClosingDay, card.DueDay)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatCreatedCard(result CardCreatorResult) string {
	label := strings.TrimSpace(result.Nickname)
	if label == "" {
		label = strings.TrimSpace(result.Name)
	}
	if label == "" {
		label = "novo cartão"
	}
	var sb strings.Builder
	sb.WriteString("💳 *Cartão cadastrado!*\n*")
	sb.WriteString(label)
	sb.WriteString("*")
	if result.LimitCents > 0 {
		sb.WriteString(" — limite ")
		sb.WriteString(formatBRL(result.LimitCents))
	}
	_, _ = fmt.Fprintf(&sb, "\n📅 Fecha dia %d, vence dia %d.", result.ClosingDay, result.DueDay)
	return sb.String()
}

func formatCardCount(total int64) string {
	switch total {
	case 0:
		return "💳 Você ainda não tem cartões cadastrados. Quer cadastrar um agora?"
	case 1:
		return "💳 Você tem *1 cartão* cadastrado."
	default:
		return fmt.Sprintf("💳 Você tem *%d cartões* cadastrados.", total)
	}
}

func createCardErrorText(err error) string {
	switch {
	case errors.Is(err, carddomain.ErrNicknameConflict):
		return "💳 Você já tem um cartão com esse apelido. Que tal escolher outro nome?"
	case errors.Is(err, carddomain.ErrInvalidClosingDay):
		return "💳 O dia de fechamento precisa estar entre 1 e 31. Me confirma o dia certo?"
	case errors.Is(err, carddomain.ErrInvalidDueDay):
		return "💳 O dia de vencimento precisa estar entre 1 e 31. Me confirma o dia certo?"
	case errors.Is(err, carddomain.ErrInvalidNickname):
		return "💳 O apelido do cartão precisa ter entre 1 e 32 caracteres. Pode me passar um nome mais curto?"
	case errors.Is(err, carddomain.ErrInvalidCardName):
		return "💳 O nome do cartão precisa ter entre 1 e 64 caracteres. Pode reformular?"
	case errors.Is(err, carddomain.ErrCardLimitTooLarge):
		return "💳 Esse limite passou do máximo que consigo registrar (R$ 1.000.000,00). Confere o valor?"
	case errors.Is(err, carddomain.ErrCardLimitNegative):
		return "💳 O limite não pode ser negativo. Me passa um valor válido?"
	default:
		return "😕 Não consegui cadastrar o cartão agora. Pode tentar de novo em instantes?"
	}
}

func formatCardNotFound(cardName string) string {
	if strings.TrimSpace(cardName) == "" {
		return "💳 Não encontrei esse cartão no seu cadastro. Que tal cadastrá-lo primeiro pra eu cuidar da fatura pra você?"
	}
	return fmt.Sprintf("💳 Não encontrei um cartão chamado %q no seu cadastro. Quer cadastrá-lo primeiro pra eu acompanhar a fatura?", cardName)
}

func formatCardInvoice(card cardoutput.Card, invoice cardoutput.Invoice) string {
	name := strings.TrimSpace(card.Name)
	if name == "" {
		name = strings.TrimSpace(card.Nickname)
	}
	base := fmt.Sprintf("💳 *Fatura do cartão %s*\nFechamento em %s, vencimento em %s.",
		name, invoice.ClosingDate, invoice.DueDate)
	if card.LimitCents > 0 {
		return base + "\nLimite: " + formatBRL(card.LimitCents) + "."
	}
	return base
}

func formatHowAmIDoing(summary budgetsoutput.MonthlySummaryOutput) string {
	alert := summary.PercentageTotal != nil && *summary.PercentageTotal >= 80
	var sb strings.Builder
	if alert {
		sb.WriteString("⚠️ *Atenção Proativa* (")
	} else {
		sb.WriteString("📊 *Como você está* (")
	}
	sb.WriteString(summary.Competence)
	sb.WriteString(")\nVocê gastou ")
	sb.WriteString(formatBRL(summary.TotalSpentCents))
	if summary.TotalPlannedCents != nil && *summary.TotalPlannedCents > 0 {
		sb.WriteString(" de ")
		sb.WriteString(formatBRL(*summary.TotalPlannedCents))
		sb.WriteString(" planejados")
		if summary.PercentageTotal != nil {
			_, _ = fmt.Fprintf(&sb, " (%.0f%%)", *summary.PercentageTotal)
			if *summary.PercentageTotal >= 90 {
				sb.WriteString(". Você está bem próximo do limite do mês. Vamos manter o foco nos seus sonhos? 🎯")
				return sb.String()
			}
			if *summary.PercentageTotal >= 80 {
				sb.WriteString(". Dá pra segurar o ritmo até o fim do mês. Vamos juntos? 🎯")
				return sb.String()
			}
			if *summary.PercentageTotal <= 50 {
				sb.WriteString(". Está em ritmo tranquilo até aqui.")
				return sb.String()
			}
		}
		sb.WriteString(".")
		return sb.String()
	}
	sb.WriteString(". Defina um planejamento para eu te ajudar a acompanhar melhor.")
	return sb.String()
}

func formatCardPurchaseCardMissing(cardHint string) string {
	if strings.TrimSpace(cardHint) == "" {
		return "💳 Pra registrar a compra parcelada eu preciso saber em qual cartão foi. Me diz o nome do cartão? (ex: nubank, itaú)"
	}
	return fmt.Sprintf("💳 Não encontrei um cartão chamado %q no seu cadastro. Quer cadastrá-lo primeiro pra eu registrar a compra parcelada?", cardHint)
}

func formatPersistedCardPurchase(result CardPurchaseLoggerResult) string {
	var sb strings.Builder
	sb.WriteString("💳 *Compra parcelada registrada!*\n*")
	sb.WriteString(formatBRL(result.AmountCents))
	sb.WriteString("*")
	_, _ = fmt.Fprintf(&sb, " em *%dx*", result.Installments)
	if strings.TrimSpace(result.CardName) != "" {
		sb.WriteString(" no *")
		sb.WriteString(result.CardName)
		sb.WriteString("*")
	}
	if strings.TrimSpace(result.CategoryPath) != "" {
		sb.WriteString("\n📂 ")
		sb.WriteString(result.CategoryPath)
	}
	sb.WriteString("\n✅ Anotei nas suas faturas.")
	return sb.String()
}

func formatTransactionList(list TransactionListResult) string {
	if len(list.Transactions) == 0 {
		return fmt.Sprintf("📭 Você não tem lançamentos em %s ainda.", list.RefMonth)
	}
	totalIn := int64(0)
	totalOut := int64(0)
	for _, t := range list.Transactions {
		switch t.Direction {
		case "income":
			totalIn += t.AmountCents
		case "outcome":
			totalOut += t.AmountCents
		}
	}
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "📋 *Lançamentos de %s* (%d)\n", list.RefMonth, len(list.Transactions))
	sb.WriteString("• Entradas: ")
	sb.WriteString(formatBRL(totalIn))
	sb.WriteString("\n• Saídas: ")
	sb.WriteString(formatBRL(totalOut))
	return sb.String()
}

func formatDeletedTransaction(view TransactionView) string {
	var sb strings.Builder
	sb.WriteString("🗑️ *Lançamento excluído!*\n")
	sb.WriteString(formatBRL(view.AmountCents))
	if strings.TrimSpace(view.Description) != "" {
		sb.WriteString(" — ")
		sb.WriteString(view.Description)
	}
	sb.WriteString(" (")
	sb.WriteString(view.OccurredAt.Format("02/01/2006"))
	sb.WriteString(")")
	return sb.String()
}

func formatEditedTransaction(result EditTransactionResult) string {
	var sb strings.Builder
	sb.WriteString("✏️ *Lançamento atualizado!*\n")
	sb.WriteString("De ")
	sb.WriteString(formatBRL(result.OldAmount))
	sb.WriteString(" para *")
	sb.WriteString(formatBRL(result.NewAmount))
	sb.WriteString("*")
	if strings.TrimSpace(result.Description) != "" {
		sb.WriteString(" — ")
		sb.WriteString(result.Description)
	}
	return sb.String()
}

func formatPersistedRecurring(result RecurringCreatorResult) string {
	var sb strings.Builder
	sb.WriteString("🔁 *Recorrência criada!*\n*")
	sb.WriteString(formatBRL(result.AmountCents))
	sb.WriteString("*")
	if result.Direction == "income" {
		sb.WriteString(" de entrada")
	} else {
		sb.WriteString(" de saída")
	}
	sb.WriteString(" ")
	sb.WriteString(frequencyLabel(result.Frequency))
	_, _ = fmt.Fprintf(&sb, " (dia %d)", result.DayOfMonth)
	if strings.TrimSpace(result.Description) != "" {
		sb.WriteString("\n📝 ")
		sb.WriteString(result.Description)
	}
	if strings.TrimSpace(result.CategoryPath) != "" {
		sb.WriteString("\n📂 ")
		sb.WriteString(result.CategoryPath)
	}
	return sb.String()
}

func formatRecurringList(items []RecurringView) string {
	if len(items) == 0 {
		return "🔁 Você ainda não tem lançamentos recorrentes cadastrados."
	}
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "🔁 *Recorrências* (%d)\n", len(items))
	for _, item := range items {
		sb.WriteString("• ")
		sb.WriteString(formatBRL(item.AmountCents))
		sb.WriteString(" ")
		sb.WriteString(frequencyLabel(item.Frequency))
		_, _ = fmt.Fprintf(&sb, " (dia %d)", item.DayOfMonth)
		if strings.TrimSpace(item.Description) != "" {
			sb.WriteString(" — ")
			sb.WriteString(item.Description)
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func frequencyLabel(frequency string) string {
	switch frequency {
	case "monthly":
		return "mensal"
	case "yearly":
		return "anual"
	default:
		return frequency
	}
}
