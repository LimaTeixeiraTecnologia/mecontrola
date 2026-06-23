package tools

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type Recorder struct {
	routedTotal observability.Counter
}

func NewRecorder(routedTotal observability.Counter) *Recorder {
	return &Recorder{routedTotal: routedTotal}
}

func (r *Recorder) Record(ctx context.Context, kind, channel string, outcome ToolOutcome) {
	r.routedTotal.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome.String()),
	)
}
