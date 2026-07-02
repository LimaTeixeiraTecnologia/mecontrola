//go:build e2e

package e2e_test

import (
	"net/http"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerUpdateSteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	sc.Step(`^o usuário atualiza o cartão informando apenas o apelido "([^"]*)"$`, e.updateCardNickname)
	sc.Step(`^o usuário atualiza o cartão informando banco "([^"]*)" e vencimento (\d+)$`, e.updateCardBankAndDueDay)
	sc.Step(`^o usuário tenta atualizar um cartão com ID inexistente informando o apelido "([^"]*)"$`, e.tryUpdateNonExistentCard)
	sc.Step(`^o usuário tenta atualizar o cartão com apelido de 33 caracteres$`, e.tryUpdateCardWithNicknameTooLong)
}

func (e *cardE2ECtx) updateCardNickname(apelido string) error {
	payload := map[string]any{
		"nickname": apelido,
	}
	return e.makeRequest(http.MethodPut, "/api/v1/cards/"+e.cardID+"/", payload)
}

func (e *cardE2ECtx) updateCardBankAndDueDay(banco string, vencimento int) error {
	payload := map[string]any{
		"bank":    banco,
		"due_day": vencimento,
	}
	return e.makeRequest(http.MethodPut, "/api/v1/cards/"+e.cardID+"/", payload)
}

func (e *cardE2ECtx) tryUpdateNonExistentCard(apelido string) error {
	payload := map[string]any{
		"nickname": apelido,
	}
	return e.makeRequest(http.MethodPut, "/api/v1/cards/"+uuid.NewString()+"/", payload)
}

func (e *cardE2ECtx) tryUpdateCardWithNicknameTooLong() error {
	payload := map[string]any{
		"nickname": strings.Repeat("x", 33),
	}
	return e.makeRequest(http.MethodPut, "/api/v1/cards/"+e.cardID+"/", payload)
}
