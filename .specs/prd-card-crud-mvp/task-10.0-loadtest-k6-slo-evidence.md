# Tarefa 10.0: Load test k6 + evidência de SLO M-02/M-03/M-04

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar evidência das métricas de sucesso do PRD via scripts k6 em `loadtest/card/`. Cobre M-02 (POST `/api/v1/cards` p99 ≤ 300 ms em até 1.000 RPS), M-03 (GET `/api/v1/cards` p99 ≤ 50 ms em 100 itens/página), M-04 (`InvoiceFor` p99 ≤ 10 ms via `GET /api/v1/cards/{id}/invoices?for=...`). Resultados publicados em dashboard "Card Module" + relatório de execução commitado para auditoria.

<requirements>
- Scripts k6 em JavaScript (`loadtest/card/*.js`).
- 3 cenários separados (um por SLO) + scenário misto para regressão.
- Thresholds k6 nativos validam SLO; falha de threshold = exit code != 0 (gate de CI opcional).
- Ambiente de execução: homologação (não produção). DSN, host e credenciais via env vars; sem hardcode.
- Cada script semeia preparatoriamente os dados que precisar (usuários, cartões base) ou reusa fixture inicial determinística.
- Idempotência respeitada: cada POST usa `Idempotency-Key` único (`uuid()` k6).
- Relatório final (`loadtest/card/reports/<data>.md`) inclui: percentis p50/p95/p99, throughput observado, total de requests, taxa de erro, comparação com SLO declarado, screenshots/links do dashboard.
- Documentar pré-requisitos: instância de homologação, capacidade do Postgres, baseline de tráfego.
- Container oficial k6 reusável em CI manual (`task loadtest:card`).
- Evidência de M-02/M-03/M-04 verde anexada ao PR final.
</requirements>

## Subtarefas

- [ ] 10.1 `loadtest/card/setup.js` — bootstrap (cria N usuários teste via API ou direto no banco; cria 3 cartões por usuário).
- [ ] 10.2 `loadtest/card/m02_post_create.js` — 1.000 RPS pico em janela curta; threshold `http_req_duration{scenario:post}: p(99)<300`.
- [ ] 10.3 `loadtest/card/m03_get_list.js` — 50 RPS médio com paginação `limit=100`; threshold `http_req_duration{scenario:list}: p(99)<50`.
- [ ] 10.4 `loadtest/card/m04_invoice_for.js` — 200 RPS; threshold `http_req_duration{scenario:invoice_for}: p(99)<10`.
- [ ] 10.5 `loadtest/card/mixed.js` — cenário misto 50/30/20 (list/get/invoice_for) por 5 min.
- [ ] 10.6 `loadtest/card/teardown.js` — cleanup dos dados teste (DELETE em loop ou `migrate down`/`migrate up` em ambiente descartável).
- [ ] 10.7 Task `loadtest:card` no `Taskfile.yml`.
- [ ] 10.8 Executar suite em homologação e gerar relatório `loadtest/card/reports/<YYYY-MM-DD>.md`.
- [ ] 10.9 Validar painéis do dashboard "Card Module" (Tarefa 9.0) durante execução; capturar screenshots.

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/prd.md` §"Métricas de Sucesso" (M-02..M-04) e §"Volumetria-alvo e SLO do MVP". Decisão da ferramenta em techspec §"Decisões tácticas resolvidas" e em ADR/inline (k6).

## Critérios de Sucesso

- 3 SLO (M-02, M-03, M-04) verificados com thresholds k6 verdes.
- Relatório `loadtest/card/reports/<data>.md` commitado.
- 0 erro inesperado (rate de erro < 0.5%) durante execução em homologação.
- Dashboard exibe métricas em tempo real e os screenshots batem com SLO declarado.
- Pré-condições de ambiente documentadas (PRD S-07 — gateway autenticando — registrado no relatório).

### Definition of Done

- [ ] Scripts k6 criados e versionados.
- [ ] Task `loadtest:card` funcional (execução manual).
- [ ] Relatório committado.
- [ ] 3 thresholds k6 verdes na última execução.
- [ ] Screenshots/links do Grafana anexados ao PR.
- [ ] M-02, M-03, M-04 explicitamente marcados como atendidos no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: N/A (scripts k6 em JS; validação por thresholds nativos).
- [ ] Testes de integração: execução fim-a-fim em homologação com Postgres real e binário wired.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `loadtest/card/setup.js` (novo)
- `loadtest/card/m02_post_create.js` (novo)
- `loadtest/card/m03_get_list.js` (novo)
- `loadtest/card/m04_invoice_for.js` (novo)
- `loadtest/card/mixed.js` (novo)
- `loadtest/card/teardown.js` (novo)
- `loadtest/card/reports/<data>.md` (novo)
- `Taskfile.yml` (modificar — task `loadtest:card`)
- `docs/grafana/card-module.json` (referência — gerado em 9.0)
