package workflows

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfirmAnswerSuite struct {
	suite.Suite
}

func TestConfirmAnswerSuite(t *testing.T) {
	suite.Run(t, new(ConfirmAnswerSuite))
}

func (s *ConfirmAnswerSuite) TestDecideConfirmAnswer() {
	scenarios := []struct {
		name   string
		text   string
		expect ConfirmAnswer
	}{
		{name: "producao: pode registrar sim com ponto", text: "Pode registrar sim.", expect: ConfirmAnswerYes},
		{name: "producao: pode registrar com ponto", text: "Pode registrar.", expect: ConfirmAnswerYes},
		{name: "producao: frase longa com afirmativa no fim", text: "Tá muito legal esse vlog. Pode registrar sim.", expect: ConfirmAnswerYes},
		{name: "producao: sim com ponto final", text: "Sim.", expect: ConfirmAnswerYes},

		{name: "legado: sim isolado", text: "sim", expect: ConfirmAnswerYes},
		{name: "legado: confirmar isolado", text: "confirmar", expect: ConfirmAnswerYes},
		{name: "legado: confirma isolado", text: "confirma", expect: ConfirmAnswerYes},
		{name: "legado: ok isolado", text: "ok", expect: ConfirmAnswerYes},
		{name: "legado: pode isolado", text: "pode", expect: ConfirmAnswerYes},
		{name: "legado: maiusculas", text: "SIM", expect: ConfirmAnswerYes},

		{name: "natural: sim pode", text: "Sim, pode", expect: ConfirmAnswerYes},
		{name: "natural: isso mesmo", text: "isso mesmo", expect: ConfirmAnswerYes},
		{name: "natural: confirmo", text: "Confirmo!", expect: ConfirmAnswerYes},
		{name: "natural: ta certo", text: "Tá certo", expect: ConfirmAnswerYes},
		{name: "natural: beleza", text: "beleza", expect: ConfirmAnswerYes},
		{name: "natural: perfeito", text: "Perfeito 👍", expect: ConfirmAnswerYes},

		{name: "legado: nao com acento", text: "não", expect: ConfirmAnswerNo},
		{name: "legado: nao sem acento", text: "nao", expect: ConfirmAnswerNo},
		{name: "legado: cancela", text: "cancela", expect: ConfirmAnswerNo},
		{name: "legado: cancels", text: "cancels", expect: ConfirmAnswerNo},
		{name: "legado: nao registra", text: "não registra", expect: ConfirmAnswerNo},
		{name: "legado: deixa pra la", text: "deixa pra lá", expect: ConfirmAnswerNo},
		{name: "natural: deixa pra la com ponto", text: "Deixa pra lá.", expect: ConfirmAnswerNo},
		{name: "natural: melhor nao", text: "melhor não", expect: ConfirmAnswerNo},
		{name: "natural: cancelar com ponto", text: "Cancelar.", expect: ConfirmAnswerNo},
		{name: "natural: ta errado", text: "tá errado", expect: ConfirmAnswerNo},

		{name: "negacao vence afirmativa: nao pode", text: "não pode", expect: ConfirmAnswerAmbiguous},
		{name: "negacao vence afirmativa: pode nao", text: "pode não", expect: ConfirmAnswerAmbiguous},

		{name: "pergunta nao confirma nem cancela", text: "Eu posso gastar 450 no relógio ou não?", expect: ConfirmAnswerAmbiguous},
		{name: "pergunta com pode nao confirma", text: "Pode me dizer quanto gastei?", expect: ConfirmAnswerAmbiguous},
		{name: "texto vazio", text: "", expect: ConfirmAnswerAmbiguous},
		{name: "so espacos", text: "   ", expect: ConfirmAnswerAmbiguous},
		{name: "so pontuacao", text: "...", expect: ConfirmAnswerAmbiguous},
		{name: "nova operacao nao confirma", text: "Quero registrar um novo cartão de crédito", expect: ConfirmAnswerAmbiguous},
		{name: "nova operacao de gasto nao confirma", text: "gastei 30 no uber", expect: ConfirmAnswerAmbiguous},
		{name: "escolha numerica permanece ambigua", text: "2", expect: ConfirmAnswerAmbiguous},
		{name: "resposta de categoria permanece ambigua", text: "Prazeres", expect: ConfirmAnswerAmbiguous},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, DecideConfirmAnswer(scenario.text))
		})
	}
}

func (s *ConfirmAnswerSuite) TestConfirmAnswerIsValid() {
	scenarios := []struct {
		name   string
		answer ConfirmAnswer
		valid  bool
		label  string
	}{
		{name: "yes", answer: ConfirmAnswerYes, valid: true, label: "yes"},
		{name: "no", answer: ConfirmAnswerNo, valid: true, label: "no"},
		{name: "ambiguous", answer: ConfirmAnswerAmbiguous, valid: true, label: "ambiguous"},
		{name: "zero value invalido", answer: ConfirmAnswer(0), valid: false, label: "unknown"},
		{name: "acima do intervalo", answer: ConfirmAnswer(99), valid: false, label: "unknown"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.valid, scenario.answer.IsValid())
			s.Equal(scenario.label, scenario.answer.String())
		})
	}
}
