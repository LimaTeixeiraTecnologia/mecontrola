package tools

import (
	"context"
	"errors"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type MonthlySummary struct {
	recorder *Recorder
	summary  MonthlySummaryReader
	loc      *time.Location
	o11y     observability.Observability
}

func NewMonthlySummary(recorder *Recorder, summary MonthlySummaryReader, loc *time.Location, o11y observability.Observability) *MonthlySummary {
	return &MonthlySummary{recorder: recorder, summary: summary, loc: loc, o11y: o11y}
}

func (t *MonthlySummary) Name() string { return "monthly_summary" }

func (t *MonthlySummary) Descriptor() ToolSpec {
	return ToolSpec{Name: "monthly_summary", IntentKind: intent.KindMonthlySummary, Description: "monthly_summary", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *MonthlySummary) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindMonthlySummary
	if t.summary == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	competence := in.Intent.RefMonth()
	if competence == "" {
		competence = currentCompetence(t.loc)
	}
	summary, err := WithReadRetry(ctx, func(ctx context.Context) (budgetsoutput.MonthlySummaryOutput, error) {
		return t.summary.Execute(ctx, in.UserID.String(), competence)
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.monthly_summary_failed",
			observability.String("competence", competence),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatMonthlySummary(summary), Outcome: OutcomeRouted, Kind: kind}, nil
}

type HowAmIDoing struct {
	recorder *Recorder
	summary  MonthlySummaryReader
	loc      *time.Location
	o11y     observability.Observability
}

func NewHowAmIDoing(recorder *Recorder, summary MonthlySummaryReader, loc *time.Location, o11y observability.Observability) *HowAmIDoing {
	return &HowAmIDoing{recorder: recorder, summary: summary, loc: loc, o11y: o11y}
}

func (t *HowAmIDoing) Name() string { return "how_am_i_doing" }

func (t *HowAmIDoing) Descriptor() ToolSpec {
	return ToolSpec{Name: "how_am_i_doing", IntentKind: intent.KindHowAmIDoing, Description: "how_am_i_doing", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *HowAmIDoing) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindHowAmIDoing
	if t.summary == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	competence := currentCompetence(t.loc)
	summary, err := WithReadRetry(ctx, func(ctx context.Context) (budgetsoutput.MonthlySummaryOutput, error) {
		return t.summary.Execute(ctx, in.UserID.String(), competence)
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.how_am_i_doing_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatHowAmIDoing(summary), Outcome: OutcomeRouted, Kind: kind}, nil
}

type QueryCategory struct {
	recorder *Recorder
	summary  MonthlySummaryReader
	loc      *time.Location
	o11y     observability.Observability
}

func NewQueryCategory(recorder *Recorder, summary MonthlySummaryReader, loc *time.Location, o11y observability.Observability) *QueryCategory {
	return &QueryCategory{recorder: recorder, summary: summary, loc: loc, o11y: o11y}
}

func (t *QueryCategory) Name() string { return "query_category" }

func (t *QueryCategory) Descriptor() ToolSpec {
	return ToolSpec{Name: "query_category", IntentKind: intent.KindQueryCategory, Description: "query_category", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *QueryCategory) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindQueryCategory
	if t.summary == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	competence := currentCompetence(t.loc)
	summary, err := WithReadRetry(ctx, func(ctx context.Context) (budgetsoutput.MonthlySummaryOutput, error) {
		return t.summary.Execute(ctx, in.UserID.String(), competence)
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.query_category_failed",
			observability.String("category", in.Intent.CategoryName()),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatCategoryAllocation(summary, in.Intent.CategoryName()), Outcome: OutcomeRouted, Kind: kind}, nil
}

type QueryGoal struct {
	recorder *Recorder
	summary  MonthlySummaryReader
	loc      *time.Location
	o11y     observability.Observability
}

func NewQueryGoal(recorder *Recorder, summary MonthlySummaryReader, loc *time.Location, o11y observability.Observability) *QueryGoal {
	return &QueryGoal{recorder: recorder, summary: summary, loc: loc, o11y: o11y}
}

func (t *QueryGoal) Name() string { return "query_goal" }

func (t *QueryGoal) Descriptor() ToolSpec {
	return ToolSpec{Name: "query_goal", IntentKind: intent.KindQueryGoal, Description: "query_goal", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *QueryGoal) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindQueryGoal
	if t.summary == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: formatGoalUnavailable(in.Intent.GoalName()), Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	competence := currentCompetence(t.loc)
	summary, err := WithReadRetry(ctx, func(ctx context.Context) (budgetsoutput.MonthlySummaryOutput, error) {
		return t.summary.Execute(ctx, in.UserID.String(), competence)
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.query_goal_failed",
			observability.String("competence", competence),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatGoalProgress(summary, in.Intent.GoalName()), Outcome: OutcomeRouted, Kind: kind}, nil
}

type QueryCard struct {
	recorder *Recorder
	lister   CardLister
	invoice  CardInvoiceReader
	loc      *time.Location
	o11y     observability.Observability
}

func NewQueryCard(recorder *Recorder, lister CardLister, invoice CardInvoiceReader, loc *time.Location, o11y observability.Observability) *QueryCard {
	return &QueryCard{recorder: recorder, lister: lister, invoice: invoice, loc: loc, o11y: o11y}
}

func (t *QueryCard) Name() string { return "query_card" }

func (t *QueryCard) Descriptor() ToolSpec {
	return ToolSpec{Name: "query_card", IntentKind: intent.KindQueryCard, Description: "query_card", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *QueryCard) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindQueryCard
	if t.lister == nil || t.invoice == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	cards, err := WithReadRetry(ctx, func(ctx context.Context) (cardoutput.CardList, error) {
		return t.lister.Execute(ctx, cardinput.ListCards{UserID: in.UserID, Limit: defaultListCardsLimit})
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.query_card_list_failed",
			observability.String("card_name", in.Intent.CardName()),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	resolved, ok := resolveCardByName(cards, in.Intent.CardName())
	if !ok {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: formatCardNotFound(in.Intent.CardName()), Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	cardID, parseErr := uuid.Parse(resolved.ID)
	if parseErr != nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	now := time.Now().UTC().In(t.loc)
	invoice, err := WithReadRetry(ctx, func(ctx context.Context) (cardoutput.Invoice, error) {
		return t.invoice.Execute(ctx, cardinput.InvoiceFor{CardID: cardID, UserID: in.UserID, Purchase: now})
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.query_card_invoice_failed",
			observability.String("card_name", in.Intent.CardName()),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatCardInvoice(resolved, invoice), Outcome: OutcomeRouted, Kind: kind}, nil
}

type ConfigureBudget struct {
	recorder *Recorder
	session  *BudgetSessionRunner
	o11y     observability.Observability
}

func NewConfigureBudget(recorder *Recorder, session *BudgetSessionRunner, o11y observability.Observability) *ConfigureBudget {
	return &ConfigureBudget{recorder: recorder, session: session, o11y: o11y}
}

func (t *ConfigureBudget) Name() string { return "configure_budget" }

func (t *ConfigureBudget) Descriptor() ToolSpec {
	return ToolSpec{Name: "configure_budget", IntentKind: intent.KindConfigureBudget, Description: "configure_budget", SchemaVersion: "v1", Timeout: 8 * time.Second, AuthzMode: AuthzUserOwned}
}

func (t *ConfigureBudget) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindConfigureBudget
	if !t.session.Enabled() {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.configure_budget_session_unavailable",
			observability.String("channel", in.Channel),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	change := budgetdraft.Change{
		TotalCents:  in.Intent.BudgetTotalCents(),
		Allocations: in.Intent.BudgetAllocations(),
	}
	return t.session.Start(ctx, in.UserID, in.Channel, in.MessageID, change), nil
}

type EditCategoryPercentage struct {
	recorder *Recorder
	editor   CategoryPercentageEditor
	loc      *time.Location
	o11y     observability.Observability
}

func NewEditCategoryPercentage(recorder *Recorder, editor CategoryPercentageEditor, loc *time.Location, o11y observability.Observability) *EditCategoryPercentage {
	return &EditCategoryPercentage{recorder: recorder, editor: editor, loc: loc, o11y: o11y}
}

func (t *EditCategoryPercentage) Name() string { return "edit_category_percentage" }

func (t *EditCategoryPercentage) Descriptor() ToolSpec {
	return ToolSpec{Name: "edit_category_percentage", IntentKind: intent.KindEditCategoryPercentage, Description: "edit_category_percentage", SchemaVersion: "v1", Timeout: 8 * time.Second, AuthzMode: AuthzUserOwned}
}

func (t *EditCategoryPercentage) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindEditCategoryPercentage
	if t.editor == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	if in.Intent.Percentage() == 0 {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeClarify)
		return ToolResult{Reply: formatCategoryPercentageMissing(in.Intent.CategoryName()), Outcome: OutcomeClarify, Kind: kind}, nil
	}
	result, err := t.editor.Execute(ctx, CategoryPercentageEditorInput{
		UserID:       in.UserID,
		Competence:   currentCompetence(t.loc),
		CategoryName: in.Intent.CategoryName(),
		Percentage:   in.Intent.Percentage(),
	})
	if err != nil {
		if errors.Is(err, ErrCategoryPercentageUnknownCategory) {
			t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeClarify)
			return ToolResult{Reply: formatCategoryNotFound(in.Intent.CategoryName()), Outcome: OutcomeClarify, Kind: kind}, nil
		}
		if errors.Is(err, ErrCategoryPercentageNoBudget) {
			t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeClarify)
			return ToolResult{Reply: budgetNotActiveText, Outcome: OutcomeClarify, Kind: kind}, nil
		}
		t.o11y.Logger().Warn(ctx, "agent.intent_router.edit_category_percentage_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatCategoryPercentageUpdated(in.Intent.CategoryName(), result.Percentage), Outcome: OutcomeRouted, Kind: kind}, nil
}

type BudgetRecurrenceCreatorTool struct {
	recorder *Recorder
	creator  BudgetRecurrenceCreator
	loc      *time.Location
	o11y     observability.Observability
}

func NewBudgetRecurrenceCreatorTool(recorder *Recorder, creator BudgetRecurrenceCreator, loc *time.Location, o11y observability.Observability) *BudgetRecurrenceCreatorTool {
	return &BudgetRecurrenceCreatorTool{recorder: recorder, creator: creator, loc: loc, o11y: o11y}
}

func (t *BudgetRecurrenceCreatorTool) Name() string { return "budget_recurrence" }

func (t *BudgetRecurrenceCreatorTool) Descriptor() ToolSpec {
	return ToolSpec{Name: "budget_recurrence", IntentKind: intent.KindBudgetRecurrence, Description: "budget_recurrence", SchemaVersion: "v1", Timeout: 8 * time.Second, AuthzMode: AuthzUserOwned}
}

func (t *BudgetRecurrenceCreatorTool) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindBudgetRecurrence
	if t.creator == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	sourceCompetence := in.Intent.SourceCompetence()
	if sourceCompetence == "" {
		sourceCompetence = currentCompetence(t.loc)
	}
	months := in.Intent.Months()
	if months <= 0 {
		months = 1
	}
	result, err := t.creator.Execute(ctx, BudgetRecurrenceCreatorInput{
		UserID:           in.UserID,
		SourceCompetence: sourceCompetence,
		Months:           months,
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.budget_recurrence_failed",
			observability.String("source_competence", sourceCompetence),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatBudgetRecurrenceCreated(result), Outcome: OutcomeRouted, Kind: kind}, nil
}
