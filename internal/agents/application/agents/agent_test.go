package agents

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
)

type AgentBuilderSuite struct {
	suite.Suite
	ctx context.Context
}

func TestAgentBuilderSuite(t *testing.T) {
	suite.Run(t, new(AgentBuilderSuite))
}

func (s *AgentBuilderSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AgentBuilderSuite) TestBuildWeatherAgent_HasCorrectID() {
	provider := llmmocks.NewProvider(s.T())
	obs := fake.NewProvider()

	a := BuildWeatherAgent(provider, nil, nil, obs)
	s.Equal(weatherAgentID, a.ID())
}

func (s *AgentBuilderSuite) TestBuildWeatherAgent_HasInstructions() {
	provider := llmmocks.NewProvider(s.T())
	obs := fake.NewProvider()

	a := BuildWeatherAgent(provider, nil, nil, obs)
	s.NotEmpty(a.Instructions())
	s.Contains(a.Instructions(), "weather")
}
