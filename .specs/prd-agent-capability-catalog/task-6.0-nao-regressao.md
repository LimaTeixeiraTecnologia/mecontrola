# Tarefa 6.0: Não-regressão — suíte agent/workflow verde + gates HITL

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Validação final de não-regressão: rodar a suíte completa de `internal/agent` e `internal/platform/workflow`, confirmar verde, e validar que os gates HITL e o comportamento de roteamento permanecem corretos após a substituição da fonte de classificação. Consolidar evidências.

<requirements>
- RF-16: a suíte de testes de `internal/agent` e `internal/platform/workflow` permanece verde após a mudança.
</requirements>

## Subtarefas

- [ ] 6.1 Rodar `go test ./internal/agent/... ./internal/platform/workflow/...` (unit) e confirmar verde.
- [ ] 6.2 Rodar a suíte de integração/e2e existente do agent (se aplicável) e confirmar verde.
- [ ] 6.3 Rodar os gates de governança aplicáveis: zero comentários (R-ADAPTER-001.1), sem SQL em adapter, cardinalidade de métricas (R-AGENT-WF-001.5), switch de domínio não cresceu em `daily_ledger_agent.go` (R-AGENT-WF-001).
- [ ] 6.4 Capturar evidências (saída dos testes e dos gates) para o relatório de execução.

## Detalhes de Implementação

Ver `techspec.md` → "Abordagem de Testes" e "Conformidade com Padrões". Esta tarefa não introduz código novo de produção; é o portão de qualidade que fecha o SDD. Depende de 3.0, 4.0 e 5.0.

## Critérios de Sucesso

- Suíte unit + integração/e2e de `internal/agent` e `internal/platform/workflow` verde.
- Gates de governança sem violação.
- Evidências capturadas (logs de teste/gates).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/...` (suíte completa)
- `internal/platform/workflow/...` (suíte completa)
- `.claude/rules/*` (gates de governança)
