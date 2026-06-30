//go:build e2e

package e2e_test

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerCategoryGetSteps(sc *godog.ScenarioContext, e *categoriesE2ECtx) {
	sc.Step(`^o cliente consulta a categoria "([^"]*)" em "([^"]*)"$`, e.whenClientGetsCategoryBySlug)
	sc.Step(`^o cliente nao autenticado consulta a categoria "([^"]*)" em "([^"]*)"$`, e.whenUnauthenticatedClientGetsCategoryBySlug)
	sc.Step(`^o cliente consulta a categoria deprecated "([^"]*)" em "([^"]*)"$`, e.whenClientGetsDeprecatedCategoryBySlug)
	sc.Step(`^o cliente consulta a categoria deprecated "([^"]*)" em "([^"]*)" incluindo deprecated$`, e.whenClientGetsDeprecatedCategoryIncludingDeprecated)
	sc.Step(`^o cliente consulta uma categoria inexistente$`, e.whenClientGetsNonexistentCategory)
	sc.Step(`^o cliente consulta uma categoria com id invalido$`, e.whenClientGetsCategoryWithInvalidID)
	sc.Step(`^a resposta deve conter o path "([^"]*)"$`, e.thenResponseContainsPath)
}

func (e *categoriesE2ECtx) whenClientGetsCategoryBySlug(slug, kind string) error {
	categoryID, err := e.categoryIDBySlug(kind, slug)
	if err != nil {
		return err
	}

	e.currentCategoryID = categoryID
	return e.get("/api/v1/categories/" + categoryID)
}

func (e *categoriesE2ECtx) whenUnauthenticatedClientGetsCategoryBySlug(slug, kind string) error {
	categoryID, err := e.categoryIDBySlug(kind, slug)
	if err != nil {
		return err
	}

	e.currentCategoryID = categoryID
	return e.getWithoutAuth("/api/v1/categories/" + categoryID)
}

func (e *categoriesE2ECtx) whenClientGetsDeprecatedCategoryBySlug(slug, parentSlug string) error {
	categoryID, err := e.ensureDeprecatedCategory("expense", parentSlug, slug, "Categoria Deprecated E2E", "consumption")
	if err != nil {
		return err
	}

	e.currentCategoryID = categoryID
	return e.get("/api/v1/categories/" + categoryID)
}

func (e *categoriesE2ECtx) whenClientGetsDeprecatedCategoryIncludingDeprecated(slug, parentSlug string) error {
	categoryID, err := e.ensureDeprecatedCategory("expense", parentSlug, slug, "Categoria Deprecated E2E", "consumption")
	if err != nil {
		return err
	}

	e.currentCategoryID = categoryID
	return e.get("/api/v1/categories/" + categoryID + "?include_deprecated=true")
}

func (e *categoriesE2ECtx) whenClientGetsNonexistentCategory() error {
	return e.get("/api/v1/categories/99999999-9999-9999-9999-999999999999")
}

func (e *categoriesE2ECtx) whenClientGetsCategoryWithInvalidID() error {
	return e.get("/api/v1/categories/invalido")
}

func (e *categoriesE2ECtx) thenResponseContainsPath(path string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	got, ok := e.lastBody["path"].(string)
	if !ok {
		return fmt.Errorf("campo path ausente")
	}

	if got != path {
		return fmt.Errorf("path esperado %q, recebido %q", path, got)
	}

	return nil
}
