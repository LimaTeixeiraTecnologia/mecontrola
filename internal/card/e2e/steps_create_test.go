//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerCreateSteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	sc.Step(`^o usuário cria um cartão com nome "([^"]*)", apelido único, fechamento (\d+), vencimento (\d+) e limite de (\d+) centavos$`, e.createCardWithParams)
	sc.Step(`^o cartão deve estar persistido no banco com nome "([^"]*)" e limite de (\d+) centavos$`, e.assertCardPersistedWithNameAndLimit)
	sc.Step(`^que já existe um cartão com o apelido "([^"]*)"$`, e.cardWithNicknameAlreadyExists)
	sc.Step(`^o usuário tenta criar um cartão com o mesmo apelido "([^"]*)"$`, e.tryCreateCardWithSameNickname)
	sc.Step(`^o usuário tenta criar um cartão com nome "", apelido único, fechamento (\d+) e vencimento (\d+)$`, e.tryCreateCardWithEmptyName)
	sc.Step(`^o usuário tenta criar um cartão com nome de 65 caracteres$`, e.tryCreateCardWithNameTooLong)
	sc.Step(`^o usuário tenta criar um cartão com apelido ""$`, e.tryCreateCardWithEmptyNickname)
	sc.Step(`^o usuário tenta criar um cartão com apelido de 33 caracteres$`, e.tryCreateCardWithNicknameTooLong)
	sc.Step(`^o usuário tenta criar um cartão com fechamento (\d+) e vencimento (\d+)$`, e.tryCreateCardWithDayRange)
	sc.Step(`^o usuário tenta criar um cartão com limite de (-?\d+) centavos$`, e.tryCreateCardWithLimitStr)
	sc.Step(`^o usuário inicia a criação do cartão "([^"]*)" com limite (\d+), fechamento (\d+), vencimento (\d+) e captura a chave de idempotência$`, e.startCreateCardCapturingIdempotencyKey)
	sc.Step(`^o usuário reenvia a mesma requisição com a chave capturada$`, e.resendRequestWithCapturedKey)
	sc.Step(`^deve existir exatamente 1 cartão com nome "([^"]*)" no banco para o usuário$`, e.assertExactlyOneCardWithName)
}

func (e *cardE2ECtx) createCardWithParams(nome string, fechamento, vencimento int, limite int64) error {
	return e.createCardViaHTTP(e.uniqueCardName(nome), fechamento, vencimento, limite)
}

func (e *cardE2ECtx) assertCardPersistedWithNameAndLimit(_ string, limiteCents int64) error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var dbName string
	var dbLimit int64
	err := e.db.QueryRowContext(
		ctx,
		`SELECT name, limit_cents FROM mecontrola.cards WHERE id = $1 AND deleted_at IS NULL`,
		e.cardID,
	).Scan(&dbName, &dbLimit)
	if err != nil {
		return fmt.Errorf("buscar cartão no banco: %w", err)
	}

	if dbName != e.cardName {
		return fmt.Errorf("nome esperado %q, encontrado %q", e.cardName, dbName)
	}

	if dbLimit != limiteCents {
		return fmt.Errorf("limite esperado %d, encontrado %d", limiteCents, dbLimit)
	}

	return nil
}

func (e *cardE2ECtx) cardWithNicknameAlreadyExists(apelido string) error {
	payload := map[string]any{
		"name":        e.uniqueCardName("Seed"),
		"nickname":    apelido,
		"closing_day": 5,
		"due_day":     12,
		"limit_cents": int64(100000),
	}

	if err := e.makeRequest("POST", "/api/v1/cards/", payload); err != nil {
		return err
	}

	if e.lastResp == nil || e.lastResp.StatusCode != 201 {
		status := 0
		if e.lastResp != nil {
			status = e.lastResp.StatusCode
		}
		return fmt.Errorf("seed cartão com apelido %q falhou, status %d", apelido, status)
	}

	return nil
}

func (e *cardE2ECtx) tryCreateCardWithSameNickname(apelido string) error {
	payload := map[string]any{
		"name":        e.uniqueCardName("Dup"),
		"nickname":    apelido,
		"closing_day": 5,
		"due_day":     12,
		"limit_cents": int64(100000),
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) tryCreateCardWithEmptyName(fechamento, vencimento int) error {
	payload := map[string]any{
		"name":        "",
		"nickname":    e.uniqueNickname("nn"),
		"closing_day": fechamento,
		"due_day":     vencimento,
		"limit_cents": int64(100000),
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) tryCreateCardWithNameTooLong() error {
	payload := map[string]any{
		"name":        strings.Repeat("a", 65),
		"nickname":    e.uniqueNickname("nn"),
		"closing_day": 5,
		"due_day":     12,
		"limit_cents": int64(100000),
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) tryCreateCardWithEmptyNickname() error {
	payload := map[string]any{
		"name":        e.uniqueCardName("Card"),
		"nickname":    "",
		"closing_day": 5,
		"due_day":     12,
		"limit_cents": int64(100000),
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) tryCreateCardWithNicknameTooLong() error {
	payload := map[string]any{
		"name":        e.uniqueCardName("Card"),
		"nickname":    strings.Repeat("a", 33),
		"closing_day": 5,
		"due_day":     12,
		"limit_cents": int64(100000),
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) tryCreateCardWithDayRange(fechamento, vencimento int) error {
	payload := map[string]any{
		"name":        e.uniqueCardName("Card"),
		"nickname":    e.uniqueNickname("nn"),
		"closing_day": fechamento,
		"due_day":     vencimento,
		"limit_cents": int64(100000),
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) tryCreateCardWithLimitStr(limitStr string) error {
	var limite int64
	if _, err := fmt.Sscanf(limitStr, "%d", &limite); err != nil {
		return fmt.Errorf("parsear limite %q: %w", limitStr, err)
	}

	payload := map[string]any{
		"name":        e.uniqueCardName("Card"),
		"nickname":    e.uniqueNickname("nn"),
		"closing_day": 5,
		"due_day":     12,
		"limit_cents": limite,
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) startCreateCardCapturingIdempotencyKey(nome string, limite int64, fechamento, vencimento int) error {
	key := uuid.NewString()
	e.capturedIdemKey = key

	cardName := e.uniqueCardName(nome)
	payload := map[string]any{
		"name":        cardName,
		"nickname":    e.uniqueNickname(nome),
		"closing_day": fechamento,
		"due_day":     vencimento,
		"limit_cents": limite,
	}
	e.capturedIdemPayload = payload

	if err := e.makeRequestWithKey("POST", "/api/v1/cards/", payload, key); err != nil {
		return err
	}

	if e.lastResp != nil && e.lastResp.StatusCode == 201 {
		if id, ok := e.lastBody["id"].(string); ok {
			e.cardID = id
		}
		e.cardName = cardName
	}

	return nil
}

func (e *cardE2ECtx) resendRequestWithCapturedKey() error {
	if e.capturedIdemKey == "" {
		return fmt.Errorf("nenhuma chave de idempotencia capturada")
	}

	return e.makeRequestWithKey("POST", "/api/v1/cards/", e.capturedIdemPayload, e.capturedIdemKey)
}

func (e *cardE2ECtx) assertExactlyOneCardWithName(_ string) error {
	if e.cardName == "" {
		return fmt.Errorf("cardName nao definido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM mecontrola.cards WHERE name = $1 AND user_id = $2 AND deleted_at IS NULL`,
		e.cardName,
		e.userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("contar cartoes no banco: %w", err)
	}

	if count != 1 {
		return fmt.Errorf("esperado exatamente 1 cartão com nome %q, encontrado %d", e.cardName, count)
	}

	return nil
}
