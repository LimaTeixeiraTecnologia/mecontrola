package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
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
)

var (
	ErrIntentParserNil    = errors.New("agent.intent_router: intent parser is nil")
	ErrObservabilityNil   = errors.New("agent.intent_router: observability is nil")
	ErrFallbackNil        = errors.New("agent.intent_router: fallback is nil")
	ErrWhatsAppGatewayNil = errors.New("agent.intent_router: whatsapp gateway is nil")
)

const (
	defaultListCardsLimit = 200
	fallbackMissingText   = "Não recebi nenhuma mensagem. Me conta o que você precisa nas suas finanças 😊"
	fallbackParseError    = "Não entendi direito. Pode reformular? Posso te ajudar com cartões, orçamento e lançamentos."
	fallbackUsecaseError  = "Tive uma instabilidade para consultar isso agora. Tente de novo em instantes 🙏"
)

type ParsedIntent struct {
	Intent intent.Intent
	Raw    []byte
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
	parser          IntentParser
	monthlySummary  MonthlySummaryReader
	cardLister      CardLister
	cardInvoice     CardInvoiceReader
	expenseLogger   ExpenseLogger
	budgetConfig    BudgetConfigurator
	onboarding      OnboardingContinuation
	fallback        Fallback
	whatsAppGateway WhatsAppOutbound
	telegramGateway TelegramOutbound
	o11y            observability.Observability
	routedTotal     observability.Counter
	loc             *time.Location
}

type IntentRouterDeps struct {
	Parser          IntentParser
	MonthlySummary  MonthlySummaryReader
	CardLister      CardLister
	CardInvoice     CardInvoiceReader
	ExpenseLogger   ExpenseLogger
	BudgetConfig    BudgetConfigurator
	Onboarding      OnboardingContinuation
	Fallback        Fallback
	WhatsAppGateway WhatsAppOutbound
	TelegramGateway TelegramOutbound
	Location        *time.Location
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
		parser:          deps.Parser,
		monthlySummary:  deps.MonthlySummary,
		cardLister:      deps.CardLister,
		cardInvoice:     deps.CardInvoice,
		expenseLogger:   deps.ExpenseLogger,
		budgetConfig:    deps.BudgetConfig,
		onboarding:      deps.Onboarding,
		fallback:        deps.Fallback,
		whatsAppGateway: deps.WhatsAppGateway,
		telegramGateway: deps.TelegramGateway,
		o11y:            o11y,
		routedTotal:     routedTotal,
		loc:             loc,
	}, nil
}

func (r *IntentRouter) RouteWhatsApp(ctx context.Context, principal Principal, msg InboundMessage) RouteResult {
	result := r.route(ctx, principal, ChannelWhatsApp, msg.WhatsAppTo, msg.Text, msg.MessageID)
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
	result := r.route(ctx, principal, ChannelTelegram, "", msg.Text, msg.MessageID)
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

func (r *IntentRouter) route(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult {
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

	switch kind {
	case intent.KindLogExpense:
		return r.routeLogExpense(ctx, principal.UserID, channel, parsed.Intent)
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
		return r.routeConfigureBudget(ctx, principal.UserID, channel)
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
		reply := formatLoggedExpense(in.AmountCents(), in.Merchant(), in.CategoryHint())
		return RouteResult{Reply: reply, Outcome: OutcomeMissingResolver, Kind: intent.KindLogExpense}
	}
	result, err := r.expenseLogger.Execute(ctx, ExpenseLoggerInput{UserID: userID.String(), Intent: in})
	if err != nil {
		r.record(ctx, intent.KindLogExpense.String(), channel, OutcomeUsecaseError)
		reply := formatLoggedExpense(in.AmountCents(), in.Merchant(), in.CategoryHint())
		return RouteResult{Reply: reply, Outcome: OutcomeUsecaseError, Kind: intent.KindLogExpense}
	}
	r.record(ctx, intent.KindLogExpense.String(), channel, OutcomeRouted)
	reply := formatPersistedExpense(result.AmountCents, in.Merchant(), result.CategoryPath)
	return RouteResult{Reply: reply, Outcome: OutcomeRouted, Kind: intent.KindLogExpense}
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

func (r *IntentRouter) routeConfigureBudget(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
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

func formatLoggedExpense(amountCents int64, merchant, categoryHint string) string {
	var sb strings.Builder
	sb.WriteString("💸 *Transação realizada!*\n*")
	sb.WriteString(formatBRL(amountCents))
	sb.WriteString("*")
	if strings.TrimSpace(merchant) != "" {
		sb.WriteString(" em *")
		sb.WriteString(merchant)
		sb.WriteString("*")
	}
	if strings.TrimSpace(categoryHint) != "" {
		sb.WriteString("\n📂 ")
		sb.WriteString(categoryHint)
	}
	sb.WriteString("\nEm breve eu associo a categoria automaticamente pra você.")
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
