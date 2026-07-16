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

func treatmentNameCases() []Case {
	return []Case{
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
