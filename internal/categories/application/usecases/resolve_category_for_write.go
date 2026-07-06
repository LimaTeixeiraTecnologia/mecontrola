package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

var (
	ErrRootCategoryNotFound = errors.New("categories: root category not found")
	ErrSubcategoryNotFound  = errors.New("categories: subcategory not found")
	ErrRootWithoutLeaf      = errors.New("categories: root used as leaf; provide a direct subcategory")
	ErrLeafNotFromRoot      = errors.New("categories: subcategory does not belong to root")
	ErrCategoryDeprecated   = errors.New("categories: category or subcategory is deprecated")
	ErrKindMismatch         = errors.New("categories: kind does not match category")
	ErrVersionDrift         = errors.New("categories: editorial version changed")
)

type ResolveCategoryForWrite struct {
	repo    interfaces.CategoryRepository
	version interfaces.VersionReader
	o11y    observability.Observability
}

func NewResolveCategoryForWrite(repo interfaces.CategoryRepository, version interfaces.VersionReader, o11y observability.Observability) *ResolveCategoryForWrite {
	return &ResolveCategoryForWrite{repo: repo, version: version, o11y: o11y}
}

func (uc *ResolveCategoryForWrite) Execute(ctx context.Context, in *input.ResolveCategoryForWriteInput) (*output.ResolveCategoryForWriteOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "categories.usecase.resolve_for_write")
	defer span.End()

	if err := in.Validate(); err != nil {
		return nil, err
	}

	currentVersion, err := uc.version.Current(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("categories/resolve_for_write: ler versao: %w", err)
	}

	if currentVersion != in.ExpectedVersion {
		return nil, ErrVersionDrift
	}

	root, err := uc.repo.GetByID(ctx, in.RootCategoryID)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, interfaces.ErrNotFound) {
			return nil, ErrRootCategoryNotFound
		}
		return nil, fmt.Errorf("categories/resolve_for_write: buscar raiz: %w", err)
	}

	if err := validateRoot(root, in.Kind); err != nil {
		return nil, err
	}

	sub, err := uc.repo.GetByID(ctx, in.SubcategoryID)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, interfaces.ErrNotFound) {
			return nil, ErrSubcategoryNotFound
		}
		return nil, fmt.Errorf("categories/resolve_for_write: buscar subcategoria: %w", err)
	}

	if err := validateSub(sub, in.RootCategoryID, in.Kind); err != nil {
		return nil, err
	}

	return &output.ResolveCategoryForWriteOutput{
		RootCategoryID:   root.ID,
		SubcategoryID:    sub.ID,
		Kind:             in.Kind.String(),
		Path:             root.Name + " > " + sub.Name,
		RootSlug:         root.Slug,
		SubcategorySlug:  sub.Slug,
		CategoryName:     root.Name,
		SubcategoryName:  sub.Name,
		EditorialVersion: currentVersion,
		Deprecated:       false,
	}, nil
}

func validateRoot(root entities.Category, kind valueobjects.Kind) error {
	if !root.IsRoot() {
		return ErrRootWithoutLeaf
	}
	if root.Kind != kind {
		return ErrKindMismatch
	}
	if !root.IsActive() {
		return ErrCategoryDeprecated
	}
	return nil
}

func validateSub(sub entities.Category, rootID uuid.UUID, kind valueobjects.Kind) error {
	if sub.IsRoot() {
		return ErrRootWithoutLeaf
	}
	if sub.ParentID == nil || *sub.ParentID != rootID {
		return ErrLeafNotFromRoot
	}
	if sub.Kind != kind {
		return ErrKindMismatch
	}
	if !sub.IsActive() {
		return ErrCategoryDeprecated
	}
	return nil
}
