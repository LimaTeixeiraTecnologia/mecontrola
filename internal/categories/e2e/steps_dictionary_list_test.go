//go:build e2e

package e2e_test

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerDictionaryListSteps(sc *godog.ScenarioContext, e *categoriesE2ECtx) {
	sc.Step(`^o cliente lista o dicionario de categorias$`, e.whenClientListsCategoryDictionary)
	sc.Step(`^o cliente nao autenticado lista o dicionario de categorias$`, e.whenUnauthenticatedClientListsCategoryDictionary)
	sc.Step(`^o cliente lista o dicionario de categorias com kind "([^"]*)"$`, e.whenClientListsCategoryDictionaryByKind)
	sc.Step(`^o cliente lista o dicionario da categoria "([^"]*)" em "([^"]*)"$`, e.whenClientListsDictionaryForCategory)
	sc.Step(`^o cliente lista o dicionario com signal_type "([^"]*)"$`, e.whenClientListsDictionaryBySignalType)
	sc.Step(`^o cliente lista o dicionario com page_size (\d+)$`, e.whenClientListsDictionaryWithPageSize)
	sc.Step(`^o cliente lista a proxima pagina do dicionario$`, e.whenClientListsNextDictionaryPage)
	sc.Step(`^o cliente lista o dicionario com kind invalido$`, e.whenClientListsDictionaryWithInvalidKind)
	sc.Step(`^o cliente lista o dicionario com signal_type invalido$`, e.whenClientListsDictionaryWithInvalidSignalType)
	sc.Step(`^todas as entradas do dicionario devem ter kind "([^"]*)"$`, e.thenAllDictionaryEntriesHaveKind)
	sc.Step(`^todas as entradas do dicionario devem ter signal_type "([^"]*)"$`, e.thenAllDictionaryEntriesHaveSignalType)
	sc.Step(`^todas as entradas do dicionario devem ter category_id da categoria "([^"]*)" em "([^"]*)"$`, e.thenAllDictionaryEntriesHaveCategoryIDForCategory)
	sc.Step(`^a resposta deve conter next_cursor$`, e.thenResponseContainsNextCursor)
}

func (e *categoriesE2ECtx) whenClientListsCategoryDictionary() error {
	return e.get("/api/v1/category-dictionary")
}

func (e *categoriesE2ECtx) whenUnauthenticatedClientListsCategoryDictionary() error {
	return e.getWithoutAuth("/api/v1/category-dictionary")
}

func (e *categoriesE2ECtx) whenClientListsCategoryDictionaryByKind(kind string) error {
	return e.get("/api/v1/category-dictionary?kind=" + kind)
}

func (e *categoriesE2ECtx) whenClientListsDictionaryForCategory(slug, kind string) error {
	categoryID, err := e.categoryIDBySlug(kind, slug)
	if err != nil {
		return err
	}

	return e.get("/api/v1/category-dictionary?kind=" + kind + "&category_id=" + categoryID)
}

func (e *categoriesE2ECtx) whenClientListsDictionaryBySignalType(signalType string) error {
	return e.get("/api/v1/category-dictionary?kind=expense&signal_type=" + signalType)
}

func (e *categoriesE2ECtx) whenClientListsDictionaryWithPageSize(pageSize int) error {
	return e.get(fmt.Sprintf("/api/v1/category-dictionary?kind=expense&page_size=%d", pageSize))
}

func (e *categoriesE2ECtx) whenClientListsNextDictionaryPage() error {
	if e.lastCursor == "" {
		return fmt.Errorf("next_cursor ausente na resposta anterior")
	}

	return e.get("/api/v1/category-dictionary?kind=expense&cursor=" + e.lastCursor)
}

func (e *categoriesE2ECtx) whenClientListsDictionaryWithInvalidKind() error {
	return e.get("/api/v1/category-dictionary?kind=invalido")
}

func (e *categoriesE2ECtx) whenClientListsDictionaryWithInvalidSignalType() error {
	return e.get("/api/v1/category-dictionary?kind=expense&signal_type=invalido")
}

func (e *categoriesE2ECtx) thenAllDictionaryEntriesHaveKind(kind string) error {
	items, err := e.bodyArray("entries")
	if err != nil {
		return err
	}

	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("entrada do dicionario nao e objeto")
		}

		got, ok := record["kind"].(string)
		if !ok {
			return fmt.Errorf("kind ausente na entrada")
		}

		if got != kind {
			return fmt.Errorf("kind esperado %q, recebido %q", kind, got)
		}
	}

	return nil
}

func (e *categoriesE2ECtx) thenAllDictionaryEntriesHaveSignalType(signalType string) error {
	items, err := e.bodyArray("entries")
	if err != nil {
		return err
	}

	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("entrada do dicionario nao e objeto")
		}

		got, ok := record["signal_type"].(string)
		if !ok {
			return fmt.Errorf("signal_type ausente na entrada")
		}

		if got != signalType {
			return fmt.Errorf("signal_type esperado %q, recebido %q", signalType, got)
		}
	}

	return nil
}

func (e *categoriesE2ECtx) thenResponseContainsNextCursor() error {
	if e.lastCursor == "" {
		return fmt.Errorf("next_cursor ausente")
	}

	return nil
}

func (e *categoriesE2ECtx) thenAllDictionaryEntriesHaveCategoryIDForCategory(slug, kind string) error {
	expectedCategoryID, err := e.categoryIDBySlug(kind, slug)
	if err != nil {
		return err
	}

	items, err := e.bodyArray("entries")
	if err != nil {
		return err
	}

	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("entrada do dicionario nao e objeto")
		}

		got, ok := record["category_id"].(string)
		if !ok {
			return fmt.Errorf("category_id ausente na entrada")
		}

		if got != expectedCategoryID {
			return fmt.Errorf("category_id esperado %q, recebido %q", expectedCategoryID, got)
		}
	}

	return nil
}
