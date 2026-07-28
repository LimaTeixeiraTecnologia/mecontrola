package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type WhatsappFormatSanitizerGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestWhatsappFormatSanitizerGuardSuite(t *testing.T) {
	suite.Run(t, new(WhatsappFormatSanitizerGuardSuite))
}

func (s *WhatsappFormatSanitizerGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *WhatsappFormatSanitizerGuardSuite) TestName() {
	guard := NewWhatsappFormatSanitizerGuard()
	s.Equal("whatsapp_format_sanitizer", guard.Name())
}

func (s *WhatsappFormatSanitizerGuardSuite) TestInspect() {
	type args struct {
		out agent.Result
	}

	prodBugContent := "Neste mês, você já gastou R$ 1.055,00. 💰\n\n" +
		"### Resumo:\n" +
		"- **Total Planejado:** R$ 1.000,00\n" +
		"- **Total Gasto:** R$ 1.055,00\n" +
		"- **Status:** Você ultrapassou o planejado em 5,5% na categoria *Prazeres*.\n\n" +
		"### Alertas:\n" +
		"- Você já atingiu 211% do seu orçamento planejado para *Prazeres*.\n\n" +
		"Continue assim, mas fique atento aos gastos! 💪"

	scenarios := []struct {
		name   string
		args   args
		expect func(decision GuardDecision)
	}{
		{
			name: "reproduz bug de producao - remove cabecalho markdown, hifen e duplo asterisco",
			args: args{out: agent.Result{Content: prodBugContent}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.NotContains(decision.Result.Content, "#")
				s.NotContains(decision.Result.Content, "**")
				s.NotContains(decision.Result.Content, "\n- ")
				s.Contains(decision.Result.Content, "Total Planejado:")
				s.Contains(decision.Result.Content, "Resumo:")
				s.Contains(decision.Result.Content, "Alertas:")
				s.Contains(decision.Result.Content, "211%")
			},
		},
		{
			name: "cabecalho h1 no inicio da mensagem -> remove",
			args: args{out: agent.Result{Content: "# Título\nTexto normal"}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal("Título\nTexto normal", decision.Result.Content)
			},
		},
		{
			name: "duplo asterisco isolado -> normaliza para simples",
			args: args{out: agent.Result{Content: "Categoria **Prazeres** confirmada"}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal("Categoria *Prazeres* confirmada", decision.Result.Content)
			},
		},
		{
			name: "lista com hifen -> remove marcador preservando texto",
			args: args{out: agent.Result{Content: "Itens:\n- primeiro\n- segundo"}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal("Itens:\nprimeiro\nsegundo", decision.Result.Content)
			},
		},
		{
			name: "hifen dentro de palavra nao e tratado como bullet -> nao trata",
			args: args{out: agent.Result{Content: "Cartão nubank-platinum parcelado em 3x"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "negrito simples ja correto sem cabecalho ou lista -> nao trata",
			args: args{out: agent.Result{Content: "Registrei sua despesa de R$ 50,00 em *Prazeres* ✅"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "confirmacao verbatim de tool sem markdown -> nao trata",
			args: args{out: agent.Result{Content: "💰 Valor: R$ 120,00\n📅 Data: hoje\n📂 Categoria: Prazeres\n\nPosso registrar?"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guard := NewWhatsappFormatSanitizerGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, scenario.args.out)
			scenario.expect(decision)
		})
	}
}
