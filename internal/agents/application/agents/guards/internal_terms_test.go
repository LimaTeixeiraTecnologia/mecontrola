package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type InternalTermsGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestInternalTermsGuardSuite(t *testing.T) {
	suite.Run(t, new(InternalTermsGuardSuite))
}

func (s *InternalTermsGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *InternalTermsGuardSuite) TestName() {
	guard := NewInternalTermsGuard()
	s.Equal("internal_terms", guard.Name())
}

func (s *InternalTermsGuardSuite) TestInspect() {
	type args struct {
		out agent.Result
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(decision GuardDecision)
	}{
		{
			name: "menciona workflow -> sanitiza",
			args: args{out: agent.Result{Content: "Seu workflow foi concluído com sucesso"}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(internalTermsFallbackMessage, decision.Result.Content)
			},
		},
		{
			name: "menciona pendencia sem acento -> sanitiza",
			args: args{out: agent.Result{Content: "Existe uma pendencia em aberto no sistema interno"}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
			},
		},
		{
			name: "vaza stack trace -> sanitiza",
			args: args{out: agent.Result{Content: "erro: goroutine 1 [running]: panic"}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
			},
		},
		{
			name: "menciona infraestrutura -> sanitiza",
			args: args{out: agent.Result{Content: "Houve uma falha na infraestrutura, tente de novo"}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(internalTermsFallbackMessage, decision.Result.Content)
			},
		},
		{
			name: "menciona correlation em ingles -> sanitiza",
			args: args{out: agent.Result{Content: "erro de correlation entre os serviços"}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(internalTermsFallbackMessage, decision.Result.Content)
			},
		},
		{
			name: "resposta natural sem termos internos -> nao trata",
			args: args{out: agent.Result{Content: "Registrei sua despesa de R$ 50,00 em *Prazeres* ✅"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "descricao legitima com palavra running nao dispara run -> nao trata",
			args: args{out: agent.Result{Content: "Registrei sua compra em *Nike Running* ✅"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guard := NewInternalTermsGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, scenario.args.out)
			scenario.expect(decision)
		})
	}
}
