package workflows

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
)

var (
	testRootCustoFixoID  = uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62")
	testRootMetasID      = uuid.MustParse("f133508e-7dc3-58a3-96db-199d8fbd2987")
	testRootPrazeresID   = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	testLeafCombustivel  = uuid.MustParse("cb13d50d-43cb-553c-99cd-8851889d7f6e")
	testLeafSupermercado = uuid.MustParse("97fa4b86-d43c-5ad5-a99b-c88c8427fb30")
	testLeafHortifruti   = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	testLeafVeiculo      = uuid.MustParse("ef1a26ec-e12d-5b3c-b7ba-3634bb89647c")
	testLeafCinema       = uuid.MustParse("33333333-3333-4333-8333-333333333333")
)

func testCategoryCatalog() []CategoryCatalogEntry {
	return []CategoryCatalogEntry{
		{RootID: testRootCustoFixoID, RootName: "Custo Fixo", RootSlug: "custo-fixo", LeafID: testLeafCombustivel, LeafName: "Combustível", LeafSlug: "combustivel"},
		{RootID: testRootCustoFixoID, RootName: "Custo Fixo", RootSlug: "custo-fixo", LeafID: testLeafSupermercado, LeafName: "Supermercado", LeafSlug: "supermercado"},
		{RootID: testRootCustoFixoID, RootName: "Custo Fixo", RootSlug: "custo-fixo", LeafID: testLeafHortifruti, LeafName: "Feira e Hortifruti", LeafSlug: "feira-e-hortifruti"},
		{RootID: testRootMetasID, RootName: "Metas", RootSlug: "metas", LeafID: testLeafVeiculo, LeafName: "Veículo", LeafSlug: "veiculo"},
		{RootID: testRootPrazeresID, RootName: "Prazeres", RootSlug: "prazeres", LeafID: testLeafCinema, LeafName: "Cinema", LeafSlug: "cinema"},
	}
}

func TestDecideUserCategoryText_MatchedLeafByPath(t *testing.T) {
	scenarios := []string{
		"Custo Fixo > Combustível",
		"custo fixo > combustivel",
		"custos fixos > combustível",
		"custo fixo e combustivel",
		"combustível",
		"combustivel",
	}
	for _, text := range scenarios {
		match := DecideUserCategoryText(testCategoryCatalog(), text)
		require.Equal(t, UserCategoryActionMatchedLeaf, match.Action, "texto %q", text)
		assert.Equal(t, testLeafCombustivel, match.Leaf.LeafID, "texto %q", text)
	}
}

func TestDecideUserCategoryText_MatchedRootSingularPlural(t *testing.T) {
	scenarios := []string{"custo fixo", "custos fixos", "Custo Fixo", "CUSTOS FIXOS", "custo-fixo"}
	for _, text := range scenarios {
		match := DecideUserCategoryText(testCategoryCatalog(), text)
		require.Equal(t, UserCategoryActionMatchedRoot, match.Action, "texto %q", text)
		assert.Equal(t, testRootCustoFixoID, match.RootID, "texto %q", text)
		assert.Len(t, match.Leaves, 3, "texto %q", text)
	}
}

func TestDecideUserCategoryText_RootWithForeignLeafFallsToRoot(t *testing.T) {
	match := DecideUserCategoryText(testCategoryCatalog(), "custo fixo e veículos")
	require.Equal(t, UserCategoryActionMatchedRoot, match.Action)
	assert.Equal(t, testRootCustoFixoID, match.RootID)
	assert.Len(t, match.Leaves, 3)
}

func TestDecideUserCategoryText_LeafOfNamedRoot(t *testing.T) {
	match := DecideUserCategoryText(testCategoryCatalog(), "custos fixos e supermercado")
	require.Equal(t, UserCategoryActionMatchedLeaf, match.Action)
	assert.Equal(t, testLeafSupermercado, match.Leaf.LeafID)
}

func TestDecideUserCategoryText_NoMatch(t *testing.T) {
	for _, text := range []string{"", "xyz", "categoria desconhecida"} {
		match := DecideUserCategoryText(testCategoryCatalog(), text)
		assert.Equal(t, UserCategoryActionNoMatch, match.Action, "texto %q", text)
	}
}

func TestNormalizeCategoryTerm_SingularizaCanonicos(t *testing.T) {
	scenarios := []struct {
		a string
		b string
	}{
		{"Custos Fixos", "custo fixo"},
		{"Prazeres", "prazer"},
		{"Metas", "meta"},
		{"Certificações", "certificação"},
		{"Ações", "acao"},
		{"Investimentos Internacionais", "investimento internacional"},
		{"Veículos", "veiculo"},
	}
	for _, sc := range scenarios {
		assert.Equal(t, normalizeCategoryTerm(sc.b), normalizeCategoryTerm(sc.a), "%q vs %q", sc.a, sc.b)
	}
}

func TestFlattenCategoryCatalog_FiltersKindAndRoots(t *testing.T) {
	roots := []interfaces.Category{
		{
			ID: testRootCustoFixoID, Slug: "custo-fixo", Name: "Custo Fixo", Kind: "expense",
			Subcategories: []interfaces.Category{
				{ID: testLeafCombustivel, Slug: "combustivel", Name: "Combustível"},
				{ID: testRootCustoFixoID, Slug: "custo-fixo", Name: "Custo Fixo"},
			},
		},
		{ID: uuid.New(), Slug: "salario", Name: "Salário", Kind: "income", Subcategories: []interfaces.Category{{ID: uuid.New(), Slug: "clt", Name: "CLT"}}},
	}

	entries := FlattenCategoryCatalog(roots, interfaces.CategoryKindExpense)

	require.Len(t, entries, 1)
	assert.Equal(t, testLeafCombustivel, entries[0].LeafID)
	assert.Equal(t, "Custo Fixo > Combustível", entries[0].Path())
}

func TestBuildRootOnlyCandidates(t *testing.T) {
	parent := testRootCustoFixoID
	roots := []interfaces.Category{
		{ID: testRootCustoFixoID, Slug: "custo-fixo", Name: "Custo Fixo", Kind: "expense"},
		{ID: testRootPrazeresID, Slug: "prazeres", Name: "Prazeres", Kind: "expense"},
		{ID: uuid.New(), Slug: "salario", Name: "Salário", Kind: "income"},
		{ID: uuid.New(), Slug: "combustivel", Name: "Combustível", Kind: "expense", ParentID: &parent},
	}

	candidates := BuildRootOnlyCandidates(roots, interfaces.CategoryKindExpense)

	require.Len(t, candidates, 2)
	assert.Equal(t, "Custo Fixo", candidates[0].Path)
	assert.Equal(t, uuid.Nil, candidates[0].SubcategoryID)
	assert.Equal(t, testRootCustoFixoID, candidates[0].RootCategoryID)
	assert.Equal(t, "Prazeres", candidates[1].Path)
}

func TestCatalogEntryToCandidate_ContratoManualCanonico(t *testing.T) {
	entry := testCategoryCatalog()[0]

	candidate := CatalogEntryToCandidate(entry)

	assert.Equal(t, entry.RootID, candidate.RootCategoryID)
	assert.Equal(t, entry.LeafID, candidate.SubcategoryID)
	assert.Equal(t, "Custo Fixo > Combustível", candidate.Path)
	assert.Equal(t, "manual_confirmed", candidate.Confidence)
	assert.Equal(t, "manual_canonical", candidate.MatchQuality)
	assert.Equal(t, "manual_canonical", candidate.SignalType)
	assert.NotEmpty(t, candidate.MatchReason)
}
