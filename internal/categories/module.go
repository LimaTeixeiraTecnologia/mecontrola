package categories

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/repositories/postgres"
)

type CategoriesModule struct {
	CategoryRouter      *server.CategoryRouter
	ResolveBySlug       *usecases.ResolveBySlug
	ValidateSubcategory *usecases.ValidateSubcategory
	VersionReader       interfaces.VersionReader
}

func NewCategoriesModule(mgr manager.Manager, o11y observability.Observability) *CategoriesModule {
	db := mgr.DBTX(context.Background())

	categoryRepo := postgres.NewCategoryRepository(o11y, db)
	dictionaryRepo := postgres.NewDictionaryRepository(o11y, db)
	versionReader := postgres.NewVersionReader(o11y, db)
	resolver := services.NewCandidateResolver()

	listCategories := usecases.NewListCategories(categoryRepo, versionReader, o11y)
	getCategory := usecases.NewGetCategory(categoryRepo, versionReader, o11y)
	listDictionary := usecases.NewListDictionary(dictionaryRepo, versionReader, o11y)
	searchDictionary := usecases.NewSearchDictionary(dictionaryRepo, categoryRepo, versionReader, resolver, o11y)
	resolveBySlug := usecases.NewResolveBySlug(categoryRepo, o11y)
	validateSubcategory := usecases.NewValidateSubcategory(categoryRepo, o11y)

	listCategoriesHandler := handlers.NewListCategoriesHandler(listCategories, versionReader, o11y)
	getCategoryHandler := handlers.NewGetCategoryHandler(getCategory, versionReader, o11y)
	listDictionaryHandler := handlers.NewListDictionaryHandler(listDictionary, versionReader, o11y)
	searchDictionaryHandler := handlers.NewSearchDictionaryHandler(searchDictionary, versionReader, o11y)

	categoryRouter := server.NewCategoryRouter(
		listCategoriesHandler,
		getCategoryHandler,
		listDictionaryHandler,
		searchDictionaryHandler,
	)

	return &CategoriesModule{
		CategoryRouter:      categoryRouter,
		ResolveBySlug:       resolveBySlug,
		ValidateSubcategory: validateSubcategory,
		VersionReader:       versionReader,
	}
}
