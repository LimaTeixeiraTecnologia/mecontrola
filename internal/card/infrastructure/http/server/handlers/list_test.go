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

type ListCardsHandlerSuite struct {
	suite.Suite
	userID uuid.UUID
	uc     *mockListCards
}

func TestListCardsHandler(t *testing.T) {
	suite.Run(t, new(ListCardsHandlerSuite))
}

func (s *ListCardsHandlerSuite) SetupTest() {
	s.userID = uuid.New()
	s.uc = &mockListCards{}
}

func (s *ListCardsHandlerSuite) TearDownTest() {
	s.uc.AssertExpectations(s.T())
}

func (s *ListCardsHandlerSuite) get(query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards"+query, nil)
	req = req.WithContext(ctxWithPrincipal(s.userID))
	rr := httptest.NewRecorder()
	handlers.NewListCardsHandler(s.uc, noop.NewProvider()).Handle(rr, req)
	return rr
}

func (s *ListCardsHandlerSuite) TestHandle_OK() {
	card := sampleCard(s.userID)
	s.uc.On("Execute", mock.Anything, mock.MatchedBy(func(in any) bool { return true })).
		Return(output.CardList{Items: []output.Card{card}}, nil).Once()

	rr := s.get("")
	s.Equal(http.StatusOK, rr.Code)
}

func (s *ListCardsHandlerSuite) TestHandle_WithCursorAndLimit() {
	s.uc.On("Execute", mock.Anything, mock.MatchedBy(func(in any) bool { return true })).
		Return(output.CardList{}, nil).Once()

	rr := s.get("?limit=5&cursor=abc123")
	s.Equal(http.StatusOK, rr.Code)
}

func (s *ListCardsHandlerSuite) TestHandle_InvalidLimit() {
	rr := s.get("?limit=notanumber")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *ListCardsHandlerSuite) TestHandle_NegativeLimit() {
	rr := s.get("?limit=-1")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *ListCardsHandlerSuite) TestHandle_InvalidCursor() {
	s.uc.On("Execute", mock.Anything, mock.MatchedBy(func(in any) bool { return true })).
		Return(output.CardList{}, domain.ErrInvalidCursor).Once()

	rr := s.get("?cursor=invalid-cursor")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *ListCardsHandlerSuite) TestHandle_LimitCapAt100() {
	s.uc.On("Execute", mock.Anything, mock.MatchedBy(func(in any) bool { return true })).
		Return(output.CardList{}, nil).Once()

	rr := s.get("?limit=500")
	s.Equal(http.StatusOK, rr.Code)
}

func (s *ListCardsHandlerSuite) TestHandle_InternalError() {
	s.uc.On("Execute", mock.Anything, mock.MatchedBy(func(in any) bool { return true })).
		Return(output.CardList{}, errUnexpected).Once()

	rr := s.get("")
	s.Equal(http.StatusInternalServerError, rr.Code)
}
