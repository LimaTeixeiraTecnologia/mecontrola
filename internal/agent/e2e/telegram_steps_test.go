//go:build e2e

package e2e_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

func registerTelegramSteps(sc *godog.ScenarioContext, e *agentE2ECtx) {
	sc.Step(`^o usuário envia "([^"]*)" via webhook do telegram$`, e.whenUserSendsViaTelegram)
	sc.Step(`^o gateway do telegram respondeu ao usuário$`, e.thenTelegramGatewayRepliedToUser)
}

func (e *agentE2ECtx) whenUserSendsViaTelegram(text string) error {
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

	updateID := time.Now().UTC().UnixNano()
	return e.postTelegramWebhook(text, updateID)
}

func (e *agentE2ECtx) thenTelegramGatewayRepliedToUser() error {
	reply, ok := e.telegramGateway.LastReply()
	if !ok {
		return fmt.Errorf("gateway do telegram nao respondeu ao usuario")
	}
	if reply.ChatID != e.telegramChatID {
		return fmt.Errorf("resposta telegram destinada a %d, esperado %d", reply.ChatID, e.telegramChatID)
	}
	if strings.TrimSpace(reply.Text) == "" {
		return fmt.Errorf("resposta do gateway do telegram vazia")
	}
	return nil
}
