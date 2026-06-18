//go:build e2e

package e2e_test

import (
	"fmt"
	"net/url"

	"github.com/cucumber/godog"
)

func registerDictionarySearchSteps(sc *godog.ScenarioContext, e *categoriesE2ECtx) {
	sc.Step(`^que existe um termo ambiguo "([^"]*)" no dicionario$`, e.givenAmbiguousDictionaryTermExists)
	sc.Step(`^o cliente busca no dicionario por "([^"]*)" em "([^"]*)"$`, e.whenClientSearchesDictionaryForQueryAndKind)
	sc.Step(`^o cliente nao autenticado busca no dicionario por "([^"]*)" em "([^"]*)"$`, e.whenUnauthenticatedClientSearchesDictionaryForQueryAndKind)
	sc.Step(`^o cliente busca no dicionario por query vazia$`, e.whenClientSearchesDictionaryWithEmptyQuery)
	sc.Step(`^o cliente busca no dicionario por query curta$`, e.whenClientSearchesDictionaryWithShortQuery)
	sc.Step(`^o cliente busca no dicionario por query com espacos$`, e.whenClientSearchesDictionaryWithBlankQuery)
	sc.Step(`^o cliente busca no dicionario sem informar kind$`, e.whenClientSearchesDictionaryWithoutKind)
	sc.Step(`^o cliente busca no dicionario com kind invalido$`, e.whenClientSearchesDictionaryWithInvalidKind)
	sc.Step(`^a resposta deve conter candidatos ambíguos$`, e.thenResponseContainsAmbiguousCandidates)
	sc.Step(`^o primeiro candidato deve conter signal_type "([^"]*)"$`, e.thenFirstCandidateContainsSignalType)
	sc.Step(`^o primeiro candidato deve conter confidence "([^"]*)"$`, e.thenFirstCandidateContainsConfidence)
	sc.Step(`^o primeiro candidato deve conter match_reason$`, e.thenFirstCandidateContainsMatchReason)
}

func (e *categoriesE2ECtx) givenAmbiguousDictionaryTermExists(term string) error {
	return e.ensureAmbiguousDictionaryEntries(term)
}

func (e *categoriesE2ECtx) whenClientSearchesDictionaryForQueryAndKind(query, kind string) error {
	return e.get("/api/v1/category-dictionary/search?q=" + url.QueryEscape(query) + "&kind=" + url.QueryEscape(kind))
}

func (e *categoriesE2ECtx) whenUnauthenticatedClientSearchesDictionaryForQueryAndKind(query, kind string) error {
	return e.getWithoutAuth("/api/v1/category-dictionary/search?q=" + url.QueryEscape(query) + "&kind=" + url.QueryEscape(kind))
}

func (e *categoriesE2ECtx) whenClientSearchesDictionaryWithEmptyQuery() error {
	return e.get("/api/v1/category-dictionary/search?q=&kind=expense")
}

func (e *categoriesE2ECtx) whenClientSearchesDictionaryWithShortQuery() error {
	return e.get("/api/v1/category-dictionary/search?q=ab&kind=expense")
}

func (e *categoriesE2ECtx) whenClientSearchesDictionaryWithBlankQuery() error {
	return e.get("/api/v1/category-dictionary/search?q=%20%20%20&kind=expense")
}

func (e *categoriesE2ECtx) whenClientSearchesDictionaryWithoutKind() error {
	return e.get("/api/v1/category-dictionary/search?q=" + url.QueryEscape("energia"))
}

func (e *categoriesE2ECtx) whenClientSearchesDictionaryWithInvalidKind() error {
	return e.get("/api/v1/category-dictionary/search?q=energia&kind=invalido")
}

func (e *categoriesE2ECtx) thenResponseContainsAmbiguousCandidates() error {
	items, err := e.bodyArray("candidates")
	if err != nil {
		return err
	}

	if len(items) < 2 {
		return fmt.Errorf("esperados ao menos 2 candidatos ambiguos")
	}

	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("candidato nao e objeto")
		}

		ambiguous, ok := record["is_ambiguous"].(bool)
		if !ok {
			return fmt.Errorf("campo is_ambiguous ausente")
		}

		if !ambiguous {
			return fmt.Errorf("candidato deveria ser ambiguo")
		}
	}

	return nil
}

func (e *categoriesE2ECtx) thenFirstCandidateContainsSignalType(signalType string) error {
	first, err := e.firstCandidate()
	if err != nil {
		return err
	}

	got, ok := first["signal_type"].(string)
	if !ok {
		return fmt.Errorf("signal_type ausente")
	}

	if got != signalType {
		return fmt.Errorf("signal_type esperado %q, recebido %q", signalType, got)
	}

	return nil
}

func (e *categoriesE2ECtx) thenFirstCandidateContainsConfidence(confidence string) error {
	first, err := e.firstCandidate()
	if err != nil {
		return err
	}

	got, ok := first["confidence"].(string)
	if !ok {
		return fmt.Errorf("confidence ausente")
	}

	if got != confidence {
		return fmt.Errorf("confidence esperada %q, recebida %q", confidence, got)
	}

	return nil
}

func (e *categoriesE2ECtx) thenFirstCandidateContainsMatchReason() error {
	first, err := e.firstCandidate()
	if err != nil {
		return err
	}

	for _, field := range []string{"path", "matched_term", "match_reason"} {
		value, ok := first[field].(string)
		if !ok || value == "" {
			return fmt.Errorf("campo %q ausente no primeiro candidato", field)
		}
	}

	return nil
}

func (e *categoriesE2ECtx) firstCandidate() (map[string]any, error) {
	items, err := e.bodyArray("candidates")
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("lista de candidatos vazia")
	}

	first, ok := items[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("primeiro candidato nao e objeto")
	}

	return first, nil
}
