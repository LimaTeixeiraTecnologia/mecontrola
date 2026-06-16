package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ListCategoriesUseCase interface {
	Execute(ctx context.Context, in *categoriesinput.ListCategoriesInput) (*categoriesoutput.ListCategoriesOutput, error)
}

type GetCategoryUseCase interface {
	Execute(ctx context.Context, in *categoriesinput.GetCategoryInput) (*categoriesoutput.CategoryDetailOutput, error)
}

type ListDictionaryUseCase interface {
	Execute(ctx context.Context, in *categoriesinput.ListDictionaryInput) (*categoriesoutput.ListDictionaryOutput, error)
}

type SearchDictionaryUseCase interface {
	Execute(ctx context.Context, in *categoriesinput.SearchDictionaryInput) (*categoriesoutput.DictionarySearchOutput, error)
}

type CategoriesAdapter struct {
	listUseCase           ListCategoriesUseCase
	getUseCase            GetCategoryUseCase
	listDictionaryUseCase ListDictionaryUseCase
	searchUseCase         SearchDictionaryUseCase
	maxItems              int
}

func NewCategoriesAdapter(listUseCase ListCategoriesUseCase) *CategoriesAdapter {
	return &CategoriesAdapter{listUseCase: listUseCase, maxItems: 8}
}

func NewCategoriesAdapterFull(
	listUseCase ListCategoriesUseCase,
	getUseCase GetCategoryUseCase,
	listDictionaryUseCase ListDictionaryUseCase,
	searchUseCase SearchDictionaryUseCase,
) *CategoriesAdapter {
	return &CategoriesAdapter{
		listUseCase:           listUseCase,
		getUseCase:            getUseCase,
		listDictionaryUseCase: listDictionaryUseCase,
		searchUseCase:         searchUseCase,
		maxItems:              8,
	}
}

type categoriesListFilters struct {
	Kind              string `json:"kind"`
	ParentID          string `json:"parent_id"`
	IncludeDeprecated bool   `json:"include_deprecated"`
}

func (a *CategoriesAdapter) List(ctx context.Context, _ json.RawMessage) (string, error) {
	return a.list(ctx, nil)
}

func (a *CategoriesAdapter) list(ctx context.Context, rawFilters json.RawMessage) (string, error) {
	if a.listUseCase == nil {
		return "", fmt.Errorf("categories.list: %w", ErrIntentUnsupported)
	}
	in := &categoriesinput.ListCategoriesInput{IncludeDeprecated: false}
	if len(rawFilters) > 0 {
		var filters categoriesListFilters
		if err := json.Unmarshal(rawFilters, &filters); err == nil {
			if parsedKind, ok := parseCategoryKind(filters.Kind); ok {
				in.Kind = &parsedKind
			}
			if parentID, err := parseOptionalUUID(filters.ParentID); err == nil && parentID != nil {
				in.ParentID = parentID
			}
			in.IncludeDeprecated = filters.IncludeDeprecated
		}
	}
	result, err := a.listUseCase.Execute(ctx, in)
	if err != nil {
		return "", fmt.Errorf("categories.list: %w", err)
	}
	if result == nil || len(result.Categories) == 0 {
		return "Voce ainda nao tem categorias configuradas.", nil
	}

	names := make([]string, 0, len(result.Categories))
	for _, c := range result.Categories {
		if c.Name == "" {
			continue
		}
		names = append(names, c.Name)
	}
	total := len(names)
	limit := a.maxItems
	if limit <= 0 || limit > total {
		limit = total
	}

	preview := strings.Join(names[:limit], ", ")
	if total > limit {
		return fmt.Sprintf("Voce tem %d categorias. Algumas delas: %s.", total, preview), nil
	}
	return fmt.Sprintf("Voce tem %d categorias: %s.", total, preview), nil
}

type categoriesGetFilters struct {
	ID                string `json:"id"`
	IncludeDeprecated bool   `json:"include_deprecated"`
}

func (a *CategoriesAdapter) Get(ctx context.Context, rawFilters json.RawMessage) (string, error) {
	if a.getUseCase == nil {
		return "", fmt.Errorf("categories.get: %w", ErrIntentUnsupported)
	}
	var filters categoriesGetFilters
	if err := json.Unmarshal(rawFilters, &filters); err != nil || strings.TrimSpace(filters.ID) == "" {
		return "", fmt.Errorf("categories.get: filtro id ausente: %w", ErrIntentUnsupported)
	}
	id, err := uuid.Parse(strings.TrimSpace(filters.ID))
	if err != nil {
		return "", fmt.Errorf("categories.get: id invalido: %w", err)
	}
	out, err := a.getUseCase.Execute(ctx, &categoriesinput.GetCategoryInput{
		ID:                id,
		IncludeDeprecated: filters.IncludeDeprecated,
	})
	if err != nil {
		return "", fmt.Errorf("categories.get: %w", err)
	}
	if out == nil {
		return "Nao encontrei essa categoria.", nil
	}
	if len(out.Subcategories) > 0 {
		return fmt.Sprintf("Categoria %s (%s). Subcategorias: %s.",
			out.Name, out.Kind, joinCategoryChildren(out.Subcategories, a.maxItems),
		), nil
	}
	return fmt.Sprintf("Categoria %s (%s). Caminho: %s.", out.Name, out.Kind, out.Path), nil
}

type categoriesDictionaryFilters struct {
	CategoryID string `json:"category_id"`
	Kind       string `json:"kind"`
	SignalType string `json:"signal_type"`
	Cursor     string `json:"cursor"`
	PageSize   int    `json:"page_size"`
}

func (a *CategoriesAdapter) ListDictionary(ctx context.Context, rawFilters json.RawMessage) (string, error) {
	if a.listDictionaryUseCase == nil {
		return "", fmt.Errorf("categories.list_dictionary: %w", ErrIntentUnsupported)
	}
	var filters categoriesDictionaryFilters
	if len(rawFilters) > 0 {
		if err := json.Unmarshal(rawFilters, &filters); err != nil {
			return "", fmt.Errorf("categories.list_dictionary: filtros invalidos: %w", err)
		}
	}
	in := &categoriesinput.ListDictionaryInput{
		CategoryID: optionalTrimmedString(filters.CategoryID),
		Cursor:     strings.TrimSpace(filters.Cursor),
		PageSize:   filters.PageSize,
	}
	if parsedKind, ok := parseCategoryKind(filters.Kind); ok {
		in.Kind = &parsedKind
	}
	if parsedSignal, ok := parseSignalType(filters.SignalType); ok {
		in.SignalType = &parsedSignal
	}
	out, err := a.listDictionaryUseCase.Execute(ctx, in)
	if err != nil {
		return "", fmt.Errorf("categories.list_dictionary: %w", err)
	}
	if out == nil || len(out.Entries) == 0 {
		return "Nao encontrei entradas no dicionario para esse filtro.", nil
	}
	preview := make([]string, 0, min(len(out.Entries), a.maxItems))
	for idx := range min(len(out.Entries), a.maxItems) {
		entry := out.Entries[idx]
		preview = append(preview, fmt.Sprintf("%s (%s)", entry.Term, entry.SignalType))
	}
	return fmt.Sprintf("Encontrei %d entrada(s) no dicionario. Algumas: %s.",
		len(out.Entries), strings.Join(preview, ", "),
	), nil
}

type categoriesSearchFilters struct {
	Query string `json:"query"`
	Kind  string `json:"kind"`
}

func (a *CategoriesAdapter) Search(ctx context.Context, rawFilters json.RawMessage) (string, error) {
	if a.searchUseCase == nil {
		return "", fmt.Errorf("categories.search: %w", ErrIntentUnsupported)
	}
	var filters categoriesSearchFilters
	if err := json.Unmarshal(rawFilters, &filters); err != nil || strings.TrimSpace(filters.Query) == "" {
		return "", fmt.Errorf("categories.search: query ausente")
	}
	kind, ok := parseCategoryKind(filters.Kind)
	if !ok {
		kind = categoriesvo.KindExpense
	}
	out, err := a.searchUseCase.Execute(ctx, &categoriesinput.SearchDictionaryInput{
		Query: filters.Query,
		Kind:  kind,
	})
	if err != nil {
		return "", fmt.Errorf("categories.search: %w", err)
	}
	if out == nil || len(out.Candidates) == 0 {
		return fmt.Sprintf("Nao encontrei categoria para %q.", strings.TrimSpace(filters.Query)), nil
	}
	top := out.Candidates[0]
	return fmt.Sprintf("Melhor categoria para %q: %s.", strings.TrimSpace(filters.Query), top.Path), nil
}

func parseCategoryKind(raw string) (categoriesvo.Kind, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	kind, err := categoriesvo.ParseKind(trimmed)
	if err != nil {
		return 0, false
	}
	return kind, true
}

func parseSignalType(raw string) (categoriesvo.SignalType, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	signalType, err := categoriesvo.ParseSignalType(trimmed)
	if err != nil {
		return 0, false
	}
	return signalType, true
}

func parseOptionalUUID(raw string) (*uuid.UUID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func optionalTrimmedString(raw string) *string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func joinCategoryChildren(children []categoriesoutput.CategoryOutput, limit int) string {
	if len(children) == 0 {
		return "nenhuma"
	}
	maxItems := limit
	if maxItems <= 0 || maxItems > len(children) {
		maxItems = len(children)
	}
	parts := make([]string, 0, maxItems)
	for idx := range maxItems {
		parts = append(parts, children[idx].Name)
	}
	return strings.Join(parts, ", ")
}
