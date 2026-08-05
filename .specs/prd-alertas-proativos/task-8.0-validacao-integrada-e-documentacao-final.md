# Tarefa 8.0: Validação integrada e documentação final

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Executar validação integrada do Release 1, atualizar documentação operacional e confirmar que todos os RF/REQ estão cobertos sem lacunas.

<requirements>
- Cobrir todos os RF-01 a RF-18 e REQ-01 a REQ-25.
- Rodar gates Go, budgets, notification, onboarding e Mastra proporcionais ao diff.
- Atualizar SDD/runbook com estado final real.
- Não declarar envio real pronto se templates Release 1 ainda não estiverem `APPROVED`.
</requirements>

## Subtarefas

- [ ] 8.1 Rodar testes direcionados dos pacotes alterados.
- [ ] 8.2 Rodar build/vet/race no escopo integrado.
- [ ] 8.3 Rodar gates Mastra quando `internal/agents` ou `internal/platform/agent|memory` forem tocados.
- [ ] 8.4 Rodar `ai-spec check-spec-drift .specs/prd-alertas-proativos`.
- [ ] 8.5 Atualizar SDD e spec Meta com status real de templates.
- [ ] 8.6 Registrar evidências de dry-run e readiness por kind.

## Detalhes de Implementação

Referenciar todo `techspec.md`, principalmente `Abordagem de Testes`, `Sequenciamento de Desenvolvimento` e `Conformidade com Padrões`.

## Critérios de Sucesso

- `ai-spec check-spec-drift` sem drift.
- Todos os RF/REQ aparecem em tasks e testes.
- Nenhum gate obrigatório falha sem diagnóstico.
- Documentação final reflete o estado real de Meta e dry-run.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — valida gates agentivos quando follow-up ou runtime forem alterados.
- `domain-modeling-production` — revisa invariantes finais de domínio e ausência de estado ilegal.
- `design-patterns-mandatory` — confirma que a decisão de pattern continua válida após o diff.

## Testes da Tarefa

- [ ] `go build ./internal/budgets/... ./internal/agents/... ./internal/platform/notification/... ./internal/onboarding/...`
- [ ] `go vet ./internal/budgets/... ./internal/agents/... ./internal/platform/notification/... ./internal/onboarding/...`
- [ ] `go test -race -count=1 ./internal/budgets/... ./internal/agents/... ./internal/platform/notification/... ./internal/onboarding/...`
- [ ] `ai-spec check-spec-drift .specs/prd-alertas-proativos`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.specs/prd-alertas-proativos/prd.md`
- `.specs/prd-alertas-proativos/techspec.md`
- `.specs/prd-alertas-proativos/tasks.md`
- `docs/refin/2026-08-05-sdd-alertas-proativos.md`
- `docs/refin/2026-08-05-meta-templates-alertas-proativos.md`
