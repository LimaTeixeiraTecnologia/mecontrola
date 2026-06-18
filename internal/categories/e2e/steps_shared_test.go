//go:build e2e

package e2e_test

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cucumber/godog"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

type stepFunc func() error

func registerSharedSteps(sc *godog.ScenarioContext, e *categoriesE2ECtx) {
	sc.Step(`^a resposta HTTP deve ter status (\d+)$`, e.thenResponseStatusShouldBe)
	sc.Step(`^a resposta deve conter o header ETag$`, e.thenResponseShouldContainETagHeader)
	sc.Step(`^uma nova consulta com o mesmo ETag deve retornar status (\d+)$`, e.thenRepeatedRequestWithSameETagShouldReturnStatus)
	sc.Step(`^o corpo da resposta deve conter o campo "([^"]*)"$`, e.thenResponseBodyShouldContainField)
	sc.Step(`^o corpo da resposta deve conter "([^"]*)" no campo "([^"]*)"$`, e.thenResponseBodyShouldContainValueInField)
	sc.Step(`^o campo de erro deve ser "([^"]*)"$`, e.thenErrorCodeShouldBe)
	sc.Step(`^a resposta deve conter version maior que zero$`, e.thenResponseShouldContainVersionGreaterThanZero)
	sc.Step(`^a resposta deve retornar uma lista nao vazia no campo "([^"]*)"$`, e.thenResponseShouldReturnNonEmptyListInField)
	sc.Step(`^o campo "([^"]*)" deve ser uma lista ordenada alfabeticamente por "([^"]*)"$`, e.thenFieldShouldBeAlphabeticallyOrderedBy)
	sc.Step(`^o corpo da resposta deve ser (.*)$`, e.thenResponseBodyShouldBe)
}

func (e *categoriesE2ECtx) thenResponseStatusShouldBe(status int) error {
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}

	if e.lastResp.StatusCode != status {
		return fmt.Errorf("status esperado %d, recebido %d, corpo: %s", status, e.lastResp.StatusCode, e.mustMarshalBody())
	}

	return nil
}

func (e *categoriesE2ECtx) thenResponseShouldContainETagHeader() error {
	if e.lastETag == "" {
		return fmt.Errorf("header ETag ausente")
	}

	return nil
}

func (e *categoriesE2ECtx) thenRepeatedRequestWithSameETagShouldReturnStatus(status int) error {
	if e.lastETag == "" {
		return fmt.Errorf("nenhum ETag anterior capturado")
	}

	if e.lastResp == nil || e.lastResp.Request == nil || e.lastResp.Request.URL == nil {
		return fmt.Errorf("request anterior ausente")
	}

	path := e.lastResp.Request.URL.Path
	if e.lastResp.Request.URL.RawQuery != "" {
		path += "?" + e.lastResp.Request.URL.RawQuery
	}

	if err := e.getWithETag(path, e.lastETag); err != nil {
		return err
	}

	return e.thenResponseStatusShouldBe(status)
}

func (e *categoriesE2ECtx) thenResponseBodyShouldContainField(field string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	value, ok := e.lastBody[field]
	if !ok {
		return fmt.Errorf("campo %q ausente", field)
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return fmt.Errorf("campo %q vazio", field)
		}
	case nil:
		return fmt.Errorf("campo %q nulo", field)
	}

	return nil
}

func (e *categoriesE2ECtx) thenResponseBodyShouldContainValueInField(expected, field string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	value, ok := e.lastBody[field].(string)
	if !ok {
		return fmt.Errorf("campo %q nao e string", field)
	}

	if value != expected {
		return fmt.Errorf("campo %q esperado %q, recebido %q", field, expected, value)
	}

	return nil
}

func (e *categoriesE2ECtx) thenErrorCodeShouldBe(code string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	errorsBody, ok := e.lastBody["errors"].(map[string]any)
	if !ok {
		return fmt.Errorf("campo errors ausente")
	}

	got, ok := errorsBody["code"].(string)
	if !ok {
		return fmt.Errorf("codigo de erro ausente")
	}

	if got != code {
		return fmt.Errorf("codigo esperado %q, recebido %q", code, got)
	}

	return nil
}

func (e *categoriesE2ECtx) thenResponseShouldContainVersionGreaterThanZero() error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	raw, ok := e.lastBody["version"].(float64)
	if !ok {
		return fmt.Errorf("campo version ausente")
	}

	if raw <= 0 {
		return fmt.Errorf("version esperada maior que zero, recebida %v", raw)
	}

	return nil
}

func (e *categoriesE2ECtx) thenResponseShouldReturnNonEmptyListInField(field string) error {
	items, err := e.bodyArray(field)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		return fmt.Errorf("campo %q retornou lista vazia", field)
	}

	return nil
}

func (e *categoriesE2ECtx) thenFieldShouldBeAlphabeticallyOrderedBy(field, itemField string) error {
	items, err := e.bodyArray(field)
	if err != nil {
		return err
	}

	values := make([]string, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("item do campo %q nao e objeto", field)
		}

		value, ok := record[itemField].(string)
		if !ok {
			return fmt.Errorf("campo %q ausente no item", itemField)
		}

		values = append(values, value)
	}

	col := collate.New(language.MustParse("pt-BR"))
	sortedValues := append([]string(nil), values...)
	sort.Slice(sortedValues, func(i, j int) bool {
		return col.CompareString(sortedValues[i], sortedValues[j]) < 0
	})

	for i := range values {
		if col.CompareString(values[i], sortedValues[i]) != 0 {
			return fmt.Errorf("lista %q nao esta ordenada por %q", field, itemField)
		}
	}

	return nil
}

func (e *categoriesE2ECtx) thenResponseBodyShouldBe(body string) error {
	if len(body) >= 2 && body[0] == '"' && body[len(body)-1] == '"' {
		body = body[1 : len(body)-1]
		body = strings.ReplaceAll(body, `\"`, `"`)
	}

	if e.lastBodyText != body {
		return fmt.Errorf("corpo esperado %q, recebido %q", body, e.lastBodyText)
	}

	return nil
}
