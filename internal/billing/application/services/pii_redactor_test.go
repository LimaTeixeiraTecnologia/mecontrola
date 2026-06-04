package services_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/services"
)

type PIIRedactorSuite struct {
	suite.Suite
	redactor *services.PIIRedactor
}

func TestPIIRedactor(t *testing.T) {
	suite.Run(t, new(PIIRedactorSuite))
}

func (s *PIIRedactorSuite) SetupTest() {
	s.redactor = services.NewPIIRedactor()
}

func (s *PIIRedactorSuite) TestScalarPaths() {
	type testCase struct {
		name      string
		payload   string
		checkPath string
		wantValue string
	}

	cases := []testCase{
		{
			name:      "redacta customer.cpf",
			payload:   `{"customer":{"cpf":"123.456.789-00","email":"a@b.com"}}`,
			checkPath: "customer.cpf",
			wantValue: "[REDACTED]",
		},
		{
			name:      "redacta customer.cnpj",
			payload:   `{"customer":{"cnpj":"12.345.678/0001-90"}}`,
			checkPath: "customer.cnpj",
			wantValue: "[REDACTED]",
		},
		{
			name:      "redacta customer.email",
			payload:   `{"customer":{"email":"secret@domain.com"}}`,
			checkPath: "customer.email",
			wantValue: "[REDACTED]",
		},
		{
			name:      "redacta customer.mobile",
			payload:   `{"customer":{"mobile":"+5511999990000"}}`,
			checkPath: "customer.mobile",
			wantValue: "[REDACTED]",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			result, err := s.redactor.Strip(json.RawMessage(tc.payload))
			s.Require().NoError(err)
			val := s.extractValue(result, tc.checkPath)
			s.Equal(tc.wantValue, val)
		})
	}
}

func (s *PIIRedactorSuite) TestWildcardCustomerAddress() {
	payload := `{"customer":{"address":{"street":"Rua A","city":"São Paulo","zip":"01001-000"}}}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)

	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	address := doc["customer"].(map[string]any)["address"].(map[string]any)
	for field, val := range address {
		s.Equal("[REDACTED]", val, "customer.address.%s deve estar redactado", field)
	}
}

func (s *PIIRedactorSuite) TestWildcardCard() {
	payload := `{"card":{"number":"4111111111111111","cvv":"123","expiry":"12/26"}}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)

	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	cardMap := doc["card"].(map[string]any)
	for field, val := range cardMap {
		s.Equal("[REDACTED]", val, "card.%s deve estar redactado", field)
	}
}

func (s *PIIRedactorSuite) TestStarMapPaymentCard() {
	payload := `{"payment":{"pix":{"card":{"pan":"4111","cvv":"111"}},"boleto":{"card":{"pan":"5200","cvv":"222"}}}}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)

	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	payment := doc["payment"].(map[string]any)
	for paymentKey, paymentVal := range payment {
		paymentMap := paymentVal.(map[string]any)
		cardMap := paymentMap["card"].(map[string]any)
		for field, val := range cardMap {
			s.Equal("[REDACTED]", val, "payment.%s.card.%s deve estar redactado", paymentKey, field)
		}
	}
}

func (s *PIIRedactorSuite) TestPreservesNonPIIFields() {
	payload := `{"product":{"id":"prod-001","name":"Plano Mensal"},"tracking":{"src":"token123"},"customer":{"email":"a@b.com"}}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)

	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	product := doc["product"].(map[string]any)
	s.Equal("prod-001", product["id"], "product.id deve ser preservado")
	tracking := doc["tracking"].(map[string]any)
	s.Equal("token123", tracking["src"], "tracking.src deve ser preservado")
	customer := doc["customer"].(map[string]any)
	s.Equal("[REDACTED]", customer["email"], "customer.email deve estar redactado")
}

func (s *PIIRedactorSuite) TestIdempotent_TwoStripsEquivalent() {
	payload := `{"customer":{"cpf":"123.456.789-00","email":"a@b.com","mobile":"+5511999990000"},"card":{"number":"4111"}}`
	raw := json.RawMessage(payload)

	first, err := s.redactor.Strip(raw)
	s.Require().NoError(err)

	second, err := s.redactor.Strip(first)
	s.Require().NoError(err)

	var docFirst, docSecond map[string]any
	s.Require().NoError(json.Unmarshal(first, &docFirst))
	s.Require().NoError(json.Unmarshal(second, &docSecond))
	s.Equal(docFirst, docSecond, "aplicar Strip 2x deve produzir o mesmo resultado (idempotência)")
}

func (s *PIIRedactorSuite) TestAbsentPath_NoOp() {
	payload := `{"product":{"id":"prod-001"}}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)

	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	product := doc["product"].(map[string]any)
	s.Equal("prod-001", product["id"], "campo sem PII deve ser preservado sem alteração")
}

func (s *PIIRedactorSuite) TestMalformedPayload_ReturnsError() {
	_, err := s.redactor.Strip(json.RawMessage(`{invalid json`))
	s.Require().Error(err)
}

func (s *PIIRedactorSuite) TestEmptyObject() {
	result, err := s.redactor.Strip(json.RawMessage(`{}`))
	s.Require().NoError(err)
	s.JSONEq(`{}`, string(result))
}

func (s *PIIRedactorSuite) TestCustomerNotAnObject_ScalarPath_NoOp() {
	// customer é uma string, não um objeto — deve ser no-op sem pânico
	payload := `{"customer":"not-an-object"}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	s.Equal("not-an-object", doc["customer"], "customer string deve ser preservado quando não é objeto")
}

func (s *PIIRedactorSuite) TestCustomerAddressNotAnObject_WildcardPath_NoOp() {
	// customer.address é uma string — wildcard deve ser no-op
	payload := `{"customer":{"address":"not-an-object","email":"a@b.com"}}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	customer := doc["customer"].(map[string]any)
	s.Equal("not-an-object", customer["address"], "address string deve ser preservado quando não é objeto")
	s.Equal("[REDACTED]", customer["email"])
}

func (s *PIIRedactorSuite) TestPaymentChildNotAnObject_StarMap_NoOp() {
	// payment.pix é uma string, não um objeto — deve ser no-op
	payload := `{"payment":{"pix":"not-an-object","credit":{"card":{"pan":"4111"}}}}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	payment := doc["payment"].(map[string]any)
	s.Equal("not-an-object", payment["pix"], "pix string deve ser preservado")
	credit := payment["credit"].(map[string]any)
	creditCard := credit["card"].(map[string]any)
	s.Equal("[REDACTED]", creditCard["pan"])
}

func (s *PIIRedactorSuite) TestPaymentChildWithoutCardField_StarMap_NoOp() {
	// payment.pix sem campo card — deve ser no-op
	payload := `{"payment":{"pix":{"amount":100}}}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	payment := doc["payment"].(map[string]any)
	pix := payment["pix"].(map[string]any)
	s.Equal(float64(100), pix["amount"], "amount deve ser preservado quando não há card")
}

func (s *PIIRedactorSuite) TestCardNotAnObject_WildcardCard_NoOp() {
	// card é uma string — deve ser no-op
	payload := `{"card":"not-an-object"}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	s.Equal("not-an-object", doc["card"], "card string deve ser preservado")
}

func (s *PIIRedactorSuite) TestPaymentNotAnObject_StarMap_NoOp() {
	// payment é uma string — star map deve ser no-op
	payload := `{"payment":"not-an-object"}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	s.Equal("not-an-object", doc["payment"], "payment string deve ser preservado")
}

func (s *PIIRedactorSuite) TestNullNestedValue_ScalarPath_NoOp() {
	// customer é null — redactScalar deve ser no-op sem pânico
	// (Go unmarshal de null JSON para map[string]any = nil interface)
	payload := `{"customer":null}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	s.Nil(doc["customer"])
}

func (s *PIIRedactorSuite) TestDeepNestedWildcardNotAnObject() {
	// customer.address.street não é objeto mas string — deve ser no-op no redactWildcard
	// quando customer.address não é objeto mas a address é uma string dentro de customer que já é um mapa
	payload := `{"customer":{"address":{"street":"R. A","city":"SP"},"name":"John"}}`
	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))
	customer := doc["customer"].(map[string]any)
	addr := customer["address"].(map[string]any)
	s.Equal("[REDACTED]", addr["street"])
	s.Equal("[REDACTED]", addr["city"])
}

func (s *PIIRedactorSuite) TestFullKiwifyLikePayload() {
	payload := `{
		"id": "event-001",
		"webhook_event_type": "compra_aprovada",
		"customer": {
			"email": "user@example.com",
			"mobile": "+5511999990000",
			"cpf": "123.456.789-00",
			"cnpj": "12.345.678/0001-90",
			"address": {
				"street": "Rua A",
				"city": "São Paulo",
				"state": "SP",
				"zip": "01001-000"
			}
		},
		"card": {
			"number": "4111111111111111",
			"cvv": "123",
			"expiry": "12/26"
		},
		"payment": {
			"credit": {
				"card": {
					"pan": "411111",
					"holder": "JOHN DOE"
				}
			}
		},
		"product": {"id": "prod-monthly"},
		"tracking": {"src": "token-abc"},
		"subscription": {"id": "sub-001"}
	}`

	result, err := s.redactor.Strip(json.RawMessage(payload))
	s.Require().NoError(err)

	var doc map[string]any
	s.Require().NoError(json.Unmarshal(result, &doc))

	customer := doc["customer"].(map[string]any)
	s.Equal("[REDACTED]", customer["email"])
	s.Equal("[REDACTED]", customer["mobile"])
	s.Equal("[REDACTED]", customer["cpf"])
	s.Equal("[REDACTED]", customer["cnpj"])
	address := customer["address"].(map[string]any)
	for field, val := range address {
		s.Equal("[REDACTED]", val, "customer.address.%s deve estar redactado", field)
	}

	cardMap := doc["card"].(map[string]any)
	for field, val := range cardMap {
		s.Equal("[REDACTED]", val, "card.%s deve estar redactado", field)
	}

	payment := doc["payment"].(map[string]any)
	credit := payment["credit"].(map[string]any)
	creditCard := credit["card"].(map[string]any)
	for field, val := range creditCard {
		s.Equal("[REDACTED]", val, "payment.credit.card.%s deve estar redactado", field)
	}

	product := doc["product"].(map[string]any)
	s.Equal("prod-monthly", product["id"], "product.id deve ser preservado")
	tracking := doc["tracking"].(map[string]any)
	s.Equal("token-abc", tracking["src"], "tracking.src deve ser preservado")
}

func (s *PIIRedactorSuite) extractValue(raw []byte, path string) string {
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(raw, &doc))

	parts := splitPath(path)
	current := doc
	for i, part := range parts {
		if i == len(parts)-1 {
			if v, ok := current[part]; ok {
				return v.(string)
			}
			return ""
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	parts = append(parts, path[start:])
	return parts
}
