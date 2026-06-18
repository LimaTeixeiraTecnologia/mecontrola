//go:build e2e

package e2e_test

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerWriteSteps(sc *godog.ScenarioContext, e *agentE2ECtx) {
	sc.Step(`^deve existir 1 compra parcelada de (\d+) em (\d+)x$`, e.thenCardPurchaseExists)
	sc.Step(`^deve existir 1 recorrência de (\d+) no dia (\d+)$`, e.thenRecurringExists)
	sc.Step(`^o valor da última transação do usuário deve ser (\d+)$`, e.thenLastTransactionAmountShouldBe)
	sc.Step(`^a versão da última transação do usuário deve ser pelo menos (\d+)$`, e.thenLastTransactionVersionAtLeast)
	sc.Step(`^a última transação do usuário deve estar excluída$`, e.thenLastTransactionShouldBeDeleted)
	sc.Step(`^o número de transações ativas do usuário diminuiu em (\d+)$`, e.thenActiveTransactionCountDecreasedBy)
	sc.Step(`^a despesa do orçamento do usuário foi removida$`, e.thenBudgetsExpenseWasRemoved)
}

func (e *agentE2ECtx) thenCardPurchaseExists(amountCents, installments int) error {
	count, err := e.countCardPurchases(e.userID)
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("esperado 1 compra parcelada, encontrado %d", count)
	}
	gotAmount, gotInstallments, err := e.latestCardPurchase(e.userID)
	if err != nil {
		return err
	}
	if gotAmount != int64(amountCents) {
		return fmt.Errorf("total_amount_cents esperado %d, recebido %d", amountCents, gotAmount)
	}
	if gotInstallments != installments {
		return fmt.Errorf("installments_total esperado %d, recebido %d", installments, gotInstallments)
	}
	return nil
}

func (e *agentE2ECtx) thenRecurringExists(amountCents, dayOfMonth int) error {
	count, err := e.countRecurringTemplates(e.userID)
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("esperado 1 recorrencia, encontrado %d", count)
	}
	gotAmount, gotDay, err := e.latestRecurringTemplate(e.userID)
	if err != nil {
		return err
	}
	if gotAmount != int64(amountCents) {
		return fmt.Errorf("amount_cents esperado %d, recebido %d", amountCents, gotAmount)
	}
	if gotDay != dayOfMonth {
		return fmt.Errorf("day_of_month esperado %d, recebido %d", dayOfMonth, gotDay)
	}
	return nil
}

func (e *agentE2ECtx) thenLastTransactionAmountShouldBe(amountCents int) error {
	got, _, err := e.latestTransactionAmountAndVersion(e.userID)
	if err != nil {
		return err
	}
	if got != int64(amountCents) {
		return fmt.Errorf("amount_cents da ultima transacao esperado %d, recebido %d", amountCents, got)
	}
	return nil
}

func (e *agentE2ECtx) thenLastTransactionVersionAtLeast(minVersion int) error {
	_, version, err := e.latestTransactionAmountAndVersion(e.userID)
	if err != nil {
		return err
	}
	if version < int64(minVersion) {
		return fmt.Errorf("version da ultima transacao esperada >= %d, recebido %d", minVersion, version)
	}
	return nil
}

func (e *agentE2ECtx) thenLastTransactionShouldBeDeleted() error {
	deleted, err := e.latestTransactionDeletedAt(e.userID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("ultima transacao nao esta soft-deletada (deleted_at IS NULL)")
	}
	return nil
}

func (e *agentE2ECtx) thenActiveTransactionCountDecreasedBy(delta int) error {
	after, err := e.countTransactions(e.userID)
	if err != nil {
		return err
	}
	if after != e.beforeUser-delta {
		return fmt.Errorf("esperado diminuicao de %d transacoes ativas (de %d para %d), recebido %d", delta, e.beforeUser, e.beforeUser-delta, after)
	}
	return nil
}

func (e *agentE2ECtx) thenBudgetsExpenseWasRemoved() error {
	if e.budgetsDeletedHits < 1 {
		return fmt.Errorf("budgets consumer nao removeu despesa (hits=%d): expense nao registrado ou evento ausente", e.budgetsDeletedHits)
	}
	return nil
}
