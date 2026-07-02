package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
)

type BestPurchaseDayHandlerSuite struct {
	suite.Suite
	uc *mockBestPurchaseDay
}

func TestBestPurchaseDayHandler(t *testing.T) {
	suite.Run(t, new(BestPurchaseDayHandlerSuite))
}

func (s *BestPurchaseDayHandlerSuite) SetupTest() {
	s.uc = &mockBestPurchaseDay{}
}

func (s *BestPurchaseDayHandlerSuite) TearDownTest() {
	s.uc.AssertExpectations(s.T())
}

func (s *BestPurchaseDayHandlerSuite) get(bank, dueDay string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/best-purchase-day?bank="+bank+"&due_day="+dueDay, nil)
	rr := httptest.NewRecorder()
	handlers.NewBestPurchaseDayHandler(s.uc, noop.NewProvider()).Handle(rr, req)
	return rr
}

func (s *BestPurchaseDayHandlerSuite) TestHandle_OK() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.BestPurchaseDay")).
		Return(output.BestPurchaseDay{ClosingDay: 13, BestPurchaseDay: 14}, nil).Once()

	rr := s.get("nubank", "20")

	s.Equal(http.StatusOK, rr.Code)
	s.Contains(rr.Body.String(), `"closing_day":13`)
	s.Contains(rr.Body.String(), `"best_purchase_day":14`)
}

func (s *BestPurchaseDayHandlerSuite) TestHandle_MissingBank_Returns400() {
	rr := s.get("", "20")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *BestPurchaseDayHandlerSuite) TestHandle_InvalidDueDayString_Returns400() {
	rr := s.get("nubank", "abc")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *BestPurchaseDayHandlerSuite) TestHandle_DueDayOutOfRange_Returns400() {
	rr := s.get("nubank", "0")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *BestPurchaseDayHandlerSuite) TestHandle_UseCaseError_Returns500() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.BestPurchaseDay")).
		Return(output.BestPurchaseDay{}, errors.New("db error")).Once()

	rr := s.get("nubank", "20")
	s.Equal(http.StatusInternalServerError, rr.Code)
}
