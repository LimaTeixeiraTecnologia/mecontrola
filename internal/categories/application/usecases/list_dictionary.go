package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

type ListDictionary struct {
	repo    interfaces.DictionaryRepository
	version interfaces.VersionReader
	o11y    observability.Observability
}

func NewListDictionary(repo interfaces.DictionaryRepository, version interfaces.VersionReader, o11y observability.Observability) *ListDictionary {
	return &ListDictionary{repo: repo, version: version, o11y: o11y}
}

func (uc *ListDictionary) Execute(ctx context.Context, in *input.ListDictionaryInput) (*output.ListDictionaryOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "categories.usecase.list_dictionary")
	defer span.End()

	if err := in.Validate(); err != nil {
		return nil, err
	}

	version, err := uc.version.Current(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("ler versao: %w", err)
	}

	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	query := interfaces.DictionaryQuery{
		CategoryID: in.CategoryID,
		Cursor:     in.Cursor,
		PageSize:   pageSize,
	}
	if in.Kind != nil {
		query.Kind = in.Kind
	}
	if in.SignalType != nil {
		query.SignalType = in.SignalType
	}

	entries, nextCursor, err := uc.repo.List(ctx, query)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("listar dicionario: %w", err)
	}

	outputs := make([]output.DictionaryEntryOutput, 0, len(entries))
	for _, e := range entries {
		outputs = append(outputs, output.NewDictionaryEntryOutputFromEntity(e))
	}

	return &output.ListDictionaryOutput{
		Entries:    outputs,
		NextCursor: nextCursor,
		Version:    version,
	}, nil
}
