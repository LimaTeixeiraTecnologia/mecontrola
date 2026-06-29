package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type RegistryTestSuite struct {
	suite.Suite
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}

func (s *RegistryTestSuite) TestRegisterAndResolve_Found() {
	reg := NewAgentRegistry()
	a := &fakeAgent{id: "test-agent"}
	reg.Register(a)

	got, err := reg.Resolve("test-agent")
	s.NoError(err)
	s.Equal(a, got)
}

func (s *RegistryTestSuite) TestResolve_NotFound() {
	reg := NewAgentRegistry()
	_, err := reg.Resolve("non-existent")
	s.Error(err)
	s.True(errors.Is(err, ErrAgentNotFound))
}

func (s *RegistryTestSuite) TestWorkflowRegistry_RegisterAndResolve() {
	reg := NewWorkflowRegistry[map[string]any]()
	def := workflow.Definition[map[string]any]{ID: "wf-1"}
	reg.Register(def)

	got, ok := reg.Resolve("wf-1")
	s.True(ok)
	s.Equal("wf-1", got.ID)
}

func (s *RegistryTestSuite) TestWorkflowRegistry_ResolveNotFound() {
	reg := NewWorkflowRegistry[map[string]any]()
	_, ok := reg.Resolve("not-there")
	s.False(ok)
}

type fakeAgent struct {
	id           string
	instructions string
	result       Result
	err          error
}

func (f *fakeAgent) ID() string { return f.id }

func (f *fakeAgent) Instructions() string { return f.instructions }

func (f *fakeAgent) Execute(_ context.Context, _ Request) (Result, error) {
	return f.result, f.err
}

func (f *fakeAgent) Stream(_ context.Context, _ Request) (ResultStream, error) {
	return nil, nil
}
