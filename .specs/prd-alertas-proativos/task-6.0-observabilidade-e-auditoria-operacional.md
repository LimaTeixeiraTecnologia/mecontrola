# Tarefa 6.0: Observabilidade e auditoria operacional

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Instrumentar avaliação, supressão, dry-run, envio e falha com métricas e logs sem cardinalidade alta nem vazamento de dados sensíveis.

<requirements>
- Cobrir RF-15, REQ-23 e REQ-24.
- Métricas não podem usar `user_id`, telefone, `message_id`, `target_key` ou texto de erro bruto.
- Logs não podem conter token Meta, telefone completo ou dados financeiros sensíveis.
</requirements>

## Subtarefas

- [ ] 6.1 Mapear observabilidade existente no módulo budgets e plataforma.
- [ ] 6.2 Adicionar métricas de avaliação/supressão/dry-run/envio.
- [ ] 6.3 Garantir labels de baixa cardinalidade.
- [ ] 6.4 Adicionar logs estruturados com IDs seguros.
- [ ] 6.5 Criar testes ou gates de cardinalidade/segredo.

## Detalhes de Implementação

Referenciar `techspec.md` seção `Monitoramento e Observabilidade`.

## Critérios de Sucesso

- Métricas cobrem caminho positivo, supressão e falha.
- Nenhum log expõe segredo ou identificador sensível completo.
- Gates detectam labels proibidas.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `golden-signals-otel-standards` — define métricas e logs com padrão OTel/Golden Signals e cardinalidade controlada.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/budgets/...`
- [ ] `go test -race -count=1 ./internal/platform/...`
- [ ] Gate grep de labels proibidas conforme AGENTS.md/Mastra.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/application/usecases/evaluate_threshold_alerts.go`
- `internal/budgets/application/usecases/notify_threshold_alert.go`
- `internal/platform/observability`
- `internal/platform/agent`
