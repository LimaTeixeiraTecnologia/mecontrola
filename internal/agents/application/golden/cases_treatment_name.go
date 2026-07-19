package golden

import "strings"

var treatmentNameToneEmojis = []string{"✅", "💰", "💳", "📂", "📥", "📊", "⚠️", "🚨", "🎉", "💚"}

func toneAdherentConfirmation(response string) bool {
	if strings.TrimSpace(response) == "" {
		return false
	}
	if strings.Contains(response, "**") {
		return false
	}
	if strings.Count(response, "*")%2 != 0 {
		return false
	}
	for _, e := range treatmentNameToneEmojis {
		if strings.Contains(response, e) {
			return true
		}
	}
	return false
}

const treatmentNameWorkingMemory = "## Nome de Tratamento\n\nJJ\n\n## Objetivo Financeiro\n\nR$ 13.874,40"

const treatmentNameAbsentWorkingMemory = "## Objetivo Financeiro\n\nR$ 13.874,40"

func usesTreatmentNameOnce(name string) ResponsePropertyFunc {
	return func(response string) bool {
		return strings.Count(response, name) == 1
	}
}

func neutralWithoutNameQuestion(response string) bool {
	if strings.TrimSpace(response) == "" {
		return false
	}
	lowered := strings.ToLower(response)
	if strings.Contains(lowered, "te chamasse") || strings.Contains(lowered, "te chamar") {
		return false
	}
	return !strings.Contains(response, "JJ")
}

func treatmentNameCases() []Case {
	return []Case{
		{
			Name:             "usa nome vigente em saudacao livre",
			Category:         CategoryTreatmentName,
			Origin:           "synthetic",
			WorkingMemory:    treatmentNameWorkingMemory,
			Input:            "bom dia!",
			ToolSubset:       []string{"category_detail"},
			NoToolExpected:   true,
			ResponseProperty: usesTreatmentNameOnce("JJ"),
			ResponseDescribe: "resposta livre contém o nome de tratamento vigente exatamente uma vez",
		},
		{
			Name:             "usa nome vigente em resposta de consulta",
			Category:         CategoryTreatmentName,
			Origin:           "synthetic",
			WorkingMemory:    treatmentNameWorkingMemory,
			Input:            "quanto já gastei esse mês?",
			ToolSubset:       []string{"category_detail_com_dados"},
			ExpectedTool:     "category_detail",
			ResponseProperty: usesTreatmentNameOnce("JJ"),
			ResponseDescribe: "resposta de consulta usa o nome de tratamento vigente exatamente uma vez",
		},
		{
			Name:             "sem nome vigente nao inventa nem pergunta nome",
			Category:         CategoryTreatmentName,
			Origin:           "synthetic",
			WorkingMemory:    treatmentNameAbsentWorkingMemory,
			Input:            "bom dia!",
			ToolSubset:       []string{"category_detail"},
			NoToolExpected:   true,
			ResponseProperty: neutralWithoutNameQuestion,
			ResponseDescribe: "sem seção de nome na working memory, responde neutro sem perguntar como chamar e sem inventar nome",
		},
		{
			Name:             "alterar nome de tratamento com nome informado",
			Category:         CategoryTreatmentName,
			Origin:           "synthetic",
			Input:            "agora me chama de Stef",
			ToolSubset:       []string{"edit_treatment_name"},
			ExpectedTool:     "edit_treatment_name",
			ExpectedArgs:     map[string]any{"name": "Stef"},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "aplica a troca de nome de tratamento em turno único, já com o nome informado na mensagem",
		},
		{
			Name:             "alterar nome de tratamento sem nome informado",
			Category:         CategoryTreatmentName,
			Origin:           "synthetic",
			Input:            "quero trocar como você me chama",
			ToolSubset:       []string{"edit_treatment_name"},
			ExpectedTool:     "edit_treatment_name",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "inicia a troca de nome de tratamento perguntando o novo nome, sem exigi-lo na primeira mensagem",
		},
		{
			Name:             "confirmacao de troca de nome no tom oficial",
			Category:         CategoryTreatmentName,
			Origin:           "synthetic",
			Input:            "agora me chama de Stef",
			ToolSubset:       []string{"edit_treatment_name"},
			ExpectedTool:     "edit_treatment_name",
			ExpectedArgs:     map[string]any{"name": "Stef"},
			ResponseProperty: toneAdherentConfirmation,
			ResponseDescribe: "confirma a troca de nome usando negrito com asterisco simples e emoji oficial do Tom de Voz",
		},
	}
}
