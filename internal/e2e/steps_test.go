//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
)

const (
	prazerosRootCategoryUUID = "ac535261-4060-56ef-b2e8-57c8cc7032d1"
)

var knownCategories = map[string]string{
	"prazeres": prazerosRootCategoryUUID,
}

func registerSteps(sc *godog.ScenarioContext, e *e2eCtx) {
	sc.Step(`^que a categoria "([^"]*)" está disponível no sistema$`, e.queACategoriaEstaDisponivel)
	sc.Step(`^o usuário cria uma transação de (\d+) centavos no método "([^"]*)"$`, e.oUsuarioCriaTransacao)
	sc.Step(`^a resposta HTTP deve ter status (\d+)$`, e.aRespostaHTTPDeveTerStatus)
	sc.Step(`^a transação deve estar salva no banco com valor (\d+)$`, e.aTransacaoDeveEstarSalvaComValor)
	sc.Step(`^o corpo da resposta deve conter o campo "([^"]*)"$`, e.oCorpoDeveConterOCampo)
	sc.Step(`^existe um produto billing configurado$`, e.existeUmProdutoBillingConfigurado)
	sc.Step(`^que existe uma assinatura billing ativa$`, e.queExisteUmaAssinaturaBillingAtiva)
	sc.Step(`^o webhook billing "([^"]*)" é enviado$`, e.oWebhookBillingEEnviado)
	sc.Step(`^a assinatura billing deve estar salva como "([^"]*)"$`, e.aAssinaturaBillingDeveEstarSalvaComo)
	sc.Step(`^o evento de domínio "([^"]*)" deve estar na outbox$`, e.oEventoDeDominioDeveEstarNaOutbox)
	sc.Step(`^o evento processado "([^"]*)" deve ter sido registrado$`, e.oEventoProcessadoDeveTerSidoRegistrado)
	sc.Step(`^o period_end da assinatura billing deve ser preservado$`, e.oPeriodEndDaAssinaturaBillingDeveSerPreservado)
	sc.Step(`^o period_end da assinatura billing deve ser estendido$`, e.oPeriodEndDaAssinaturaBillingDeveSerEstendido)
	registerCardSteps(sc, e)
}

func (e *e2eCtx) queACategoriaEstaDisponivel(nome string) error {
	id, ok := knownCategories[nome]
	if !ok {
		return fmt.Errorf("categoria %q não mapeada nos dados de seed", nome)
	}
	e.categoryID = id
	return nil
}

func (e *e2eCtx) oUsuarioCriaTransacao(centavos int, metodo string) error {
	payload := map[string]any{
		"direction":      "outcome",
		"payment_method": metodo,
		"amount_cents":   centavos,
		"description":    "e2e test transaction",
		"category_id":    e.categoryID,
		"occurred_at":    time.Now().UTC().Format(time.RFC3339),
	}
	if err := e.makeRequest(http.MethodPost, "/api/v1/transactions", payload); err != nil {
		return err
	}
	if id, ok := e.lastBody["id"].(string); ok {
		e.txID = id
	}
	return nil
}

func (e *e2eCtx) aRespostaHTTPDeveTerStatus(statusEsperado int) error {
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}
	if e.lastResp.StatusCode != statusEsperado {
		return fmt.Errorf("status esperado %d, recebido %d, corpo: %s", statusEsperado, e.lastResp.StatusCode, e.lastBodyText)
	}
	return nil
}

func (e *e2eCtx) aTransacaoDeveEstarSalvaComValor(centavosEsperados int) error {
	if e.txID == "" {
		return fmt.Errorf("ID da transação não capturado da resposta")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var valor int64
	row := e.mgr.DBTX(ctx).QueryRowContext(ctx,
		"SELECT amount_cents FROM mecontrola.transactions WHERE id = $1",
		e.txID,
	)
	if err := row.Scan(&valor); err != nil {
		return fmt.Errorf("transação %q não encontrada no banco: %w", e.txID, err)
	}
	if int(valor) != centavosEsperados {
		return fmt.Errorf("valor esperado %d centavos, banco contém %d", centavosEsperados, valor)
	}
	return nil
}

func (e *e2eCtx) oCorpoDeveConterOCampo(campo string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo da resposta está vazio ou não é JSON")
	}
	val, ok := e.lastBody[campo]
	if !ok {
		return fmt.Errorf("campo %q ausente no corpo da resposta", campo)
	}
	if val == nil || val == "" {
		return fmt.Errorf("campo %q presente mas vazio", campo)
	}
	return nil
}
