package agents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type MultiItemGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestMultiItemGuardSuite(t *testing.T) {
	suite.Run(t, new(MultiItemGuardSuite))
}

func (s *MultiItemGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *MultiItemGuardSuite) TestDetectMultipleMonetaryValues() {
	type args struct {
		message string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(matched bool)
	}{
		{
			name: "dois valores separados por conectivo devem disparar",
			args: args{message: "gastei 30 no ônibus e 15 no café"},
			expect: func(matched bool) {
				s.True(matched)
			},
		},
		{
			name: "valor unico com separador de milhar BR nao deve disparar",
			args: args{message: "Recebi meu salário de R$ 13.874,40"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "valor unico com milhar e centavos nao deve disparar",
			args: args{message: "gastei R$ 1.234,56 no mercado"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "dois valores inteiros separados por e devem disparar",
			args: args{message: "paguei 50 e 30 de uber"},
			expect: func(matched bool) {
				s.True(matched)
			},
		},
		{
			name: "dois decimais sem milhar devem disparar",
			args: args{message: "30,50 e 15,20"},
			expect: func(matched bool) {
				s.True(matched)
			},
		},
		{
			name: "dois inteiros simples devem disparar",
			args: args{message: "30 e 15"},
			expect: func(matched bool) {
				s.True(matched)
			},
		},
		{
			name: "ordinal isolado nao deve disparar falso positivo",
			args: args{message: "Recebi meu 13º salário e décimo terceiro"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "data no formato dd/mm nao deve ser contada como segundo valor",
			args: args{message: "gastei 30 no dia 15/07"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "parcelamento 12x nao deve ser contado como segundo valor",
			args: args{message: "parcelei em 12x de 100 reais"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "parcelamento 10x com valor alto nao deve disparar",
			args: args{message: "comprei um celular de 2000 em 10x"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "valor unico sem separador de milhar nao deve disparar",
			args: args{message: "gastei 1234 no mercado"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "compra parcelada de valor sem simbolo colidindo com padrao de ano nao dispara falso positivo por design (aceito: preferimos nao bloquear consultas legitimas de fatura/cartao com ano de 4 digitos a cobrir este caso raro)",
			args: args{message: "comprei celular de 2000 em 12x e tênis de 300"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "compra parcelada com valores explicitos em reais dispara corretamente",
			args: args{message: "comprei celular de 2000 reais em 12x e tênis de 300 reais"},
			expect: func(matched bool) {
				s.True(matched)
			},
		},
		{
			name: "mensagem sem nenhum valor nao deve disparar",
			args: args{message: "quero saber meu saldo"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "ordinal seguido de valor monetario real nao deve disparar falso positivo (regressao G2)",
			args: args{message: "recebi meu 13º salário de R$ 5.000,00"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "dois ordinais com dois valores monetarios reais devem disparar",
			args: args{message: "recebi meu 1º salário de R$ 5.000,00 e o 2º de R$ 3.000,00"},
			expect: func(matched bool) {
				s.True(matched)
			},
		},
		{
			name: "recorrencia com valor unico e dia do mes nao deve disparar (regressao RF-29)",
			args: args{message: "quero criar um lançamento recorrente de academia 150 reais todo dia 10"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "consulta de fatura com id de cartao e mes/ano nao deve disparar (regressao RF-29)",
			args: args{message: "qual é a fatura do meu cartão cartao-001 em julho 2026?"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "melhor dia de compra com dia de vencimento nao deve disparar",
			args: args{message: "qual é o melhor dia para comprar no banco nubank com vencimento dia 10?"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "id de cartao isolado nao deve disparar",
			args: args{message: "me mostra os dados do cartão id cartao-001"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "ajuste de percentual isolado nao deve disparar",
			args: args{message: "ajusta a alocação da categoria custo_fixo para 35 porcento"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "sugestao de distribuicao com valor unico em reais nao deve disparar",
			args: args{message: "me sugere como distribuir 8000 reais no orçamento"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "consulta de orcamento por mes/ano textual nao deve disparar",
			args: args{message: "como foi meu orçamento de janeiro/2026?"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "id alfanumerico com hifen isolado nao deve disparar",
			args: args{message: "busca o lançamento com id abc-123"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
		{
			name: "uuid e itemseq de contexto tecnico do harness de teste nao devem disparar (regressao CA-03)",
			args: args{message: "meu userId é c67ee18d-1234-4321-abcd-abcdefabcdef e o wamid é wamid-ca03-001, itemSeq 1. gastei 80 reais no mercado hoje. paymentMethod: debit"},
			expect: func(matched bool) {
				s.False(matched)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			matched := detectMultipleMonetaryValues(scenario.args.message)
			scenario.expect(matched)
		})
	}
}

type stubUnderlyingAgent struct {
	executeCalled bool
	result        agent.Result
	err           error
}

func (a *stubUnderlyingAgent) ID() string           { return "stub-agent" }
func (a *stubUnderlyingAgent) Instructions() string { return "" }
func (a *stubUnderlyingAgent) Stream(ctx context.Context, in agent.Request) (agent.ResultStream, error) {
	return nil, nil
}

func (a *stubUnderlyingAgent) Execute(ctx context.Context, in agent.Request) (agent.Result, error) {
	a.executeCalled = true
	return a.result, a.err
}

func (s *MultiItemGuardSuite) TestExecute_MultiItemGuard() {
	type args struct {
		messages []llm.Message
	}
	type dependencies struct {
		underlying *stubUnderlyingAgent
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(underlying *stubUnderlyingAgent, output agent.Result, err error)
	}{
		{
			name: "dois lancamentos reais bloqueiam determinísticamente sem chamar o agente subjacente",
			args: args{messages: []llm.Message{
				{Role: "system", Content: "instructions"},
				{Role: "user", Content: "gastei 30 no ônibus e 15 no café"},
			}},
			dependencies: dependencies{underlying: &stubUnderlyingAgent{}},
			expect: func(underlying *stubUnderlyingAgent, output agent.Result, err error) {
				s.NoError(err)
				s.False(underlying.executeCalled, "nao deve chamar o LLM subjacente quando multiplos valores sao detectados")
				s.Equal(workflows.MultiItemOrientationMessage, output.Content)
				s.Equal(agent.ToolOutcomeClarify, output.ToolOutcome)
				s.Equal(agent.ExecutionModeSync, output.Mode)
			},
		},
		{
			name: "valor unico delega normalmente ao agente subjacente",
			args: args{messages: []llm.Message{
				{Role: "system", Content: "instructions"},
				{Role: "user", Content: "Recebi meu salário de R$ 13.874,40"},
			}},
			dependencies: dependencies{underlying: &stubUnderlyingAgent{result: agent.Result{Content: "ok"}}},
			expect: func(underlying *stubUnderlyingAgent, output agent.Result, err error) {
				s.NoError(err)
				s.True(underlying.executeCalled, "deve delegar ao agente subjacente quando apenas um valor e detectado")
				s.Equal("ok", output.Content)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guarded := WithMultiItemGuard(scenario.dependencies.underlying)
			output, err := guarded.Execute(s.ctx, agent.Request{Messages: scenario.args.messages})
			scenario.expect(scenario.dependencies.underlying, output, err)
		})
	}
}
