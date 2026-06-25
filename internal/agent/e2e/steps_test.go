//go:build e2e

package e2e_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerSteps(sc *godog.ScenarioContext, e *agentE2ECtx) {
	sc.Step(`^que o usuário está ativo$`, e.givenUserIsActive)
	sc.Step(`^existe um segundo usuário "([^"]*)" ativo$`, e.givenSecondUserIsActive)
	sc.Step(`^o usuário envia "([^"]*)" via webhook$`, e.whenUserSendsMessageViaWebhook)
	sc.Step(`^o usuário reenvia a última mensagem com o mesmo identificador$`, e.whenUserResendsSameMessage)
	sc.Step(`^a resposta HTTP deve ter status (\d+)$`, e.thenResponseStatusShouldBe)
	sc.Step(`^o número de transações do usuário aumentou em (\d+)$`, e.thenUserTransactionCountIncreasedBy)
	sc.Step(`^nenhuma transação foi persistida$`, e.thenNoTransactionWasPersisted)
	sc.Step(`^o gateway respondeu ao usuário$`, e.thenGatewayRepliedToUser)
	sc.Step(`^a resposta do gateway não contém "([^"]*)"$`, e.thenGatewayReplyDoesNotContain)
	sc.Step(`^o segundo usuário não vê transações novas$`, e.thenSecondUserSeesNoNewTransactions)
	sc.Step(`^o evento "([^"]*)" deve estar no outbox do usuário$`, e.thenOutboxContainsEventForUser)
	sc.Step(`^o outbox é processado$`, e.whenOutboxIsDrained)
	sc.Step(`^o resumo mensal do usuário reflete a despesa$`, e.thenMonthlySummaryReflectsExpense)
	sc.Step(`^a despesa do orçamento do usuário foi registrada$`, e.thenBudgetsExpenseWasRecorded)
	sc.Step(`^o orçamento do usuário registrou (\d+) parcelas da compra$`, e.thenBudgetsRecordedInstallments)
	registerWriteSteps(sc, e)
	registerReadSteps(sc, e)
	registerCardsSteps(sc, e)
	registerByRefSteps(sc, e)
}

func (e *agentE2ECtx) givenUserIsActive() error {
	if e.userID == uuid.Nil {
		return fmt.Errorf("usuario ativo nao foi semeado")
	}
	return nil
}

func (e *agentE2ECtx) givenSecondUserIsActive(waNumber string) error {
	e.secondID = SeedActiveUserWA(e.t, e.db, waNumber)

	before, err := e.countTransactions(e.secondID)
	if err != nil {
		return err
	}
	e.beforeOther = before
	return nil
}

func (e *agentE2ECtx) whenUserSendsMessageViaWebhook(text string) error {
	before, err := e.countTransactions(e.userID)
	if err != nil {
		return err
	}
	e.beforeUser = before
	e.lastRefMonth = time.Now().UTC().Format("2006-01")

	beforeCard, err := e.countCardPurchases(e.userID)
	if err != nil {
		return err
	}
	e.beforeCardPurchases = beforeCard

	beforeCards, err := e.countCards(e.userID)
	if err != nil {
		return err
	}
	e.beforeCards = beforeCards

	wamid := "wamid.e2e." + uuid.New().String()
	return e.postWebhook(text, wamid)
}

func (e *agentE2ECtx) whenUserResendsSameMessage() error {
	before, err := e.countTransactions(e.userID)
	if err != nil {
		return err
	}
	e.beforeUser = before

	beforeCards, err := e.countCards(e.userID)
	if err != nil {
		return err
	}
	e.beforeCards = beforeCards

	if e.lastWAMID == "" {
		return fmt.Errorf("nenhuma mensagem anterior para reenviar")
	}
	return e.postWebhook("gastei 50 no mercado", e.lastWAMID)
}

func (e *agentE2ECtx) thenResponseStatusShouldBe(status int) error {
	if e.lastStatus != status {
		return fmt.Errorf("status esperado %d, recebido %d", status, e.lastStatus)
	}
	return nil
}

func (e *agentE2ECtx) thenUserTransactionCountIncreasedBy(delta int) error {
	after, err := e.countTransactions(e.userID)
	if err != nil {
		return err
	}
	if after != e.beforeUser+delta {
		return fmt.Errorf("esperado aumento de %d transacoes (de %d para %d), recebido %d", delta, e.beforeUser, e.beforeUser+delta, after)
	}
	return nil
}

func (e *agentE2ECtx) thenNoTransactionWasPersisted() error {
	after, err := e.countTransactions(e.userID)
	if err != nil {
		return err
	}
	if after != e.beforeUser {
		return fmt.Errorf("esperado nenhuma transacao nova (%d), recebido %d", e.beforeUser, after)
	}
	return nil
}

func (e *agentE2ECtx) thenGatewayRepliedToUser() error {
	reply, ok := e.gateway.LastReply()
	if !ok {
		return fmt.Errorf("gateway nao respondeu ao usuario")
	}
	if reply.To != e.waNumber {
		return fmt.Errorf("resposta destinada a %q, esperado %q", reply.To, e.waNumber)
	}
	if reply.Text == "" {
		return fmt.Errorf("resposta do gateway vazia")
	}
	return nil
}

func (e *agentE2ECtx) thenGatewayReplyDoesNotContain(needle string) error {
	reply, ok := e.gateway.LastReply()
	if !ok {
		return fmt.Errorf("gateway nao respondeu ao usuario")
	}
	if strings.Contains(strings.ToLower(reply.Text), strings.ToLower(needle)) {
		return fmt.Errorf("resposta do gateway contem %q: %s", needle, reply.Text)
	}
	return nil
}

func (e *agentE2ECtx) thenSecondUserSeesNoNewTransactions() error {
	after, err := e.countTransactions(e.secondID)
	if err != nil {
		return err
	}
	if after != e.beforeOther {
		return fmt.Errorf("segundo usuario viu transacoes novas: esperado %d, recebido %d", e.beforeOther, after)
	}
	return nil
}
