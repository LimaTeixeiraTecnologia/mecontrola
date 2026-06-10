package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
)

const categoriesCacheTTL = 60 * time.Second

var OfficialRootSlugs = []string{
	"expense.custo_fixo",
	"expense.conhecimento",
	"expense.prazeres",
	"expense.metas",
	"expense.liberdade_financeira",
}

type subcategoryEntry struct {
	rootSlug   string
	deprecated bool
	cachedAt   time.Time
}

type CategoriesCache struct {
	reader      interfaces.CategoriesReader
	roots       map[string]uuid.UUID
	mu          sync.Mutex
	subcache    map[uuid.UUID]subcategoryEntry
	lastVersion int64
}

func NewCategoriesCache(reader interfaces.CategoriesReader) *CategoriesCache {
	return &CategoriesCache{
		reader:   reader,
		subcache: make(map[uuid.UUID]subcategoryEntry),
	}
}

func (c *CategoriesCache) Boot(ctx context.Context) error {
	roots, err := c.reader.ResolveRootsBySlug(ctx, OfficialRootSlugs)
	if err != nil {
		return fmt.Errorf("budgets/categories_cache: resolver raízes no boot: %w", err)
	}

	if len(roots) != len(OfficialRootSlugs) {
		return fmt.Errorf("budgets/categories_cache: esperado %d raízes, obtidas %d", len(OfficialRootSlugs), len(roots))
	}

	version, err := c.reader.EditorialVersion(ctx)
	if err != nil {
		return fmt.Errorf("budgets/categories_cache: ler versão editorial no boot: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.roots = roots
	c.lastVersion = version

	return nil
}

func (c *CategoriesCache) RootID(slug string) (uuid.UUID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, ok := c.roots[slug]
	return id, ok
}

func (c *CategoriesCache) ValidateExpenseSubcategory(ctx context.Context, id uuid.UUID) (string, bool, error) {
	c.mu.Lock()
	entry, cached := c.subcache[id]
	lastVersion := c.lastVersion
	c.mu.Unlock()

	currentVersion, err := c.reader.EditorialVersion(ctx)
	if err != nil {
		return "", false, fmt.Errorf("budgets/categories_cache: ler versão editorial: %w", err)
	}

	if cached && currentVersion == lastVersion && time.Since(entry.cachedAt) < categoriesCacheTTL {
		return entry.rootSlug, entry.deprecated, nil
	}

	rootSlug, deprecated, err := c.reader.ValidateExpenseSubcategory(ctx, id)
	if err != nil {
		return "", false, err
	}

	c.mu.Lock()
	if currentVersion != c.lastVersion {
		c.subcache = make(map[uuid.UUID]subcategoryEntry)
		c.lastVersion = currentVersion
	}
	c.subcache[id] = subcategoryEntry{
		rootSlug:   rootSlug,
		deprecated: deprecated,
		cachedAt:   time.Now().UTC(),
	}
	c.mu.Unlock()

	return rootSlug, deprecated, nil
}

func (c *CategoriesCache) EditorialVersion(ctx context.Context) (int64, error) {
	return c.reader.EditorialVersion(ctx)
}

func (c *CategoriesCache) ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error) {
	c.mu.Lock()
	roots := c.roots
	c.mu.Unlock()

	result := make(map[string]uuid.UUID, len(slugs))
	for _, slug := range slugs {
		id, ok := roots[slug]
		if !ok {
			return nil, fmt.Errorf("budgets/categories_cache: slug %q não encontrado nas raízes em cache: %w", slug, interfaces.ErrCategoriesReaderUnavailable)
		}
		result[slug] = id
	}

	return result, nil
}
