package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
)

const categoriesCacheTTL = 60 * time.Second

var OfficialRootSlugs = []string{
	"expense.custo_fixo",
	"expense.conhecimento",
	"expense.prazeres",
	"expense.metas",
	"expense.liberdade_financeira",
}

var OfficialIncomeRootSlugs = []string{
	"income.salario",
	"income.renda_variavel",
	"income.investimentos",
	"income.aluguel_recebido",
	"income.restituicoes_e_cashback",
	"income.presentes_recebidos",
	"income.vendas",
	"income.outras_receitas",
}

type subcategoryEntry struct {
	name       string
	kind       string
	parentName string
	parentID   uuid.UUID
	cachedAt   time.Time
}

type rootEntry struct {
	id   uuid.UUID
	kind string
}

type CategoriesReader interface {
	ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error)
	ValidateSubcategory(ctx context.Context, id uuid.UUID, expectedParentID uuid.UUID) (interfaces.CategorySnapshot, error)
	EditorialVersion(ctx context.Context) (int64, error)
}

type CategoriesCache struct {
	reader      CategoriesReader
	roots       map[string]rootEntry
	mu          sync.Mutex
	subcache    map[uuid.UUID]subcategoryEntry
	lastVersion int64
}

func NewCategoriesCache(reader CategoriesReader) *CategoriesCache {
	return &CategoriesCache{
		reader:   reader,
		subcache: make(map[uuid.UUID]subcategoryEntry),
	}
}

func (c *CategoriesCache) Boot(ctx context.Context) error {
	roots, err := c.reader.ResolveRootsBySlug(ctx, OfficialRootSlugs)
	if err != nil {
		return fmt.Errorf("transactions/categories_cache: resolver raízes no boot: %w", err)
	}

	if len(roots) != len(OfficialRootSlugs) {
		return fmt.Errorf("transactions/categories_cache: esperado %d raízes, obtidas %d", len(OfficialRootSlugs), len(roots))
	}

	incomeRoots, err := c.reader.ResolveRootsBySlug(ctx, OfficialIncomeRootSlugs)
	if err != nil {
		return fmt.Errorf("transactions/categories_cache: resolver raízes de receita no boot: %w", err)
	}

	if len(incomeRoots) != len(OfficialIncomeRootSlugs) {
		return fmt.Errorf("transactions/categories_cache: esperado %d raízes de receita, obtidas %d", len(OfficialIncomeRootSlugs), len(incomeRoots))
	}

	version, err := c.reader.EditorialVersion(ctx)
	if err != nil {
		return fmt.Errorf("transactions/categories_cache: ler versão editorial no boot: %w", err)
	}

	merged := make(map[string]rootEntry, len(roots)+len(incomeRoots))
	for slug, id := range roots {
		merged[slug] = rootEntry{id: id, kind: "expense"}
	}
	for slug, id := range incomeRoots {
		merged[slug] = rootEntry{id: id, kind: "income"}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.roots = merged
	c.lastVersion = version

	return nil
}

func (c *CategoriesCache) RootID(slug string) (uuid.UUID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.roots[slug]
	return entry.id, ok
}

func (c *CategoriesCache) Validate(ctx context.Context, categoryID uuid.UUID, subcategoryID *uuid.UUID) (interfaces.CategorySnapshot, error) {
	rootKind, isRoot := c.rootKind(categoryID)
	if !isRoot {
		return interfaces.CategorySnapshot{}, fmt.Errorf("transactions/categories_cache: categoria %s não é raiz: %w", categoryID, interfaces.ErrCategoryNotFound)
	}

	if subcategoryID == nil {
		return interfaces.CategorySnapshot{ID: categoryID, Kind: rootKind}, nil
	}

	c.mu.Lock()
	entry, cached := c.subcache[*subcategoryID]
	lastVersion := c.lastVersion
	c.mu.Unlock()

	currentVersion, err := c.reader.EditorialVersion(ctx)
	if err != nil {
		return interfaces.CategorySnapshot{}, fmt.Errorf("transactions/categories_cache: ler versão editorial: %w", err)
	}

	if cached && entry.parentID == categoryID && currentVersion == lastVersion && time.Since(entry.cachedAt) < categoriesCacheTTL {
		return interfaces.CategorySnapshot{
			ID:         *subcategoryID,
			Name:       entry.name,
			Kind:       entry.kind,
			ParentID:   &entry.parentID,
			ParentName: entry.parentName,
		}, nil
	}

	snapshot, err := c.reader.ValidateSubcategory(ctx, *subcategoryID, categoryID)
	if err != nil {
		return interfaces.CategorySnapshot{}, err
	}
	snapshot.Kind = rootKind

	c.mu.Lock()
	if currentVersion != c.lastVersion {
		c.subcache = make(map[uuid.UUID]subcategoryEntry)
		c.lastVersion = currentVersion
	}
	parentID := uuid.Nil
	if snapshot.ParentID != nil {
		parentID = *snapshot.ParentID
	}
	c.subcache[*subcategoryID] = subcategoryEntry{
		name:       snapshot.Name,
		kind:       snapshot.Kind,
		parentName: snapshot.ParentName,
		parentID:   parentID,
		cachedAt:   time.Now().UTC(),
	}
	c.mu.Unlock()

	return snapshot, nil
}

func (c *CategoriesCache) rootKind(categoryID uuid.UUID) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, entry := range c.roots {
		if entry.id == categoryID {
			return entry.kind, true
		}
	}
	return "", false
}
