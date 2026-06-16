package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
)

type UpdateCardLimitHandlerSuite struct {
	suite.Suite
	userID uuid.UUID
	uc     *mockUpdateCardLimit
}

func TestUpdateCardLimitHandler(t *testing.T) {
	suite.Run(t, new(UpdateCardLimitHandlerSuite))
}

func (s *UpdateCardLimitHandlerSuite) SetupTest() {
	s.userID = uuid.New()
	s.uc = &mockUpdateCardLimit{}
}

func (s *UpdateCardLimitHandlerSuite) TearDownTest() {
	s.uc.AssertExpectations(s.T())
}

func (s *UpdateCardLimitHandlerSuite) patch(cardID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cards/"+cardID+"/limit", strings.NewReader(body))
	req = req.WithContext(ctxWithPrincipal(s.userID))
	req = withChiParam(req, "id", cardID)
	rr := httptest.NewRecorder()
	handlers.NewUpdateCardLimitHandler(s.uc, noop.NewProvider()).Handle(rr, req)
	return rr
}

func (s *UpdateCardLimitHandlerSuite) TestHandle_OK() {
	card := sampleCard(s.userID)
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCardLimit")).
		Return(card, nil).Once()

	rr := s.patch(uuid.New().String(), `{"limit_cents":500000}`)
	s.Equal(http.StatusOK, rr.Code)
}

func (s *UpdateCardLimitHandlerSuite) TestHandle_InvalidJSON() {
	rr := s.patch(uuid.New().String(), "{bad-json}")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *UpdateCardLimitHandlerSuite) TestHandle_InvalidCardID() {
	rr := s.patch("not-a-uuid", `{"limit_cents":500000}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *UpdateCardLimitHandlerSuite) TestHandle_MissingLimitCents() {
	rr := s.patch(uuid.New().String(), `{}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *UpdateCardLimitHandlerSuite) TestHandle_NotFound() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCardLimit")).
		Return(output.Card{}, domain.ErrCardNotFound).Once()

	rr := s.patch(uuid.New().String(), `{"limit_cents":500000}`)
	s.Equal(http.StatusNotFound, rr.Code)
}

func (s *UpdateCardLimitHandlerSuite) TestHandle_NegativeLimit() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCardLimit")).
		Return(output.Card{}, domain.ErrCardLimitNegative).Once()

	rr := s.patch(uuid.New().String(), `{"limit_cents":-1}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *UpdateCardLimitHandlerSuite) TestHandle_TooLarge() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCardLimit")).
		Return(output.Card{}, domain.ErrCardLimitTooLarge).Once()

	rr := s.patch(uuid.New().String(), `{"limit_cents":100000000001}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *UpdateCardLimitHandlerSuite) TestHandle_InternalError() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCardLimit")).
		Return(output.Card{}, errUnexpected).Once()

	rr := s.patch(uuid.New().String(), `{"limit_cents":500000}`)
	s.Equal(http.StatusInternalServerError, rr.Code)
}
