package categories

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/repositories/postgres"
)

type CategoriesModule struct {
	CategoryRouter *server.CategoryRouter
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

	listCategoriesHandler := handlers.NewListCategoriesHandler(listCategories, o11y)
	getCategoryHandler := handlers.NewGetCategoryHandler(getCategory, o11y)
	listDictionaryHandler := handlers.NewListDictionaryHandler(listDictionary, o11y)
	searchDictionaryHandler := handlers.NewSearchDictionaryHandler(searchDictionary, o11y)

	categoryRouter := server.NewCategoryRouter(
		listCategoriesHandler,
		getCategoryHandler,
		listDictionaryHandler,
		searchDictionaryHandler,
	)

	return &CategoriesModule{
		CategoryRouter: categoryRouter,
	}
}
