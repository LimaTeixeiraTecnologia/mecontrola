//go:build e2e

package transactions_e2e_test

import (
	"net/http"

	"github.com/cucumber/godog"
)

func registerMonthlySteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	sc.Step(`^o usuário obtém o resumo do mês "([^"]*)"$`, e.oUsuarioObtemOResumoDoMes)
	sc.Step(`^o usuário lista as entradas do mês "([^"]*)"$`, e.oUsuarioListaAsEntradasDoMes)
}

func (e *txE2ECtx) oUsuarioObtemOResumoDoMes(refMonth string) error {
	return e.makeRequest(http.MethodGet, "/api/v1/months/"+refMonth, nil)
}

func (e *txE2ECtx) oUsuarioListaAsEntradasDoMes(refMonth string) error {
	return e.makeRequest(http.MethodGet, "/api/v1/months/"+refMonth+"/entries", nil)
}
