# Tarefa 3.0: Contrato de Tool em `internal/platform/tool`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar `internal/platform/tool`: contrato de tool tipado equivalente a `@mastra/core/tools`. `ToolHandle` (adapter fino, não-genérico) + helper genérico `NewTool[I,O]` que encapsula marshaling/validação contra schema, e um `Registry` por id. Consumível por agents (6.0) e por steps de workflow (4.0/6.0).

<requirements>
- RF-09: contrato de tool com invocação e resultado tipado (I/O por schema).
- RF-10: tools consumíveis por agents e por steps de workflow.
- Tool é adapter fino (R-AGENT-WF-001.2 / R-ADAPTER-001): sem regra de negócio, sem SQL, sem branching de domínio.
- Zero comentários em Go; sem semântica de domínio; layering (não importa agent).
</requirements>

## Subtarefas

- [ ] 3.1 Definir `ToolHandle` (`ID`, `Description`, `Parameters() map[string]any`, `Invoke(ctx, argsJSON) ([]byte, error)`).
- [ ] 3.2 Implementar `NewTool[I,O](id, desc string, in, out Schema, exec func(ctx, I) (O,error)) ToolHandle` com unmarshal/validação de entrada e marshal de saída.
- [ ] 3.3 Implementar `Registry` (registro/resolução por id) e erro tipado para tool ausente.

## Detalhes de Implementação

Ver techspec.md "Interfaces Chave > Tool" e ADR-002 (layering). `Schema` reutiliza o tipo de `internal/platform/llm` (ou tipo compartilhado), evitando duplicação. Sem chamada LLM aqui.

## Critérios de Sucesso

- `internal/platform/tool` compila e não importa `internal/platform/agent` nem domínio.
- `NewTool` valida entrada inválida com erro explícito; round-trip I/O coberto por unit.
- Zero comentários em Go; lint verde.
- Gate: `grep -rn "QueryContext\|ExecContext\|db\.Exec\|internal/platform/agent" internal/platform/tool/ --include="*.go" --exclude="*_test.go"` retorna vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `go-implementation` — implementação Go obrigatória (CLAUDE.md) do contrato, generics e testes.
- `mastra` — Tool é primitivo canônico do padrão Mastro portado (createTool).

## Testes da Tarefa

- [ ] Testes unitários (testify/suite whitebox): `NewTool` sucesso, entrada inválida, registry resolve/ausente, `Invoke` round-trip.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/tool/` (novo) — `ToolHandle`, `NewTool`, `Registry`.
- `internal/platform/llm/` — tipo `Schema` reutilizado.
