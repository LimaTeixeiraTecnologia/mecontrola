# C-6 — Load test k6 do módulo card (implementação)

# Generated: 2026-06-09T21:27:00Z

## Resumo

Materializados os 6 scripts k6 referenciados pelo `Taskfile.yml` (`loadtest:card*`)
e a documentação operacional. Não havia o diretório `loadtest/`; toda a estrutura
foi criada do zero. Execução real contra serviço com volumetria de SLO
(M-02 1.000 RPS / M-03 200 RPS / M-04 200 RPS) requer ambiente de homologação com
app, Postgres com migrations aplicadas e usuário de teste — ficou explicitamente
pendente.

## Arquivos criados

Produção (loadtest):
- `loadtest/README.md` — guia operacional PT-BR (pré-requisitos, env vars, ordem
  de execução, interpretação, exportação de evidência).
- `loadtest/card/common.js` — helpers compartilhados (`BASE_URL`, headers,
  `Idempotency-Key`, paths configuráveis via `RESULTS_DIR`/`STATE_DIR`).
- `loadtest/card/setup.js` — semeia N cartões (`SEED_COUNT`, default 20); emite
  IDs em `state/cards.json` e `results/setup-<ts>.json` via `handleSummary`.
- `loadtest/card/m02_post_create.js` — `constant-arrival-rate` 1.000 RPS / 60s,
  `preAllocatedVUs=200`, `maxVUs=500`; thresholds
  `http_req_duration{op:create}: p(99)<300`, `http_req_failed{op:create}: rate<0.005`.
- `loadtest/card/m03_get_list.js` — 200 RPS / 60s, `limit=100`; threshold
  `http_req_duration{op:list}: p(99)<50`.
- `loadtest/card/m04_invoice_for.js` — 200 RPS / 60s; threshold `p(99)<60` para a
  request HTTP fim-a-fim. O SLO puro `<10ms` do PRD refere-se ao cálculo
  `InvoiceFor` em memória e deve ser observado via métrica custom no dashboard
  "Card Module" (Tarefa 9.0). A decisão e o raciocínio estão documentados no
  cabeçalho do script e no `README.md`.
- `loadtest/card/mixed.js` — 300 RPS / 120s, mix 70% list / 20% invoice / 10% POST.
- `loadtest/card/teardown.js` — DELETE em loop dos cartões do `state/cards.json`.
- `loadtest/card/results/.gitkeep` (implícito via dir).
- `loadtest/card/state/.gitkeep` (implícito via dir).

Evidência:
- `.specs/prd-card-crud-mvp/loadtest-evidence/c6-implementation-report.md` (este).

## Decisões de implementação

1. **Header de autenticação**: middleware `InjectPrincipalFromHeader` (lido em
   `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go:13`)
   exige `X-User-ID`. Scripts enviam exatamente este header. Não há `Authorization:
   Bearer` no MVP (S-07 — gateway autenticando — é responsabilidade externa).
2. **Schema do POST**: `name`, `nickname`, `closing_day`, `due_day` conforme
   `internal/card/infrastructure/http/server/handlers/create.go:23-28`.
3. **Idempotência**: `Idempotency-Key = <prefix>-<scope>-<__VU>-<__ITER>-<Date.now()>`
   garante unicidade entre VUs/iterações sem precisar de UUID externo.
4. **Persistência de IDs entre setup/m04/mixed/teardown**: k6 não escreve arquivos
   em tempo de execução; usamos `handleSummary` para emitir `state/cards.json` no
   path montado (`-v ./loadtest:/loadtest` no Taskfile). Documentado no README.
5. **Threshold de M-04**: 10ms se aplica ao cálculo puro do `InvoiceFor`
   (decidido em PRD §"Métricas de Sucesso"). Como a request HTTP soma rede +
   middleware + logging, o threshold do script é 60ms. Regressão real do
   algoritmo aparece em ambos os pontos.
6. **Paths configuráveis**: `RESULTS_DIR`/`STATE_DIR` permitem execução nativa
   (sem docker mount), preservando o comportamento default `/loadtest/...`
   esperado pelo container `grafana/k6`.

## Comandos executados

- `ai-spec skills check` → 6 skills verificadas, sem drift bloqueante.
- `ai-spec check-spec-drift .specs/prd-card-crud-mvp/tasks.md` → `OK: sem drift detectado`.
- `go build -o /tmp/mecontrola-server ./cmd/server` → `BUILD OK`.
- `k6 inspect` em cada um dos 6 scripts → todos retornaram JSON válido com
  scenarios, thresholds e tags conforme planejado.
- `k6 run` smoke (2-3s, BASE_URL inválido) em `setup.js`, `m03_get_list.js`,
  `m02_post_create.js` → execução percorre default function, `handleSummary`
  emite arquivos esperados (`results/`, `state/cards.json`). Thresholds falham
  como esperado (dial refused), confirmando que o gate de regressão funciona.

## Validações

- Parse: 6/6 scripts validados via `k6 inspect`.
- Smoke: 3/6 scripts executados nativamente por 2-3s; output e arquivos
  produzidos conforme esperado.
- Build: `go build ./cmd/server` OK (sem alteração em código Go).
- Lint / gates Go: N/A — sem alteração em `*.go`.

## Pendente (entregar fora do escopo de C-6)

- Subir app `mecontrola` em homologação (host real, Postgres, migrations).
- Rodar `task loadtest:card:setup` → `task loadtest:card` → coletar resultados.
- Capturar screenshots do dashboard "Card Module" durante a execução.
- Anexar evidências verdes (`results/m02-*.json`, `m03-*.json`, `m04-*.json`) em
  `.specs/prd-card-crud-mvp/loadtest-evidence/<YYYY-MM-DD>/`.
- Gerar `loadtest/card/reports/<YYYY-MM-DD>.md` conforme `requirements` do task
  10.0 (item de "Definition of Done" remanescente).

## Riscos residuais

- `host.docker.internal` (default `BASE_URL`) só funciona em Docker Desktop
  (macOS/Windows). Linux exige `--add-host=host.docker.internal:host-gateway` ou
  override de `BASE_URL`. Documentado no README.
- 1.000 RPS local tipicamente exige `ulimit -n` elevado e Postgres tuned;
  execução fora de homologação dimensionada pode gerar falsos negativos.
- `state/cards.json` é commit-sensível — recomendado adicionar a `.gitignore` em
  follow-up (não bloqueia C-6).
