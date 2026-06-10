# Load tests — módulo `card`

Suite k6 que materializa evidência das métricas de sucesso do PRD `prd-card-crud-mvp`:

| SLO  | Endpoint                                           | Meta                          | Script                  |
|------|----------------------------------------------------|-------------------------------|-------------------------|
| M-02 | `POST /api/v1/cards`                               | p99 ≤ 300 ms @ 1.000 RPS      | `card/m02_post_create.js` |
| M-03 | `GET /api/v1/cards?limit=100`                      | p99 ≤ 50 ms @ 200 RPS         | `card/m03_get_list.js`    |
| M-04 | `GET /api/v1/cards/{id}/invoices?for=YYYY-MM-DD`   | p99 ≤ 10 ms in-memory (ver §) | `card/m04_invoice_for.js` |
| Misto | combinação 70/20/10                               | p99 ≤ 400 ms @ 300 RPS        | `card/mixed.js`           |

## Pré-requisitos

- Aplicação `mecontrola` rodando em ambiente de homologação acessível.
- Postgres com migrations aplicadas (`task migrate:up`).
- Usuário de teste com UUID conhecido (passado via `X_USER_ID`); middleware
  `InjectPrincipalFromHeader` exige `X-User-ID` válido.
- Uma das opções a seguir:
  - Docker (preferido) — usa imagem oficial `grafana/k6`.
  - `k6` instalado localmente (`brew install k6`).

## Variáveis de ambiente

| Variável                  | Default                                | Descrição                                    |
|---------------------------|----------------------------------------|----------------------------------------------|
| `BASE_URL`                | `http://host.docker.internal:8080`     | URL base da API.                              |
| `X_USER_ID`               | (obrigatória)                          | UUID v4 enviado em `X-User-ID`.               |
| `IDEMPOTENCY_KEY_PREFIX`  | `k6-loadtest`                          | Prefixo aplicado em `Idempotency-Key`.        |
| `DURATION`                | `60s`                                  | Duração dos cenários M-02/M-03/M-04.          |
| `MIXED_DURATION`          | `120s`                                 | Duração do cenário misto.                     |
| `RATE`, `PRE_VUS`, `MAX_VUS` | depende do script                   | Override de carga por cenário.                |
| `SEED_COUNT`              | `20`                                   | Cartões criados pelo `setup.js`.              |
| `CARD_IDS`                | (lista CSV)                            | Fallback se `state/cards.json` ausente.       |
| `INVOICE_FOR`             | data de hoje                           | Data alvo para M-04 (`YYYY-MM-DD`).           |

## Ordem de execução recomendada

```bash
export BASE_URL=https://card-hml.mecontrola.io
export X_USER_ID=11111111-1111-4111-8111-111111111111

# 1. Semear cartões base (gera state/cards.json)
task loadtest:card:setup

# 2. Rodar os 3 cenários SLO
task loadtest:card        # executa M-02, M-03 e M-04 em sequência

# 3. Cenário misto (opcional, regressão)
task loadtest:card:mixed

# 4. Limpeza
task loadtest:card:teardown
```

## Interpretando resultados

- Cada script imprime no stdout um sumário com p99 medido e o threshold do SLO.
- Resultado bruto JSON é gravado em `loadtest/card/results/<test>-<timestamp>.json`
  via `handleSummary`.
- Exit code ≠ 0 indica violação de threshold (gate de CI).

### Nota sobre M-04

O PRD define `InvoiceFor` como cálculo puro com p99 ≤ 10 ms. A request HTTP inclui
overhead de rede, parsing e middlewares, então o threshold do script é p99 ≤ 60 ms
(http req duration end-to-end). A latência pura do cálculo deve ser observada via
métrica custom no dashboard "Card Module" (tarefa 9.0). Regressões funcionais que
empurram o cálculo acima de 10 ms aparecem em ambos os pontos.

### Sobre o `setup.js` e persistência de IDs

`k6` não escreve arquivos arbitrários em runtime. A função `handleSummary` exporta
`state/cards.json` e `results/setup-<ts>.json` para o caminho montado no container.
Quando rodando via Docker (`task loadtest:card:setup`), o bind mount
`-v ./loadtest:/loadtest` garante a persistência local.

## Exportar evidência para o PRD

Após uma execução verde em homologação:

```bash
ts=$(date +%Y-%m-%d)
mkdir -p .specs/prd-card-crud-mvp/loadtest-evidence/${ts}
cp loadtest/card/results/m0{2,3,4}-*.json \
   .specs/prd-card-crud-mvp/loadtest-evidence/${ts}/
# Anexar screenshots do dashboard "Card Module" + relatório em
# loadtest/card/reports/<ts>.md
```
