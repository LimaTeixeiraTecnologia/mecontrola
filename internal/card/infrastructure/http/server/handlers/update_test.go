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

type UpdateCardHandlerSuite struct {
	suite.Suite
	userID uuid.UUID
	uc     *mockUpdateCard
}

func TestUpdateCardHandler(t *testing.T) {
	suite.Run(t, new(UpdateCardHandlerSuite))
}

func (s *UpdateCardHandlerSuite) SetupTest() {
	s.userID = uuid.New()
	s.uc = &mockUpdateCard{}
}

func (s *UpdateCardHandlerSuite) TearDownTest() {
	s.uc.AssertExpectations(s.T())
}

func (s *UpdateCardHandlerSuite) put(cardID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cards/"+cardID, strings.NewReader(body))
	req = req.WithContext(ctxWithPrincipal(s.userID))
	req = withChiParam(req, "id", cardID)
	rr := httptest.NewRecorder()
	handlers.NewUpdateCardHandler(s.uc, noop.NewProvider()).Handle(rr, req)
	return rr
}

func (s *UpdateCardHandlerSuite) TestHandle_OK() {
	card := sampleCard(s.userID)
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCard")).
		Return(card, nil).Once()

	rr := s.put(uuid.New().String(), `{"name":"Itau","nickname":"It","closing_day":10,"due_day":20}`)
	s.Equal(http.StatusOK, rr.Code)
}

func (s *UpdateCardHandlerSuite) TestHandle_InvalidJSON() {
	rr := s.put(uuid.New().String(), "{bad-json}")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *UpdateCardHandlerSuite) TestHandle_InvalidCardID() {
	rr := s.put("not-a-uuid", `{"name":"Itau","nickname":"It","closing_day":10,"due_day":20}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *UpdateCardHandlerSuite) TestHandle_NotFound() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCard")).
		Return(output.Card{}, domain.ErrCardNotFound).Once()

	rr := s.put(uuid.New().String(), `{"name":"Itau","nickname":"It","closing_day":10,"due_day":20}`)
	s.Equal(http.StatusNotFound, rr.Code)
}

func (s *UpdateCardHandlerSuite) TestHandle_NicknameConflict() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCard")).
		Return(output.Card{}, domain.ErrNicknameConflict).Once()

	rr := s.put(uuid.New().String(), `{"name":"Itau","nickname":"It","closing_day":10,"due_day":20}`)
	s.Equal(http.StatusConflict, rr.Code)
}

func (s *UpdateCardHandlerSuite) TestHandle_InvalidName() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCard")).
		Return(output.Card{}, domain.ErrInvalidCardName).Once()

	rr := s.put(uuid.New().String(), `{"name":"","nickname":"It","closing_day":10,"due_day":20}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *UpdateCardHandlerSuite) TestHandle_InternalError() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCard")).
		Return(output.Card{}, errUnexpected).Once()

	rr := s.put(uuid.New().String(), `{"name":"Itau","nickname":"It","closing_day":10,"due_day":20}`)
	s.Equal(http.StatusInternalServerError, rr.Code)
}
