//go:build e2e

package e2e_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerDeleteSteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	sc.Step(`^o usuário exclui o cartão cadastrado$`, e.deleteRegisteredCard)
	sc.Step(`^o cartão deve estar marcado como excluído no banco$`, e.assertCardSoftDeletedInDB)
	sc.Step(`^o usuário tenta excluir um cartão com ID aleatório inexistente$`, e.tryDeleteNonExistentCard)
	sc.Step(`^o cartão excluído não deve constar na lista retornada$`, e.assertDeletedCardNotInList)
	sc.Step(`^o usuário tenta excluir o mesmo cartão novamente$`, e.tryDeleteSameCardAgain)
}

func (e *cardE2ECtx) deleteRegisteredCard() error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}
	return e.makeRequest(http.MethodDelete, "/api/v1/cards/"+e.cardID+"/", nil)
}

func (e *cardE2ECtx) assertCardSoftDeletedInDB() error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var deletedAt sql.NullTime
	err := e.db.QueryRowContext(
		ctx,
		`SELECT deleted_at FROM mecontrola.cards WHERE id = $1`,
		e.cardID,
	).Scan(&deletedAt)
	if err != nil {
		return fmt.Errorf("consultar deleted_at no banco: %w", err)
	}

	if !deletedAt.Valid {
		return fmt.Errorf("cartao nao foi marcado como excluido (deleted_at nulo)")
	}

	return nil
}

func (e *cardE2ECtx) tryDeleteNonExistentCard() error {
	return e.makeRequest(http.MethodDelete, "/api/v1/cards/"+uuid.NewString()+"/", nil)
}

func (e *cardE2ECtx) assertDeletedCardNotInList() error {
	for _, item := range e.listItems {
		if id, ok := item["id"].(string); ok && id == e.cardID {
			return fmt.Errorf("cartao excluido %q ainda consta na lista", e.cardID)
		}
	}
	return nil
}

func (e *cardE2ECtx) tryDeleteSameCardAgain() error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}
	return e.makeRequest(http.MethodDelete, "/api/v1/cards/"+e.cardID+"/", nil)
}
