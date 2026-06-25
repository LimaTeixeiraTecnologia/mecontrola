package loader_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/loader"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
)

type stubListCategories struct {
	result *categoriesoutput.ListCategoriesOutput
	err    error
}

func (s *stubListCategories) Execute(_ context.Context, _ *categoriesinput.ListCategoriesInput) (*categoriesoutput.ListCategoriesOutput, error) {
	return s.result, s.err
}

type stubListCards struct {
	result cardoutput.CardList
	err    error
}

func (s *stubListCards) Execute(_ context.Context, _ cardinput.ListCards) (cardoutput.CardList, error) {
	return s.result, s.err
}

func TestPromptContextLoader_HappyPath(t *testing.T) {
	categoriesUC := &stubListCategories{
		result: &categoriesoutput.ListCategoriesOutput{
			Categories: []categoriesoutput.CategoryTreeOutput{
				{ID: uuid.New(), Name: "Alimentacao"},
				{ID: uuid.New(), Name: "Transporte"},
			},
		},
	}
	cardsUC := &stubListCards{
		result: cardoutput.CardList{
			Items: []cardoutput.Card{
				{ID: uuid.New().String(), Nickname: "Nubank"},
				{ID: uuid.New().String(), Name: "Inter"},
			},
		},
	}

	sut := loader.NewPromptContextLoader(categoriesUC, cardsUC, noop.NewProvider())
	seed, err := sut.Load(context.Background(), uuid.New(), "whatsapp")

	require.NoError(t, err)
	assert.Len(t, seed.Categories, 2)
	assert.Equal(t, "Alimentacao", seed.Categories[0].Name)
	assert.Len(t, seed.Cards, 2)
	assert.Equal(t, "Nubank", seed.Cards[0].Nickname)
	assert.Equal(t, "Inter", seed.Cards[1].Nickname)
	assert.Equal(t, []string{"read", "write"}, seed.Permissions)
}

func TestPromptContextLoader_DropsEmptyCategoryName(t *testing.T) {
	categoriesUC := &stubListCategories{
		result: &categoriesoutput.ListCategoriesOutput{
			Categories: []categoriesoutput.CategoryTreeOutput{
				{ID: uuid.New(), Name: ""},
				{ID: uuid.New(), Name: "Lazer"},
			},
		},
	}
	cardsUC := &stubListCards{result: cardoutput.CardList{}}

	sut := loader.NewPromptContextLoader(categoriesUC, cardsUC, noop.NewProvider())
	seed, err := sut.Load(context.Background(), uuid.New(), "whatsapp")

	require.NoError(t, err)
	assert.Len(t, seed.Categories, 1)
	assert.Equal(t, "Lazer", seed.Categories[0].Name)
}

func TestPromptContextLoader_DegradesOnCategoriesError(t *testing.T) {
	categoriesUC := &stubListCategories{err: errors.New("db down")}
	cardsUC := &stubListCards{result: cardoutput.CardList{Items: []cardoutput.Card{{ID: "a", Nickname: "n"}}}}

	sut := loader.NewPromptContextLoader(categoriesUC, cardsUC, noop.NewProvider())
	seed, err := sut.Load(context.Background(), uuid.New(), "whatsapp")

	require.NoError(t, err)
	assert.Empty(t, seed.Categories)
	assert.Len(t, seed.Cards, 1)
}

func TestPromptContextLoader_DegradesOnCardsError(t *testing.T) {
	categoriesUC := &stubListCategories{
		result: &categoriesoutput.ListCategoriesOutput{
			Categories: []categoriesoutput.CategoryTreeOutput{{ID: uuid.New(), Name: "Mercado"}},
		},
	}
	cardsUC := &stubListCards{err: errors.New("db down")}

	sut := loader.NewPromptContextLoader(categoriesUC, cardsUC, noop.NewProvider())
	seed, err := sut.Load(context.Background(), uuid.New(), "whatsapp")

	require.NoError(t, err)
	assert.Len(t, seed.Categories, 1)
	assert.Empty(t, seed.Cards)
}

func TestPromptContextLoader_NilUseCasesReturnEmpty(t *testing.T) {
	sut := loader.NewPromptContextLoader(nil, nil, noop.NewProvider())
	seed, err := sut.Load(context.Background(), uuid.New(), "whatsapp")

	require.NoError(t, err)
	assert.Empty(t, seed.Categories)
	assert.Empty(t, seed.Cards)
	assert.Equal(t, []string{"read", "write"}, seed.Permissions)
}

func TestPromptContextLoader_FallsBackToNameWhenNicknameEmpty(t *testing.T) {
	cardsUC := &stubListCards{
		result: cardoutput.CardList{
			Items: []cardoutput.Card{{ID: "a", Name: "Banco do Brasil"}},
		},
	}

	sut := loader.NewPromptContextLoader(nil, cardsUC, noop.NewProvider())
	seed, err := sut.Load(context.Background(), uuid.New(), "whatsapp")

	require.NoError(t, err)
	require.Len(t, seed.Cards, 1)
	assert.Equal(t, "Banco do Brasil", seed.Cards[0].Nickname)
}
