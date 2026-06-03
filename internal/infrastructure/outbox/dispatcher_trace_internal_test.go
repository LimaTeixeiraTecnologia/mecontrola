package outbox

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/trace"
)

type DispatcherTraceInternalSuite struct {
	suite.Suite
}

func TestDispatcherTraceInternal(t *testing.T) {
	suite.Run(t, new(DispatcherTraceInternalSuite))
}

func (s *DispatcherTraceInternalSuite) TestContextWithEventTraceparentExtractsW3CContext() {
	traceID := "11111111111111111111111111111111"
	spanID := "2222222222222222"
	ctx := contextWithEventTraceparent(
		context.Background(),
		Headers{"traceparent": "00-" + traceID + "-" + spanID + "-01"},
	)

	spanCtx := trace.SpanContextFromContext(ctx)
	s.True(spanCtx.IsValid())
	s.True(spanCtx.IsRemote())
	s.Equal(traceID, spanCtx.TraceID().String())
	s.Equal(spanID, spanCtx.SpanID().String())
}

func (s *DispatcherTraceInternalSuite) TestContextWithEventTraceparentIgnoresMissingHeader() {
	ctx := contextWithEventTraceparent(context.Background(), Headers{})

	spanCtx := trace.SpanContextFromContext(ctx)
	s.False(spanCtx.IsValid())
}
