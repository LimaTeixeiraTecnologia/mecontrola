package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type stubRegisterWriter struct {
	outcome agent.ToolOutcome
	execErr error
	called  int
}

func (w *stubRegisterWriter) Execute(ctx context.Context, _ uuid.UUID, _ string, _ int, _, _ string, write WriteFn) (IdempotentWriteResult, error) {
	w.called++
	if w.execErr != nil {
		return IdempotentWriteResult{}, w.execErr
	}
	id, _, err := write(ctx)
	if err != nil {
		return IdempotentWriteResult{}, err
	}
	return IdempotentWriteResult{ResourceID: id, Outcome: w.outcome}, nil
}

type RegisterEntrySuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	catMock    *imocks.CategoriesReader
	ledgerMock *imocks.TransactionsLedger
	userID     uuid.UUID
	rootID     uuid.UUID
	leafID     uuid.UUID
	resourceID uuid.UUID
}

func TestRegisterEntrySuite(t *testing.T) {
	suite.Run(t, new(RegisterEntrySuite))
}

func (s *RegisterEntrySuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.catMock = imocks.NewCategoriesReader(s.T())
	s.ledgerMock = imocks.NewTransactionsLedger(s.T())
	s.userID = uuid.New()
	s.rootID = uuid.New()
	s.leafID = uuid.New()
	s.resourceID = uuid.New()
}

func (s *RegisterEntrySuite) confidentResult() interfaces.CategorySearchResult {
	return interfaces.CategorySearchResult{
		Outcome: interfaces.ClassifyOutcomeMatched,
		Version: 1,
		Candidates: []interfaces.CategoryCandidate{
			{
				CategoryID:     s.leafID,
				RootCategoryID: s.rootID,
				Path:           "Alimentação > Restaurante",
				Score:          0.95,
				SignalType:     "alias",
				Confidence:     "high",
				MatchQuality:   "exact",
				MatchReason:    "alias match",
			},
		},
	}
}

func (s *RegisterEntrySuite) TestRegisterExpense() {
	type dependencies struct {
		catMock    *imocks.CategoriesReader
		ledgerMock *imocks.TransactionsLedger
		writer     *stubRegisterWriter
		captured   *interfaces.RawTransaction
	}

	scenarios := []struct {
		name         string
		dependencies func() dependencies
		expect       func(d dependencies, result RegisterResult, err error)
	}{
		{
			name: "deve classificar e gravar quando categoria confiável",
			dependencies: func() dependencies {
				captured := &interfaces.RawTransaction{}
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Almoço", "expense").
							Return(s.confidentResult(), nil).Once()
						return s.catMock
					}(),
					ledgerMock: func() *imocks.TransactionsLedger {
						s.ledgerMock.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
							RunAndReturn(func(_ context.Context, raw interfaces.RawTransaction) (interfaces.EntryRef, error) {
								*captured = raw
								return interfaces.EntryRef{ID: s.resourceID, Kind: interfaces.EntryKindTransaction}, nil
							}).Once()
						return s.ledgerMock
					}(),
					writer:   &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
					captured: captured,
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeRouted, result.Outcome)
				s.Equal(s.resourceID, result.ResourceID)
				s.Equal("transaction", result.Kind)
				s.Equal(1, d.writer.called)
				s.Equal("outcome", d.captured.Direction)
				s.Equal(s.rootID, d.captured.CategoryID)
				s.Require().NotNil(d.captured.SubcategoryID)
				s.Equal(s.leafID, *d.captured.SubcategoryID)
			},
		},
		{
			name: "deve retornar clarify quando categoria ambígua sem gravar",
			dependencies: func() dependencies {
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Almoço", "expense").
							Return(interfaces.CategorySearchResult{
								Outcome: interfaces.ClassifyOutcomeAmbiguous,
								Version: 1,
								Candidates: []interfaces.CategoryCandidate{
									{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Path: "A"},
									{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Path: "B"},
								},
							}, nil).Once()
						return s.catMock
					}(),
					ledgerMock: s.ledgerMock,
					writer:     &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeClarify, result.Outcome)
				s.Equal(uuid.Nil, result.ResourceID)
				s.Equal(0, d.writer.called)
			},
		},
		{
			name: "deve retornar clarify quando nenhum candidato sem gravar",
			dependencies: func() dependencies {
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Almoço", "expense").
							Return(interfaces.CategorySearchResult{Outcome: interfaces.ClassifyOutcomeNoMatch, Version: 1}, nil).Once()
						return s.catMock
					}(),
					ledgerMock: s.ledgerMock,
					writer:     &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeClarify, result.Outcome)
				s.Equal(0, d.writer.called)
			},
		},
		{
			name: "deve retornar erro quando version editorial ausente",
			dependencies: func() dependencies {
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Almoço", "expense").
							Return(interfaces.CategorySearchResult{Outcome: interfaces.ClassifyOutcomeMatched, Version: 0, Candidates: []interfaces.CategoryCandidate{{CategoryID: s.leafID, RootCategoryID: s.rootID}}}, nil).Once()
						return s.catMock
					}(),
					ledgerMock: s.ledgerMock,
					writer:     &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.Error(err)
				s.Equal(0, d.writer.called)
			},
		},
		{
			name: "deve propagar erro do dicionário de categorias",
			dependencies: func() dependencies {
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Almoço", "expense").
							Return(interfaces.CategorySearchResult{}, errors.New("dictionary down")).Once()
						return s.catMock
					}(),
					ledgerMock: s.ledgerMock,
					writer:     &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.Error(err)
				s.Equal(0, d.writer.called)
			},
		},
		{
			name: "deve propagar erro de infraestrutura na escrita",
			dependencies: func() dependencies {
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Almoço", "expense").
							Return(s.confidentResult(), nil).Once()
						return s.catMock
					}(),
					ledgerMock: func() *imocks.TransactionsLedger {
						s.ledgerMock.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
							Return(interfaces.EntryRef{}, errors.New("db error")).Once()
						return s.ledgerMock
					}(),
					writer: &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			d := scenario.dependencies()
			uc := NewRegisterEntry(d.catMock, d.ledgerMock, d.writer, s.obs)
			result, err := uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
				UserID:        s.userID,
				WAMID:         "wamid-exp",
				ItemSeq:       0,
				AmountCents:   5000,
				Description:   "Almoço",
				PaymentMethod: "debit_card",
				OccurredAt:    "2026-07-03",
			})
			scenario.expect(d, result, err)
		})
	}
}

func (s *RegisterEntrySuite) TestRegisterIncome() {
	type dependencies struct {
		catMock    *imocks.CategoriesReader
		ledgerMock *imocks.TransactionsLedger
		writer     *stubRegisterWriter
		captured   *interfaces.RawTransaction
	}

	scenarios := []struct {
		name         string
		dependencies func() dependencies
		expect       func(d dependencies, result RegisterResult, err error)
	}{
		{
			name: "deve classificar com kind income e gravar direção income",
			dependencies: func() dependencies {
				captured := &interfaces.RawTransaction{}
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Salário", "income").
							Return(s.confidentResult(), nil).Once()
						return s.catMock
					}(),
					ledgerMock: func() *imocks.TransactionsLedger {
						s.ledgerMock.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
							RunAndReturn(func(_ context.Context, raw interfaces.RawTransaction) (interfaces.EntryRef, error) {
								*captured = raw
								return interfaces.EntryRef{ID: s.resourceID, Kind: interfaces.EntryKindTransaction}, nil
							}).Once()
						return s.ledgerMock
					}(),
					writer:   &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
					captured: captured,
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeRouted, result.Outcome)
				s.Equal("income", d.captured.Direction)
				s.Equal("pix", d.captured.PaymentMethod)
				s.Equal(s.rootID, d.captured.CategoryID)
				s.Require().NotNil(d.captured.SubcategoryID)
				s.Equal(s.leafID, *d.captured.SubcategoryID)
			},
		},
		{
			name: "deve retornar clarify quando ambíguo sem gravar",
			dependencies: func() dependencies {
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Salário", "income").
							Return(interfaces.CategorySearchResult{
								Outcome: interfaces.ClassifyOutcomeMatched,
								Version: 1,
								Candidates: []interfaces.CategoryCandidate{
									{CategoryID: s.leafID, RootCategoryID: s.rootID, IsAmbiguous: true},
								},
							}, nil).Once()
						return s.catMock
					}(),
					ledgerMock: s.ledgerMock,
					writer:     &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeClarify, result.Outcome)
				s.Equal(0, d.writer.called)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			d := scenario.dependencies()
			uc := NewRegisterEntry(d.catMock, d.ledgerMock, d.writer, s.obs)
			result, err := uc.RegisterIncome(s.ctx, RegisterIncomeCommand{
				UserID:      s.userID,
				WAMID:       "wamid-inc",
				ItemSeq:     0,
				AmountCents: 100000,
				Description: "Salário",
				OccurredAt:  "2026-07-03",
			})
			scenario.expect(d, result, err)
		})
	}
}

func (s *RegisterEntrySuite) TestRegisterExpenseCreditCard() {
	type dependencies struct {
		catMock    *imocks.CategoriesReader
		ledgerMock *imocks.TransactionsLedger
		writer     *stubRegisterWriter
		captured   *interfaces.RawTransaction
	}

	cardID := uuid.New()

	scenarios := []struct {
		name         string
		installments int
		dependencies func() dependencies
		expect       func(d dependencies, result RegisterResult, err error)
	}{
		{
			name:         "deve rotear compra parcelada no crédito via CreateTransaction unificado",
			installments: 3,
			dependencies: func() dependencies {
				captured := &interfaces.RawTransaction{}
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Notebook", "expense").
							Return(s.confidentResult(), nil).Once()
						return s.catMock
					}(),
					ledgerMock: func() *imocks.TransactionsLedger {
						s.ledgerMock.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
							RunAndReturn(func(_ context.Context, raw interfaces.RawTransaction) (interfaces.EntryRef, error) {
								*captured = raw
								return interfaces.EntryRef{ID: s.resourceID, Kind: interfaces.EntryKindTransaction}, nil
							}).Once()
						return s.ledgerMock
					}(),
					writer:   &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
					captured: captured,
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeRouted, result.Outcome)
				s.Equal("transaction", result.Kind)
				s.Equal("outcome", d.captured.Direction)
				s.Equal("credit_card", d.captured.PaymentMethod)
				s.Require().NotNil(d.captured.CardID)
				s.Equal(cardID, *d.captured.CardID)
				s.Equal(3, d.captured.Installments)
				s.Equal(s.rootID, d.captured.CategoryID)
				s.Require().NotNil(d.captured.SubcategoryID)
				s.Equal(s.leafID, *d.captured.SubcategoryID)
			},
		},
		{
			name:         "deve rotear compra à vista no crédito com installments=1",
			installments: 1,
			dependencies: func() dependencies {
				captured := &interfaces.RawTransaction{}
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().SearchDictionary(mock.Anything, "Notebook", "expense").
							Return(s.confidentResult(), nil).Once()
						return s.catMock
					}(),
					ledgerMock: func() *imocks.TransactionsLedger {
						s.ledgerMock.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
							RunAndReturn(func(_ context.Context, raw interfaces.RawTransaction) (interfaces.EntryRef, error) {
								*captured = raw
								return interfaces.EntryRef{ID: s.resourceID, Kind: interfaces.EntryKindTransaction}, nil
							}).Once()
						return s.ledgerMock
					}(),
					writer:   &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
					captured: captured,
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeRouted, result.Outcome)
				s.Equal("credit_card", d.captured.PaymentMethod)
				s.Equal(1, d.captured.Installments)
				s.Require().NotNil(d.captured.CardID)
				s.Equal(cardID, *d.captured.CardID)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			d := scenario.dependencies()
			uc := NewRegisterEntry(d.catMock, d.ledgerMock, d.writer, s.obs)
			cid := cardID
			result, err := uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
				UserID:        s.userID,
				WAMID:         "wamid-credit",
				ItemSeq:       0,
				AmountCents:   300000,
				Description:   "Notebook",
				PaymentMethod: "credit_card",
				CardID:        &cid,
				Installments:  scenario.installments,
				OccurredAt:    "2026-07-03",
			})
			scenario.expect(d, result, err)
		})
	}
}

func (s *RegisterEntrySuite) TestClassifyBlocking() {
	type dependencies struct {
		catMock    *imocks.CategoriesReader
		ledgerMock *imocks.TransactionsLedger
		writer     *stubRegisterWriter
	}

	scenarios := []struct {
		name         string
		searchResult interfaces.CategorySearchResult
		expect       func(d dependencies, result RegisterResult, err error)
	}{
		{
			name: "candidato unico com outcome nao matched bloqueia",
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeAmbiguous,
				Version: 1,
				Candidates: []interfaces.CategoryCandidate{
					{CategoryID: s.leafID, RootCategoryID: s.rootID, Score: 0.7, Confidence: "low", MatchQuality: "fuzzy"},
				},
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeClarify, result.Outcome)
				s.Equal(0, d.writer.called)
			},
		},
		{
			name: "root igual a leaf bloqueia",
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeMatched,
				Version: 1,
				Candidates: []interfaces.CategoryCandidate{
					{CategoryID: s.rootID, RootCategoryID: s.rootID, Score: 0.9, Confidence: "high", MatchQuality: "exact"},
				},
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeClarify, result.Outcome)
				s.Equal(0, d.writer.called)
			},
		},
		{
			name: "evidencia incompleta sem confidence bloqueia",
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeMatched,
				Version: 1,
				Candidates: []interfaces.CategoryCandidate{
					{CategoryID: s.leafID, RootCategoryID: s.rootID, Score: 0.9, Confidence: "", MatchQuality: "exact"},
				},
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeClarify, result.Outcome)
				s.Equal(0, d.writer.called)
			},
		},
		{
			name: "evidencia incompleta sem match quality bloqueia",
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeMatched,
				Version: 1,
				Candidates: []interfaces.CategoryCandidate{
					{CategoryID: s.leafID, RootCategoryID: s.rootID, Score: 0.9, Confidence: "high", MatchQuality: ""},
				},
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeClarify, result.Outcome)
				s.Equal(0, d.writer.called)
			},
		},
		{
			name: "multi-candidato bloqueia mesmo com primeiro valido",
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeMatched,
				Version: 1,
				Candidates: []interfaces.CategoryCandidate{
					{CategoryID: s.leafID, RootCategoryID: s.rootID, Score: 0.9, Confidence: "high", MatchQuality: "exact"},
					{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Score: 0.8, Confidence: "medium", MatchQuality: "token"},
				},
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeClarify, result.Outcome)
				s.Equal(0, d.writer.called)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			d := dependencies{
				catMock: func() *imocks.CategoriesReader {
					s.catMock.EXPECT().SearchDictionary(mock.Anything, "Almoço", "expense").
						Return(scenario.searchResult, nil).Once()
					return s.catMock
				}(),
				ledgerMock: s.ledgerMock,
				writer:     &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
			}
			uc := NewRegisterEntry(d.catMock, d.ledgerMock, d.writer, s.obs)
			result, err := uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
				UserID:        s.userID,
				WAMID:         "wamid-block",
				ItemSeq:       0,
				AmountCents:   5000,
				Description:   "Almoço",
				PaymentMethod: "debit_card",
				OccurredAt:    "2026-07-06",
			})
			scenario.expect(d, result, err)
		})
	}
}

func (s *RegisterEntrySuite) TestRegisterExpenseExplicitCategory() {
	type dependencies struct {
		catMock    *imocks.CategoriesReader
		ledgerMock *imocks.TransactionsLedger
		writer     *stubRegisterWriter
		captured   *interfaces.RawTransaction
	}

	scenarios := []struct {
		name         string
		dependencies func() dependencies
		expect       func(d dependencies, result RegisterResult, err error)
	}{
		{
			name: "deve gravar via ResolveForWrite com subcategoria explícita sem consultar dicionário",
			dependencies: func() dependencies {
				captured := &interfaces.RawTransaction{}
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().ResolveForWrite(mock.Anything, interfaces.CategoryWriteRequest{
							RootCategoryID:  s.rootID,
							SubcategoryID:   s.leafID,
							Kind:            interfaces.CategoryKindExpense,
							ExpectedVersion: 7,
						}).Return(interfaces.CategoryWriteDecision{
							RootCategoryID:   s.rootID,
							SubcategoryID:    s.leafID,
							Kind:             interfaces.CategoryKindExpense,
							Path:             "Custo Fixo > Supermercado",
							EditorialVersion: 7,
						}, nil).Once()
						return s.catMock
					}(),
					ledgerMock: func() *imocks.TransactionsLedger {
						s.ledgerMock.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
							RunAndReturn(func(_ context.Context, raw interfaces.RawTransaction) (interfaces.EntryRef, error) {
								*captured = raw
								return interfaces.EntryRef{ID: s.resourceID, Kind: interfaces.EntryKindTransaction}, nil
							}).Once()
						return s.ledgerMock
					}(),
					writer:   &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
					captured: captured,
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeRouted, result.Outcome)
				s.Equal(s.resourceID, result.ResourceID)
				s.Equal(1, d.writer.called)
				s.Equal(s.rootID, d.captured.CategoryID)
				s.Require().NotNil(d.captured.SubcategoryID)
				s.Equal(s.leafID, *d.captured.SubcategoryID)
				s.Equal("user_selected_candidate", d.captured.CategorySource)
				s.Equal("matched", d.captured.CategoryOutcome)
				s.Equal("high", d.captured.CategoryConfidence)
				s.Equal("exact", d.captured.CategoryQuality)
				s.Equal("canonical_name", d.captured.CategorySignalType)
				s.Equal(int64(7), d.captured.CategoryVersion)
			},
		},
		{
			name: "deve retornar clarify quando ResolveForWrite falha sem gravar",
			dependencies: func() dependencies {
				return dependencies{
					catMock: func() *imocks.CategoriesReader {
						s.catMock.EXPECT().ResolveForWrite(mock.Anything, mock.AnythingOfType("interfaces.CategoryWriteRequest")).
							Return(interfaces.CategoryWriteDecision{}, errors.New("version drift")).Once()
						return s.catMock
					}(),
					ledgerMock: s.ledgerMock,
					writer:     &stubRegisterWriter{outcome: agent.ToolOutcomeRouted},
				}
			},
			expect: func(d dependencies, result RegisterResult, err error) {
				s.NoError(err)
				s.Equal(agent.ToolOutcomeClarify, result.Outcome)
				s.Equal(0, d.writer.called)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			d := scenario.dependencies()
			uc := NewRegisterEntry(d.catMock, d.ledgerMock, d.writer, s.obs)
			result, err := uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
				UserID:          s.userID,
				WAMID:           "wamid-explicit",
				ItemSeq:         0,
				AmountCents:     15000,
				Description:     "mercado",
				PaymentMethod:   "pix",
				OccurredAt:      "2026-07-06",
				CategoryID:      s.rootID,
				SubcategoryID:   s.leafID,
				CategoryVersion: 7,
			})
			scenario.expect(d, result, err)
		})
	}
}
