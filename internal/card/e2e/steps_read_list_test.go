//go:build e2e

package e2e_test

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerReadListSteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	sc.Step(`^o usuário busca o cartão pelo ID cadastrado$`, e.getCardByRegisteredID)
	sc.Step(`^o usuário busca um cartão com ID aleatório inexistente$`, e.getCardWithRandomID)
	sc.Step(`^o usuário lista todos os seus cartões$`, e.listAllCards)
	sc.Step(`^a lista de cartões deve conter ao menos 1 item$`, e.assertListHasAtLeastOneItem)
	sc.Step(`^o campo "items" da lista deve estar presente$`, e.assertItemsFieldPresent)
	sc.Step(`^a lista de cartões deve conter (\d+) item$`, e.assertListHasExactItems)
	sc.Step(`^a resposta deve conter next_cursor$`, e.assertResponseHasNextCursor)
	sc.Step(`^o usuário lista os cartões com limit (\d+)$`, e.listCardsWithLimit)
	sc.Step(`^o usuário lista os cartões usando o cursor retornado$`, e.listCardsUsingReturnedCursor)
	sc.Step(`^o usuário lista os cartões passando cursor inválido "([^"]*)"$`, e.listCardsWithInvalidCursor)
}

func (e *cardE2ECtx) getCardByRegisteredID() error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}
	return e.makeRequest(http.MethodGet, "/api/v1/cards/"+e.cardID+"/", nil)
}

func (e *cardE2ECtx) getCardWithRandomID() error {
	return e.makeRequest(http.MethodGet, "/api/v1/cards/"+uuid.NewString()+"/", nil)
}

func (e *cardE2ECtx) listAllCards() error {
	if err := e.makeRequest(http.MethodGet, "/api/v1/cards/", nil); err != nil {
		return err
	}
	return e.parseListResponse()
}

func (e *cardE2ECtx) assertListHasAtLeastOneItem() error {
	if len(e.listItems) < 1 {
		return fmt.Errorf("lista vazia, esperava ao menos 1 item")
	}
	return nil
}

func (e *cardE2ECtx) assertItemsFieldPresent() error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}
	if _, ok := e.lastBody["items"]; !ok {
		return fmt.Errorf("campo items ausente")
	}
	return nil
}

func (e *cardE2ECtx) assertListHasExactItems(n int) error {
	if len(e.listItems) != n {
		return fmt.Errorf("esperava %d item(s), recebeu %d", n, len(e.listItems))
	}
	return nil
}

func (e *cardE2ECtx) assertResponseHasNextCursor() error {
	if e.lastCursor == "" {
		return fmt.Errorf("next_cursor ausente ou vazio")
	}
	return nil
}

func (e *cardE2ECtx) listCardsWithLimit(limit int) error {
	if err := e.makeRequest(http.MethodGet, fmt.Sprintf("/api/v1/cards/?limit=%d", limit), nil); err != nil {
		return err
	}
	return e.parseListResponse()
}

func (e *cardE2ECtx) listCardsUsingReturnedCursor() error {
	if e.lastCursor == "" {
		return fmt.Errorf("nenhum cursor retornado previamente")
	}
	path := "/api/v1/cards/?cursor=" + url.QueryEscape(e.lastCursor) + "&limit=10"
	if err := e.makeRequest(http.MethodGet, path, nil); err != nil {
		return err
	}
	return e.parseListResponse()
}

func (e *cardE2ECtx) listCardsWithInvalidCursor(cursor string) error {
	path := "/api/v1/cards/?cursor=" + url.QueryEscape(cursor)
	return e.makeRequest(http.MethodGet, path, nil)
}

func (e *cardE2ECtx) parseListResponse() error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	rawItems, ok := e.lastBody["items"]
	if !ok {
		e.listItems = nil
		return nil
	}

	slice, ok := rawItems.([]any)
	if !ok {
		return fmt.Errorf("campo items nao e array")
	}

	items := make([]map[string]any, 0, len(slice))
	for i, raw := range slice {
		item, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("item[%d] nao e objeto", i)
		}
		items = append(items, item)
	}
	e.listItems = items

	if cursor, ok := e.lastBody["next_cursor"].(string); ok {
		e.lastCursor = cursor
	} else {
		e.lastCursor = ""
	}

	return nil
}
