//go:build e2e

package e2e_test

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerCategoriesListSteps(sc *godog.ScenarioContext, e *categoriesE2ECtx) {
	sc.Step(`^que existe uma categoria deprecated em "([^"]*)" com slug "([^"]*)"$`, e.givenDeprecatedCategoryExistsUnderSlug)
	sc.Step(`^o cliente lista categorias de "([^"]*)"$`, e.whenClientListsCategoriesByKind)
	sc.Step(`^o cliente nao autenticado lista categorias de "([^"]*)"$`, e.whenUnauthenticatedClientListsCategoriesByKind)
	sc.Step(`^o cliente lista subcategorias de "([^"]*)" em "([^"]*)"$`, e.whenClientListsSubcategoriesByParent)
	sc.Step(`^o cliente lista subcategorias incluindo deprecated de "([^"]*)" em "([^"]*)"$`, e.whenClientListsSubcategoriesIncludingDeprecatedByParent)
	sc.Step(`^o cliente lista categorias com kind invalido$`, e.whenClientListsCategoriesWithInvalidKind)
	sc.Step(`^o cliente lista categorias com parent_id invalido$`, e.whenClientListsCategoriesWithInvalidParentID)
	sc.Step(`^a resposta deve conter as categorias raiz "([^"]*)"$`, e.thenResponseContainsRootCategories)
	sc.Step(`^a resposta deve conter a subcategoria "([^"]*)"$`, e.thenResponseContainsSubcategory)
	sc.Step(`^todas as categorias retornadas devem ter kind "([^"]*)"$`, e.thenAllReturnedCategoriesHaveKind)
}

func (e *categoriesE2ECtx) givenDeprecatedCategoryExistsUnderSlug(parentSlug, slug string) error {
	_, err := e.ensureDeprecatedCategory("expense", parentSlug, slug, "Categoria Deprecated E2E", "consumption")
	return err
}

func (e *categoriesE2ECtx) whenClientListsCategoriesByKind(kind string) error {
	return e.get("/api/v1/categories?kind=" + kind)
}

func (e *categoriesE2ECtx) whenUnauthenticatedClientListsCategoriesByKind(kind string) error {
	return e.getWithoutAuth("/api/v1/categories?kind=" + kind)
}

func (e *categoriesE2ECtx) whenClientListsSubcategoriesByParent(kind, parentSlug string) error {
	parentID, err := e.categoryIDBySlug(kind, parentSlug)
	if err != nil {
		return err
	}

	e.currentParentID = parentID
	return e.get("/api/v1/categories?kind=" + kind + "&parent_id=" + parentID)
}

func (e *categoriesE2ECtx) whenClientListsSubcategoriesIncludingDeprecatedByParent(kind, parentSlug string) error {
	parentID, err := e.categoryIDBySlug(kind, parentSlug)
	if err != nil {
		return err
	}

	e.currentParentID = parentID
	return e.get("/api/v1/categories?kind=" + kind + "&parent_id=" + parentID + "&include_deprecated=true")
}

func (e *categoriesE2ECtx) whenClientListsCategoriesWithInvalidKind() error {
	return e.get("/api/v1/categories?kind=invalido")
}

func (e *categoriesE2ECtx) whenClientListsCategoriesWithInvalidParentID() error {
	return e.get("/api/v1/categories?kind=expense&parent_id=invalido")
}

func (e *categoriesE2ECtx) thenResponseContainsRootCategories(expected string) error {
	items, err := e.bodyArray("categories")
	if err != nil {
		return err
	}

	for _, name := range splitCSV(expected) {
		if _, ok := e.arrayItemByStringField("name", name, items); !ok {
			return fmt.Errorf("categoria raiz %q nao encontrada", name)
		}
	}

	return nil
}

func (e *categoriesE2ECtx) thenResponseContainsSubcategory(name string) error {
	items, err := e.bodyArray("categories")
	if err != nil {
		return err
	}

	if _, ok := e.arrayItemByStringField("name", name, items); ok {
		return nil
	}

	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}

		rawSubs, ok := record["subcategories"].([]any)
		if !ok {
			continue
		}

		if _, ok := e.arrayItemByStringField("name", name, rawSubs); ok {
			return nil
		}
	}

	return fmt.Errorf("subcategoria %q nao encontrada", name)
}

func (e *categoriesE2ECtx) thenAllReturnedCategoriesHaveKind(kind string) error {
	items, err := e.bodyArray("categories")
	if err != nil {
		return err
	}

	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("item de categories nao e objeto")
		}

		got, ok := record["kind"].(string)
		if !ok {
			return fmt.Errorf("campo kind ausente")
		}

		if got != kind {
			return fmt.Errorf("kind esperado %q, recebido %q", kind, got)
		}
	}

	return nil
}
