package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
)

type InvoiceForHandlerSuite struct {
	suite.Suite
	userID uuid.UUID
	uc     *mockInvoiceFor
}

func TestInvoiceForHandler(t *testing.T) {
	suite.Run(t, new(InvoiceForHandlerSuite))
}

func (s *InvoiceForHandlerSuite) SetupTest() {
	s.userID = uuid.New()
	s.uc = &mockInvoiceFor{}
}

func (s *InvoiceForHandlerSuite) TearDownTest() {
	s.uc.AssertExpectations(s.T())
}

func (s *InvoiceForHandlerSuite) getInvoice(cardID, forParam string) *httptest.ResponseRecorder {
	path := "/api/v1/cards/" + cardID + "/invoices"
	if forParam != "" {
		path += "?for=" + forParam
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = req.WithContext(ctxWithPrincipal(s.userID))
	req = withChiParam(req, "id", cardID)
	rr := httptest.NewRecorder()
	handlers.NewInvoiceForHandler(s.uc, noop.NewProvider()).Handle(rr, req)
	return rr
}

func (s *InvoiceForHandlerSuite) TestHandle_OK() {
	invoice := output.Invoice{
		ClosingDate: "2024-01-15",
		DueDate:     "2024-01-22",
	}
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.InvoiceFor")).
		Return(invoice, nil).Once()

	rr := s.getInvoice(uuid.New().String(), "2024-01-10")
	s.Equal(http.StatusOK, rr.Code)
}

func (s *InvoiceForHandlerSuite) TestHandle_MissingForParam() {
	rr := s.getInvoice(uuid.New().String(), "")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *InvoiceForHandlerSuite) TestHandle_InvalidDateFormat() {
	rr := s.getInvoice(uuid.New().String(), "10-01-2024")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *InvoiceForHandlerSuite) TestHandle_InvalidCardID() {
	rr := s.getInvoice("not-a-uuid", "2024-01-10")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *InvoiceForHandlerSuite) TestHandle_CardNotFound() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.InvoiceFor")).
		Return(output.Invoice{}, domain.ErrCardNotFound).Once()

	rr := s.getInvoice(uuid.New().String(), "2024-01-10")
	s.Equal(http.StatusNotFound, rr.Code)
}

func (s *InvoiceForHandlerSuite) TestHandle_InvalidPurchaseDate() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.InvoiceFor")).
		Return(output.Invoice{}, domain.ErrInvalidPurchaseDate).Once()

	rr := s.getInvoice(uuid.New().String(), "2024-01-10")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *InvoiceForHandlerSuite) TestHandle_InternalError() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.InvoiceFor")).
		Return(output.Invoice{}, errUnexpected).Once()

	rr := s.getInvoice(uuid.New().String(), "2024-01-10")
	s.Equal(http.StatusInternalServerError, rr.Code)
}
