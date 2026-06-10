# Tarefa 2.0: Domínio puro — entities, value objects e services

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Modelar o domínio do módulo em pacotes puros (sem IO, sem driver, sem cliente): entidades, value objects e dois domain services stateless. O `AllocationDistributor` implementa half-even + distribuição determinística de centavos residuais na **ordem canônica dos slugs** RF-11a; o `ThresholdEvaluator` é função pura `(spent, planned, currently_crossed) → Transition` cobrindo RF-59, RF-60, RF-60a, RF-60b. Cobertura por testes table-driven.

<requirements>
- Pacote `domain/` puro: sem importar `application`, `infrastructure`, `platform`, banco, HTTP, SDK externo (AGENTS.md).
- Enums em VOs seguem `iota + 1` (R5.8). Zero value reservado para "não inicializado".
- Cents (`int64`), BasisPoints (`int` 0..10000), Competence (`YYYY-MM` em America/Sao_Paulo com `time.LoadLocation` resolvida uma vez no boot, mantida em memória).
- `ExternalTransactionID` validado como UUID v4 (lowercase) **OU** ULID canônico (uppercase Crockford base32) — RT-26.
- `MutationKind`: `create|update|delete` (iota+1).
- `RootSlug`: enum dos 5 slugs editoriais imutáveis (constantes Go).
- `Threshold`: enum `t80|t100` mapeado para SMALLINT 80/100.
- `Budget.Activate()` impõe RF-07/RF-07a/RF-07b (soma = 10000 bp, total > 0).
- `Expense` carrega `Version int64`, `TombstoneVersion *int64`, `DeletedAt *time.Time` e expõe métodos para edição (gera próxima versão) e soft-delete (preenche tombstone).
- Zero comentários em `.go` de produção (R-ADAPTER-001.1 inegociável).
</requirements>

## Subtarefas

- [ ] 2.1 Value objects (`competence.go`, `cents.go`, `basis_points.go`, `root_slug.go`, `threshold.go`, `mutation_kind.go`, `external_transaction_id.go`, `producer_source.go`) com construtores validados e testes table-driven.
- [ ] 2.2 Entidades (`budget.go`, `allocation.go`, `expense.go`, `expense_tombstone.go`, `alert.go`, `threshold_state.go`, `pending_event.go`) com invariantes e métodos puros.
- [ ] 2.3 `services/allocation_distributor.go` — half-even + ordem determinística de slugs RF-11a; testes cobrindo centavos residuais positivos e negativos.
- [ ] 2.4 `services/threshold_evaluator.go` — função pura para 80% e 100%, com casos limite (`spent==planned`, `spent==0`, `planned==0` rejeitado).
- [ ] 2.5 `go test -race -count=1 ./internal/budgets/domain/...` verde.

## Detalhes de Implementação

Ver seção **Modelos de Dados** e a sub-seção **Interfaces Chave** da `techspec.md`. O distribuidor de centavos residuais deve referenciar a ordem por slug em `RF-11a`. Para Competence, carregar `time.LoadLocation("America/Sao_Paulo")` em variável de pacote inicializada no boot do `module.go` (não em `init()` — R0); domain consome o valor já resolvido por injeção via construtor do parser.

## Critérios de Sucesso

- Cobertura unitária ≥ 90% nos pacotes `domain/...`.
- Nenhuma chamada a `time.Now()` em entidade/VO de domínio (R6.7).
- Linter `golangci-lint run ./internal/budgets/domain/...` sem warnings.
- Sem `var _ Interface = (*Type)(nil)` (R6.4).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `object-calisthenics-go` — estruturas de domínio com múltiplos VOs e invariantes; aplicar heurísticas para manter níveis de aninhamento baixos e responsabilidades estreitas.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/budgets/domain/entities/*.go` (novo)
- `internal/budgets/domain/valueobjects/*.go` (novo)
- `internal/budgets/domain/services/*.go` (novo)
- Referência de estilo: `internal/categories/domain/`, `internal/billing/domain/`
