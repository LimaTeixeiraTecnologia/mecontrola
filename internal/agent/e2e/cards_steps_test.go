//go:build e2e

package e2e_test

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func registerCardsSteps(sc *godog.ScenarioContext, e *agentE2ECtx) {
	sc.Step(`^o número de cartões do usuário aumentou em (\d+)$`, e.thenUserCardCountIncreasedBy)
	sc.Step(`^o número de cartões do usuário permaneceu igual$`, e.thenUserCardCountUnchanged)
	sc.Step(`^a resposta do gateway informa a quantidade real de cartões do usuário$`, e.thenGatewayReplyMatchesCardCount)
}

func (e *agentE2ECtx) thenUserCardCountIncreasedBy(delta int) error {
	after, err := e.countCards(e.userID)
	if err != nil {
		return err
	}
	if after != e.beforeCards+delta {
		return fmt.Errorf("esperado aumento de %d cartoes (de %d para %d), recebido %d", delta, e.beforeCards, e.beforeCards+delta, after)
	}
	return nil
}

func (e *agentE2ECtx) thenUserCardCountUnchanged() error {
	after, err := e.countCards(e.userID)
	if err != nil {
		return err
	}
	if after != e.beforeCards {
		return fmt.Errorf("esperado numero de cartoes inalterado (%d), recebido %d", e.beforeCards, after)
	}
	return nil
}

func (e *agentE2ECtx) thenGatewayReplyMatchesCardCount() error {
	reply, ok := e.gateway.LastReply()
	if !ok {
		return fmt.Errorf("gateway nao respondeu ao usuario")
	}
	total, err := e.countCards(e.userID)
	if err != nil {
		return err
	}
	needle := strconv.Itoa(total) + " cart"
	if !strings.Contains(strings.ToLower(reply.Text), strings.ToLower(needle)) {
		return fmt.Errorf("resposta do gateway nao reflete a contagem real de %d cartoes: %s", total, reply.Text)
	}
	return nil
}
