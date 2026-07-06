package categories

import (
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/repositories/postgres"
)

type CategoriesModule struct {
	CategoryRouter            *server.CategoryRouter
	GetCategoryUC             *usecases.GetCategory
	ListDictionaryUC          *usecases.ListDictionary
	ResolveBySlug             *usecases.ResolveBySlug
	ValidateSubcategory       *usecases.ValidateSubcategory
	VersionReader             interfaces.VersionReader
	ListCategoriesUC          *usecases.ListCategories
	SearchDictionaryUC        *usecases.SearchDictionary
	ResolveCategoryForWriteUC *usecases.ResolveCategoryForWrite
}

func NewCategoriesModule(db *sqlx.DB, o11y observability.Observability, gatewayAuth func(http.Handler) http.Handler) *CategoriesModule {
	categoryRepo := postgres.NewCategoryRepository(o11y, db)
	dictionaryRepo := postgres.NewDictionaryRepository(o11y, db)
	versionReader := postgres.NewVersionReader(o11y, db)
	resolver := services.NewCandidateResolver()
	collator := services.NewPTBRCollator()

	listCategories := usecases.NewListCategories(categoryRepo, versionReader, collator, o11y)
	getCategory := usecases.NewGetCategory(categoryRepo, versionReader, collator, o11y)
	listDictionary := usecases.NewListDictionary(dictionaryRepo, versionReader, o11y)
	searchDictionary := usecases.NewSearchDictionary(dictionaryRepo, categoryRepo, versionReader, resolver, o11y)
	resolveBySlug := usecases.NewResolveBySlug(categoryRepo, o11y)
	validateSubcategory := usecases.NewValidateSubcategory(categoryRepo, o11y)
	resolveCategoryForWrite := usecases.NewResolveCategoryForWrite(categoryRepo, versionReader, o11y)

	listCategoriesHandler := handlers.NewListCategoriesHandler(listCategories, versionReader, o11y)
	getCategoryHandler := handlers.NewGetCategoryHandler(getCategory, versionReader, o11y)
	listDictionaryHandler := handlers.NewListDictionaryHandler(listDictionary, versionReader, o11y)
	searchDictionaryHandler := handlers.NewSearchDictionaryHandler(searchDictionary, versionReader, o11y)

	categoryRouter := server.NewCategoryRouter(
		listCategoriesHandler,
		getCategoryHandler,
		listDictionaryHandler,
		searchDictionaryHandler,
		gatewayAuth,
	)

	return &CategoriesModule{
		CategoryRouter:            categoryRouter,
		GetCategoryUC:             getCategory,
		ListDictionaryUC:          listDictionary,
		ResolveBySlug:             resolveBySlug,
		ValidateSubcategory:       validateSubcategory,
		VersionReader:             versionReader,
		ListCategoriesUC:          listCategories,
		SearchDictionaryUC:        searchDictionary,
		ResolveCategoryForWriteUC: resolveCategoryForWrite,
	}
}
