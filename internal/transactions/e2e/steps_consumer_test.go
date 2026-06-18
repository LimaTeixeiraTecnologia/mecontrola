//go:build e2e

package transactions_e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

func registerConsumerSteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	sc.Step(`^o consumer processa os eventos pendentes do outbox$`, e.oConsumerProcessaOsEventosPendentesDoOutbox)
	sc.Step(`^o consumer processa os eventos pendentes do outbox novamente$`, e.oConsumerProcessaOsEventosPendentesDoOutbox)
	sc.Step(`^a tabela monthly_summary deve conter registro para o usuário em "([^"]*)"$`, e.aTabelaMonthlySummaryDeveConterRegistroParaOUsuarioEm)
	sc.Step(`^a tabela monthly_summary deve conter exatamente (\d+) registro para o usuário em "([^"]*)"$`, e.aTabelaMonthlySummaryDeveConterExatamenteRegistroParaOUsuarioEm)
}

func (e *txE2ECtx) oConsumerProcessaOsEventosPendentesDoOutbox() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return drainOutboxToConsumer(ctx, e.db, e.recomputeConsumer, 50*time.Millisecond)
}

func (e *txE2ECtx) aTabelaMonthlySummaryDeveConterRegistroParaOUsuarioEm(refMonth string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := countMonthlySummaryRows(ctx, e.db, e.userID, refMonth)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("esperado pelo menos 1 registro em monthly_summary para usuário em %s, encontrado 0", refMonth)
	}
	return nil
}

func (e *txE2ECtx) aTabelaMonthlySummaryDeveConterExatamenteRegistroParaOUsuarioEm(expected int, refMonth string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := countMonthlySummaryRows(ctx, e.db, e.userID, refMonth)
	if err != nil {
		return err
	}
	if n != expected {
		return fmt.Errorf("esperado %d registro(s) em monthly_summary para usuário em %s, encontrado %d", expected, refMonth, n)
	}
	return nil
}
