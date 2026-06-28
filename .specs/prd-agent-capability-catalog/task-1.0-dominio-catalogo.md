# Tarefa 1.0: Domínio do catálogo — CapabilityMode, CapabilitySpec, Catalog

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o domínio puro do catálogo de capabilities em `internal/agent/application/capability/`: o tipo fechado `CapabilityMode` (DMMF state-as-type), a struct `CapabilitySpec` e a coleção imutável `Catalog` com smart constructor `NewCatalog` e os métodos `Lookup`/`List`/`Classify`. Sem IO, sem wiring — 100% testável isoladamente. Habilita todas as tarefas seguintes.

<requirements>
- RF-01: `CapabilitySpec` com os campos `ID`, `Description`, `Kind`, `WorkflowID`, `ToolName`, `Mode`, `RequiresConfirmation`, `SupportsSuspend`, `SupportsResume`, `Channels`, `MetricsKey`.
- RF-02: `CapabilityMode` tipo fechado (`ModeRead`/`ModeWrite`) com `String()`/`IsValid()`/`ParseCapabilityMode()`; nunca string livre.
- RF-04: `Catalog.Lookup(kind) (CapabilitySpec, bool)`.
- RF-05: `Catalog.List() []CapabilitySpec` com cópia defensiva e ordem estável.
- RF-06: campos de classificação operacional mínima presentes (`Mode`, `RequiresConfirmation`, `SupportsSuspend`, `SupportsResume`).
- RF-11: `Catalog.Classify(kind) (workflow, tool string)` retorna fallback `conversational`/`""` para kind não-catalogado.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `internal/agent/application/capability/spec.go`: `CapabilityMode` (iota + `String`/`IsValid`/`ParseCapabilityMode`) e `CapabilitySpec`.
- [ ] 1.2 Criar `internal/agent/application/capability/catalog.go`: `Catalog`, `NewCatalog(specs...)` (smart constructor com `errors.Join` validando unicidade de `ID`/`Kind`, `Mode` válido, `WorkflowID` não-vazio), `Lookup`, `List` (cópia defensiva), `Classify` (fallback `conversational`/`""`).
- [ ] 1.3 Testes unitários whitebox cobrindo enum, construtor (erros), `Lookup` hit/miss, `List` cópia/ordem, `Classify` fallback.

## Detalhes de Implementação

Ver `techspec.md` → "Interfaces Chave" (assinaturas de `CapabilityMode`, `CapabilitySpec`, `Catalog`) e "Modelos de Dados". As constantes de `WorkflowID` espelham as de `agent_runtime.go` (`transactions`/`budget`/`cards`/`conversational`). `Classify` deriva de `Lookup`: se presente, retorna `(spec.WorkflowID, spec.ToolName)`; senão `("conversational", "")` (RF-11, ADR-002). Conformidade DMMF (state-as-type, smart constructor) e R-ADAPTER-001.1 (zero comentários).

## Critérios de Sucesso

- `CapabilityMode` rejeita zero-value e string inválida; round-trip `String`/`Parse` correto.
- `NewCatalog` agrega todos os erros via `errors.Join` e nomeia o campo/kind no erro.
- `Classify` nunca entra em panic e sempre retorna o fallback para kind desconhecido.
- Pacote compila sem comentários e passa `go vet`/lint.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — modela capability/kind do `internal/agent`; gatilho da skill (adicionar/alterar tool/outcome/kind) acionado.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/capability/spec.go` (novo)
- `internal/agent/application/capability/catalog.go` (novo)
- `internal/agent/application/capability/spec_test.go`, `catalog_test.go` (novos)
- Referência: `internal/agent/application/services/agent_runtime.go` (constantes de workflow)
