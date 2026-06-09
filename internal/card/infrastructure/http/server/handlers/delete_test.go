package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
)

type DeleteCardHandlerSuite struct {
	suite.Suite
	userID uuid.UUID
	uc     *mockSoftDeleteCard
}

func TestDeleteCardHandler(t *testing.T) {
	suite.Run(t, new(DeleteCardHandlerSuite))
}

func (s *DeleteCardHandlerSuite) SetupTest() {
	s.userID = uuid.New()
	s.uc = &mockSoftDeleteCard{}
}

func (s *DeleteCardHandlerSuite) TearDownTest() {
	s.uc.AssertExpectations(s.T())
}

func (s *DeleteCardHandlerSuite) del(cardID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cards/"+cardID, nil)
	req = req.WithContext(ctxWithPrincipal(s.userID))
	req = withChiParam(req, "id", cardID)
	rr := httptest.NewRecorder()
	handlers.NewDeleteCardHandler(s.uc, noop.NewProvider()).Handle(rr, req)
	return rr
}

func (s *DeleteCardHandlerSuite) TestHandle_NoContent() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.SoftDeleteCard")).
		Return(nil).Once()

	rr := s.del(uuid.New().String())
	s.Equal(http.StatusNoContent, rr.Code)
}

func (s *DeleteCardHandlerSuite) TestHandle_NotFound() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.SoftDeleteCard")).
		Return(domain.ErrCardNotFound).Once()

	rr := s.del(uuid.New().String())
	s.Equal(http.StatusNotFound, rr.Code)
}

func (s *DeleteCardHandlerSuite) TestHandle_InvalidCardID() {
	rr := s.del("not-a-uuid")
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *DeleteCardHandlerSuite) TestHandle_InternalError() {
	s.uc.On("Execute", mock.Anything, mock.AnythingOfType("input.SoftDeleteCard")).
		Return(errUnexpected).Once()

	rr := s.del(uuid.New().String())
	s.Equal(http.StatusInternalServerError, rr.Code)
}
