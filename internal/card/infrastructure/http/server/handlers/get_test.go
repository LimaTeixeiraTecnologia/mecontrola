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

type GetCardHandlerSuite struct {
	suite.Suite
	userID uuid.UUID
	uc     *mockGetCard
}

func TestGetCardHandler(t *testing.T) {
	suite.Run(t, new(GetCardHandlerSuite))
}

func (s *GetCardHandlerSuite) SetupTest() {
	s.userID = uuid.New()
	s.uc = &mockGetCard{}
}

func (s *GetCardHandlerSuite) TearDownTest() {
	s.uc.AssertExpectations(s.T())
}

func (s *GetCardHandlerSuite) get(cardID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/"+cardID, nil)
	req = req.WithContext(ctxWithPrincipal(s.userID))
	req = withChiParam(req, "id", cardID)
	rr := httptest.NewRecorder()
	handlers.NewGetCardHandler(s.uc, noop.NewProvider()).Handle(rr, req)
	return rr
}

func (s *GetCardHandlerSuite) TestHandle_OK() {
	card := sampleCard(s.userID)
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.GetCard")).
		Return(card, nil).Once()

	rr := s.get(uuid.New().String())
	s.Equal(http.StatusOK, rr.Code)
}

func (s *GetCardHandlerSuite) TestHandle_NotFound() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.GetCard")).
		Return(output.Card{}, domain.ErrCardNotFound).Once()

	rr := s.get(uuid.New().String())
	s.Equal(http.StatusNotFound, rr.Code)
}

func (s *GetCardHandlerSuite) TestHandle_InvalidUUID() {
	rr := s.get("not-a-uuid")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *GetCardHandlerSuite) TestHandle_InternalError() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.GetCard")).
		Return(output.Card{}, errUnexpected).Once()

	rr := s.get(uuid.New().String())
	s.Equal(http.StatusInternalServerError, rr.Code)
}
