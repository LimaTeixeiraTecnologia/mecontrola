package tools_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

var categoriesTestUserID = uuid.MustParse("00000000-0000-0000-0000-000000000042")

func newCategoriesRecorder(obs observability.Observability) *tools.Recorder {
	return tools.NewRecorder(obs.Metrics().Counter("test_routed_total", "", "1"))
}

type BudgetDetailsSuite struct {
	suite.Suite
	ctx         context.Context
	obs         observability.Observability
	loc         *time.Location
	summaryMock *mocks.MonthlySummaryReader
}

func TestBudgetDetailsSuite(t *testing.T) {
	suite.Run(t, new(BudgetDetailsSuite))
}

func (s *BudgetDetailsSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.loc = time.UTC
	s.summaryMock = mocks.NewMonthlySummaryReader(s.T())
}

func (s *BudgetDetailsSuite) input() tools.ToolInput {
	i, _ := intent.NewBudgetDetails("2026-06")
	return tools.ToolInput{UserID: categoriesTestUserID, Channel: "whatsapp", Intent: i}
}

func (s *BudgetDetailsSuite) TestExecute() {
	planned := int64(100000)
	type dependencies struct {
		summary tools.MonthlySummaryReader
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(result tools.ToolResult, err error)
	}{
		{
			name: "deve retornar detalhes formatados em sucesso",
			dependencies: dependencies{
				summary: func() *mocks.MonthlySummaryReader {
					s.summaryMock.EXPECT().
						Execute(s.ctx, categoriesTestUserID.String(), "2026-06").
						Return(budgetsoutput.MonthlySummaryOutput{
							Competence:        "2026-06",
							TotalSpentCents:   50000,
							TotalPlannedCents: &planned,
							Allocations: []budgetsoutput.AllocationSummary{
								{RootSlug: "expense.custo_fixo", PlannedCents: &planned, SpentCents: 50000},
							},
						}, nil).
						Once()
					return s.summaryMock
				}(),
			},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeRouted, result.Outcome)
				s.Equal(intent.KindBudgetDetails, result.Kind)
				s.Contains(result.Reply, "Detalhes do orçamento")
				s.Contains(result.Reply, "R$")
			},
		},
		{
			name:         "deve retornar OutcomeMissingResolver quando summary e nil",
			dependencies: dependencies{summary: nil},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeMissingResolver, result.Outcome)
				s.Equal(intent.KindBudgetDetails, result.Kind)
			},
		},
		{
			name: "deve retornar OutcomeUsecaseError quando summary falha",
			dependencies: dependencies{
				summary: func() *mocks.MonthlySummaryReader {
					s.summaryMock.EXPECT().
						Execute(s.ctx, categoriesTestUserID.String(), "2026-06").
						Return(budgetsoutput.MonthlySummaryOutput{}, errors.New("falha no banco")).
						Once()
					return s.summaryMock
				}(),
			},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeUsecaseError, result.Outcome)
				s.Equal(intent.KindBudgetDetails, result.Kind)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			tool := tools.NewBudgetDetails(newCategoriesRecorder(s.obs), scenario.dependencies.summary, s.loc, s.obs)
			result, err := tool.Execute(s.ctx, s.input())
			scenario.expect(result, err)
		})
	}
}

type ListCategoriesSuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	listerMock *mocks.CategoryLister
}

func TestListCategoriesSuite(t *testing.T) {
	suite.Run(t, new(ListCategoriesSuite))
}

func (s *ListCategoriesSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.listerMock = mocks.NewCategoryLister(s.T())
}

func (s *ListCategoriesSuite) input() tools.ToolInput {
	return tools.ToolInput{UserID: categoriesTestUserID, Channel: "whatsapp", Intent: intent.NewListCategories()}
}

func (s *ListCategoriesSuite) TestExecute() {
	type dependencies struct {
		lister tools.CategoryLister
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(result tools.ToolResult, err error)
	}{
		{
			name: "deve listar categorias em sucesso",
			dependencies: dependencies{
				lister: func() *mocks.CategoryLister {
					s.listerMock.EXPECT().
						Execute(s.ctx, categoriesTestUserID).
						Return(tools.CategoryListResult{Categories: []tools.CategoryView{
							{Slug: "expense.custo_fixo.aluguel", Name: "Aluguel", ParentSlug: "expense.custo_fixo"},
						}}, nil).
						Once()
					return s.listerMock
				}(),
			},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeRouted, result.Outcome)
				s.Equal(intent.KindListCategories, result.Kind)
				s.Contains(result.Reply, "Aluguel")
			},
		},
		{
			name:         "deve retornar OutcomeMissingResolver quando lister e nil",
			dependencies: dependencies{lister: nil},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeMissingResolver, result.Outcome)
				s.Equal(intent.KindListCategories, result.Kind)
			},
		},
		{
			name: "deve retornar OutcomeUsecaseError quando lister falha",
			dependencies: dependencies{
				lister: func() *mocks.CategoryLister {
					s.listerMock.EXPECT().
						Execute(s.ctx, categoriesTestUserID).
						Return(tools.CategoryListResult{}, errors.New("falha no banco")).
						Once()
					return s.listerMock
				}(),
			},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeUsecaseError, result.Outcome)
				s.Equal(intent.KindListCategories, result.Kind)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			tool := tools.NewListCategories(newCategoriesRecorder(s.obs), scenario.dependencies.lister, s.obs)
			result, err := tool.Execute(s.ctx, s.input())
			scenario.expect(result, err)
		})
	}
}

type ClassifyCategorySuite struct {
	suite.Suite
	ctx            context.Context
	obs            observability.Observability
	classifierMock *mocks.CategoryClassifier
}

func TestClassifyCategorySuite(t *testing.T) {
	suite.Run(t, new(ClassifyCategorySuite))
}

func (s *ClassifyCategorySuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.classifierMock = mocks.NewCategoryClassifier(s.T())
}

func (s *ClassifyCategorySuite) input(i intent.Intent) tools.ToolInput {
	return tools.ToolInput{UserID: categoriesTestUserID, Channel: "whatsapp", Intent: i}
}

func (s *ClassifyCategorySuite) TestExecute() {
	classifyIntent, _ := intent.NewClassifyCategory("padaria")
	type args struct {
		intent intent.Intent
	}
	type dependencies struct {
		classifier tools.CategoryClassifier
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result tools.ToolResult, err error)
	}{
		{
			name: "deve classificar em sucesso",
			args: args{intent: classifyIntent},
			dependencies: dependencies{
				classifier: func() *mocks.CategoryClassifier {
					s.classifierMock.EXPECT().
						Execute(s.ctx, tools.CategoryClassifyInput{Query: "padaria"}).
						Return(tools.CategoryClassifyResult{Matched: true, Slug: "expense.custo_fixo.alimentacao", Name: "Alimentação", Path: "Custo Fixo > Alimentação"}, nil).
						Once()
					return s.classifierMock
				}(),
			},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeRouted, result.Outcome)
				s.Equal(intent.KindClassifyCategory, result.Kind)
				s.Contains(result.Reply, "Alimentação")
			},
		},
		{
			name:         "deve pedir o item quando query e vazia",
			args:         args{intent: intent.Intent{}},
			dependencies: dependencies{classifier: s.classifierMock},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeClarify, result.Outcome)
				s.Equal(intent.KindClassifyCategory, result.Kind)
			},
		},
		{
			name: "deve clarificar quando nao ha match",
			args: args{intent: classifyIntent},
			dependencies: dependencies{
				classifier: func() *mocks.CategoryClassifier {
					s.classifierMock.EXPECT().
						Execute(s.ctx, tools.CategoryClassifyInput{Query: "padaria"}).
						Return(tools.CategoryClassifyResult{Matched: false}, nil).
						Once()
					return s.classifierMock
				}(),
			},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeClarify, result.Outcome)
				s.Equal(intent.KindClassifyCategory, result.Kind)
				s.Contains(result.Reply, "padaria")
			},
		},
		{
			name:         "deve retornar OutcomeMissingResolver quando classifier e nil",
			args:         args{intent: classifyIntent},
			dependencies: dependencies{classifier: nil},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeMissingResolver, result.Outcome)
				s.Equal(intent.KindClassifyCategory, result.Kind)
			},
		},
		{
			name: "deve retornar OutcomeUsecaseError quando classifier falha",
			args: args{intent: classifyIntent},
			dependencies: dependencies{
				classifier: func() *mocks.CategoryClassifier {
					s.classifierMock.EXPECT().
						Execute(s.ctx, mock.AnythingOfType("tools.CategoryClassifyInput")).
						Return(tools.CategoryClassifyResult{}, errors.New("falha no banco")).
						Once()
					return s.classifierMock
				}(),
			},
			expect: func(result tools.ToolResult, err error) {
				s.NoError(err)
				s.Equal(tools.OutcomeUsecaseError, result.Outcome)
				s.Equal(intent.KindClassifyCategory, result.Kind)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			tool := tools.NewClassifyCategory(newCategoriesRecorder(s.obs), scenario.dependencies.classifier, s.obs)
			result, err := tool.Execute(s.ctx, s.input(scenario.args.intent))
			scenario.expect(result, err)
		})
	}
}
