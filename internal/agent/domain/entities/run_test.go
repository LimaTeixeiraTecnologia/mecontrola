package entities_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

type RunSuite struct {
	suite.Suite
}

func TestRunSuite(t *testing.T) {
	suite.Run(t, new(RunSuite))
}

func baseStartRunParams() entities.StartRunParams {
	return entities.StartRunParams{
		ThreadID:   uuid.New(),
		UserID:     uuid.New(),
		Channel:    "whatsapp",
		MessageID:  "wamid.ABC",
		AgentID:    "daily",
		Workflow:   "cards",
		ToolName:   "createCardTool",
		IntentKind: "create_card",
	}
}

func (s *RunSuite) TestRunStatusString() {
	s.Equal("running", entities.RunStatusRunning.String())
	s.Equal("succeeded", entities.RunStatusSucceeded.String())
	s.Equal("failed", entities.RunStatusFailed.String())
	s.Equal("", entities.RunStatus(0).String())
}

func (s *RunSuite) TestParseRunStatus() {
	cases := []struct {
		raw    string
		want   entities.RunStatus
		hasErr bool
	}{
		{raw: "running", want: entities.RunStatusRunning},
		{raw: " SUCCEEDED ", want: entities.RunStatusSucceeded},
		{raw: "failed", want: entities.RunStatusFailed},
		{raw: "unknown", hasErr: true},
		{raw: "", hasErr: true},
	}
	for _, tc := range cases {
		s.Run(tc.raw, func() {
			got, err := entities.ParseRunStatus(tc.raw)
			if tc.hasErr {
				s.Require().ErrorIs(err, entities.ErrRunStatusInvalid)
				return
			}
			s.Require().NoError(err)
			s.Equal(tc.want, got)
			s.True(got.IsValid())
		})
	}
}

func (s *RunSuite) TestStartRunValidation() {
	cases := []struct {
		name    string
		mutate  func(*entities.StartRunParams)
		wantErr error
	}{
		{name: "valido", mutate: func(*entities.StartRunParams) {}, wantErr: nil},
		{name: "thread nil", mutate: func(p *entities.StartRunParams) { p.ThreadID = uuid.Nil }, wantErr: entities.ErrRunThreadRequired},
		{name: "user nil", mutate: func(p *entities.StartRunParams) { p.UserID = uuid.Nil }, wantErr: entities.ErrRunUserRequired},
		{name: "channel vazio", mutate: func(p *entities.StartRunParams) { p.Channel = "  " }, wantErr: entities.ErrRunChannelRequired},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			params := baseStartRunParams()
			tc.mutate(&params)
			run, err := entities.StartRun(params)
			if tc.wantErr != nil {
				s.Require().ErrorIs(err, tc.wantErr)
				return
			}
			s.Require().NoError(err)
			s.NotEqual(uuid.Nil, run.ID())
			s.Equal(entities.RunStatusRunning, run.Status())
			s.False(run.StartedAt().IsZero())
			_, ended := run.EndedAt()
			s.False(ended)
			_, hasDecision := run.DecisionID()
			s.False(hasDecision)
		})
	}
}

func (s *RunSuite) TestStartRunSchemaVersionDefault() {
	params := baseStartRunParams()
	params.SchemaVersion = ""

	run, err := entities.StartRun(params)
	s.Require().NoError(err)
	s.Equal("v1", run.SchemaVersion())
}

func (s *RunSuite) TestStartRunSchemaVersionExplicit() {
	params := baseStartRunParams()
	params.SchemaVersion = "v2"

	run, err := entities.StartRun(params)
	s.Require().NoError(err)
	s.Equal("v2", run.SchemaVersion())

	finished := run.Finish("routed", true, "")
	s.Equal("v2", finished.SchemaVersion())
}

func (s *RunSuite) TestStartRunWithDecision() {
	params := baseStartRunParams()
	decisionID := uuid.New()
	params.DecisionID = decisionID

	run, err := entities.StartRun(params)
	s.Require().NoError(err)
	got, ok := run.DecisionID()
	s.True(ok)
	s.Equal(decisionID, got)
}

func (s *RunSuite) TestRunningHasNoEndedAt() {
	run, err := entities.StartRun(baseStartRunParams())
	s.Require().NoError(err)
	_, ok := run.EndedAt()
	s.False(ok)
	s.Equal(int64(0), run.DurationMs())
}

func (s *RunSuite) TestFinishSucceeded() {
	run, err := entities.StartRun(baseStartRunParams())
	s.Require().NoError(err)

	finished := run.Finish("routed", true, "ignored")
	s.Equal(entities.RunStatusSucceeded, finished.Status())
	s.Equal("routed", finished.Outcome())
	s.Empty(finished.ErrText())
	_, ended := finished.EndedAt()
	s.True(ended)
	s.GreaterOrEqual(finished.DurationMs(), int64(0))

	s.Equal(entities.RunStatusRunning, run.Status())
	_, stillEnded := run.EndedAt()
	s.False(stillEnded)
}

func (s *RunSuite) TestFinishFailed() {
	run, err := entities.StartRun(baseStartRunParams())
	s.Require().NoError(err)

	finished := run.Finish("usecase_error", false, "boom")
	s.Equal(entities.RunStatusFailed, finished.Status())
	s.Equal("usecase_error", finished.Outcome())
	s.Equal("boom", finished.ErrText())
	_, ended := finished.EndedAt()
	s.True(ended)
}
