package usecases

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
)

type ConfigureBudgetConversationSuite struct {
	suite.Suite
	ctx context.Context
}

func TestConfigureBudgetConversationSuite(t *testing.T) {
	suite.Run(t, new(ConfigureBudgetConversationSuite))
}

func (s *ConfigureBudgetConversationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ConfigureBudgetConversationSuite) newSUT() *ConfigureBudgetConversation {
	uc, ucErr := NewConfigureBudgetConversation(fake.NewProvider())
	s.Require().NoError(ucErr)
	return uc
}

func (s *ConfigureBudgetConversationSuite) TestNewNilDeps() {
	_, err := NewConfigureBudgetConversation(nil)
	s.Require().Error(err)
}

func (s *ConfigureBudgetConversationSuite) TestPartialTurnAsksWhatIsMissing() {
	uc := s.newSUT()
	out, err := uc.Execute(s.ctx, ConfigureBudgetInput{
		Change: budgetdraft.Change{
			TotalCents:  500000,
			Allocations: map[string]int{budgetdraft.SlugCustoFixo: 3500},
		},
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.False(out.Complete)
	s.Equal(int64(500000), out.Draft.TotalCents())
	s.Equal(3500, out.Draft.SumBasisPoints())
	s.NotEmpty(out.Reply)
}

func (s *ConfigureBudgetConversationSuite) TestAsksForIncomeWhenTotalMissing() {
	uc := s.newSUT()
	out, err := uc.Execute(s.ctx, ConfigureBudgetInput{
		Change: budgetdraft.Change{
			Allocations: map[string]int{budgetdraft.SlugMetas: 2000},
		},
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.False(out.Complete)
	s.Contains(out.Reply, "renda")
}

func (s *ConfigureBudgetConversationSuite) TestCompleteWhenSumIs10000() {
	uc := s.newSUT()
	out, err := uc.Execute(s.ctx, ConfigureBudgetInput{
		Change: budgetdraft.Change{
			TotalCents: 800000,
			Allocations: map[string]int{
				budgetdraft.SlugCustoFixo:           3500,
				budgetdraft.SlugConhecimento:        1000,
				budgetdraft.SlugPrazeres:            2000,
				budgetdraft.SlugMetas:               2000,
				budgetdraft.SlugLiberdadeFinanceira: 1500,
			},
		},
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.True(out.Complete)
	s.Empty(out.Reply)
	s.Equal(int64(800000), out.Draft.TotalCents())
	s.Equal(10000, out.Draft.SumBasisPoints())
}

func (s *ConfigureBudgetConversationSuite) TestMergeOverMultipleTurns() {
	uc := s.newSUT()
	out1, err := uc.Execute(s.ctx, ConfigureBudgetInput{
		Change: budgetdraft.Change{
			TotalCents:  500000,
			Allocations: map[string]int{budgetdraft.SlugCustoFixo: 5000},
		},
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.False(out1.Complete)

	out2, err := uc.Execute(s.ctx, ConfigureBudgetInput{
		Change: budgetdraft.Change{
			Allocations: map[string]int{
				budgetdraft.SlugConhecimento:        1000,
				budgetdraft.SlugPrazeres:            1000,
				budgetdraft.SlugMetas:               1000,
				budgetdraft.SlugLiberdadeFinanceira: 2000,
			},
		},
		Draft: out1.Draft,
	})
	s.Require().NoError(err)
	s.True(out2.Complete)
	s.Equal(int64(500000), out2.Draft.TotalCents())
}

func (s *ConfigureBudgetConversationSuite) TestEmptyChangeKeepsDraftAndAsks() {
	uc := s.newSUT()
	out, err := uc.Execute(s.ctx, ConfigureBudgetInput{
		Change: budgetdraft.Change{},
		Draft:  budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.False(out.Complete)
	s.NotEmpty(out.Reply)
	s.Equal(int64(0), out.Draft.TotalCents())
}

func (s *ConfigureBudgetConversationSuite) TestInvalidSlugKeepsDraftAndAsks() {
	uc := s.newSUT()
	out, err := uc.Execute(s.ctx, ConfigureBudgetInput{
		Change: budgetdraft.Change{
			TotalCents:  500000,
			Allocations: map[string]int{"expense.invalido": 3000},
		},
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.False(out.Complete)
	s.NotEmpty(out.Reply)
	s.Equal(int64(0), out.Draft.TotalCents())
}
