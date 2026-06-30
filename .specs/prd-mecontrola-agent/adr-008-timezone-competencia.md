# Registro de Decisão Arquitetural (ADR-008)

## Metadados

- **Título:** Timezone de negócio America/Sao_Paulo para data-default e competência
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-21.1, D-20), techspec.md; `internal/transactions/domain/valueobjects/ref_month.go:10`; `internal/transactions/domain/valueobjects/card_billing_snapshot.go:40`; memória `feedback_no_time_abstraction`

## Contexto

Quando o usuário registra um gasto sem data, o agente assume "hoje" (RF-21.1/D-20). A data obrigatória (`occurredAt`/`purchasedAt`) determina a competência (`ref_month`). O domínio declara intenção America/Sao_Paulo: `ref_month.go:10` — *"esperado YYYY-MM em America/Sao_Paulo"* — e o teste `ref_month_test.go` usa `time.LoadLocation("America/Sao_Paulo")`. Porém `card_billing_snapshot.go:40` chama `RefMonthFromTime(purchasedAt, time.UTC)` — **inconsistência pré-existente**. Para um usuário BR (UTC-3), derivar "hoje"/competência em UTC pode jogar um gasto da noite para o mês/dia seguinte.

## Decisão

O `MeControlaAgent` deriva "hoje" e a competência convertendo `time.Now().UTC()` para **`America/Sao_Paulo`** no ponto de uso (inline, sem abstrair tempo — memória `feedback_no_time_abstraction`), alinhado à intenção declarada do domínio (`ref_month.go:10`). A inconsistência de `card_billing_snapshot.go` (UTC) **não** é corrigida neste escopo (mudaria comportamento de `internal/transactions`/`internal/card` fora do PRD); fica **sinalizada como risco** e candidata a ADR futura no módulo de transações.

## Alternativas Consideradas

- **UTC** — simples e já usado em `card_billing_snapshot`. Desvantagem: competência/data errada perto da meia-noite para o usuário BR; contraria a intenção declarada de `ref_month.go`. Rejeitada.
- **Timezone configurável por usuário** — correto para multi-região, mas o produto é BR-first e não há campo de timezone por usuário hoje. Rejeitada (overkill no MVP).
- **Corrigir o domínio (card para America/Sao_Paulo) neste PRD** — fora de escopo; mudaria contrato de outro módulo. Rejeitada; registrada como follow-up.

## Consequências

### Benefícios Esperados

- Competência/data corretas para o usuário BR; consistência com a intenção declarada do domínio.

### Trade-offs e Custos

- Convergência incompleta enquanto `card_billing_snapshot` permanecer em UTC (risco conhecido).

### Riscos e Mitigações

- **Divergência agente (SP) × card (UTC)** em compras de cartão perto da virada → sinalizar e propor follow-up de unificação no domínio; documentar a semântica adotada.
- **DST/fuso** → usar `time.LoadLocation("America/Sao_Paulo")` (trata regras vigentes), não offset fixo.

## Plano de Implementação

1. Helper inline de derivação de data/competência em America/Sao_Paulo nas tools de escrita e nos steps de onboarding.
2. Testes com horários de virada (ex.: 23:30 BRT) validando a competência.
3. Registrar follow-up de unificação de timezone no domínio de transações.

## Monitoramento e Validação

- Testes determinísticos de borda; auditoria pontual de competência em lançamentos noturnos.

## Impacto em Documentação e Operação

- Runbook: nota sobre timezone de negócio e a divergência conhecida no card.

## Revisão Futura

- Revisar quando o domínio unificar o timezone ou se o produto expandir para fora do BR.
