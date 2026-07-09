# Tarefa 5.0: Guarda de kind + reclassificação + clarify único

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Impedir que receita seja gravada em categoria de despesa (e vice-versa), reclassificando por kind
antes do write e pedindo esclarecimento uma única vez quando não houver categoria compatível.
Depende de 3.0 (para não engolir). Ver ADR-005.

<requirements>
- RF-06: income só grava em categoria kind income; expense só em kind expense.
- RF-07: ao detectar incompatibilidade de kind na resolução, reclassificar usando o kind correto
  antes de iniciar o pending write.
- RF-08: sem categoria compatível, pedir esclarecimento uma única vez, sem gerar `usecaseError`.
- RF-09: gate `ErrKindMismatch` do módulo transactions permanece como defesa final.
</requirements>

## Subtarefas

- [x] 5.1 Ajustar a resolução de candidatos (`category_resolution.go`) para filtrar/priorizar por
  `state.Kind` (income/expense).
- [x] 5.2 Em `validateCategoryForWrite`, tratar ausência de candidato compatível como clarify único
  (`AwaitingSlotCategory`), não como erro (herda a semântica não-silenciosa de 3.0).
- [x] 5.3 Manter `ResolveForWrite`/`ErrKindMismatch` como defesa final, com erro propagado.

## Detalhes de Implementação

Ver ADR-005 e techspec.md. `Kind` já é tipo fechado (`valueobjects.Kind`). Nenhum
`switch case intent.Kind` de roteamento (R-AGENT-WF-001.1). Regra de kind vive na resolução do
consumidor, não em adapter nem no kernel.

## Critérios de Sucesso

- Income com candidato expense ⇒ reclassifica para categoria income; não inicia escrita em "Metas".
- Sem candidato income compatível ⇒ clarify único; run não termina como `usecaseError` silencioso.
- Expense simétrico; `ErrKindMismatch` continua barrando como última defesa (não engolido).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — resolução de categoria e pending workflow no consumidor `internal/agents`.
- `domain-modeling-production` — `Kind` income/expense como tipo fechado e reclassificação por invariante.
- `design-patterns-mandatory` — gate `não aplicar padrão` (filtro direto por kind, sem estrutura nova).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/category_resolution.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`
- `internal/categories/application/usecases/resolve_category_for_write.go`
