<!-- spec-hash-prd: 1c03f9d959dc6c103436cd8a6c396ef2a8a5760e8f477f933d645f9f6972d461 -->

# Especificacao Tecnica — Contrato Deterministico de Categorias para Transacoes Agentivas

## Resumo Executivo

Esta especificacao define um contrato tecnico unico para autorizar escrita financeira categorizada com 0 falso positivo conhecido. `internal/categories` permanece como fonte canonica de catalogo, dicionario, candidatos, outcome e versao editorial. `internal/agents` consome uma resolucao rica para classificar, explicar e pedir clarificacao. `internal/transactions` executa o gate final antes de qualquer persistencia de transacao ou template recorrente.

Como o banco e novo, a entrega altera o baseline de schema em `migrations/000001_initial_schema.up.sql`; nao ha backfill nem remediacao de legado. Toda transacao/template categorizado deve ter subcategoria folha e evidencia persistida completa no mesmo write.

## Arquitetura do Sistema

### Componentes Modificados

- `internal/categories/application/usecases/SearchDictionary`: manter como calculo canonico de candidatos, score, outcome e versao; expor internamente `Outcome`, `Version`, `SignalType`, `Confidence`, `MatchQuality`, `MatchedTerm`, `MatchReason` e `HasMore`.
- `internal/categories/application/usecases/ResolveCategoryForWrite`: novo use case canonico para validar uma escolha concreta por IDs antes da escrita, incluindo raiz, subcategoria folha, kind, active/deprecated e versao editorial.
- `internal/agents/application/interfaces.CategoriesReader`: trocar `SearchDictionary(ctx, term, kind) ([]CategoryCandidate, error)` por retorno rico `CategorySearchResult`.
- `internal/agents/infrastructure/binding/categories_reader_adapter.go`: preservar todos os campos vindos de `categories`; nenhum campo de evidência pode ser descartado.
- `internal/agents/application/usecases.RegisterEntry`: aceitar escrita automatica somente com `Outcome=matched`, exatamente um candidato, `IsAmbiguous=false`, subcategoria folha, score/confidence/quality validos e versao editorial presente.
- `internal/agents/application/tools/classify_category`: tool explicativa; retorna candidatos, outcome, version, evidence fields e `writeDecision`, mas nao autoriza persistencia.
- `internal/agents/application/workflows/destructive_confirm_workflow`: remover uso de primeiro candidato; revalidar por gate completo e derivar kind da direcao do draft.
- `internal/transactions/application/interfaces.CategoryWriteGate`: nova interface consumidora para aprovar ou bloquear escrita categorizada.
- `internal/transactions/domain/valueobjects.CategoryWriteEvidence`: novo VO com smart constructor e zero value invalido.
- `internal/transactions/domain/valueobjects.CategoryDecisionSource`: enum fechado `auto_matched`, `user_selected_candidate`, `manual_canonical_id`, `system_migration`.
- `internal/transactions/application/usecases.{CreateTransaction,UpdateTransaction,CreateRecurringTemplate,UpdateRecurringTemplate}`: exigir `CategoryWriteEvidence` aprovada, aplicar folha obrigatoria, kind/direction, deprecated=false e version drift antes de persistir.
- `internal/transactions/application/usecases.{UpdateTransaction,UpdateRecurringTemplate}`: revalidar e atualizar evidencia da categoria atual em todo update, mesmo quando `category_id` e `subcategory_id` nao mudarem.
- `internal/transactions/domain/entities.{Transaction,RecurringTemplate}`: armazenar evidencia funcional, alem de snapshots de nomes.
- `internal/transactions/infrastructure/repositories/postgres`: persistir e reconstituir todos os campos de evidencia.
- `migrations/000001_initial_schema.up.sql`: tornar `subcategory_id` e `subcategory_name_snapshot` obrigatorios em `transactions` e `transactions_recurring_templates`; adicionar colunas, FKs, triggers semanticos e constraints de evidencia.

### Estado Real das Migrations de Categorias

O baseline atual de `categories` ja oferece a base canonica necessaria:

- `mecontrola.category_editorial_version` existe e e incrementada pelos seeds de categorias/dicionario.
- `mecontrola.categories` possui `kind IN ('income','expense')`, `allocation_type`, `deprecated_at`, `parent_id` e trigger `categories_parent_same_kind`.
- `mecontrola.category_dictionary` possui `signal_type`, `confidence`, `is_ambiguous`, `deprecated_at`, `term_normalized`, unaccent e indice trigram `dictionary_term_trgm_idx`.
- Os seeds atuais criam raizes e folhas para `expense` e `income`; portanto folha obrigatoria e viavel no catalogo inicial.
- O schema atual de transacoes nao possui FK para `categories` nem protecao de banco para raiz/folha/kind/deprecated; esta feature deve adicionar essa defesa no baseline.

### Fluxo de Dados

1. Entrada agentiva por descricao chama `categories.SearchDictionary` e recebe `CategorySearchResult`.
2. Entrada manual com IDs chama `ResolveCategoryForWrite` com `source=manual_canonical_id`.
3. `agents` transforma resolucao aceita em comando para `transactions`; casos ambiguos retornam clarificacao e nao chamam writer.
4. `transactions` chama `CategoryWriteGate.Approve` imediatamente antes do write.
5. O gate compara a versao editorial da evidencia com a versao atual de `categories`.
6. Se qualquer gate falhar, retorna erro tipado e nada e persistido.
7. Se aprovado, transacao/template e evidencia sao persistidos no mesmo `UnitOfWork`.

## Design de Implementacao

### Contratos de `categories`

`SearchDictionary` deve preservar o contrato atual e retornar resultado rico para consumidores internos. O DTO de saida deve deixar `Outcome` e `Version` disponiveis para adapters internos.

```go
type CategorySearchResult struct {
	Outcome    CategoryOutcome
	Version    int64
	HasMore    bool
	Candidates []CategoryCandidate
}

type CategoryCandidate struct {
	RootCategoryID uuid.UUID
	CategoryID     uuid.UUID
	Path           string
	MatchedTerm    string
	SignalType     string
	Confidence     string
	MatchQuality   string
	Score          float64
	IsAmbiguous    bool
	MatchReason    string
}
```

`ResolveCategoryForWrite` deve validar IDs concretos sem depender de texto e deve usar `categories.GetByID`/`ListByIDs` para provar raiz, folha, parent, kind e active status:

```go
type ResolveCategoryForWriteInput struct {
	RootCategoryID uuid.UUID
	SubcategoryID  uuid.UUID
	Kind           string
	ExpectedVersion int64
}

type ResolveCategoryForWriteOutput struct {
	RootCategoryID   uuid.UUID
	SubcategoryID    uuid.UUID
	Kind             string
	Path             string
	RootSlug         string
	SubcategorySlug  string
	CategoryName     string
	SubcategoryName  string
	EditorialVersion int64
	Deprecated       bool
}
```

### Contratos de `agents`

`internal/agents/application/interfaces.CategoriesReader` deve transportar o contrato rico:

```go
type CategoriesReader interface {
	SearchDictionary(ctx context.Context, term, kind string) (CategorySearchResult, error)
	ResolveForWrite(ctx context.Context, input CategoryWriteRequest) (CategoryWriteDecision, error)
	ListCategories(ctx context.Context, userID uuid.UUID) ([]Category, error)
}
```

`RegisterEntry.classify` deve retornar uma evidencia candidata para `transactions`, nao apenas `CategoryCandidate`. O comportamento obrigatorio e:

- `Outcome != matched`: retorna `ToolOutcomeClarify`.
- `len(Candidates) != 1`: retorna `ToolOutcomeClarify`.
- `candidate.IsAmbiguous=true`: retorna `ToolOutcomeClarify`.
- `candidate.RootCategoryID == candidate.CategoryID`: retorna `ToolOutcomeClarify`.
- `Version <= 0`: retorna erro de use case.
- Caso aceito: source `auto_matched`.

Clarificacao retomada deve usar source `user_selected_candidate` e passar novamente por `ResolveForWrite`.

### Contrato de `transactions`

`transactions` declara a interface consumidora:

```go
type CategoryWriteGate interface {
	Approve(ctx context.Context, input CategoryWriteGateInput) (CategoryWriteEvidence, error)
}

type CategoryWriteGateInput struct {
	Direction        string
	RootCategoryID   uuid.UUID
	SubcategoryID    uuid.UUID
	Source           CategoryDecisionSource
	Outcome          string
	Score            float64
	Confidence       string
	Quality          string
	SignalType       string
	MatchedTerm      string
	MatchReason      string
	ExpectedVersion  int64
}
```

`CategoryWriteEvidence` deve ser value object no dominio de `transactions` com smart constructor. O construtor deve rejeitar:

- source vazio ou fora do enum fechado;
- outcome diferente de `matched`;
- score fora de `[0,1]`;
- confidence fora de `high`, `medium`, `low`, `manual_confirmed`;
- quality fora de `exact`, `token`, `fuzzy`, `manual_canonical`;
- root ou leaf UUID zero;
- root igual a leaf;
- kind fora de `expense`/`income`;
- editorial version menor ou igual a zero;
- path vazio;
- source `manual_canonical_id` sem `score=1.0`, `confidence=manual_confirmed`, `quality=manual_canonical`.
- source `manual_canonical_id` sem `signal_type=manual_canonical`, `matched_term=<subcategory_slug>` e `match_reason=manual canonical id validated`.

### Regras de Gate

`CategoryWriteGate.Approve` deve:

1. Validar shape do input.
2. Consultar `categories.ResolveCategoryForWrite`.
3. Bloquear se subcategoria nao for filha direta da raiz.
4. Bloquear se categoria ou subcategoria estiver deprecated.
5. Bloquear se kind da categoria divergir da direcao da transacao.
6. Bloquear se versao editorial atual divergir de `ExpectedVersion`.
7. Construir `CategoryWriteEvidence`.

Erros funcionais devem ser tipados:

- `ErrCategoryWriteBlocked`
- `ErrCategoryNeedsClarification`
- `ErrCategoryVersionChanged`
- `ErrInvalidCategoryDecisionSource`
- `ErrCategoryEvidenceRequired`
- `ErrCategoryRootWithoutLeaf`
- `ErrCategoryDeprecated`
- `ErrCategoryKindDirectionMismatch`

### Persistencia

Alterar o baseline de `mecontrola.transactions` e `mecontrola.transactions_recurring_templates`:

- `subcategory_id UUID NOT NULL`
- `subcategory_name_snapshot TEXT NOT NULL`
- `category_kind TEXT NOT NULL`
- `category_path TEXT NOT NULL`
- `category_outcome TEXT NOT NULL`
- `category_score NUMERIC(5,4) NOT NULL`
- `category_confidence TEXT NOT NULL`
- `category_match_quality TEXT NOT NULL`
- `category_signal_type TEXT NOT NULL`
- `category_matched_term TEXT NOT NULL`
- `category_match_reason TEXT NOT NULL`
- `category_decision_source TEXT NOT NULL`
- `category_editorial_version BIGINT NOT NULL`
- `category_decided_at TIMESTAMPTZ NOT NULL`

Constraints obrigatorias:

- `FOREIGN KEY (category_id) REFERENCES mecontrola.categories(id) ON DELETE RESTRICT`
- `FOREIGN KEY (subcategory_id) REFERENCES mecontrola.categories(id) ON DELETE RESTRICT`
- `category_outcome = 'matched'`
- `category_score >= 0 AND category_score <= 1`
- `category_kind IN ('expense','income')`
- `category_confidence IN ('high','medium','low','manual_confirmed')`
- `category_match_quality IN ('exact','token','fuzzy','manual_canonical')`
- `category_signal_type IN ('canonical_name','alias','phrase','merchant','segment','manual_canonical')`
- `category_decision_source IN ('auto_matched','user_selected_candidate','manual_canonical_id','system_migration')`
- `subcategory_id <> category_id`
- `category_editorial_version > 0`

Triggers semanticos obrigatorios no baseline:

- `transactions_category_write_gate_trg` em `mecontrola.transactions`.
- `transactions_recurring_templates_category_write_gate_trg` em `mecontrola.transactions_recurring_templates`.
- Ambos devem chamar funcao compartilhada de validacao que verifica:
  - `category_id` referencia uma raiz (`parent_id IS NULL`);
  - `subcategory_id` referencia folha direta (`parent_id = category_id`);
  - raiz e folha possuem mesmo `kind`;
  - `kind` da categoria e compativel com `direction` (`1 -> income`, `2 -> expense`);
  - raiz e folha nao possuem `deprecated_at`;
  - `category_kind` persistido corresponde ao kind real;
  - `category_editorial_version` e igual a versao atual em `category_editorial_version`;
  - `category_path`, snapshots e evidencia textual nao estao vazios.

Para source `manual_canonical_id`, o use case deve persistir:

- `category_score=1.0`
- `category_confidence=manual_confirmed`
- `category_match_quality=manual_canonical`
- `category_decision_source=manual_canonical_id`
- `category_signal_type=manual_canonical`
- `category_matched_term=<subcategory_slug>`
- `category_match_reason=manual canonical id validated`

As constraints e triggers sao defesa em profundidade. O gate de aplicacao continua obrigatorio e deve falhar antes do banco em cenarios esperados.

## Pontos de Integracao

- `internal/agents/infrastructure/binding/categories_reader_adapter.go` implementa o contrato rico de `agents` chamando `categories`.
- `internal/transactions/infrastructure/repositories/postgres/categories_reader_adapter.go` implementa o acesso de `CategoryWriteGate` a `categories`.
- `internal/transactions/module.go` injeta o gate nos quatro use cases de escrita.
- Handlers HTTP e tools permanecem adapters finos; nao recebem repositories nem clients de categorias diretamente.

## Abordagem de Testes

### Testes Unitarios

- `categories`: `ClassifyByScore` e `SearchDictionary` com exact, alias, token, fuzzy, baixa confianca, multi-candidato, no match e version presente.
- `transactions/domain/valueobjects`: `CategoryDecisionSource` e `CategoryWriteEvidence` com zero value invalido, manual deterministico, source invalido e score fora do intervalo.
- `transactions/application`: `CreateTransaction`, `UpdateTransaction`, `CreateRecurringTemplate`, `UpdateRecurringTemplate` bloqueiam sem evidencia, raiz sem folha, deprecated, kind incompativel, version drift e subcategoria fora da raiz.
- `transactions/application`: updates sem troca de categoria revalidam e atualizam evidencia antes de persistir.
- `agents/application`: `RegisterEntry.classify` bloqueia outcome nao aceito, candidato unico ambiguo, score/quality/confidence insuficiente e version ausente.
- `agents/workflows`: confirmacao retomavel nao usa primeiro candidato e revalida escolha.
- `agents/tools`: `classify_category` retorna outcome, version, candidatos ricos e `writeDecision`.

### Testes de Integracao

Integration tests sao obrigatorios porque o risco envolve Postgres, constraints, adapters reais e mocks que hoje podem aceitar kind invalido.

- `categories` + adapter de `agents`: preserva todos os campos de `DictionarySearchOutput`.
- `transactions` + Postgres: write aprovado persiste evidencia completa; constraints rejeitam enum invalido e raiz sem folha.
- `transactions` + Postgres: FKs rejeitam IDs inexistentes e triggers rejeitam raiz/folha/kind/deprecated/version drift mesmo se o use case for bypassado.
- `transactions` + `categories`: version drift retorna erro tipado.
- `transactions` recorrencia: create/update template seguem a mesma matriz de bloqueio de transacao direta.

Usar `testcontainers-go` com build tag `integration`.

### Testes E2E

- Despesa com match inequivoco persiste evidencia completa.
- Receita com match inequivoco persiste evidencia completa.
- No match, multi-candidato e fuzzy/token abaixo de aceite solicitam clarificacao.
- Clarificacao por candidato canonico revalida e persiste.
- Confirmacao retomavel ambigua nao persiste.
- Write manual nao agentivo aprovado persiste evidencia deterministica.

## Sequenciamento de Desenvolvimento

1. Criar value objects de `transactions`: `CategoryDecisionSource`, `CategoryWriteEvidence`, confidence persistida e quality persistida.
2. Expandir DTO interno de `categories.SearchDictionary` para expor `Outcome` e evidencia.
3. Implementar `ResolveCategoryForWrite` em `categories`.
4. Implementar `CategoryWriteGate` em `transactions` com adapter fino para `categories`.
5. Alterar schema baseline, entidades e repositories de `transactions` e templates recorrentes.
6. Alterar `CreateTransaction` e `UpdateTransaction`.
7. Alterar `CreateRecurringTemplate` e `UpdateRecurringTemplate`.
8. Alterar `agents`: interface, adapter, `RegisterEntry`, `classify_category`, workflow de confirmacao e mocks.
9. Implementar matriz completa de testes unitarios, integracao e E2E.

## Dependencias Tecnicas

- Go 1.26.4 conforme `go.mod`.
- Postgres via infraestrutura de teste existente.
- `testcontainers-go` ja presente no projeto.
- Sem dependencia de LLM, embedding ou scorer para autorizar escrita.

## Monitoramento e Observabilidade

Registrar eventos com logs estruturados e traces usando campos de baixa cardinalidade:

- `status`
- `reason`
- `source`
- `kind`
- `surface`

Metricas obrigatorias:

- `category_write_gate_total{status,reason,source,kind,surface}`
- `category_write_version_drift_total{kind,surface}`
- `category_write_persisted_total{source,kind,surface}`
- `category_clarification_requested_total{reason,kind,surface}`

Labels proibidos:

- `user_id`
- `transaction_id`
- `category_id`
- `subcategory_id`
- termo buscado
- descricao da transacao
- texto do usuario

## Decisoes Chave

- ADR-001: [Gate unico de categoria antes da persistencia](adr-001-gate-unico-categoria.md)
- ADR-002: [Evidencia persistida em colunas normalizadas](adr-002-evidencia-normalizada.md)
- ADR-003: [Fonte da decisao como enum fechado e manual deterministico](adr-003-source-enum-manual-deterministico.md)
- ADR-004: [Bloqueio por drift de versao editorial](adr-004-bloqueio-drift-versao-editorial.md)
- ADR-005: [Subcategoria folha obrigatoria](adr-005-subcategoria-folha-obrigatoria.md)
- ADR-006: [Defesa de banco para categoria canonica](adr-006-defesa-banco-categoria-canonica.md)

## Riscos Conhecidos

- Duplicacao de regra entre `agents` e `transactions`: mitigado por gate final em `transactions`; `agents` so classifica e explica.
- Mocks mascarando contratos invalidos: mitigado por integration tests com `categories` real, Postgres, FKs e triggers semanticos.
- Evidencia manual virando escape hatch: mitigado por mesmo gate completo e valores deterministicos fixos.
- Aumento de latencia por leitura de versao: mitigado por bloqueio antes do write e cache de catalogo com invalidacao por versao.
- Drift editorial frequente: mitigado por metrica especifica e processo operacional de curadoria.

## Conformidade com Padroes

- DDD: invariantes de evidencia em value objects; use cases orquestram; handlers/tools ficam finos.
- DMMF: outcomes, sources e block reasons como tipos fechados; pipeline `classificar -> validar -> decidir -> persistir`.
- Go: interface no consumidor, DI por construtor, sem `init`, sem `panic`, erros com wrapping e sentinels.
- Mastra: tool como adapter fino; runtime/scorer/LLM nao autorizam escrita.
- Segurança: input externo e resposta de LLM nao desbloqueiam persistencia.
- Testes: table-driven unitarios, integration tests para Postgres/adapters e E2E para fluxos conversacionais.

## Mapeamento RF -> Decisao -> Teste

| RF | Decisao tecnica | Teste minimo |
| --- | --- | --- |
| RF-01 | `categories` como autoridade via `SearchDictionary` e `ResolveCategoryForWrite` | Integration adapter preserva output canonico |
| RF-02, RF-03 | `kind` fechado `expense/income` | Unit de parse e integration rejeita kind invalido |
| RF-04 | todo write exige `CategoryWriteEvidence` | Use cases falham com `ErrCategoryEvidenceRequired` |
| RF-05, RF-06 | status/reason fechados para aceite/bloqueio | Unit table de outcomes e reasons |
| RF-07, RF-08 | candidato unico sem outcome aceito bloqueia | Unit `RegisterEntry` e `CategoryWriteGate` |
| RF-09, RF-10, RF-15, RF-18 | raiz ativa e subcategoria folha obrigatoria com FK e trigger | Integration com subcategoria ausente/fora da raiz e bypass de use case |
| RF-11 | deprecated bloqueia novas escritas | Integration com categoria deprecated |
| RF-12, RF-13, RF-14 | clarificacao sempre revalida pelo gate | E2E de clarificacao aceita e falha |
| RF-16 | version drift bloqueia | Integration simulando mudanca de version |
| RF-17, RF-28 | evidencia persistida em transacao/template | Repository integration verifica colunas |
| RF-19 | source enum fechado | Unit VO e constraint SQL |
| RF-20, RF-21 | manual deterministico | Use case manual persiste score/confidence/quality/source fixos |
| RF-22 | manual persiste signal/matched term/reason deterministicos | Use case manual verifica signal_type, matched_term e reason |
| RF-23 | update sempre revalida evidencia atual | Unit update sem troca de categoria chama gate e atualiza evidencia |
| RF-24 | banco novo sem backfill | Migration test valida baseline |
| RF-25, RF-26 | agents consome contrato rico | Unit de tool e RegisterEntry |
| RF-27 | workflow nao usa primeiro candidato | Unit do destructive confirm ambiguo |
| RF-28, RF-29 | paridade create/update/recorrencia | Matriz unit/integration nos quatro use cases |
| RF-31 | LLM/scorer nao desbloqueia escrita | Unit garante escrita depende de gate |
| RF-32 | diagnostico diferenciado | Unit de erro/reason por falha |
| RF-33 | sem string livre critica | Unit de parse de enums e zero value invalido |
| RF-34 | criterios reproduziveis por qualidade | Unit table exact/alias/token/fuzzy/manual |
| RF-35 | mocks nao mascaram invalidos | Integration real com `categories` + Postgres |

## Arquivos Relevantes

- `internal/categories/application/usecases/search_dictionary.go`
- `internal/categories/application/usecases/validate_subcategory.go`
- `internal/categories/application/dtos/output/dictionary_search_output.go`
- `internal/categories/domain/services/candidate_resolver.go`
- `internal/categories/domain/valueobjects/search_outcome.go`
- `internal/agents/application/interfaces/categories_reader.go`
- `internal/agents/infrastructure/binding/categories_reader_adapter.go`
- `internal/agents/application/tools/classify_category.go`
- `internal/agents/application/usecases/register_entry.go`
- `internal/agents/application/workflows/destructive_confirm_workflow.go`
- `internal/transactions/application/interfaces/category_validator.go`
- `internal/transactions/application/interfaces/types.go`
- `internal/transactions/application/usecases/create_transaction.go`
- `internal/transactions/application/usecases/update_transaction.go`
- `internal/transactions/application/usecases/create_recurring_template.go`
- `internal/transactions/application/usecases/update_recurring_template.go`
- `internal/transactions/domain/entities/transaction.go`
- `internal/transactions/domain/entities/recurring_template.go`
- `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go`
- `internal/transactions/infrastructure/repositories/postgres/recurring_template_repository.go`
- `migrations/000001_initial_schema.up.sql`
