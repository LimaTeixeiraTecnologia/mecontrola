package usecases

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type ClassifierSuite struct {
	suite.Suite
}

func TestClassifier(t *testing.T) {
	suite.Run(t, new(ClassifierSuite))
}

func (s *ClassifierSuite) TestClassifyCardOutcome_AllBranches() {
	scenarios := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil retorna internal_error", err: nil, want: "internal_error"},
		{name: "not_found direto", err: domain.ErrCardNotFound, want: "not_found"},
		{name: "not_found embrulhado", err: fmt.Errorf("ctx: %w", domain.ErrCardNotFound), want: "not_found"},
		{name: "conflict direto", err: domain.ErrNicknameConflict, want: "conflict"},
		{name: "invalid bank", err: domain.ErrInvalidBank, want: "invalid"},
		{name: "invalid nickname", err: domain.ErrInvalidNickname, want: "invalid"},
		{name: "invalid closing day", err: domain.ErrInvalidClosingDay, want: "invalid"},
		{name: "invalid due day", err: domain.ErrInvalidDueDay, want: "invalid"},
		{name: "invalid purchase date", err: domain.ErrInvalidPurchaseDate, want: "invalid"},
		{name: "invalid cursor", err: domain.ErrInvalidCursor, want: "invalid"},
		{name: "erro desconhecido", err: errors.New("unexpected"), want: "internal_error"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.Equal(sc.want, classifyCardOutcome(sc.err))
		})
	}
}

func (s *ClassifierSuite) TestIsCardValidationError_AllSentinels() {
	sentinels := []error{
		domain.ErrInvalidBank,
		domain.ErrInvalidNickname,
		domain.ErrInvalidClosingDay,
		domain.ErrInvalidDueDay,
		domain.ErrInvalidPurchaseDate,
		domain.ErrInvalidCursor,
	}
	for _, err := range sentinels {
		s.True(isCardValidationError(err))
		s.True(isCardValidationError(fmt.Errorf("wrap: %w", err)))
	}
	s.False(isCardValidationError(domain.ErrCardNotFound))
	s.False(isCardValidationError(domain.ErrNicknameConflict))
	s.False(isCardValidationError(errors.New("random")))
	s.False(isCardValidationError(nil))
}
