package tools

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type QueryIncomeSummary struct {
	recorder *Recorder
	reader   IncomeSummaryReader
	loc      *time.Location
	o11y     observability.Observability
}

func NewQueryIncomeSummary(recorder *Recorder, reader IncomeSummaryReader, loc *time.Location, o11y observability.Observability) *QueryIncomeSummary {
	return &QueryIncomeSummary{recorder: recorder, reader: reader, loc: loc, o11y: o11y}
}

func (t *QueryIncomeSummary) Name() string { return "query_income_summary" }

func (t *QueryIncomeSummary) Descriptor() ToolSpec {
	return ToolSpec{Name: "query_income_summary", IntentKind: intent.KindQueryIncomeSummary, Description: "query_income_summary", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *QueryIncomeSummary) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindQueryIncomeSummary
	if t.reader == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	refMonth := in.Intent.RefMonth()
	if refMonth == "" {
		refMonth = currentCompetence(t.loc)
	}
	result, err := WithReadRetry(ctx, func(ctx context.Context) (IncomeSummaryResult, error) {
		return t.reader.Execute(ctx, IncomeSummaryInput{UserID: in.UserID.String(), RefMonth: refMonth})
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.query_income_summary_failed",
			observability.String("ref_month", refMonth),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatIncomeSummary(result), Outcome: OutcomeRouted, Kind: kind}, nil
}
