package postdeploy

import "time"

const (
	MinimumSampleRuns        = 100
	MinimumSampleWindowDays  = 14
	ScorerImprovementMargin  = 0.05
	BaselineSucceededRuns    = 19
	BaselineFailedRuns       = 4
	BaselineTotalRuns        = 23
	BaselineToolCallAccuracy = 0.304
	BaselineCompleteness     = 0.149
	BaselineCategorization   = 0.565
	BaselineFailureRate      = float64(BaselineFailedRuns) / float64(BaselineTotalRuns)
	RequiredToolCallAccuracy = BaselineToolCallAccuracy + ScorerImprovementMargin
	RequiredCompleteness     = BaselineCompleteness + ScorerImprovementMargin
	RequiredCategorization   = BaselineCategorization + ScorerImprovementMargin
	ScorerIDToolCallAccuracy = "tool-call-accuracy"
	ScorerIDCompleteness     = "completeness"
	ScorerIDCategorization   = "categorization"
	ScorerIDNoDuplicateWrite = "no_duplicate_write"
)

type RunAggregate struct {
	AgentID          string
	WindowStart      time.Time
	WindowEnd        time.Time
	TotalRuns        int
	SucceededRuns    int
	FailedRuns       int
	ExpectedToolRuns int
	TruncatedRuns    int
}

type ScorerAggregate struct {
	ScorerID  string
	AgentID   string
	SampleN   int
	MeanScore float64
}

type OperationalCounters struct {
	AgentID                  string
	RunUpdateErrors          int64
	MessageAppendErrors      int64
	DuplicateWriteViolations int64
}

func (a RunAggregate) SampleWindowDays() float64 {
	if a.WindowEnd.Before(a.WindowStart) {
		return 0
	}
	return a.WindowEnd.Sub(a.WindowStart).Hours() / 24
}

func (a RunAggregate) MeetsMinimumSample() bool {
	return a.TotalRuns >= MinimumSampleRuns || a.SampleWindowDays() >= MinimumSampleWindowDays
}

func (a RunAggregate) FailureRate() float64 {
	if a.TotalRuns == 0 {
		return 0
	}
	return float64(a.FailedRuns) / float64(a.TotalRuns)
}

func (a RunAggregate) RedefinedToolCallAccuracy(hits int) float64 {
	if a.ExpectedToolRuns == 0 {
		return 0
	}
	return float64(hits) / float64(a.ExpectedToolRuns)
}

type MetricVerdict struct {
	MetricID string
	Actual   float64
	Required float64
	Passed   bool
}

type GateVerdict struct {
	AgentID                 string
	SampleSufficient        bool
	FailureRatePassed       bool
	FailureRateActual       float64
	MetricVerdicts          []MetricVerdict
	NoRegressionOperational bool
	Promote                 bool
	Reasons                 []string
}

func EvaluateGate(runs RunAggregate, toolCallHits int, scorers map[string]ScorerAggregate, ops OperationalCounters) GateVerdict {
	verdict := GateVerdict{AgentID: runs.AgentID}

	verdict.SampleSufficient = runs.MeetsMinimumSample()
	if !verdict.SampleSufficient {
		verdict.Reasons = append(verdict.Reasons, "amostra insuficiente: exige N>=100 runs ou janela>=14 dias")
	}

	verdict.FailureRateActual = runs.FailureRate()
	verdict.FailureRatePassed = verdict.FailureRateActual <= BaselineFailureRate
	if !verdict.FailureRatePassed {
		verdict.Reasons = append(verdict.Reasons, "taxa de falha acima da baseline")
	}

	toolCallAccuracy := runs.RedefinedToolCallAccuracy(toolCallHits)
	verdict.MetricVerdicts = append(verdict.MetricVerdicts, buildMetricVerdict(ScorerIDToolCallAccuracy, toolCallAccuracy, RequiredToolCallAccuracy))

	allMetricsPassed := true

	if agg, ok := scorers[ScorerIDCompleteness]; ok {
		verdict.MetricVerdicts = append(verdict.MetricVerdicts, buildMetricVerdict(ScorerIDCompleteness, agg.MeanScore, RequiredCompleteness))
	} else {
		allMetricsPassed = false
		verdict.Reasons = append(verdict.Reasons, "métrica ausente na amostra: "+ScorerIDCompleteness)
	}
	if agg, ok := scorers[ScorerIDCategorization]; ok {
		verdict.MetricVerdicts = append(verdict.MetricVerdicts, buildMetricVerdict(ScorerIDCategorization, agg.MeanScore, RequiredCategorization))
	} else {
		allMetricsPassed = false
		verdict.Reasons = append(verdict.Reasons, "métrica ausente na amostra: "+ScorerIDCategorization)
	}

	for _, mv := range verdict.MetricVerdicts {
		if !mv.Passed {
			allMetricsPassed = false
			verdict.Reasons = append(verdict.Reasons, "métrica abaixo da margem exigida: "+mv.MetricID)
		}
	}

	verdict.NoRegressionOperational = ops.RunUpdateErrors == 0 && ops.MessageAppendErrors == 0 && ops.DuplicateWriteViolations == 0 && runs.TruncatedRuns == 0
	if !verdict.NoRegressionOperational {
		verdict.Reasons = append(verdict.Reasons, "regressão operacional detectada (truncamento, update de run, append de mensagem ou escrita duplicada)")
	}

	verdict.Promote = verdict.SampleSufficient && verdict.FailureRatePassed && allMetricsPassed && verdict.NoRegressionOperational
	return verdict
}

func buildMetricVerdict(id string, actual, required float64) MetricVerdict {
	return MetricVerdict{
		MetricID: id,
		Actual:   actual,
		Required: required,
		Passed:   actual >= required,
	}
}
