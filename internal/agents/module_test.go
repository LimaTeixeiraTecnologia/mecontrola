package agents

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

type fakeDB struct{}

func (f *fakeDB) PrepareContext(_ context.Context, _ string) (*sql.Stmt, error)  { return nil, nil }
func (f *fakeDB) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row { return nil }
func (f *fakeDB) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	return nil, nil
}
func (f *fakeDB) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, nil
}

func TestNewModule_RequiredDepsValidation(t *testing.T) {
	o11y := fake.NewProvider()
	validLLM := LLMConfig{APIKey: "key", Model: "openai/gpt-4o-mini"}

	scenarios := []struct {
		name    string
		deps    Deps
		wantErr string
	}{
		{
			name:    "db ausente",
			deps:    Deps{DB: nil, O11y: o11y, LLM: validLLM},
			wantErr: "agents.module: db is required",
		},
		{
			name:    "o11y ausente",
			deps:    Deps{DB: &fakeDB{}, O11y: nil, LLM: validLLM},
			wantErr: "agents.module: o11y is required",
		},
		{
			name:    "llm api_key ausente",
			deps:    Deps{DB: &fakeDB{}, O11y: o11y, LLM: LLMConfig{APIKey: ""}},
			wantErr: "agents.module: llm api_key is required",
		},
		{
			name: "deps validas constroem modulo sem erro",
			deps: Deps{
				DB:   &fakeDB{},
				O11y: o11y,
				LLM: LLMConfig{
					APIKey:  "key",
					Model:   "openai/gpt-4o-mini",
					BaseURL: "https://openrouter.ai",
				},
				CategoriesModule:   &categories.CategoriesModule{},
				CardModule:         card.CardModule{},
				BudgetsModule:      &budgets.BudgetsModule{},
				TransactionsModule: transactions.TransactionsModule{},
			},
			wantErr: "",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			_, err := NewModule(sc.deps)
			if sc.wantErr != "" {
				assert.ErrorContains(t, err, sc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
