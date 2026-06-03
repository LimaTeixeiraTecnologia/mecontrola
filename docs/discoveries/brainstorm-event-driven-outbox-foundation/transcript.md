# Transcript do Brainstorming Decisório

## Contexto Inicial
- Origem: prompt em `docs/prompts/event-driven-outbox-brainstorming.md` enriquecido pela skill `prompt-enricher`.
- Pedido: definir arquitetura de mensageria interna reativa para o monólito Go MeControla, baseada no padrão Transactional Outbox, sem broker externo, com dispatcher + DLQ + housekeeping + suporte a múltiplas instâncias.
- Stack confirmada no repositório:
  - Monólito Go 1.26.3 com CLI Cobra (`cmd/server`, `cmd/worker`, `cmd/migrate`).
  - PostgreSQL via `pgx/v5` (suporta `FOR UPDATE SKIP LOCKED` e `LISTEN/NOTIFY`).
  - Migrations via `golang-migrate`.
  - `cmd/worker` atualmente é idle (`worker idle — aguardando jobs`), pronto para hospedar dispatcher + cron.
  - Testes de integração com `testcontainers-go/modules/postgres`.
  - OpenTelemetry SDK 1.44 disponível para instrumentar métricas/tracing.

## Rodada 1 - Entendimento do Problema

### Pergunta 1.1 — Problema real
- Opções:
  - A) Garantir entrega at-least-once sem perder consistência transacional.
  - B) Desacoplar handlers/projeções/integrações sem broker externo no MVP.
  - C) Habilitar retries + DLQ para integrações externas que falham silenciosamente.
  - D) **Todas as anteriores — Outbox como fundação multi-propósito da plataforma.**
- Resposta: **D**.
- Implicação: a decisão é de plataforma; critérios não-funcionais (atomicidade, extensibilidade, resiliência) têm peso igual.

### Pergunta 1.2 — Resultado mínimo de sucesso
- Opções:
  - A) Caso de uso real ponta a ponta no MVP.
  - B) Biblioteca/infra pronta, sem caso de uso acoplado.
  - C) **Outbox + dispatcher + DLQ + cron rodando em produção, mesmo com handler dummy.**
  - D) Apenas decisão documentada + schema migrado, implementação iterativa depois.
- Resposta: **C**.
- Implicação: o ciclo completo (publish → poll → deliver → retry → DLQ → housekeeping) precisa estar provado operacionalmente; handler real pode vir depois, mas o pipeline tem que estar vivo.

### Pergunta 1.3 — Risco de adiar
- Opções:
  - A) **Acúmulo de dívida — side-effects ad-hoc (goroutine fire-and-forget, HTTP inline) difíceis de migrar.**
  - B) Perda real de dados em produção por side-effect crítico falhando.
  - C) Bloqueio de roadmap de features dependentes.
  - D) Baixo — adiar é seguro.
- Resposta: **A**.
- Implicação: decisão é preventiva/estrutural; a urgência é estabelecer o padrão canônico antes que features novas reinventem soluções ad-hoc. Não há incidente em curso forçando shortcut.

### Pergunta 1.4 — Volumetria alvo (1º ano)
- Opções:
  - A) Baixa: <10 eventos/s, <100k/dia.
  - B) **Média: 10–100 eventos/s, ~1M/dia, p95 < 1s.**
  - C) Alta: >100 eventos/s, >10M/dia, p99 sub-segundo.
  - D) Desconhecida — assumir baixa e instrumentar.
- Resposta: **B**.
- Implicação: polling puro com intervalo grande não atende; precisa polling agressivo (centenas de ms) + batching + índices adequados. Latência alvo `< 1s p95` define orçamento de polling.

## Rodada 2 - Escopo e Restrições

### Pergunta 2.1 — Cardinalidade de consumo
- Opções: 1×1 / **1×N in-process** / 1×N com tabela de deliveries / 1×1 com schema preparado.
- Resposta: **1×N in-process**.
- Implicação: registry mapeia `event_type → []Handler`. Como há múltiplos handlers por evento, idempotência precisa ser por par (event_id, handler) — o que combinado com a restrição "simplicidade operacional" cria tensão; precisa ser resolvida na Rodada 3 (single-table com bitmap/status agregado vs. two-table `outbox_events` + `outbox_deliveries`).

### Pergunta 2.2 — Fora de escopo (múltipla escolha)
- Opções marcadas: **Broker externo + integrações HTTP/webhooks externos**, **Ordenação global FIFO entre event_types**.
- Não marcadas (interpretadas como possivelmente in-scope): CDC/Debezium (descartado por coerência com restrição "Postgres fixo" + simplicidade — registrado como assumption A2), versionamento de payloads (mantido in-scope via campo `event_version` desde o MVP).
- Implicação: dispatcher só lida com transporte interno (chamada de função Go); retry policy não modela 429/5xx remotos. Ordem é garantida apenas dentro de `aggregate_id`/`partition_key` quando o producer especificar.

### Pergunta 2.3 — Restrição dominante
- Opções: **Simplicidade operacional** / Confiabilidade at-least-once / Velocidade de entrega / Capacidade do time.
- Resposta: **Simplicidade operacional**.
- Implicação: empata com confiabilidade na Rodada 1 (Pergunta 1.2 exige DLQ + housekeeping em produção), mas em conflito direto prevalece simplicidade. Tradução: **polling com `FOR UPDATE SKIP LOCKED`** é a base; `LISTEN/NOTIFY`, advisory locks e fan-out paralelo só entram se trouxerem ganho mensurável — não por elegância.

### Pergunta 2.4 — Backend de armazenamento
- Resposta: **Postgres fixo** (já é o DB do produto; usa `FOR UPDATE SKIP LOCKED`).
- Implicação: Storage é direto via `pgx`. Sem interface de Storage trocável no MVP — abstração só se demanda real aparecer. Outbox vive no MESMO schema/DB do agregado para preservar atomicidade transacional (qualquer banco separado quebra a garantia do padrão).

### Restrições e premissas consolidadas para Rodada 3
- Linguagem: Go 1.26.x, monólito existente.
- Runtime: dispatcher e cron rodam em goroutines separadas dentro do binário `cmd/worker`.
- Scheduler: `github.com/robfig/cron/v3` para housekeeping (limpeza 90d) e tarefas periódicas operacionais. Dispatcher principal pode usar polling com `time.Ticker` próprio ou cron; decidir na Rodada 3.
- Concorrência multi-instância: `SELECT ... FOR UPDATE SKIP LOCKED` no Postgres.
- DLQ: linhas movidas/marcadas como `dead_letter` após N tentativas (N a definir; default proposto = 8 com backoff exponencial cap).
- Retenção: 90d para `processed` e `dead_letter`; housekeeping diário.
- Schema: payload em JSONB; metadados genéricos (event_type, event_version, aggregate_type, aggregate_id, partition_key, correlation_id, causation_id, headers).

## Rodada 3 - Alternativas

### Espaço de soluções
Cada alternativa precisa atender: at-least-once após commit de negócio, 1×N in-process, multi-instância sem broker, DLQ, housekeeping 90d, latência p95 < 1s, Postgres-only.

### Alternativa A — Polling Single-Table com fan-out agregado
- **Schema:** uma tabela `outbox_events(id, event_type, event_version, aggregate_type, aggregate_id, partition_key, payload jsonb, headers jsonb, status, attempts, next_retry_at, last_error, created_at, processed_at, claimed_at, claimed_by)`.
- **Dispatcher:** `time.Ticker` ~250ms. `SELECT … WHERE status='pending' AND next_retry_at <= now() ORDER BY id LIMIT $batch FOR UPDATE SKIP LOCKED`. Para cada linha, executa **todos os handlers** registrados para `event_type` de forma sequencial dentro da mesma transação de claim. Marca `processed` apenas se todos retornarem OK; falha de qualquer handler incrementa `attempts` e agenda `next_retry_at` por backoff exponencial.
- **Idempotência:** ônus é do handler (ele já precisa ser idempotente em qualquer at-least-once); a chave natural é `event_id`.
- **Multi-instância:** `FOR UPDATE SKIP LOCKED` + `claimed_by`/`claimed_at` para reaper.
- **DLQ:** `status='dead_letter'` após N (default 8) tentativas.
- **Housekeeping:** `robfig/cron` diário deleta `processed` e `dead_letter` com `> 90d`.
- **Reaper:** `robfig/cron` a cada 1min libera linhas `claimed` com `claimed_at < now()-5min` (proteção contra crash do worker).
- **Prós:** schema mínimo, 1 tabela, código simples. **Contras:** se o handler B falha após A ter sucedido, A é re-executado na retry — exige idempotência (aceitável). Sem visibilidade granular por handler.

### Alternativa B — Polling Two-Table (events + deliveries)
- **Schema:** `outbox_events(id, event_type, event_version, aggregate_*, partition_key, payload, headers, created_at)` imutável após publish + `outbox_deliveries(id, event_id, subscription_name, status, attempts, next_retry_at, last_error, processed_at, dead_letter_at, claimed_at, claimed_by)`.
- **Publish:** transação de negócio insere `outbox_events` E uma linha em `outbox_deliveries` por handler registrado (resolução do registry no momento do publish — handlers conhecidos em build time).
- **Dispatcher:** mesmo polling, mas lê de `outbox_deliveries`. Cada delivery é independente — re-execução só atinge handlers com falha.
- **DLQ:** por delivery; um handler em DLQ não impede os outros.
- **Housekeeping:** evento limpo quando todas suas deliveries estão `processed` ou `dead_letter` há > 90d.
- **Prós:** observabilidade granular (qual handler está falhando), retries independentes, modelo extensível para subscriptions dinâmicas. **Contras:** 2 escritas por evento na transação de publish (custo no caminho quente), schema mais rico, mais SQL.

### Alternativa C — LISTEN/NOTIFY + Polling Fallback (two-table)
- **Schema:** igual à B.
- **Trigger / hook de aplicação:** após commit, `pg_notify('outbox','')` (trigger DB ou chamada explícita após `tx.Commit()`).
- **Dispatcher:** mantém conexão dedicada via `pgx` em `LISTEN outbox`. Notify acorda imediatamente um polling do `outbox_deliveries`. `time.Ticker` ~5s como fallback de segurança contra notificações perdidas (conexão caída).
- **Multi-instância:** todas as réplicas recebem o notify; vence quem pegar com `SKIP LOCKED`.
- **Prós:** latência p95 ~50–200ms, baixo overhead em fila ociosa. **Contras:** conexão dedicada por instância, mais um caminho para depurar (notify perdido vs polling), trigger DB acopla schema, complexidade operacional extra.

### Alternativa D — Polling Particionado por hash(partition_key)
- **Schema:** B + coluna `partition_id smallint` calculada por `hash(partition_key) % N`.
- **Dispatcher:** pool de N workers; cada worker poll apenas seu shard. Multi-instância usa advisory lock por `partition_id` para garantir que apenas uma instância processa cada partition por vez → preserva ordem por `aggregate_id` mesmo entre réplicas.
- **Prós:** ordem por aggregate garantida, paralelismo controlado por partition. **Contras:** **over-engineering** dado que ordenação global FIFO foi explicitamente fora-de-escopo (Rodada 2); ordem por aggregate pode ser obtida apenas no MVP via `ORDER BY id` dentro do batch sem advisory lock. Mantida no scorecard por transparência, mas tende a ser descartada.

### Pergunta 3.1 — Validação do conjunto
- Resposta: **Aprovado** (A, B, C, D).

### Pergunta 3.2 — Padrão de schedule
- Opções: cron-para-tudo / **cron só para housekeeping+reaper; ticker próprio (200–500ms) para dispatcher** / sem cron / backoff dinâmico.
- Resposta: **cron para housekeeping/reaper; dispatcher com `time.Ticker` próprio (200–500ms)**.
- Implicação aplicada a TODAS as alternativas (A, B, C, D): `robfig/cron/v3` governa apenas housekeeping (`@daily`) e reaper (`@every 1m`). Polling do dispatcher usa `time.Ticker` configurável (default 250ms) — atende SLO p95 < 1s sem onerar cron.

## Rodada 4 - Trade-offs

### Pergunta 4.1 — Simplicidade (A) vs Observabilidade granular (B)
- Resposta: **Aceito B** — paga complexidade extra de 2 tabelas e +1 escrita por handler em troca de retries independentes, métricas por handler e DLQ por delivery.
- Implicação: schema two-table confirmado. `outbox_events` imutável + `outbox_deliveries` por handler. Observabilidade por handler é insumo obrigatório da plataforma multi-propósito.

### Pergunta 4.2 — LISTEN/NOTIFY (C) no MVP
- Resposta: **Fora do MVP** — polling 250ms atende p95 < 1s. Reavaliar com métricas reais antes de promover.
- Implicação: registrar como decisão pendente futura no decision-brief; techspec não cria conexão dedicada de LISTEN no MVP.

### Pergunta 4.3 — Política de retry + DLQ
- Resposta: **8 tentativas; backoff exponencial com jitter (base 1s, cap 5min); status='dead_letter' + `dead_letter_at` após esgotar**.
- Implicação: janela total de retry ~30–60 minutos no pior caso. Cobre janelas de manutenção/deploy curtas sem prender em DLQ. Policy fica no código do dispatcher, configurável via config (não por subscription no MVP).

### Pergunta 4.4 — Risco residual aceito explicitamente
- Resposta: **Latência ocasional 1–2s em pico de carga (p99) — aceitável se p95 < 1s for mantido**.
- Implicação: SLO formal = p95 < 1s. Plano de evolução: se p99 degradar consistentemente acima de 2s, ativar Alt C (LISTEN/NOTIFY) sem mudança de schema.
- Riscos NÃO explicitamente confirmados pelo usuário nesta rodada (registrar como premissas técnicas obrigatórias / decisões pendentes a validar em techspec):
  - **Idempotência de handler** — hard requirement do padrão at-least-once; não é "risco aceito", é regra obrigatória de implementação para qualquer subscription. Registrar como premissa A3.
  - **Crescimento da tabela em caso de falha do housekeeping** — risco operacional padrão; mitigação = métrica de `count(*)` + alerta de capacidade. Registrar como risco residual operacional a tratar em techspec.

### Trade-offs aceitos explicitamente (consolidação)
1. +1 escrita transacional por handler no caminho de publish → ganho de observabilidade + retries independentes (4.1).
2. Latência média ~250ms (uma janela de Ticker) → simplicidade operacional sem LISTEN/NOTIFY (4.2).
3. Janela de retry de até 60min para falhas transitórias → reduz pressão em DLQ em manutenções (4.3).
4. p99 ocasional 1–2s aceitável desde que p95 < 1s (4.4).

## Rodada 5 - Seleção de Direção

### Síntese apresentada ao usuário
- Recomendação preliminar consolidada: Alternativa 2 - Polling Two-Table (events + deliveries).
- Parâmetros codificados: `time.Ticker` 250ms para o dispatcher; `robfig/cron/v3` para housekeeping (`@daily`) e reaper (`@every 1m`); `FOR UPDATE SKIP LOCKED` para coordenação multi-instância; retry 8× com backoff exponencial + jitter (base 1s, cap 5min); DLQ por delivery via `status='dead_letter'` + `dead_letter_at`; housekeeping de 90d.
- Trade-offs aceitos (Rodada 4): +1 escrita por handler no caminho de publish, latência média ~250ms, janela de retry de até ~60min, p99 ocasional 1–2s.
- Alternativas descartadas: 1 (perda de observabilidade granular), 3 (complexidade extra não justificada pelo SLO atual), 4 (over-engineering vs. escopo aprovado).
- Riscos residuais documentados: contenção de escrita extra, idempotência obrigatória por handler, crescimento de tabela em caso de falha de housekeeping, polling pressionando DB, retenção 90d vs. compliance.

### Pergunta 5.1 — Confirmação da direção
- Opções: **Confirmo Alt 2 → seguir para technical-discovery-production** / Confirmo Alt 2 mas ir direto para create-prd / Reabrir / Reconsiderar Alt 1.
- Resposta: **Confirmo Alt 2 com todos os parâmetros decididos — seguir para technical-discovery-production**.

### Pergunta 5.2 — Próxima skill recomendada
- Resposta: **technical-discovery-production** — detalhar arquitetura, contratos Go, schema SQL final, métricas e ADRs antes do PRD.
- Justificativa: a natureza é fundação técnica de plataforma (não feature de produto), portanto o caminho mais coerente é técnico→PRD, não o inverso.

## Decisões Registradas
1. **Direção arquitetural**: adotar a Alternativa 2 (Polling Two-Table — `outbox_events` imutável + `outbox_deliveries` por handler) como fundação canônica de mensageria interna reativa do MeControla.
2. **Schema**: two-table com payload JSONB e metadados genéricos (event_type, event_version, aggregate_type, aggregate_id, partition_key, correlation_id, causation_id, headers).
3. **Runtime**: dispatcher + cron como goroutines do `cmd/worker` existente; sem novo binário.
4. **Scheduler**: `github.com/robfig/cron/v3` apenas para housekeeping (`@daily`) e reaper (`@every 1m`); dispatcher usa `time.Ticker` próprio de 250ms.
5. **Coordenação multi-instância**: `SELECT ... FOR UPDATE SKIP LOCKED` em `outbox_deliveries`, sem leader election externo.
6. **Política de retry**: 8 tentativas, backoff exponencial com jitter (base 1s, cap 5min), depois `status='dead_letter'` + `dead_letter_at` preenchido.
7. **Retenção**: housekeeping diário apaga `processed` e `dead_letter` com >90d, condicionado à validação prévia com Legal/Compliance (H7).
8. **SLO**: latência de entrega p95 < 1s; p99 ocasional 1–2s aceitável; degradação consistente do p99 dispara plano de evolução para LISTEN/NOTIFY.
9. **Idempotência**: regra obrigatória e documentada de implementação de handler; chave canônica = `event_id`.
10. **Fora do MVP**: broker externo, HTTP/webhooks externos, CDC, LISTEN/NOTIFY, ordenação global FIFO, schema registry, particionamento por advisory lock.
11. **Próxima skill**: `technical-discovery-production` para detalhar arquitetura, interfaces Go, schema SQL final, ADRs, métricas OTel, testes e runbook.
