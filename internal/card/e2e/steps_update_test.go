//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerUpdateSteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	sc.Step(`^o usuário atualiza o cartão informando apenas o nome "([^"]*)"$`, e.updateCardName)
	sc.Step(`^o usuário atualiza o cartão informando fechamento (\d+) e vencimento (\d+)$`, e.updateCardCycleDays)
	sc.Step(`^o usuário tenta atualizar um cartão com ID inexistente informando o nome "([^"]*)"$`, e.tryUpdateNonExistentCard)
	sc.Step(`^o usuário tenta atualizar o cartão com nome de 65 caracteres$`, e.tryUpdateCardWithNameTooLong)
	sc.Step(`^o usuário atualiza o limite do cartão para (\d+) centavos$`, e.updateCardLimit)
	sc.Step(`^o usuário atualiza o limite do cartão para (\d+) centavos com expected_version (\d+)$`, e.updateCardLimitWithVersion)
	sc.Step(`^a versão do cartão no banco deve ser (\d+)$`, e.assertCardVersionInDB)
	sc.Step(`^o usuário tenta atualizar o limite do cartão para (-?\d+) centavos$`, e.tryUpdateCardLimitStr)
	sc.Step(`^o usuário tenta atualizar o limite de um cartão com ID inexistente para (\d+) centavos$`, e.tryUpdateLimitOfNonExistentCard)
}

func (e *cardE2ECtx) updateCardName(nome string) error {
	payload := map[string]any{
		"name":        nome,
		"closing_day": 5,
		"due_day":     12,
	}
	return e.makeRequest(http.MethodPut, "/api/v1/cards/"+e.cardID+"/", payload)
}

func (e *cardE2ECtx) updateCardCycleDays(fechamento, vencimento int) error {
	payload := map[string]any{
		"name":        e.cardName,
		"closing_day": fechamento,
		"due_day":     vencimento,
	}
	return e.makeRequest(http.MethodPut, "/api/v1/cards/"+e.cardID+"/", payload)
}

func (e *cardE2ECtx) tryUpdateNonExistentCard(nome string) error {
	payload := map[string]any{
		"name":        nome,
		"closing_day": 5,
		"due_day":     12,
	}
	return e.makeRequest(http.MethodPut, "/api/v1/cards/"+uuid.NewString()+"/", payload)
}

func (e *cardE2ECtx) tryUpdateCardWithNameTooLong() error {
	payload := map[string]any{
		"name":        strings.Repeat("x", 65),
		"closing_day": 5,
		"due_day":     12,
	}
	return e.makeRequest(http.MethodPut, "/api/v1/cards/"+e.cardID+"/", payload)
}

func (e *cardE2ECtx) updateCardLimit(limiteCents int64) error {
	payload := map[string]any{
		"limit_cents": limiteCents,
	}
	return e.makeRequest(http.MethodPatch, "/api/v1/cards/"+e.cardID+"/limit", payload)
}

func (e *cardE2ECtx) updateCardLimitWithVersion(limiteCents, expectedVersion int64) error {
	payload := map[string]any{
		"limit_cents":      limiteCents,
		"expected_version": expectedVersion,
	}
	return e.makeRequest(http.MethodPatch, "/api/v1/cards/"+e.cardID+"/limit", payload)
}

func (e *cardE2ECtx) assertCardVersionInDB(expectedVersion int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var got int64
	err := e.db.QueryRowContext(ctx, `SELECT version FROM mecontrola.cards WHERE id = $1`, e.cardID).Scan(&got)
	if err != nil {
		return fmt.Errorf("consulta versao do cartao: %w", err)
	}

	if got != expectedVersion {
		return fmt.Errorf("versao esperada %d, recebida %d", expectedVersion, got)
	}

	return nil
}

func (e *cardE2ECtx) tryUpdateCardLimitStr(limitStr string) error {
	var limiteCents int64
	if _, err := fmt.Sscanf(limitStr, "%d", &limiteCents); err != nil {
		return fmt.Errorf("parse limit: %w", err)
	}
	payload := map[string]any{
		"limit_cents": limiteCents,
	}
	return e.makeRequest(http.MethodPatch, "/api/v1/cards/"+e.cardID+"/limit", payload)
}

func (e *cardE2ECtx) tryUpdateLimitOfNonExistentCard(limiteCents int64) error {
	payload := map[string]any{
		"limit_cents": limiteCents,
	}
	return e.makeRequest(http.MethodPatch, "/api/v1/cards/"+uuid.NewString()+"/limit", payload)
}
