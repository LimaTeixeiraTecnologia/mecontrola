package tool

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type ToolSuite struct {
	suite.Suite
	ctx context.Context
}

func TestToolSuite(t *testing.T) {
	suite.Run(t, new(ToolSuite))
}

func (s *ToolSuite) SetupTest() {
	s.ctx = context.Background()
}

type weatherInput struct {
	City string `json:"city"`
}

type weatherOutput struct {
	Temperature float64 `json:"temperature"`
}

func (s *ToolSuite) TestNewTool_Invoke_Success() {
	h := NewTool[weatherInput, weatherOutput](
		"weather",
		"get weather for city",
		llm.Schema{Name: "weatherInput", Schema: map[string]any{"type": "object"}},
		llm.Schema{Name: "weatherOutput", Schema: map[string]any{"type": "object"}},
		func(_ context.Context, in weatherInput) (weatherOutput, error) {
			return weatherOutput{Temperature: 25.0}, nil
		},
	)

	s.Equal("weather", h.ID())
	s.Equal("get weather for city", h.Description())
	s.NotNil(h.Parameters())

	args, _ := json.Marshal(weatherInput{City: "São Paulo"})
	out, err := h.Invoke(s.ctx, args)
	s.NoError(err)

	var result weatherOutput
	s.NoError(json.Unmarshal(out, &result))
	s.Equal(25.0, result.Temperature)
}

func (s *ToolSuite) TestNewTool_Invoke_SchemaValidation() {
	schema := llm.Schema{
		Name: "weatherInput",
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"city": map[string]any{"type": "string"}},
			"required":             []string{"city"},
			"additionalProperties": false,
		},
	}
	h := NewTool[weatherInput, weatherOutput](
		"weather",
		"get weather for city",
		schema,
		llm.Schema{},
		func(_ context.Context, in weatherInput) (weatherOutput, error) {
			return weatherOutput{Temperature: 1.0}, nil
		},
	)

	_, err := h.Invoke(s.ctx, []byte(`{}`))
	s.Error(err)
	s.Contains(err.Error(), "schema validation")

	out, okErr := h.Invoke(s.ctx, []byte(`{"city":"São Paulo"}`))
	s.NoError(okErr)
	s.NotNil(out)
}

func (s *ToolSuite) TestNewTool_Invoke_MalformedSchema_FailsExplicitly() {
	schema := llm.Schema{
		Name: "weatherInput",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"city": map[string]any{"type": "string", "pattern": "("}},
		},
	}
	h := NewTool[weatherInput, weatherOutput](
		"weather",
		"get weather for city",
		schema,
		llm.Schema{},
		func(_ context.Context, in weatherInput) (weatherOutput, error) {
			return weatherOutput{Temperature: 1.0}, nil
		},
	)

	_, err := h.Invoke(s.ctx, []byte(`{"city":"São Paulo"}`))
	s.Error(err)
	s.Contains(err.Error(), "compile schema")
}

func (s *ToolSuite) TestNewTool_Invoke_EnumConstraint() {
	schema := llm.Schema{
		Name: "weatherInput",
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"city": map[string]any{"type": "string", "enum": []string{"São Paulo", "Rio"}}},
			"required":             []string{"city"},
			"additionalProperties": false,
		},
	}
	h := NewTool[weatherInput, weatherOutput](
		"weather",
		"get weather for city",
		schema,
		llm.Schema{},
		func(_ context.Context, in weatherInput) (weatherOutput, error) {
			return weatherOutput{Temperature: 1.0}, nil
		},
	)

	_, err := h.Invoke(s.ctx, []byte(`{"city":"Berlin"}`))
	s.Error(err)
	s.Contains(err.Error(), "schema validation")

	out, okErr := h.Invoke(s.ctx, []byte(`{"city":"Rio"}`))
	s.NoError(okErr)
	s.NotNil(out)
}

func (s *ToolSuite) TestNewTool_Invoke_InvalidInput() {
	h := NewTool[weatherInput, weatherOutput](
		"weather",
		"desc",
		llm.Schema{},
		llm.Schema{},
		func(_ context.Context, in weatherInput) (weatherOutput, error) {
			return weatherOutput{}, nil
		},
	)

	_, err := h.Invoke(s.ctx, []byte("not-json{{{"))
	s.Error(err)
	s.Contains(err.Error(), "unmarshal input")
}

func (s *ToolSuite) TestNewTool_Invoke_ExecError() {
	execErr := errors.New("upstream failure")
	h := NewTool[weatherInput, weatherOutput](
		"weather",
		"desc",
		llm.Schema{},
		llm.Schema{},
		func(_ context.Context, in weatherInput) (weatherOutput, error) {
			return weatherOutput{}, execErr
		},
	)

	args, _ := json.Marshal(weatherInput{City: "X"})
	_, err := h.Invoke(s.ctx, args)
	s.Error(err)
	s.ErrorIs(err, execErr)
}

type RegistrySuite struct {
	suite.Suite
	ctx context.Context
	reg Registry
}

func TestRegistrySuite(t *testing.T) {
	suite.Run(t, new(RegistrySuite))
}

func (s *RegistrySuite) SetupTest() {
	s.ctx = context.Background()
	s.reg = NewRegistry()
}

func (s *RegistrySuite) TestRegisterAndResolve_Success() {
	h := NewTool[weatherInput, weatherOutput](
		"weather",
		"desc",
		llm.Schema{},
		llm.Schema{},
		func(_ context.Context, in weatherInput) (weatherOutput, error) {
			return weatherOutput{Temperature: 10.0}, nil
		},
	)

	s.reg.Register(h)
	got, err := s.reg.Resolve("weather")
	s.NoError(err)
	s.Equal("weather", got.ID())
}

func (s *RegistrySuite) TestResolve_NotFound() {
	_, err := s.reg.Resolve("unknown-tool")
	s.Error(err)
	s.ErrorIs(err, ErrToolNotFound)
}

func (s *RegistrySuite) TestResolve_RoundTrip() {
	h := NewTool[weatherInput, weatherOutput](
		"city-tool",
		"desc",
		llm.Schema{Name: "in", Schema: map[string]any{"type": "object"}},
		llm.Schema{Name: "out", Schema: map[string]any{"type": "object"}},
		func(_ context.Context, in weatherInput) (weatherOutput, error) {
			return weatherOutput{Temperature: float64(len(in.City))}, nil
		},
	)

	s.reg.Register(h)
	got, err := s.reg.Resolve("city-tool")
	s.NoError(err)

	args, _ := json.Marshal(weatherInput{City: "ABC"})
	out, err := got.Invoke(s.ctx, args)
	s.NoError(err)

	var result weatherOutput
	s.NoError(json.Unmarshal(out, &result))
	s.Equal(float64(3), result.Temperature)
}
