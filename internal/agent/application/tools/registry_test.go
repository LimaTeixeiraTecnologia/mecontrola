package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type RegistrySuite struct {
	suite.Suite
}

func TestRegistrySuite(t *testing.T) {
	suite.Run(t, new(RegistrySuite))
}

func (s *RegistrySuite) TestNewRegistry() {
	type args struct {
		specs []ToolSpec
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(reg *Registry, err error)
	}{
		{
			name: "deve construir com specs validas",
			args: args{specs: []ToolSpec{
				{Name: "a", IntentKind: intent.KindRecordExpense, Description: "desc a"},
				{Name: "b", IntentKind: intent.KindMonthlySummary, Description: "desc b"},
			}},
			expect: func(reg *Registry, err error) {
				s.NoError(err)
				s.NotNil(reg)
				s.Len(reg.Specs(), 2)
			},
		},
		{
			name: "deve preservar ordem de insercao",
			args: args{specs: []ToolSpec{
				{Name: "b", IntentKind: intent.KindMonthlySummary, Description: "desc b"},
				{Name: "a", IntentKind: intent.KindRecordExpense, Description: "desc a"},
			}},
			expect: func(reg *Registry, err error) {
				s.NoError(err)
				s.Equal("b", reg.Specs()[0].Name)
				s.Equal("a", reg.Specs()[1].Name)
			},
		},
		{
			name: "deve rejeitar registry vazio",
			args: args{specs: nil},
			expect: func(reg *Registry, err error) {
				s.ErrorIs(err, ErrEmptyRegistry)
				s.Nil(reg)
			},
		},
		{
			name: "deve rejeitar name vazio",
			args: args{specs: []ToolSpec{
				{Name: "  ", IntentKind: intent.KindRecordExpense, Description: "desc"},
			}},
			expect: func(reg *Registry, err error) {
				s.ErrorIs(err, ErrToolNameEmpty)
				s.Nil(reg)
			},
		},
		{
			name: "deve rejeitar description vazia",
			args: args{specs: []ToolSpec{
				{Name: "a", IntentKind: intent.KindRecordExpense, Description: "   "},
			}},
			expect: func(reg *Registry, err error) {
				s.ErrorIs(err, ErrToolDescriptionEmpty)
				s.Nil(reg)
			},
		},
		{
			name: "deve rejeitar name duplicado",
			args: args{specs: []ToolSpec{
				{Name: "a", IntentKind: intent.KindRecordExpense, Description: "desc"},
				{Name: "a", IntentKind: intent.KindMonthlySummary, Description: "desc"},
			}},
			expect: func(reg *Registry, err error) {
				s.ErrorIs(err, ErrDuplicateToolName)
				s.Nil(reg)
			},
		},
		{
			name: "deve rejeitar intent kind duplicado",
			args: args{specs: []ToolSpec{
				{Name: "a", IntentKind: intent.KindRecordExpense, Description: "desc"},
				{Name: "b", IntentKind: intent.KindRecordExpense, Description: "desc"},
			}},
			expect: func(reg *Registry, err error) {
				s.ErrorIs(err, ErrDuplicateIntentKind)
				s.Nil(reg)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			reg, err := NewRegistry(scenario.args.specs...)
			scenario.expect(reg, err)
		})
	}
}

func (s *RegistrySuite) TestDefaultRegistry() {
	reg, err := DefaultRegistry()
	s.NoError(err)
	s.NotNil(reg)
	s.Len(reg.Specs(), 6)

	kinds := []intent.Kind{
		intent.KindRecordExpense,
		intent.KindMonthlySummary,
		intent.KindListCards,
		intent.KindCreateCard,
		intent.KindCountCards,
		intent.KindConfigureBudget,
	}
	for _, kind := range kinds {
		spec, ok := reg.SpecByIntent(kind)
		s.True(ok, "intent kind %q deve resolver para uma tool", kind.String())
		s.NotEmpty(spec.Name)
		s.NotEmpty(spec.Description)
	}
}

func (s *RegistrySuite) TestSpecByIntentNotFound() {
	reg, err := DefaultRegistry()
	s.NoError(err)
	_, ok := reg.SpecByIntent(intent.KindUnknown)
	s.False(ok)
}

func (s *RegistrySuite) TestRenderSystemPrompt() {
	reg, err := DefaultRegistry()
	s.NoError(err)

	prompt, err := reg.RenderSystemPrompt()
	s.NoError(err)
	s.NotEmpty(prompt)

	for _, name := range []string{
		"record_transaction",
		"monthly_summary",
		"list_cards",
		"create_card",
		"count_cards",
		"configure_budget",
	} {
		s.True(strings.Contains(prompt, name), "prompt deve conter %q", name)
	}

	again, err := reg.RenderSystemPrompt()
	s.NoError(err)
	s.Equal(prompt, again)
}
