//go:build integration

package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RF34NegativeSuite struct {
	CanonicalScenariosIntegrationSuite
}

func TestRF34NegativeSuite(t *testing.T) {
	suite.Run(t, new(RF34NegativeSuite))
}

func (s *RF34NegativeSuite) TestAmbiguousTerms_NoUnambiguousMatch() {
	terms := []string{
		"compra", "pix", "boleto", "cartao", "parcela", "transferencia",
		"debito", "mercado", "farmacia", "remedio", "amazon", "celular",
		"telefone", "cafe", "pao", "posto", "hotel", "evento", "ingresso",
		"viagem", "curso", "presente", "investimento",
	}

	for _, term := range terms {
		s.Run(term, func() {
			for _, kind := range []string{"income", "expense"} {
				path := "/api/v1/category-dictionary/search?q=" + url.QueryEscape(term) + "&kind=" + kind
				resp := s.makeRequest("GET", path, nil)
				s.Require().Equal(http.StatusOK, resp.StatusCode, "term=%s kind=%s", term, kind)
				body := s.parseBody(resp)

				if body["result"] == "no_match" {
					continue
				}

				candidates, ok := body["candidates"].([]any)
				s.Require().True(ok, "term=%s kind=%s expected candidates or no_match", term, kind)

				for _, c := range candidates {
					cm := c.(map[string]any)
					s.Truef(cm["is_ambiguous"].(bool),
						"RF-34 violation: term=%q kind=%q returned candidate without is_ambiguous=true: %s",
						term, kind, mustJSON(cm))
				}
			}
		})
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
