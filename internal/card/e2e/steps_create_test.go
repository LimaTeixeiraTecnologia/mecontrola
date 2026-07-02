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
	sc.Step(`^o usuário cria um cartão com banco "([^"]*)" e vencimento (\d+)$`, e.createCardWithBankAndDueDay)
	sc.Step(`^o cartão deve estar persistido no banco com banco "([^"]*)"$`, e.assertCardPersistedWithBank)
	sc.Step(`^que já existe um cartão com o apelido "([^"]*)"$`, e.cardWithNicknameAlreadyExists)
	sc.Step(`^o usuário tenta criar um cartão com o mesmo apelido "([^"]*)"$`, e.tryCreateCardWithSameNickname)
	sc.Step(`^o usuário tenta criar um cartão com apelido ""$`, e.tryCreateCardWithEmptyNickname)
	sc.Step(`^o usuário tenta criar um cartão com apelido de 33 caracteres$`, e.tryCreateCardWithNicknameTooLong)
	sc.Step(`^o usuário tenta criar um cartão com banco "([^"]*)" e vencimento (\d+)$`, e.tryCreateCardWithDueDay)
	sc.Step(`^o usuário inicia a criação do cartão com banco "([^"]*)" e vencimento (\d+) e captura a chave de idempotência$`, e.startCreateCardCapturingIdempotencyKey)
	sc.Step(`^o usuário reenvia a mesma requisição com a chave capturada$`, e.resendRequestWithCapturedKey)
	sc.Step(`^deve existir exatamente 1 cartão com o apelido capturado no banco para o usuário$`, e.assertExactlyOneCardWithCapturedNickname)
}

func (e *cardE2ECtx) createCardWithBankAndDueDay(banco string, vencimento int) error {
	return e.createCardViaHTTP(banco, vencimento)
}

func (e *cardE2ECtx) assertCardPersistedWithBank(banco string) error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var dbBank string
	err := e.db.QueryRowContext(
		ctx,
		`SELECT bank FROM mecontrola.cards WHERE id = $1 AND deleted_at IS NULL`,
		e.cardID,
	).Scan(&dbBank)
	if err != nil {
		return fmt.Errorf("buscar cartão no banco: %w", err)
	}

	if dbBank != banco {
		return fmt.Errorf("banco esperado %q, encontrado %q", banco, dbBank)
	}

	return nil
}

func (e *cardE2ECtx) cardWithNicknameAlreadyExists(apelido string) error {
	payload := map[string]any{
		"nickname": apelido,
		"bank":     "nubank",
		"due_day":  20,
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
		"nickname": apelido,
		"bank":     "nubank",
		"due_day":  20,
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) tryCreateCardWithEmptyNickname() error {
	payload := map[string]any{
		"nickname": "",
		"bank":     "nubank",
		"due_day":  20,
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) tryCreateCardWithNicknameTooLong() error {
	payload := map[string]any{
		"nickname": strings.Repeat("a", 33),
		"bank":     "nubank",
		"due_day":  20,
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) tryCreateCardWithDueDay(banco string, vencimento int) error {
	payload := map[string]any{
		"nickname": e.uniqueNickname("nn"),
		"bank":     banco,
		"due_day":  vencimento,
	}

	return e.makeRequest("POST", "/api/v1/cards/", payload)
}

func (e *cardE2ECtx) startCreateCardCapturingIdempotencyKey(banco string, vencimento int) error {
	key := uuid.NewString()
	e.capturedIdemKey = key

	nick := e.uniqueNickname("card")
	e.capturedNickname = nick
	payload := map[string]any{
		"nickname": nick,
		"bank":     banco,
		"due_day":  vencimento,
	}
	e.capturedIdemPayload = payload

	if err := e.makeRequestWithKey("POST", "/api/v1/cards/", payload, key); err != nil {
		return err
	}

	if e.lastResp != nil && e.lastResp.StatusCode == 201 {
		if id, ok := e.lastBody["id"].(string); ok {
			e.cardID = id
		}
		e.cardNickname = nick
	}

	return nil
}

func (e *cardE2ECtx) resendRequestWithCapturedKey() error {
	if e.capturedIdemKey == "" {
		return fmt.Errorf("nenhuma chave de idempotencia capturada")
	}

	return e.makeRequestWithKey("POST", "/api/v1/cards/", e.capturedIdemPayload, e.capturedIdemKey)
}

func (e *cardE2ECtx) assertExactlyOneCardWithCapturedNickname() error {
	if e.capturedNickname == "" {
		return fmt.Errorf("capturedNickname nao definido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM mecontrola.cards WHERE nickname = $1 AND user_id = $2 AND deleted_at IS NULL`,
		e.capturedNickname,
		e.userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("contar cartoes no banco: %w", err)
	}

	if count != 1 {
		return fmt.Errorf("esperado exatamente 1 cartão com apelido %q, encontrado %d", e.capturedNickname, count)
	}

	return nil
}
