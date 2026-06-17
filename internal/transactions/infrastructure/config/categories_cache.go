package config

import (
	"context"
	"fmt"
	"maps"
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
	parentName string
	parentID   uuid.UUID
	cachedAt   time.Time
}

type CategoriesReader interface {
	ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error)
	ValidateSubcategory(ctx context.Context, id uuid.UUID) (interfaces.CategorySnapshot, error)
	EditorialVersion(ctx context.Context) (int64, error)
}

type CategoriesCache struct {
	reader      CategoriesReader
	roots       map[string]uuid.UUID
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

	merged := make(map[string]uuid.UUID, len(roots)+len(incomeRoots))
	maps.Copy(merged, roots)
	maps.Copy(merged, incomeRoots)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.roots = merged
	c.lastVersion = version

	return nil
}

func (c *CategoriesCache) RootID(slug string) (uuid.UUID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, ok := c.roots[slug]
	return id, ok
}

func (c *CategoriesCache) Validate(ctx context.Context, categoryID uuid.UUID, subcategoryID *uuid.UUID) (interfaces.CategorySnapshot, error) {
	if subcategoryID == nil {
		c.mu.Lock()
		roots := c.roots
		c.mu.Unlock()

		for _, rootID := range roots {
			if rootID == categoryID {
				return interfaces.CategorySnapshot{ID: categoryID}, nil
			}
		}
		return interfaces.CategorySnapshot{}, fmt.Errorf("transactions/categories_cache: categoria %s não encontrada: %w", categoryID, interfaces.ErrCategoryNotFound)
	}

	c.mu.Lock()
	entry, cached := c.subcache[*subcategoryID]
	lastVersion := c.lastVersion
	c.mu.Unlock()

	currentVersion, err := c.reader.EditorialVersion(ctx)
	if err != nil {
		return interfaces.CategorySnapshot{}, fmt.Errorf("transactions/categories_cache: ler versão editorial: %w", err)
	}

	if cached && currentVersion == lastVersion && time.Since(entry.cachedAt) < categoriesCacheTTL {
		return interfaces.CategorySnapshot{
			ID:         *subcategoryID,
			Name:       entry.name,
			ParentID:   &entry.parentID,
			ParentName: entry.parentName,
		}, nil
	}

	snapshot, err := c.reader.ValidateSubcategory(ctx, *subcategoryID)
	if err != nil {
		return interfaces.CategorySnapshot{}, err
	}

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
		parentName: snapshot.ParentName,
		parentID:   parentID,
		cachedAt:   time.Now().UTC(),
	}
	c.mu.Unlock()

	return snapshot, nil
}
