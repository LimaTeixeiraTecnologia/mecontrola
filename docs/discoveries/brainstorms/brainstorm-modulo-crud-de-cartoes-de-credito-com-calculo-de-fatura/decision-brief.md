# DECISION BRIEF

**Título**: Módulo CRUD de Cartões de Crédito com Cálculo de Fatura
**Repositório**: `mecontrola` (Go)
**Data**: 2026-06-06
**Idioma**: pt-BR

## Problema
O `mecontrola` precisa de uma entidade dedicada para cartões de crédito do usuário antes que o módulo de transações seja construído. Hoje não há agregado de cartão e a regra de "em qual fatura uma compra cai" não está formalizada em lugar nenhum do código. Sem essa fundação:
- O módulo de transações fica bloqueado.
- Qualquer hack temporário em categoria/conta vira dívida que exigirá recalculo do histórico financeiro depois.
- A regra de ciclo de fatura precisa ser robusta, resiliente e eficiente desde o primeiro registro persistido.

## Objetivo
Entregar MVP **production-ready/proof inegociável** contendo:
1. CRUD HTTP de cartões: criar, listar, obter, atualizar, soft-delete.
2. Função pura `InvoiceFor(purchaseDate, cycle, tz) → Invoice{closingDate, dueDate}` no domínio.
3. Endpoint público de consulta de fatura: `GET /cards/:id/invoices?for=<date>`.
4. Aderência total a R0–R7 do `AGENTS.md`, sem flexibilização.
5. Cobertura por testes table-driven com ≥ 50 fixtures + property-based.

## Escopo Inicial
Inclui:
- Bounded context `internal/card` com `domain/`, `usecase/`, `adapter/` (Postgres), `handler/` (HTTP), `wiring/`.
- Agregado `Card{ID, UserID, Name, Nickname, ClosingDay, DueDay, CreatedAt, UpdatedAt, DeletedAt}`.
- Serviço de domínio puro `BillingCycle.InvoiceFor(purchase time.Time, tz *time.Location) Invoice`.
- Migration Postgres com unicidade parcial `(user_id, nickname) WHERE deleted_at IS NULL`.
- Idempotência via header `Idempotency-Key` em POST/PUT/DELETE (TTL 24h).
- Observabilidade: logs estruturados com `trace_id` + OTel spans em domain/adapter.
- Contrato HTTP OpenAPI publicado.

Exclui:
- Limite de crédito, saldo disponível.
- Bandeira (`brand`) e `last4`.
- Multi-titular / cartão adicional.
- Integração Open Finance / Belvo / Pluggy.
- Emissor (banco) — `mecontrola` não modela emissor no MVP.
- Audit log de domínio (`card_audit_log`).
- Versionamento histórico de ciclos por cartão.
- Métricas Prometheus dedicadas (logs + traces suficientes no MVP).
- Recálculo retroativo de faturas.

## Restrições
- Stack Go obrigatória conforme `AGENTS.md`: R0 (sem `init()`), R5.12 (sem `panic` em prod), R6 (`context.Context` em IO, interface no consumidor), R7.6 (`errors.Join`, `fmt.Errorf("ctx: %w", err)`).
- LGPD + PCI-DSS 4.0: aplicação não-PCI; proibido armazenar PAN completo, CVV/CVC, trilha magnética, PIN.
- Padrão Obrigatório de Módulo (igual a `internal/identity` e `internal/billing`) — sem inventar wiring, routers, jobs ou consumers fora desse padrão.
- Cálculo canônico em `America/Sao_Paulo`; persistência em UTC.
- Histórico financeiro imutável: alteração de ciclo NÃO recalcula transações antigas.

## Hipóteses
- H1 (confirmada): Ciclo de fatura é definido pelo emissor, não pela bandeira. Evidência: Visa Core Rules + Mastercard Rules + BACEN Res. 4.658/2018 + BCB 468/2025 (nenhuma fixa `closing_day`).
- H2 (confirmada): `mecontrola` é não-PCI e só armazena dados não-sensíveis. Evidência: PCI-DSS 4.0 + LGPD art. 6º e 46.
- H3 (confirmada): regra do usuário é monotônica e determinística (compra antes do fechamento vence no próximo `due_day`; senão, no `due_day` seguinte).
- H4 (confirmada): clamp `min(day, daysInMonth)` cobre fev/29 e meses de 30 dias sem precisar de spillover.

## Alternativas Avaliadas

### Alternativa 1 - Função pura InvoiceFor + clamp + tests exaustivos
Resumo: Função pura, stateless, O(1), sem IO, em `internal/card/domain`. Algoritmo: converte `purchase_date` para `America/Sao_Paulo`, calcula `closing_date` e `due_date` do mês de referência usando clamp `min(day, daysInMonth(year, month))`, decide convenção `closing_day > due_day` (fechamento mês anterior) vs `closing_day < due_day` (mesmo mês) automaticamente, e empurra para o próximo ciclo se `purchase.date() > closing_date.date()`. Cobertura por 50+ fixtures table-driven + property-based tests (`testing/quick` ou fuzz).

Viabilidade:
- Técnica: trivial em Go puro com `time.Time` e `time.LoadLocation`. Aderente a R0–R7.
- Operacional: nenhuma infraestrutura adicional além de Postgres existente.
- Financeira: ~6 dias de desenvolvimento de uma pessoa sênior.

### Alternativa 2 - Pré-computação materializada (invoices table)
Resumo: Ao criar/editar cartão, gera N registros futuros em `invoices(card_id, period, closing_at, due_at)`. Lookup vira SQL. Exige job de rollover (cron) para gerar próximos meses.

Viabilidade:
- Técnica: viável, mas adiciona job, migration extra, sincronização entre regra e dados.
- Operacional: cron precisa de monitoramento; falha silenciosa = compras "sem fatura".
- Financeira: ~9 dias; risco de drift entre fonte de verdade da regra e da tabela.

### Alternativa 3 - Híbrido função pura + materialized view Postgres
Resumo: Cálculo runtime + view materializada `next_invoices` para dashboards. REFRESH a cada UPDATE em `cards`.

Viabilidade:
- Técnica: viável, mas materialização precoce sem caso de uso real comprovado no MVP.
- Operacional: REFRESH síncrono em trigger ou job assíncrono adicional.
- Financeira: ~8 dias; ganho marginal para escala MVP.

### Alternativa 4 - Biblioteca de recorrência (RRULE/rrule-go)
Resumo: Modela ciclo como recorrência RFC 5545. Reusa motor robusto.

Viabilidade:
- Técnica: viável, mas RRULE não expressa diretamente "compra X cai em fatura Y" — exige adapter custom que anula o ganho.
- Operacional: dependência externa para regra trivial.
- Financeira: ~6 dias; lock-in em biblioteca de terceiros.

## Trade-offs
- **Alternativa A escolhida**: aceita ~6 dias para blindar testes exaustivos (vs 5 dias com cobertura padrão). Justificativa: "production-ready inegociável".
- **Histórico imutável**: aceita não permitir recalculo retroativo automático. Mudança de ciclo só vale para transações futuras. Justificativa: integridade contábil > flexibilidade.
- **Cálculo inline na criação da transação + denormalização**: aceita persistir `closing_date`/`due_date` no agregado Transaction (futuro módulo). Justificativa: O(1) consulta vs O(n) recalculo a cada relatório.
- **Endpoint público de consulta de fatura**: aceita expor contrato HTTP antes do módulo de transações existir. Justificativa: front pode preparar UI sem workaround.
- **Sem audit log de domínio no MVP**: aceita risco em troca de tempo. Mitigado por logs estruturados + traces OTel.

## Riscos
- Risco: Bug em edge case raro de calendário (ex.: fev/29 em ano bissexto secular).
  Impacto: compra cai em fatura errada — quebra financeira do usuário.
  Probabilidade: baixa após robustez extrema.
  Mitigação: 50+ fixtures table-driven + property-based tests; revisar regra de clamp manualmente; auditar saída em ambiente de homologação contra calendário oficial 2024–2030.

- Risco: Mudança futura na regulação BACEN/BCB que altere ciclo de fatura.
  Impacto: necessidade de migration / versionamento de ciclos.
  Probabilidade: baixa no curto prazo.
  Mitigação: design permite adicionar `card_billing_cycles` histórico em fase 2 sem quebra de contrato.

- Risco: Drift de timezone se DST hipotético retornar ao Brasil.
  Impacto: cálculo borderline (compra meia-noite BRT) cai no dia errado.
  Probabilidade: baixa.
  Mitigação: `time.LoadLocation("America/Sao_Paulo")` respeita zoneinfo do SO; cobertura de teste com datas históricas onde DST existia (até 2019).

- Risco: Uso indevido por outro módulo Go importando `domain` em vez de `usecase`.
  Impacto: quebra de fronteira DDD.
  Probabilidade: média.
  Mitigação: porta pública explícita `CardLookup`/`BillingCycleService` em pacote `port/`; linter de fronteira (`go-cleanarch` ou import-boundaries).

## Custos
Estimativa relativa: **baixa-média** (~6 dias de uma pessoa sênior Go).

Drivers de custo:
- Cobertura de testes table-driven + property-based (~1,5 dias dedicado).
- Wiring + adapter Postgres seguindo padrão de `internal/identity` (~1 dia).
- Handler HTTP + middleware de idempotência (~1 dia).
- Observabilidade (logs + OTel spans, ~0,5 dia).
- Domain + InvoiceFor (~1 dia).
- Migrations + revisão final (~1 dia).

## Impactos Operacionais
- Nova migration Postgres (`cards` + `card_idempotency_keys`).
- Sem novos serviços/jobs/consumers — apenas novos endpoints em servidor HTTP existente.
- Rollback: drop migration + revert handler — trivial.
- Operação: zero infraestrutura adicional; observabilidade via stack OTel existente.

## Segurança
- Nenhum dado sensível (PAN/CVV/trilha) é coletado, transitado ou persistido.
- Autorização: cartões pertencem a `user_id`; toda query usa `user_id` do contexto de autenticação (assumido herdado de `internal/identity`).
- Auditoria: logs estruturados + traces OTel cobrem create/update/delete; audit log dedicado entra em fase 2.
- LGPD: dado pessoal mínimo (apelido + nome do cartão). Base legal: execução de contrato + legítimo interesse.

## Observabilidade
- Logs estruturados JSON com `trace_id`, `user_id`, `card_id`, `operation`, `duration_ms`.
- OTel spans: `card.handler.<op>`, `card.usecase.<op>`, `card.domain.InvoiceFor`, `card.adapter.pg.<query>`.
- Diagnóstico de incidentes: trace completo de qualquer chamada CRUD ou consulta de fatura.
- Métricas Prometheus dedicadas (counters/histograms) entram em fase 2 — telemetria atual cobre necessidade do MVP.

## Escalabilidade
- `InvoiceFor` é O(1) e stateless — paraleliza linearmente sem coordenação.
- CRUD: tabela `cards` cresce O(N) com `users × cartões_por_usuário`; sem hot path crítico.
- Idempotência: TTL 24h limita crescimento de `card_idempotency_keys`; job de limpeza em fase 2 se necessário.

## Alternativa Recomendada
Função pura InvoiceFor + clamp + tests exaustivos, encapsulada em bounded context próprio `internal/card`.

## Justificativa
- Pontuação 44/45 no scorecard (Alternativa B = 26, C = 31, D = 28).
- Aderência total ao Padrão Obrigatório de Módulo do `AGENTS.md`.
- Zero dependências externas adicionais.
- Função pura é determinística, auditável, reentrante e reutilizável por todos os módulos consumidores (transações, relatórios, jobs).
- Robustez via cobertura exaustiva de testes em vez de complexidade de infraestrutura.
- Histórico financeiro imutável é garantido por desenho.
- Custo de implementação: ~6 dias — compatível com expectativa "MVP production-ready/proof".

## Decisões Pendentes
- Estratégia exata de property-based testing (escolha entre `testing/quick`, `go-fuzz`, ou `gopter`) — decisão técnica que entra no discovery técnico.
- Formato exato do payload OpenAPI (`Card` resource representation) — entra no discovery técnico.
- Política de versionamento de schema HTTP (`/v1/cards` vs `/cards`) — verificar convenção atual do repositório.
- Confirmar se idempotência reutilizará tabela genérica existente em `internal/identity`/`internal/billing` ou se `card` terá tabela própria.

## Próximo Passo Recomendado
`technical-discovery-production` consumindo este bundle para detalhar: schema PostgreSQL final, contratos OpenAPI, ADRs, plano de testes (table-driven + property-based), estratégia de migração e observabilidade. Em seguida, `create-prd` produzirá o PRD do módulo.
