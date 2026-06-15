package dispatcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/dispatcher"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
)

type stubListCategoriesUC struct {
	called bool
	resp   *categoriesoutput.ListCategoriesOutput
	err    error
}

func (s *stubListCategoriesUC) Execute(_ context.Context, _ *categoriesinput.ListCategoriesInput) (*categoriesoutput.ListCategoriesOutput, error) {
	s.called = true
	return s.resp, s.err
}

func TestCategoriesAdapter_List_NoCategories(t *testing.T) {
	uc := &stubListCategoriesUC{resp: &categoriesoutput.ListCategoriesOutput{}}
	sut := dispatcher.NewCategoriesAdapter(uc)

	reply, err := sut.List(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.True(t, uc.called)
	assert.Contains(t, reply, "ainda nao tem categorias")
}

func TestCategoriesAdapter_List_FewCategories(t *testing.T) {
	uc := &stubListCategoriesUC{resp: &categoriesoutput.ListCategoriesOutput{
		Categories: []categoriesoutput.CategoryTreeOutput{
			{ID: uuid.New(), Name: "Alimentacao"},
			{ID: uuid.New(), Name: "Transporte"},
			{ID: uuid.New(), Name: "Lazer"},
		},
	}}
	sut := dispatcher.NewCategoriesAdapter(uc)

	reply, err := sut.List(context.Background(), nil)
	require.NoError(t, err)
	assert.Contains(t, reply, "Alimentacao")
	assert.Contains(t, reply, "Transporte")
	assert.Contains(t, reply, "Lazer")
	assert.Contains(t, reply, "3 categorias")
}

func TestCategoriesAdapter_List_MoreThanLimit_TruncatesPreview(t *testing.T) {
	categories := make([]categoriesoutput.CategoryTreeOutput, 0, 15)
	for i := range 15 {
		categories = append(categories, categoriesoutput.CategoryTreeOutput{
			ID:   uuid.New(),
			Name: "cat-" + string(rune('A'+i)),
		})
	}
	uc := &stubListCategoriesUC{resp: &categoriesoutput.ListCategoriesOutput{Categories: categories}}
	sut := dispatcher.NewCategoriesAdapter(uc)

	reply, err := sut.List(context.Background(), nil)
	require.NoError(t, err)
	assert.Contains(t, reply, "15 categorias")
	assert.Contains(t, reply, "Algumas delas", "deve usar texto truncado quando excede limite")
}

func TestCategoriesAdapter_List_DropsCategoryWithEmptyName(t *testing.T) {
	uc := &stubListCategoriesUC{resp: &categoriesoutput.ListCategoriesOutput{
		Categories: []categoriesoutput.CategoryTreeOutput{
			{ID: uuid.New(), Name: ""},
			{ID: uuid.New(), Name: "Saude"},
		},
	}}
	sut := dispatcher.NewCategoriesAdapter(uc)

	reply, err := sut.List(context.Background(), nil)
	require.NoError(t, err)
	assert.NotContains(t, reply, ",  ,", "não deve emitir vírgulas separando nome vazio")
	assert.Contains(t, reply, "Saude")
}

func TestCategoriesAdapter_List_UseCaseError_Propagates(t *testing.T) {
	uc := &stubListCategoriesUC{err: errors.New("db down")}
	sut := dispatcher.NewCategoriesAdapter(uc)

	_, err := sut.List(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "categories.list")
}

func TestCategoriesAdapter_List_NilOutput_HandledAsEmpty(t *testing.T) {
	uc := &stubListCategoriesUC{resp: nil}
	sut := dispatcher.NewCategoriesAdapter(uc)

	reply, err := sut.List(context.Background(), nil)
	require.NoError(t, err)
	assert.Contains(t, reply, "ainda nao tem categorias")
}
