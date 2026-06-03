# Tarefa 10.0: Dashboard Grafana, runbook, ADR-016, AGENTS/CLAUDE, módulo AGENTS e PR template

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fecha a entrega com toda a camada de documentação, governança e enforcement de revisão: dashboard Grafana com 6 painéis padrão consumindo as métricas instrumentadas em 7.0/9.0; runbook cobrindo DLQ, re-enfileiramento, LGPD purge, kill-switch e plano de rollout 2-deploys; ADR-016 ancorada no diretório do PRD foundation; atualizações do `AGENTS.md`/`CLAUDE.md` raiz e criação do `internal/infrastructure/outbox/AGENTS.md` por módulo; e `.github/PULL_REQUEST_TEMPLATE.md` com checklist condicional Outbox.

<requirements>
- RF-05 (parte doc): critério explícito de quando usar `events.Bus` (volátil) vs `outbox.Publisher` (persistente).
- RF-16: consultas/queries para identificar quantas deliveries em DLQ por subscription_name + último erro.
- RF-17: runbook documenta re-enfileiramento manual (reset `status`, `attempts`, `next_retry_at`).
- RF-25: dashboard sugerido (6 painéis) + runbook inicial cobrindo desligar/religar Dispatcher, inspecionar DLQ, re-enfileirar delivery, purgar evento por LGPD, diagnosticar pending crescente.
- RF-27: rollout em 2 etapas (deploy 1 flag off; deploy 2 ativação após smoke staging).
- RF-29: rollback operacional < 2min via flag + restart.
- RF-30: regra de "sem segredos no payload" documentada e imposta via revisão.
- RF-32: LGPD por procedimento manual documentado.
- RF-37/D-12: ADR `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md` coexistindo com ADR-003.
- RF-38: atualização de `AGENTS.md`/`CLAUDE.md` raiz + criação de `internal/infrastructure/outbox/AGENTS.md` com referência ao README/godoc do pacote.
- RF-40: `.github/PULL_REQUEST_TEMPLATE.md` com checklist condicional Outbox + seções genéricas reaproveitáveis.
</requirements>

## Subtarefas

- [ ] 10.1 Criar `docs/observability/outbox-dashboard.json` com 6 painéis Grafana: (a) pending por `subscription_name`; (b) p95/p99 latency por `subscription_name`; (c) processed rate; (d) DLQ count + último erro por subscription; (e) idade do mais antigo pendente; (f) atividade reaper + housekeeping (contadores e gauges).
- [ ] 10.2 Validar JSON do dashboard via `jq '.' < docs/observability/outbox-dashboard.json > /dev/null` e revisar consistência de queries Prometheus contra nomes de métrica documentados (`outbox.events.published.total`, `outbox.delivery.latency_ms`, etc.).
- [ ] 10.3 Criar `docs/runbooks/outbox.md` cobrindo: (a) desligar/religar Dispatcher (`OUTBOX_DISPATCHER_ENABLED=false` + restart, RTO 2min); (b) inspecionar DLQ (queries SQL); (c) re-enfileirar delivery do DLQ (UPDATE manual com `status=pending`, `attempts=0`, `next_retry_at=now()`); (d) purgar por demanda LGPD (DELETE por `aggregate_id` com janela atual); (e) diagnosticar pending crescente (correlação dashboard + logs). Todos os snippets SQL prontos para copy-paste.
- [ ] 10.4 Criar `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md` no formato dos ADRs existentes (`adr-001` a `adr-015`): contexto, decisão, alternativas consideradas, consequências, referência explícita à ADR-003 + critério de quando cada Publisher prevalece.
- [ ] 10.5 Editar `AGENTS.md` raiz adicionando seção "Outbox vs events.Bus" com o contrato do `outbox.Publisher`, regra obrigatória de idempotência por `event_id`, e critério de escolha entre `events.Bus` (volátil) e `outbox.Publisher` (persistente).
- [ ] 10.6 Editar `CLAUDE.md` raiz com referência cruzada à mesma seção (apontando para `AGENTS.md`).
- [ ] 10.7 Criar `internal/infrastructure/outbox/AGENTS.md` no padrão por-módulo (similar a `internal/identity/AGENTS.md`/`internal/finance/AGENTS.md`) com: papel do módulo, contrato Publisher/Subscription/Handler, regra obrigatória de idempotência, link para `doc.go` e para o runbook.
- [ ] 10.8 Criar `.github/PULL_REQUEST_TEMPLATE.md` (raiz `.github/`, hoje inexistente) com seções genéricas mínimas (descrição, tipo de mudança, testes, breaking changes) + seção condicional "Outbox / Event Handler" com checklist do PRD RF-40 (idempotência, `event_id` como chave, registro em Registry, sem segredos no payload, critério `Publisher` vs `events.Bus`).
- [ ] 10.9 Validar (lint markdown opcional + revisão humana) que todos os arquivos novos têm o frontmatter/cabeçalho consistente com o restante do repositório.

## Detalhes de Implementação

Ver techspec.md seções **Monitoramento e Observabilidade → Dashboards e Runbook (RF-25)** (6 painéis listados), **→ Alertas sugeridos** (incluir referência no runbook), **Plano de Rollout (RF-27 / RF-29 / R-12)** (Deploy 1 / Deploy 2 / Rollback — copiar literal no runbook) e **Arquivos Relevantes e Dependentes → Criados** (lista de arquivos).

PRD seções **RF-16 / RF-17 / RF-25 / RF-30 / RF-32 / RF-37 / RF-38 / RF-40** consolidam o conteúdo obrigatório. ADR-016 segue convenção `.specs/prd-mecontrola-foundation/adr-NNN-titulo.md` — verificar `adr-015` como referência de formato.

## Critérios de Sucesso

- `docs/observability/outbox-dashboard.json` é JSON válido e contém 6 painéis com `datasource: Prometheus` e queries referindo as métricas exatas instrumentadas em 7.0/9.0.
- `docs/runbooks/outbox.md` contém os 5 procedimentos com SQL copy-paste testado contra schema da 0002 migration.
- `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md` referencia explicitamente `adr-003-event-bus-volatile.md` e estabelece critério.
- `AGENTS.md` raiz tem seção "Outbox vs events.Bus" com critério claro (3-5 linhas de decisão).
- `internal/infrastructure/outbox/AGENTS.md` segue padrão da família `internal/<modulo>/AGENTS.md` existente.
- `.github/PULL_REQUEST_TEMPLATE.md` tem 5 checkboxes do bloco Outbox idênticos ao listado em RF-40 + seções genéricas.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `otel-grafana-dashboards` — geração do dashboard Grafana JSON com 6 painéis sobre métricas OpenTelemetry instrumentadas no pacote outbox; gatilho exatamente do escopo da skill.

## Testes da Tarefa

- [ ] Testes unitários: não aplicável (artefatos de documentação/governança).
- [ ] Testes de integração: validação manual + JSON-lint do dashboard (`jq '.' < docs/observability/outbox-dashboard.json > /dev/null`); revisão de queries Prometheus contra nomes de métricas em `metrics.go` (assert manual via cross-reference).

**Definition of Done**:
- [ ] Dashboard JSON parseável por `jq` e referenciando exatamente as 10 métricas instrumentadas (cross-check via `grep "outbox\." docs/observability/outbox-dashboard.json`).
- [ ] Runbook cobre os 5 procedimentos com SQL pronto, sem placeholders.
- [ ] ADR-016 numerada corretamente (próxima sequencial após adr-015) e linkada na ADR-003.
- [ ] `AGENTS.md` raiz inclui seção nova; `CLAUDE.md` referencia.
- [ ] `internal/infrastructure/outbox/AGENTS.md` existe e segue padrão do projeto.
- [ ] `.github/PULL_REQUEST_TEMPLATE.md` ativo no próximo PR aberto (verificar via abertura de PR de teste em branch descartável).
- [ ] Revisão humana confirma alinhamento entre dashboard, runbook, métricas e logs.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `docs/observability/outbox-dashboard.json` (novo)
- `docs/runbooks/outbox.md` (novo)
- `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md` (novo)
- `AGENTS.md` (modificado — seção "Outbox vs events.Bus")
- `CLAUDE.md` (modificado — referência cruzada)
- `internal/infrastructure/outbox/AGENTS.md` (novo)
- `.github/PULL_REQUEST_TEMPLATE.md` (novo)
- `internal/infrastructure/outbox/metrics.go` (consumido — fonte dos nomes de métrica para o dashboard)
- `internal/infrastructure/outbox/doc.go` (consumido — referência cruzada no AGENTS de módulo)
