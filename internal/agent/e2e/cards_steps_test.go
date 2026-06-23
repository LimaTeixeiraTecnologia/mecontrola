//go:build e2e

package e2e_test

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

func registerCardsSteps(sc *godog.ScenarioContext, e *agentE2ECtx) {
	sc.Step(`^o número de cartões do usuário aumentou em (\d+)$`, e.thenUserCardCountIncreasedBy)
	sc.Step(`^o número de cartões do usuário permaneceu igual$`, e.thenUserCardCountUnchanged)
	sc.Step(`^a resposta do gateway informa a quantidade real de cartões do usuário$`, e.thenGatewayReplyMatchesCardCount)
	sc.Step(`^o apelido do cartão "([^"]*)" passou a ser "([^"]*)"$`, e.thenCardNicknameBecame)
	sc.Step(`^o vencimento do cartão "([^"]*)" passou a ser dia (\d+)$`, e.thenCardDueDayBecame)
	sc.Step(`^o cartão "([^"]*)" não aparece mais na listagem$`, e.thenCardNotInListing)
	sc.Step(`^o percentual da categoria "([^"]*)" passou a ser (\d+)%$`, e.thenCategoryPercentageBecame)
	sc.Step(`^a soma dos percentuais do orçamento permanece 100%$`, e.thenAllocationSumStays100)
	sc.Step(`^o usuário possui um orçamento ativo$`, e.givenActiveBudgetSeeded)
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

func (e *agentE2ECtx) thenCardNicknameBecame(cardName, expected string) error {
	got, err := e.cardNicknameByName(e.userID, cardName)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(expected)) {
		return fmt.Errorf("apelido esperado %q para o cartao %q, persistido %q", expected, cardName, got)
	}
	return nil
}

func (e *agentE2ECtx) thenCardDueDayBecame(nickname string, expected int) error {
	got, err := e.cardDueDayByNickname(e.userID, nickname)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("vencimento esperado dia %d para o cartao %q, persistido dia %d", expected, nickname, got)
	}
	return nil
}

func (e *agentE2ECtx) thenCardNotInListing(nickname string) error {
	exists, err := e.cardExistsByNickname(e.userID, nickname)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("cartao %q ainda aparece na listagem apos apagar", nickname)
	}
	return nil
}

func (e *agentE2ECtx) thenCategoryPercentageBecame(rootSlug string, expected int) error {
	got, err := e.allocationBasisPoints(e.userID, e.lastRefMonth, rootSlug)
	if err != nil {
		return err
	}
	if got != expected*100 {
		return fmt.Errorf("percentual esperado %d%% (%d bps) para %q, persistido %d bps", expected, expected*100, rootSlug, got)
	}
	return nil
}

func (e *agentE2ECtx) thenAllocationSumStays100() error {
	sum, err := e.allocationBasisPointsSum(e.userID, e.lastRefMonth)
	if err != nil {
		return err
	}
	if sum != 10000 {
		return fmt.Errorf("soma das alocacoes esperada 10000 bps (100%%), persistida %d bps", sum)
	}
	return nil
}

func (e *agentE2ECtx) givenActiveBudgetSeeded() error {
	competence := time.Now().UTC().Format("2006-01")
	e.lastRefMonth = competence
	return e.seedActiveBudget(e.userID, competence)
}
