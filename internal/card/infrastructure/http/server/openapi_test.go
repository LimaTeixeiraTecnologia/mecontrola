package server_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/suite"
)

type OpenAPIValidationSuite struct {
	suite.Suite
	loader *openapi3.Loader
	doc    *openapi3.T
}

func TestOpenAPIValidation(t *testing.T) {
	suite.Run(t, new(OpenAPIValidationSuite))
}

func (s *OpenAPIValidationSuite) SetupSuite() {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	yamlPath := filepath.Join(dir, "..", "..", "..", "openapi.yaml")

	s.loader = openapi3.NewLoader()
	doc, err := s.loader.LoadFromFile(yamlPath)
	s.Require().NoError(err, "openapi.yaml deve ser carregado sem erro")
	s.doc = doc
}

func (s *OpenAPIValidationSuite) TestDoc_NotNil() {
	s.Require().NotNil(s.doc)
}

func (s *OpenAPIValidationSuite) TestDoc_Info() {
	s.Equal("3.2.0", s.doc.OpenAPI)
	s.Require().NotNil(s.doc.Info)
	s.Equal("MeControla Cards API", s.doc.Info.Title)
}

func (s *OpenAPIValidationSuite) TestDoc_Endpoints() {
	s.Require().NotNil(s.doc.Paths)

	paths := map[string][]string{
		"/api/v1/cards":                   {"POST", "GET"},
		"/api/v1/cards/best-purchase-day": {"GET"},
		"/api/v1/cards/{id}":              {"GET", "PUT", "DELETE"},
		"/api/v1/cards/{id}/invoices":     {"GET"},
	}

	for path, methods := range paths {
		pathItem := s.doc.Paths.Find(path)
		s.Require().NotNil(pathItem, "path %s deve existir", path)
		for _, method := range methods {
			switch method {
			case "POST":
				s.NotNil(pathItem.Post, "POST %s deve existir", path)
			case "GET":
				s.NotNil(pathItem.Get, "GET %s deve existir", path)
			case "PUT":
				s.NotNil(pathItem.Put, "PUT %s deve existir", path)
			case "DELETE":
				s.NotNil(pathItem.Delete, "DELETE %s deve existir", path)
			case "PATCH":
				s.NotNil(pathItem.Patch, "PATCH %s deve existir", path)
			}
		}
	}

	s.Nil(s.doc.Paths.Find("/api/v1/cards/{id}/limit"), "rota /limit deve ter sido removida")
}

func (s *OpenAPIValidationSuite) TestDoc_Schemas() {
	s.Require().NotNil(s.doc.Components)
	schemas := s.doc.Components.Schemas
	s.Contains(schemas, "Card")
	s.Contains(schemas, "CardList")
	s.Contains(schemas, "Invoice")
	s.Contains(schemas, "CreateCardRequest")
	s.Contains(schemas, "UpdateCardRequest")
	s.Contains(schemas, "BestPurchaseDayResponse")
	s.Contains(schemas, "ProblemDetail")
}

func (s *OpenAPIValidationSuite) TestDoc_BestPurchaseDayResponse_Has200And400And401And500() {
	path := s.doc.Paths.Find("/api/v1/cards/best-purchase-day")
	s.Require().NotNil(path, "path /api/v1/cards/best-purchase-day deve existir")
	s.Require().NotNil(path.Get)
	responses := path.Get.Responses
	s.NotNil(responses.Status(200), "200 deve existir em GET /api/v1/cards/best-purchase-day")
	s.NotNil(responses.Status(400), "400 deve existir em GET /api/v1/cards/best-purchase-day")
	s.NotNil(responses.Status(401), "401 deve existir em GET /api/v1/cards/best-purchase-day")
	s.NotNil(responses.Status(500), "500 deve existir em GET /api/v1/cards/best-purchase-day")
}

func (s *OpenAPIValidationSuite) TestDoc_CreateCardRequest_HasBankNotLimitCents() {
	s.Require().NotNil(s.doc.Components)
	schema := s.doc.Components.Schemas["CreateCardRequest"]
	s.Require().NotNil(schema)
	props := schema.Value.Properties
	s.Contains(props, "bank", "CreateCardRequest deve ter campo bank")
	s.NotContains(props, "limit_cents", "CreateCardRequest nao deve ter campo limit_cents")
	s.NotContains(props, "closing_day", "CreateCardRequest nao deve ter campo closing_day como entrada")
}

func (s *OpenAPIValidationSuite) TestDoc_Card_HasBestPurchaseDayNotLimitCents() {
	s.Require().NotNil(s.doc.Components)
	schema := s.doc.Components.Schemas["Card"]
	s.Require().NotNil(schema)
	props := schema.Value.Properties
	s.Contains(props, "best_purchase_day", "Card deve ter campo best_purchase_day")
	s.Contains(props, "bank", "Card deve ter campo bank")
	s.NotContains(props, "limit_cents", "Card nao deve ter campo limit_cents")
}

func (s *OpenAPIValidationSuite) TestDoc_PostCards_Has201And400And401And409And500() {
	path := s.doc.Paths.Find("/api/v1/cards")
	s.Require().NotNil(path)
	s.Require().NotNil(path.Post)
	responses := path.Post.Responses
	s.Require().NotNil(responses)
	s.NotNil(responses.Status(201), "201 deve existir em POST /api/v1/cards")
	s.NotNil(responses.Status(400), "400 deve existir em POST /api/v1/cards")
	s.NotNil(responses.Status(401), "401 deve existir em POST /api/v1/cards")
	s.NotNil(responses.Status(409), "409 deve existir em POST /api/v1/cards")
	s.NotNil(responses.Status(500), "500 deve existir em POST /api/v1/cards")
}

func (s *OpenAPIValidationSuite) TestDoc_DeleteCard_Has204() {
	path := s.doc.Paths.Find("/api/v1/cards/{id}")
	s.Require().NotNil(path)
	s.Require().NotNil(path.Delete)
	s.NotNil(path.Delete.Responses.Status(204), "204 deve existir em DELETE /api/v1/cards/{id}")
}

func (s *OpenAPIValidationSuite) TestDoc_GetInvoices_Has200And400And401And404And500() {
	path := s.doc.Paths.Find("/api/v1/cards/{id}/invoices")
	s.Require().NotNil(path)
	s.Require().NotNil(path.Get)
	responses := path.Get.Responses
	s.NotNil(responses.Status(200), "200 deve existir em GET invoices")
	s.NotNil(responses.Status(400), "400 deve existir em GET invoices")
	s.NotNil(responses.Status(401), "401 deve existir em GET invoices")
	s.NotNil(responses.Status(404), "404 deve existir em GET invoices")
	s.NotNil(responses.Status(500), "500 deve existir em GET invoices")
}
