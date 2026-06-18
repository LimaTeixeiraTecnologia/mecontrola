package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
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
)

var (
	ErrIntentParserNil    = errors.New("agent.intent_router: intent parser is nil")
	ErrObservabilityNil   = errors.New("agent.intent_router: observability is nil")
	ErrFallbackNil        = errors.New("agent.intent_router: fallback is nil")
	ErrWhatsAppGatewayNil = errors.New("agent.intent_router: whatsapp gateway is nil")
)

const (
	defaultListCardsLimit   = 200
	fallbackMissingText     = "Não recebi nenhuma mensagem. Me conta o que você precisa nas suas finanças 😊"
	fallbackParseError      = "Não entendi direito. Pode reformular? Posso te ajudar com cartões, orçamento e lançamentos."
	fallbackUsecaseError    = "Tive uma instabilidade para consultar isso agora. Tente de novo em instantes 🙏"
	registerUnavailableText = "Ainda não consigo registrar lançamentos por aqui. Já já isso fica disponível pra você 🙏"
	noTransactionsText      = "Não encontrei nenhum lançamento recente seu para mexer. Quer registrar um agora? 😊"
	budgetCancelledText     = "Ok, cancelei a configuração do orçamento. Quando quiser, é só chamar de novo. 😊"
)

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
	Intent      intent.Intent
	Raw         []byte
	DirectReply string
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
	parser            IntentParser
	monthlySummary    MonthlySummaryReader
	cardLister        CardLister
	cardInvoice       CardInvoiceReader
	cardCreator       CardCreator
	cardCounter       CardCounter
	expenseLogger     ExpenseLogger
	cardPurchaseLog   CardPurchaseLogger
	transactionLister TransactionLister
	lastDeleter       LastTransactionDeleter
	lastEditor        LastTransactionEditor
	recurringCreator  RecurringCreator
	recurringLister   RecurringLister
	budgetConfig      BudgetConfigurator
	budgetConvo       BudgetConversation
	budgetCommitter   BudgetConfigCommitter
	budgetSession     BudgetSessionGateway
	onboarding        OnboardingContinuation
	fallback          Fallback
	whatsAppGateway   WhatsAppOutbound
	telegramGateway   TelegramOutbound
	eventPublisher    interfaces.IntentEventPublisher
	o11y              observability.Observability
	routedTotal       observability.Counter
	loc               *time.Location
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
	Fallback          Fallback
	WhatsAppGateway   WhatsAppOutbound
	TelegramGateway   TelegramOutbound
	EventPublisher    interfaces.IntentEventPublisher
	Location          *time.Location
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
	return &IntentRouter{
		parser:            deps.Parser,
		monthlySummary:    deps.MonthlySummary,
		cardLister:        deps.CardLister,
		cardInvoice:       deps.CardInvoice,
		cardCreator:       deps.CardCreator,
		cardCounter:       deps.CardCounter,
		expenseLogger:     deps.ExpenseLogger,
		cardPurchaseLog:   deps.CardPurchaseLog,
		transactionLister: deps.TransactionLister,
		lastDeleter:       deps.LastDeleter,
		lastEditor:        deps.LastEditor,
		recurringCreator:  deps.RecurringCreator,
		recurringLister:   deps.RecurringLister,
		budgetConfig:      deps.BudgetConfig,
		budgetConvo:       deps.BudgetConvo,
		budgetCommitter:   deps.BudgetCommitter,
		budgetSession:     deps.BudgetSession,
		onboarding:        deps.Onboarding,
		fallback:          deps.Fallback,
		whatsAppGateway:   deps.WhatsAppGateway,
		telegramGateway:   deps.TelegramGateway,
		eventPublisher:    deps.EventPublisher,
		o11y:              o11y,
		routedTotal:       routedTotal,
		loc:               loc,
	}, nil
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

func (r *IntentRouter) route(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult { //nolint:revive // dispatch exaustivo por intent kind
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.intent_router.route")
	defer span.End()

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		r.record(ctx, intent.KindUnknown.String(), channel, OutcomeEmptyText)
		return RouteResult{Reply: fallbackMissingText, Outcome: OutcomeEmptyText, Kind: intent.KindUnknown}
	}

	if r.onboarding != nil {
		conversation, err := r.onboarding.Continue(ctx, principal.UserID, channel, peer, trimmed, messageID)
		if err == nil && conversation.Handled {
			r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
			return RouteResult{Reply: conversation.Reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
		}
	}

	if r.budgetSessionEnabled() {
		if handled, result := r.continuePendingBudgetSession(ctx, principal.UserID, channel, trimmed); handled {
			return result
		}
	}

	parsed, err := r.parser.Parse(ctx, principal.UserID, trimmed)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Warn(ctx, "agent.intent_router.parse_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		reply := r.delegateFallback(ctx, principal.UserID, channel, trimmed)
		r.record(ctx, intent.KindUnknown.String(), channel, OutcomeParseError)
		return RouteResult{Reply: reply, Outcome: OutcomeParseError, Kind: intent.KindUnknown}
	}

	kind := parsed.Intent.Kind()
	span.SetAttributes(
		observability.String("kind", kind.String()),
		observability.String("channel", channel),
	)

	if kind == intent.KindUnknown && strings.TrimSpace(parsed.DirectReply) != "" {
		r.record(ctx, intent.KindUnknown.String(), channel, OutcomeRouted)
		return RouteResult{Reply: parsed.DirectReply, Outcome: OutcomeRouted, Kind: intent.KindUnknown}
	}

	switch kind {
	case intent.KindLogExpense:
		return r.routeLogExpense(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindLogIncome:
		return r.routeLogIncome(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindMonthlySummary:
		return r.routeMonthlySummary(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindQueryCategory:
		return r.routeQueryCategory(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindQueryGoal:
		return r.routeQueryGoal(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindQueryCard:
		return r.routeQueryCard(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindHowAmIDoing:
		return r.routeHowAmIDoing(ctx, principal.UserID, channel)
	case intent.KindConfigureBudget:
		return r.routeConfigureBudget(ctx, principal.UserID, channel, trimmed)
	case intent.KindLogCardPurchase:
		return r.routeLogCardPurchase(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindListTransactions:
		return r.routeListTransactions(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindDeleteLastTransaction:
		return r.routeDeleteLastTransaction(ctx, principal.UserID, channel)
	case intent.KindEditLastTransaction:
		return r.routeEditLastTransaction(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindCreateRecurring:
		return r.routeCreateRecurring(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindListRecurring:
		return r.routeListRecurring(ctx, principal.UserID, channel)
	case intent.KindListCards:
		return r.routeListCards(ctx, principal.UserID, channel)
	case intent.KindCreateCard:
		return r.routeCreateCard(ctx, principal.UserID, channel, parsed.Intent)
	case intent.KindCountCards:
		return r.routeCountCards(ctx, principal.UserID, channel)
	case intent.KindUnknown:
		reply := r.delegateFallback(ctx, principal.UserID, channel, trimmed)
		r.record(ctx, intent.KindUnknown.String(), channel, OutcomeFallback)
		return RouteResult{Reply: reply, Outcome: OutcomeFallback, Kind: intent.KindUnknown}
	default:
		reply := r.delegateFallback(ctx, principal.UserID, channel, trimmed)
		r.record(ctx, kind.String(), channel, OutcomeFallback)
		return RouteResult{Reply: reply, Outcome: OutcomeFallback, Kind: kind}
	}
}

func (r *IntentRouter) routeLogExpense(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.expenseLogger == nil {
		r.record(ctx, intent.KindLogExpense.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindLogExpense}
	}
	result, err := r.expenseLogger.Execute(ctx, ExpenseLoggerInput{UserID: userID.String(), Intent: in})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.log_expense_failed",
			observability.Error(err),
		)
		r.record(ctx, intent.KindLogExpense.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: registerFailedText(in.AmountCents(), in.Merchant()), Outcome: OutcomeUsecaseError, Kind: intent.KindLogExpense}
	}
	r.record(ctx, intent.KindLogExpense.String(), channel, OutcomeRouted)
	reply := formatPersistedExpense(result.AmountCents, in.Merchant(), result.CategoryPath)
	return RouteResult{Reply: reply, Outcome: OutcomeRouted, Kind: intent.KindLogExpense}
}

func (r *IntentRouter) routeLogIncome(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.expenseLogger == nil {
		r.record(ctx, intent.KindLogIncome.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindLogIncome}
	}
	result, err := r.expenseLogger.Execute(ctx, ExpenseLoggerInput{UserID: userID.String(), Intent: in})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.log_income_failed",
			observability.Error(err),
		)
		r.record(ctx, intent.KindLogIncome.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: registerFailedText(in.AmountCents(), in.Merchant()), Outcome: OutcomeUsecaseError, Kind: intent.KindLogIncome}
	}
	r.record(ctx, intent.KindLogIncome.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatPersistedIncome(result.AmountCents, in.Merchant(), result.CategoryPath), Outcome: OutcomeRouted, Kind: intent.KindLogIncome}
}

func (r *IntentRouter) routeMonthlySummary(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.monthlySummary == nil {
		r.record(ctx, intent.KindMonthlySummary.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindMonthlySummary}
	}
	competence := in.RefMonth()
	if competence == "" {
		now := time.Now().UTC().In(r.loc)
		competence = fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	}
	summary, err := r.monthlySummary.Execute(ctx, userID.String(), competence)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.monthly_summary_failed",
			observability.String("competence", competence),
			observability.Error(err),
		)
		r.record(ctx, intent.KindMonthlySummary.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindMonthlySummary}
	}
	r.record(ctx, intent.KindMonthlySummary.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatMonthlySummary(summary), Outcome: OutcomeRouted, Kind: intent.KindMonthlySummary}
}

func (r *IntentRouter) routeQueryCategory(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.monthlySummary == nil {
		r.record(ctx, intent.KindQueryCategory.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindQueryCategory}
	}
	now := time.Now().UTC().In(r.loc)
	competence := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	summary, err := r.monthlySummary.Execute(ctx, userID.String(), competence)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.query_category_failed",
			observability.String("category", in.CategoryName()),
			observability.Error(err),
		)
		r.record(ctx, intent.KindQueryCategory.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryCategory}
	}
	r.record(ctx, intent.KindQueryCategory.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCategoryAllocation(summary, in.CategoryName()), Outcome: OutcomeRouted, Kind: intent.KindQueryCategory}
}

func (r *IntentRouter) routeQueryGoal(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.monthlySummary == nil {
		r.record(ctx, intent.KindQueryGoal.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: formatGoalUnavailable(in.GoalName()), Outcome: OutcomeMissingResolver, Kind: intent.KindQueryGoal}
	}
	now := time.Now().UTC().In(r.loc)
	competence := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	summary, err := r.monthlySummary.Execute(ctx, userID.String(), competence)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.query_goal_failed",
			observability.String("competence", competence),
			observability.Error(err),
		)
		r.record(ctx, intent.KindQueryGoal.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryGoal}
	}
	r.record(ctx, intent.KindQueryGoal.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatGoalProgress(summary, in.GoalName()), Outcome: OutcomeRouted, Kind: intent.KindQueryGoal}
}

func (r *IntentRouter) routeQueryCard(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.cardLister == nil || r.cardInvoice == nil {
		r.record(ctx, intent.KindQueryCard.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindQueryCard}
	}
	cards, err := r.cardLister.Execute(ctx, cardinput.ListCards{UserID: userID, Limit: defaultListCardsLimit})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.query_card_list_failed",
			observability.String("card_name", in.CardName()),
			observability.Error(err),
		)
		r.record(ctx, intent.KindQueryCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryCard}
	}
	resolved, ok := resolveCardByName(cards, in.CardName())
	if !ok {
		r.record(ctx, intent.KindQueryCard.String(), channel, OutcomeMissingResolver)
		return RouteResult{
			Reply:   formatCardNotFound(in.CardName()),
			Outcome: OutcomeMissingResolver,
			Kind:    intent.KindQueryCard,
		}
	}
	cardID, parseErr := uuid.Parse(resolved.ID)
	if parseErr != nil {
		r.record(ctx, intent.KindQueryCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryCard}
	}
	now := time.Now().UTC().In(r.loc)
	invoice, err := r.cardInvoice.Execute(ctx, cardinput.InvoiceFor{
		CardID:   cardID,
		UserID:   userID,
		Purchase: now,
	})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.query_card_invoice_failed",
			observability.String("card_name", in.CardName()),
			observability.Error(err),
		)
		r.record(ctx, intent.KindQueryCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryCard}
	}
	r.record(ctx, intent.KindQueryCard.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCardInvoice(resolved, invoice), Outcome: OutcomeRouted, Kind: intent.KindQueryCard}
}

func (r *IntentRouter) routeConfigureBudget(ctx context.Context, userID uuid.UUID, channel, text string) RouteResult {
	if r.budgetSessionEnabled() {
		return r.startBudgetSession(ctx, userID, channel, text)
	}
	if r.budgetConfig == nil {
		r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindConfigureBudget}
	}
	reply, err := r.budgetConfig.Start(ctx, userID, channel)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.configure_budget_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
	}
	if strings.TrimSpace(reply) == "" {
		reply = "Beleza! Qual a sua renda mensal? Pode me dizer o valor."
	}
	r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
	return RouteResult{Reply: reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
}

func (r *IntentRouter) budgetSessionEnabled() bool {
	return r.budgetSession != nil && r.budgetConvo != nil && r.budgetCommitter != nil
}

func (r *IntentRouter) startBudgetSession(ctx context.Context, userID uuid.UUID, channel, text string) RouteResult {
	now := time.Now().UTC().In(r.loc)
	competence := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	return r.advanceBudgetSession(ctx, userID, channel, text, budgetdraft.New(competence))
}

func (r *IntentRouter) continuePendingBudgetSession(ctx context.Context, userID uuid.UUID, channel, text string) (bool, RouteResult) {
	draft, found, err := r.budgetSession.Load(ctx, userID, channel)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_load_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return false, RouteResult{}
	}
	if !found {
		return false, RouteResult{}
	}
	if matchesBudgetCancel(text) {
		if clearErr := r.budgetSession.Clear(ctx, userID, channel); clearErr != nil {
			r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_clear_failed",
				observability.String("channel", channel),
				observability.Error(clearErr),
			)
		}
		r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
		return true, RouteResult{Reply: budgetCancelledText, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
	}
	return true, r.advanceBudgetSession(ctx, userID, channel, text, draft)
}

func (r *IntentRouter) advanceBudgetSession(ctx context.Context, userID uuid.UUID, channel, text string, draft budgetdraft.Draft) RouteResult {
	result, err := r.budgetConvo.Configure(ctx, text, draft)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_configure_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
	}

	if !result.Complete {
		if saveErr := r.budgetSession.Save(ctx, userID, channel, result.Draft); saveErr != nil {
			r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_save_failed",
				observability.String("channel", channel),
				observability.Error(saveErr),
			)
			r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
			return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
		}
		r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
		return RouteResult{Reply: result.Reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
	}

	reply, commitErr := r.budgetCommitter.Commit(ctx, userID, result.Draft)
	if commitErr != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_commit_failed",
			observability.String("channel", channel),
			observability.Error(commitErr),
		)
		if saveErr := r.budgetSession.Save(ctx, userID, channel, result.Draft); saveErr != nil {
			r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_save_failed",
				observability.String("channel", channel),
				observability.Error(saveErr),
			)
		}
		r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: reply, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
	}

	if clearErr := r.budgetSession.Clear(ctx, userID, channel); clearErr != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_clear_failed",
			observability.String("channel", channel),
			observability.Error(clearErr),
		)
	}
	r.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
	return RouteResult{Reply: reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
}

func (r *IntentRouter) routeHowAmIDoing(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if r.monthlySummary == nil {
		r.record(ctx, intent.KindHowAmIDoing.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindHowAmIDoing}
	}
	now := time.Now().UTC().In(r.loc)
	competence := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	summary, err := r.monthlySummary.Execute(ctx, userID.String(), competence)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.how_am_i_doing_failed",
			observability.Error(err),
		)
		r.record(ctx, intent.KindHowAmIDoing.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindHowAmIDoing}
	}
	r.record(ctx, intent.KindHowAmIDoing.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatHowAmIDoing(summary), Outcome: OutcomeRouted, Kind: intent.KindHowAmIDoing}
}

func (r *IntentRouter) routeLogCardPurchase(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.cardPurchaseLog == nil {
		r.record(ctx, intent.KindLogCardPurchase.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindLogCardPurchase}
	}
	result, err := r.cardPurchaseLog.Execute(ctx, CardPurchaseLoggerInput{UserID: userID.String(), Intent: in})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.log_card_purchase_failed",
			observability.Error(err),
		)
		r.record(ctx, intent.KindLogCardPurchase.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: registerFailedText(in.AmountCents(), in.Merchant()), Outcome: OutcomeUsecaseError, Kind: intent.KindLogCardPurchase}
	}
	if !result.CardFound {
		r.record(ctx, intent.KindLogCardPurchase.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: formatCardPurchaseCardMissing(in.CardHint()), Outcome: OutcomeMissingResolver, Kind: intent.KindLogCardPurchase}
	}
	r.record(ctx, intent.KindLogCardPurchase.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatPersistedCardPurchase(result), Outcome: OutcomeRouted, Kind: intent.KindLogCardPurchase}
}

func (r *IntentRouter) routeListTransactions(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.transactionLister == nil {
		r.record(ctx, intent.KindListTransactions.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindListTransactions}
	}
	refMonth := in.RefMonth()
	if refMonth == "" {
		refMonth = r.currentCompetence()
	}
	list, err := r.transactionLister.Execute(ctx, TransactionListInput{UserID: userID.String(), RefMonth: refMonth})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.list_transactions_failed",
			observability.String("ref_month", refMonth),
			observability.Error(err),
		)
		r.record(ctx, intent.KindListTransactions.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindListTransactions}
	}
	r.record(ctx, intent.KindListTransactions.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatTransactionList(list), Outcome: OutcomeRouted, Kind: intent.KindListTransactions}
}

func (r *IntentRouter) routeDeleteLastTransaction(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if r.transactionLister == nil || r.lastDeleter == nil {
		r.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindDeleteLastTransaction}
	}
	last, found, err := r.mostRecentTransaction(ctx, userID)
	if err != nil {
		r.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindDeleteLastTransaction}
	}
	if !found {
		r.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeRouted)
		return RouteResult{Reply: noTransactionsText, Outcome: OutcomeRouted, Kind: intent.KindDeleteLastTransaction}
	}
	if err := r.lastDeleter.Execute(ctx, userID.String(), last.ID, last.Version); err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.delete_last_transaction_failed",
			observability.Error(err),
		)
		r.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindDeleteLastTransaction}
	}
	r.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatDeletedTransaction(last), Outcome: OutcomeRouted, Kind: intent.KindDeleteLastTransaction}
}

func (r *IntentRouter) routeEditLastTransaction(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.transactionLister == nil || r.lastEditor == nil {
		r.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindEditLastTransaction}
	}
	last, found, err := r.mostRecentTransaction(ctx, userID)
	if err != nil {
		r.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindEditLastTransaction}
	}
	if !found {
		r.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeRouted)
		return RouteResult{Reply: noTransactionsText, Outcome: OutcomeRouted, Kind: intent.KindEditLastTransaction}
	}
	result, err := r.lastEditor.Execute(ctx, EditTransactionInput{UserID: userID.String(), Current: last, NewAmount: in.AmountCents()})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.edit_last_transaction_failed",
			observability.Error(err),
		)
		r.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindEditLastTransaction}
	}
	r.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatEditedTransaction(result), Outcome: OutcomeRouted, Kind: intent.KindEditLastTransaction}
}

func (r *IntentRouter) routeCreateRecurring(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.recurringCreator == nil {
		r.record(ctx, intent.KindCreateRecurring.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindCreateRecurring}
	}
	result, err := r.recurringCreator.Execute(ctx, RecurringCreatorInput{UserID: userID.String(), Intent: in})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.create_recurring_failed",
			observability.Error(err),
		)
		r.record(ctx, intent.KindCreateRecurring.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: registerFailedText(in.AmountCents(), in.Merchant()), Outcome: OutcomeUsecaseError, Kind: intent.KindCreateRecurring}
	}
	r.record(ctx, intent.KindCreateRecurring.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatPersistedRecurring(result), Outcome: OutcomeRouted, Kind: intent.KindCreateRecurring}
}

func (r *IntentRouter) routeListRecurring(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if r.recurringLister == nil {
		r.record(ctx, intent.KindListRecurring.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindListRecurring}
	}
	items, err := r.recurringLister.Execute(ctx, userID.String())
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.list_recurring_failed",
			observability.Error(err),
		)
		r.record(ctx, intent.KindListRecurring.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindListRecurring}
	}
	r.record(ctx, intent.KindListRecurring.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatRecurringList(items), Outcome: OutcomeRouted, Kind: intent.KindListRecurring}
}

func (r *IntentRouter) routeListCards(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if r.cardLister == nil {
		r.record(ctx, intent.KindListCards.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindListCards}
	}
	cards, err := r.cardLister.Execute(ctx, cardinput.ListCards{UserID: userID, Limit: defaultListCardsLimit})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.list_cards_failed", observability.Error(err))
		r.record(ctx, intent.KindListCards.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindListCards}
	}
	r.record(ctx, intent.KindListCards.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCardList(cards), Outcome: OutcomeRouted, Kind: intent.KindListCards}
}

func (r *IntentRouter) routeCreateCard(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if r.cardCreator == nil {
		r.record(ctx, intent.KindCreateCard.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindCreateCard}
	}
	result, err := r.cardCreator.Execute(ctx, userID, in)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.create_card_failed", observability.Error(err))
		r.record(ctx, intent.KindCreateCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: createCardErrorText(err), Outcome: OutcomeUsecaseError, Kind: intent.KindCreateCard}
	}
	r.record(ctx, intent.KindCreateCard.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCreatedCard(result), Outcome: OutcomeRouted, Kind: intent.KindCreateCard}
}

func (r *IntentRouter) routeCountCards(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if r.cardCounter == nil {
		r.record(ctx, intent.KindCountCards.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindCountCards}
	}
	total, err := r.cardCounter.Execute(ctx, userID)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.count_cards_failed", observability.Error(err))
		r.record(ctx, intent.KindCountCards.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindCountCards}
	}
	r.record(ctx, intent.KindCountCards.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCardCount(total), Outcome: OutcomeRouted, Kind: intent.KindCountCards}
}

func (r *IntentRouter) currentCompetence() string {
	now := time.Now().UTC().In(r.loc)
	return fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
}

func (r *IntentRouter) mostRecentTransaction(ctx context.Context, userID uuid.UUID) (TransactionView, bool, error) {
	list, err := r.transactionLister.Execute(ctx, TransactionListInput{UserID: userID.String(), RefMonth: r.currentCompetence()})
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.most_recent_transaction_failed",
			observability.Error(err),
		)
		return TransactionView{}, false, err
	}
	return pickMostRecent(list.Transactions)
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

func (r *IntentRouter) delegateFallback(ctx context.Context, userID uuid.UUID, channel, text string) string {
	reply, err := r.fallback.Reply(ctx, userID, channel, text)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.fallback_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return fallbackParseError
	}
	if strings.TrimSpace(reply) == "" {
		return fallbackParseError
	}
	return reply
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
