# Tarefa 4.0: Conclusão — persistência condicional + mensagem final value-aware

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

No `step-conclusion`, persistir o valor da meta no `metadata` apenas quando informado e mencioná-lo na mensagem final. A working memory markdown permanece inalterada.

<requirements>
- RF-11: valor válido persistido em `metadata` via merge JSONB sob `objetivo_financeiro_valor_centavos` (int64 centavos), sem migration.
- RF-12: sem valor válido, nenhuma chave de valor é gravada (omissão), preservando o payload atual.
- RF-15: com valor, a mensagem final menciona-o junto do objetivo; sem valor, mensagem idêntica à atual.
- RF-16: a working memory markdown (`"## Objetivo Financeiro\n\n"+Goal`) permanece intocada; valor não vai ao system prompt.
</requirements>

## Subtarefas

- [ ] 4.1 Em `BuildConclusionStep` (~L774-780), montar `metadata` condicionalmente: incluir `objetivo_financeiro_valor_centavos` apenas quando `state.GoalValueCents > 0`.
- [ ] 4.2 Alterar `conclusionFinalMessage` (~L468) para `conclusionFinalMessage(goal string, valueCents int64)`, mencionando `(meta de <formatBRL>)` quando `valueCents > 0`; reusar `formatBRL` (~L291).
- [ ] 4.3 Atualizar o único call-site (~L780) para a nova assinatura; confirmar via `grep "conclusionFinalMessage("` (Risco R4).
- [ ] 4.4 Não tocar a chamada `workingMem.Upsert` do markdown (RF-16).
- [ ] 4.5 Testes: `UpsertMetadata` recebe a chave de valor só quando `>0`; `conclusionFinalMessage` com/sem valor; `Upsert` (markdown) sem valor.

## Detalhes de Implementação

Ver `techspec.md` seções "Modelos de Dados" (bloco de persistência) e "Interfaces Chave" (`conclusionFinalMessage`). Merge JSONB `||` em `WorkingMemoryRepository.UpsertMetadata` (`working_memory_repository.go:75`) → omitir a chave preserva o payload; presença adiciona. Sem migration.

## Critérios de Sucesso

- Chave de valor presente sse e só se `GoalValueCents > 0`.
- Mensagem final sem valor byte-idêntica à atual (zero regressão de UX).
- WM markdown inalterada.
- `go build`, `go vet`, `go test -race ./internal/agents/...` verdes; zero comentários; testes R-TESTING-001.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — persistência via port `memory.WorkingMemory.UpsertMetadata` e mensagem final do step de conclusão do workflow.
- `domain-modeling-production` — semântica sentinela (`>0`) na fronteira de persistência sem duplicar validação.
- `design-patterns-mandatory` — gate confirmou "sem padrão de catálogo" para a construção condicional do metadata/mensagem.
- `go-testing` — testes de persistência condicional e da mensagem final com mock de `memory.WorkingMemory`.

## Testes da Tarefa

- [ ] Testes unitários (metadata condicional; mensagem final com/sem valor; WM markdown intocada)
- [ ] Testes de integração (não aplicável nesta tarefa)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — `BuildConclusionStep` (~L731), `conclusionFinalMessage` (~L468), `formatBRL` (~L291).
- `internal/platform/memory/ports.go` — `WorkingMemory.UpsertMetadata` (consumido).
- Teste: `internal/agents/application/workflows/onboarding_workflow_test.go`.
