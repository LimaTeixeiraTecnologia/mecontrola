# Tarefa 4.0: Domínio: value objects, entidades e CandidateResolver

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar a camada de domínio pura do módulo categories: value objects (`Kind`, `SignalType`, `Confidence`, `AllocationType`), entidades (`Category`, `DictionaryEntry`), e o domain service `CandidateResolver` que aplica as regras de deduplicação e ambiguidade estrita.

<requirements>
- RF-24: termos ambíguos/fuzzy nunca promovidos em runtime
- RF-25: sem fuzzy matching, IA ou inferência semântica
- RF-26: correspondência exata do termo completo
- RF-27: deduplicação por precedência editorial (`canonical > alias > phrase > merchant > segment`) e ambiguidade estrita quando >1 candidato
- ADR-006: algoritmo de resolução de candidatos
- R-ADAPTER-001: zero comentários em .go; adaptadores finos (não se aplica aqui pois é domínio puro)
</requirements>

## Subtarefas

- [ ] 4.1 Value objects: `Kind`, `SignalType`, `Confidence`, `AllocationType` (enums iota+1)
- [ ] 4.2 Entidade `Category` com métodos `IsRoot()`, `IsActive()`
- [ ] 4.3 Entidade `DictionaryEntry`
- [ ] 4.4 Factory `NewCategoryID(kind, slug string) uuid.UUID` com namespace fixo
- [ ] 4.5 Domain service `CandidateResolver` com método `Resolve(entries []DictionaryEntry) []Candidate`
- [ ] 4.6 Unit tests em `testify/suite` cobrindo todos os cenários de RF-27

## Detalhes de Implementação

Ver techspec.md seção **Modelos de Dados** e ADR-006.

Regras Go mandatórias:
- Carregar obrigatoriamente `go-implementation`
- Carregar exemplos apenas sob demanda
- Verificar `go.mod` antes de usar recursos da linguagem
- Partir de `cmd/server/server.go`
- Zero comentários em arquivos `.go`

Pontos críticos:
- Enums com `iota + 1` (R5.8); zero value reservado para não inicializado.
- `CandidateResolver` deve ser stateless e puro (sem IO, sem dependências externas).
- Precedência de `signal_type`: `canonical_name (5) > alias (4) > phrase (3) > merchant (2) > segment (1)`.
- Empate no mesmo `signal_type`: resolver por caminho alfabético PT-BR (`root_name > subcategory_name`).
- Regra de ambiguidade estrita: se o número de candidatos distintos após deduplicação for > 1, todos recebem `is_ambiguous=true` na resposta.
- `has_more`: true quando existe ao menos um `category_id` correspondente descartado pelo limite de 3.

## Critérios de Sucesso

- [ ] Todos os value objects validam valores inválidos e retornam erro
- [ ] `CandidateResolver` cobre: high inequívoco, merchant ambíguo, sem match, empate em alta confiança, deduplicação por category_id
- [ ] Unit tests em `testify/suite`, table-driven, com mockery quando necessário
- [ ] Gate R0-R7 passa: `go build`, `go vet`, `go test -race -count=1`

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit test: cenário CC-B1 (high inequívoco)
- [ ] Unit test: cenário CC-B2 (merchant ambíguo com várias subcategorias)
- [ ] Unit test: cenário CC-B3 (sem correspondência)
- [ ] Unit test: cenário CC-B5 (empate em alta confiança)
- [ ] Unit test: deduplicação por precedência editorial
- [ ] Unit test: has_more quando há mais de 3 candidatos

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/categories/domain/valueobjects/kind.go`
- `internal/categories/domain/valueobjects/signal_type.go`
- `internal/categories/domain/valueobjects/confidence.go`
- `internal/categories/domain/valueobjects/allocation_type.go`
- `internal/categories/domain/entities/category.go`
- `internal/categories/domain/entities/dictionary_entry.go`
- `internal/categories/domain/services/candidate_resolver.go`
- Arquivos `_test.go` correspondentes
- `mockery.yml` (atualizar se necessário)
