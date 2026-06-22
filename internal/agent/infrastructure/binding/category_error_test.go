package binding

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	transactionsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CategoryErrorSuite struct {
	suite.Suite
}

func TestCategoryErrorSuite(t *testing.T) {
	suite.Run(t, new(CategoryErrorSuite))
}

func (s *CategoryErrorSuite) TestTranslateCategoryAmbiguous() {
	src := &usecases.CategoryAmbiguousError{Hint: "academia", Candidates: []string{"Prazeres", "Custo Fixo"}}

	translated := translateCategoryError(src)

	var ambiguous *appservices.CategoryAmbiguousError
	s.Require().ErrorAs(translated, &ambiguous)
	s.Equal("academia", ambiguous.Hint)
	s.Equal([]string{"Prazeres", "Custo Fixo"}, ambiguous.Candidates)
}

func (s *CategoryErrorSuite) TestTranslateCategoryNotFound() {
	translated := translateCategoryError(usecases.ErrLogTransactionCategoryNotFound)
	s.Require().ErrorIs(translated, appservices.ErrCategoryNotFound)
}

func (s *CategoryErrorSuite) TestTranslateCategoryHintMissing() {
	translated := translateCategoryError(usecases.ErrLogTransactionNoCategoryHint)
	s.Require().ErrorIs(translated, appservices.ErrCategoryHintMissing)
}

func (s *CategoryErrorSuite) TestTranslateCategoryPassthrough() {
	other := errors.New("boom")
	s.Require().ErrorIs(translateCategoryError(other), other)
	s.Require().Nil(translateCategoryError(nil))
}

func (s *CategoryErrorSuite) TestTranslateRecurringInvalidDay() {
	wrapped := fmt.Errorf("agent: recurring creator: criar: %w", transactionsvo.ErrDayOfMonthOutOfRange)
	translated := translateRecurringError(wrapped)
	s.Require().ErrorIs(translated, appservices.ErrRecurringInvalidDay)
}

func (s *CategoryErrorSuite) TestTranslateRecurringCategory() {
	translated := translateRecurringError(usecases.ErrLogTransactionNoCategoryHint)
	s.Require().ErrorIs(translated, appservices.ErrCategoryHintMissing)
}

func (s *CategoryErrorSuite) TestTranslateRecurringPassthrough() {
	other := errors.New("boom")
	s.Require().ErrorIs(translateRecurringError(other), other)
	s.Require().Nil(translateRecurringError(nil))
}
