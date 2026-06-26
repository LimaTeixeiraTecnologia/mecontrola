package tools

import (
	"context"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

type BudgetDetails struct {
	recorder *Recorder
	summary  MonthlySummaryReader
	loc      *time.Location
	o11y     observability.Observability
}

func NewBudgetDetails(recorder *Recorder, summary MonthlySummaryReader, loc *time.Location, o11y observability.Observability) *BudgetDetails {
	return &BudgetDetails{recorder: recorder, summary: summary, loc: loc, o11y: o11y}
}

func (t *BudgetDetails) Name() string { return "get_budget_details" }

func (t *BudgetDetails) Descriptor() ToolSpec {
	return ToolSpec{Name: "get_budget_details", IntentKind: intent.KindBudgetDetails, Description: "get_budget_details", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *BudgetDetails) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindBudgetDetails
	if t.summary == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	refMonth := in.Intent.RefMonth()
	if refMonth == "" {
		refMonth = currentCompetence(t.loc)
	}
	summary, err := WithReadRetry(ctx, func(ctx context.Context) (budgetsoutput.MonthlySummaryOutput, error) {
		return t.summary.Execute(ctx, in.UserID.String(), refMonth)
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.budget_details_failed",
			observability.String("competence", refMonth),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatBudgetDetails(summary), Outcome: OutcomeRouted, Kind: kind}, nil
}

type ListCategories struct {
	recorder *Recorder
	lister   CategoryLister
	o11y     observability.Observability
}

func NewListCategories(recorder *Recorder, lister CategoryLister, o11y observability.Observability) *ListCategories {
	return &ListCategories{recorder: recorder, lister: lister, o11y: o11y}
}

func (t *ListCategories) Name() string { return "list_categories" }

func (t *ListCategories) Descriptor() ToolSpec {
	return ToolSpec{Name: "list_categories", IntentKind: intent.KindListCategories, Description: "list_categories", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *ListCategories) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindListCategories
	if t.lister == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	result, err := WithReadRetry(ctx, func(ctx context.Context) (CategoryListResult, error) {
		return t.lister.Execute(ctx, in.UserID)
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.list_categories_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatCategoryList(result), Outcome: OutcomeRouted, Kind: kind}, nil
}

type ClassifyCategory struct {
	recorder   *Recorder
	classifier CategoryClassifier
	o11y       observability.Observability
}

func NewClassifyCategory(recorder *Recorder, classifier CategoryClassifier, o11y observability.Observability) *ClassifyCategory {
	return &ClassifyCategory{recorder: recorder, classifier: classifier, o11y: o11y}
}

func (t *ClassifyCategory) Name() string { return "classify_category" }

func (t *ClassifyCategory) Descriptor() ToolSpec {
	return ToolSpec{Name: "classify_category", IntentKind: intent.KindClassifyCategory, Description: "classify_category", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *ClassifyCategory) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindClassifyCategory
	if t.classifier == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	query := strings.TrimSpace(in.Intent.SearchQuery())
	if query == "" {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeClarify)
		return ToolResult{Reply: classifyQueryMissingText, Outcome: OutcomeClarify, Kind: kind}, nil
	}
	result, err := WithReadRetry(ctx, func(ctx context.Context) (CategoryClassifyResult, error) {
		return t.classifier.Execute(ctx, CategoryClassifyInput{Query: query})
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.classify_category_failed",
			observability.String("query", query),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	if !result.Matched {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeClarify)
		return ToolResult{Reply: formatClassifyNotFound(query), Outcome: OutcomeClarify, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatCategoryClassification(result), Outcome: OutcomeRouted, Kind: kind}, nil
}
