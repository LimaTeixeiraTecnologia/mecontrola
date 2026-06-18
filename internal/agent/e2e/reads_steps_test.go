//go:build e2e

package e2e_test

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerReadSteps(sc *godog.ScenarioContext, e *agentE2ECtx) {
	sc.Step(`^a resposta do gateway contém "([^"]*)"$`, e.thenGatewayReplyContains)
	sc.Step(`^o número de transações do usuário permaneceu igual$`, e.thenTransactionCountUnchanged)
	sc.Step(`^o usuário envia "([^"]*)" via webhook com assinatura inválida$`, e.whenUserSendsWithInvalidSignature)
	sc.Step(`^nenhuma compra parcelada foi persistida$`, e.thenNoCardPurchasePersisted)
}

func (e *agentE2ECtx) thenGatewayReplyContains(needle string) error {
	reply, ok := e.gateway.LastReply()
	if !ok {
		return fmt.Errorf("gateway nao respondeu ao usuario")
	}
	if !strings.Contains(strings.ToLower(reply.Text), strings.ToLower(needle)) {
		return fmt.Errorf("resposta do gateway nao contem %q: %s", needle, reply.Text)
	}
	return nil
}

func (e *agentE2ECtx) thenTransactionCountUnchanged() error {
	after, err := e.countTransactions(e.userID)
	if err != nil {
		return err
	}
	if after != e.beforeUser {
		return fmt.Errorf("esperado numero de transacoes inalterado (%d), recebido %d", e.beforeUser, after)
	}
	return nil
}

func (e *agentE2ECtx) whenUserSendsWithInvalidSignature(text string) error {
	before, err := e.countTransactions(e.userID)
	if err != nil {
		return err
	}
	e.beforeUser = before

	wamid := "wamid.e2e.invalid." + uuid.New().String()
	return e.postWebhookInvalidSignature(text, wamid)
}

func (e *agentE2ECtx) thenNoCardPurchasePersisted() error {
	count, err := e.countCardPurchases(e.userID)
	if err != nil {
		return err
	}
	if count != e.beforeCardPurchases {
		return fmt.Errorf("esperado nenhuma compra parcelada nova (%d), encontrado %d", e.beforeCardPurchases, count)
	}
	return nil
}
