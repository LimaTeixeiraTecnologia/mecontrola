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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
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
	OutcomePolicyBlocked   = "policy_blocked"
	OutcomeReplay          = "replay"
)

const (
	maxReadRetryAttempts = 3
	readRetryBaseBackoff = 50 * time.Millisecond
)

const authzDeniedText = "Não consegui concluir essa ação agora. Tente de novo em instantes 🙏"

const policyLowConfidenceText = "Não tenho certeza se entendi direito pra registrar isso. Pode reescrever com mais detalhes (valor, descrição e categoria)? 🙂"

const alreadyProcessedText = "Essa mensagem já foi processada ✅"

const auditWriteFailedText = "Não foi possível processar sua mensagem agora. Pode tentar de novo em instantes? 🙏"

var (
	ErrIntentParserNil    = errors.New("agent.intent_router: intent parser is nil")
	ErrObservabilityNil   = errors.New("agent.intent_router: observability is nil")
	ErrFallbackNil        = errors.New("agent.intent_router: fallback is nil")
	ErrWhatsAppGatewayNil = errors.New("agent.intent_router: whatsapp gateway is nil")
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
	budgetNotActiveText     = "Você ainda não tem um orçamento ativo neste mês pra eu ajustar. Quer configurar um agora? 🙂"
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

func formatCategoryNeedsConfirmation(candidates []string) string {
	top := ""
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed != "" {
			top = trimmed
			break
		}
	}
	if top == "" {
		return "Não tenho certeza da categoria certa pra isso. Me diz qual categoria você quer usar? 🙂"
	}
	return fmt.Sprintf("Acho que isso entra em *%s*. Posso registrar assim? Se não for, me diz a categoria certa. 🙂", top)
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
	Confidence   valueobjects.Confidence
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

type CardUpdater interface {
	Execute(ctx context.Context, userID uuid.UUID, in intent.Intent) (CardUpdaterResult, error)
}

type CardUpdaterResult struct {
	Nickname   string
	Name       string
	ClosingDay int
	DueDay     int
	LimitCents int64
}

type CardDeleter interface {
	Execute(ctx context.Context, userID uuid.UUID, cardName string) (CardDeleterResult, error)
}

type CardDeleterResult struct {
	Name string
}

type CategoryPercentageEditorInput struct {
	UserID       uuid.UUID
	Competence   string
	CategoryName string
	Percentage   int
}

type CategoryPercentageEditorResult struct {
	Competence string
	RootSlug   string
	Percentage int
}

type CategoryPercentageEditor interface {
	Execute(ctx context.Context, in CategoryPercentageEditorInput) (CategoryPercentageEditorResult, error)
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

type PendingExpenseConfirmationGateway interface {
	Load(ctx context.Context, userID uuid.UUID, channel string) (pendingexpense.Draft, bool, error)
	Save(ctx context.Context, userID uuid.UUID, channel string, draft pendingexpense.Draft) error
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

type ExpenseRecorder interface {
	Execute(ctx context.Context, in ExpenseRecorderInput) (ExpenseRecorderResult, error)
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

type ExpenseRecorderInput struct {
	UserID        string
	Intent        intent.Intent
	ForceCategory *string
	AmountCents   int64
	Merchant      string
	PaymentMethod string
	Direction     string
	OccurredAt    string
}

type ExpenseRecorderResult struct {
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
	runtime         *AgentRuntime
}

func (r *IntentRouter) EnableRuntime(threads ThreadGateway, runs RunGateway) {
	r.runtime = NewAgentRuntime(r.o11y, r, threads, runs)
}

func (r *IntentRouter) dispatch(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult {
	if r.runtime != nil {
		return r.runtime.Execute(ctx, principal, channel, peer, text, messageID)
	}
	return r.route(ctx, principal, channel, peer, text, messageID)
}

type IntentRouterDeps struct {
	Parser                     IntentParser
	MonthlySummary             MonthlySummaryReader
	CardLister                 CardLister
	CardInvoice                CardInvoiceReader
	CardCreator                CardCreator
	CardCounter                CardCounter
	CardUpdater                CardUpdater
	CardDeleter                CardDeleter
	CategoryPercentageEditor   CategoryPercentageEditor
	ExpenseRecorder            ExpenseRecorder
	CardPurchaseLog            CardPurchaseLogger
	TransactionLister          TransactionLister
	LastDeleter                LastTransactionDeleter
	LastEditor                 LastTransactionEditor
	RecurringCreator           RecurringCreator
	RecurringLister            RecurringLister
	BudgetConfig               BudgetConfigurator
	BudgetConvo                BudgetConversation
	BudgetCommitter            BudgetConfigCommitter
	BudgetSession              BudgetSessionGateway
	PendingExpenseConfirmation PendingExpenseConfirmationGateway
	Onboarding                 OnboardingContinuation
	OnboardingRunner           OnboardingTurnRunner
	Fallback                   Fallback
	WhatsAppGateway            WhatsAppOutbound
	TelegramGateway            TelegramOutbound
	EventPublisher             interfaces.IntentEventPublisher
	Decision                   DecisionAuditDeps
	Redactor                   DecisionRedactor
	Location                   *time.Location
	PolicyMinConfidence        float64
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
	policyBlockedTotal := o11y.Metrics().Counter(
		"agent_policy_blocks_total",
		"Total de dispatches de escrita bloqueados pela politica de confianca por kind",
		"1",
	)
	idempotencyReplayTotal := o11y.Metrics().Counter(
		"agent_idempotency_replay_total",
		"Total de dispatches de escrita servidos por replay idempotente por kind",
		"1",
	)
	daily, err := newDailyLedgerAgent(o11y, routedTotal, authzDeniedTotal, policyBlockedTotal, idempotencyReplayTotal, loc, deps)
	if err != nil {
		return nil, err
	}
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
		"record_transaction": deps.ExpenseRecorder != nil,
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
	result := r.dispatch(ctx, principal, ChannelWhatsApp, msg.WhatsAppTo, msg.Text, msg.MessageID)
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
	result := r.dispatch(ctx, principal, ChannelTelegram, "", msg.Text, msg.MessageID)
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

func (r *IntentRouter) record(ctx context.Context, kind, channel, outcome string) {
	r.routedTotal.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome),
	)
}
