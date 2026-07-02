package handlers_test

import (
	"bytes"
	"encoding/json"
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

type CreateCardHandlerSuite struct {
	suite.Suite
	userID uuid.UUID
	uc     *mockCreateCard
}

func TestCreateCardHandler(t *testing.T) {
	suite.Run(t, new(CreateCardHandlerSuite))
}

func (s *CreateCardHandlerSuite) SetupTest() {
	s.userID = uuid.New()
	s.uc = &mockCreateCard{}
}

func (s *CreateCardHandlerSuite) TearDownTest() {
	s.uc.AssertExpectations(s.T())
}

func (s *CreateCardHandlerSuite) post(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cards", strings.NewReader(body))
	req = req.WithContext(ctxWithPrincipal(s.userID))
	rr := httptest.NewRecorder()
	handlers.NewCreateCardHandler(s.uc, noop.NewProvider()).Handle(rr, req)
	return rr
}

func (s *CreateCardHandlerSuite) TestHandle_Created() {
	card := sampleCard(s.userID)
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).Return(card, nil).Once()

	body := `{"name":"Nubank","nickname":"Nu","closing_day":15,"due_day":22}`
	rr := s.post(body)

	s.Equal(http.StatusCreated, rr.Code)
	s.NotEmpty(rr.Header().Get("Location"))
	s.Contains(rr.Header().Get("Location"), card.ID)
}

func (s *CreateCardHandlerSuite) TestHandle_InvalidJSON() {
	rr := s.post("{bad-json}")
	s.Equal(http.StatusBadRequest, rr.Code)

	var resp map[string]any
	s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &resp))
	detail, _ := resp["detail"].(string)
	s.Equal("payload inválido", detail)
}

func (s *CreateCardHandlerSuite) TestHandle_InvalidBank() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).
		Return(output.Card{}, domain.ErrInvalidBank).Once()

	rr := s.post(`{"nickname":"Nu","bank":"","due_day":22}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *CreateCardHandlerSuite) TestHandle_InvalidNickname() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).
		Return(output.Card{}, domain.ErrInvalidNickname).Once()

	rr := s.post(`{"name":"Nubank","nickname":"","closing_day":15,"due_day":22}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *CreateCardHandlerSuite) TestHandle_InvalidClosingDay() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).
		Return(output.Card{}, domain.ErrInvalidClosingDay).Once()

	rr := s.post(`{"name":"Nubank","nickname":"Nu","closing_day":0,"due_day":22}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *CreateCardHandlerSuite) TestHandle_InvalidDueDay() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).
		Return(output.Card{}, domain.ErrInvalidDueDay).Once()

	rr := s.post(`{"name":"Nubank","nickname":"Nu","closing_day":15,"due_day":0}`)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *CreateCardHandlerSuite) TestHandle_NicknameConflict() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).
		Return(output.Card{}, domain.ErrNicknameConflict).Once()

	rr := s.post(`{"name":"Nubank","nickname":"Nu","closing_day":15,"due_day":22}`)
	s.Equal(http.StatusConflict, rr.Code)
}

func (s *CreateCardHandlerSuite) TestHandle_InternalError() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).
		Return(output.Card{}, errUnexpected).Once()

	rr := s.post(`{"name":"Nubank","nickname":"Nu","closing_day":15,"due_day":22}`)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *CreateCardHandlerSuite) TestHandle_NoPIIInResponse() {
	card := sampleCard(s.userID)
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).Return(card, nil).Once()

	body := `{"name":"Nubank","nickname":"Nu","closing_day":15,"due_day":22}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cards", bytes.NewBufferString(body))
	req = req.WithContext(ctxWithPrincipal(s.userID))
	rr := httptest.NewRecorder()
	handlers.NewCreateCardHandler(s.uc, noop.NewProvider()).Handle(rr, req)
	s.Equal(http.StatusCreated, rr.Code)
}
