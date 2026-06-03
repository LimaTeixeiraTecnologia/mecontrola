# DECISION BRIEF

## Problema
O monólito Go MeControla não possui um substrato canônico para entrega assíncrona/reativa de eventos de domínio. Sem isso, side-effects (notificações, projeções, integrações, retries, DLQ) tendem a ser implementados de forma ad-hoc dentro de cada feature — via goroutines fire-and-forget, chamadas HTTP inline na transação de negócio ou cronjobs específicos por caso de uso. Esse caminho gera dívida acumulada que será difícil de migrar e deixa o produto exposto a perda silenciosa de eventos quando um side-effect falha após o commit transacional do agregado. A decisão de adotar Outbox como fundação multi-propósito precisa ser tomada agora, antes que novas features estabeleçam padrões divergentes.

## Objetivo
Definir e materializar a fundação de mensageria interna reativa do MeControla baseada no padrão Transactional Outbox em PostgreSQL, sem broker externo no MVP, capaz de:
- garantir entrega at-least-once de eventos publicados na mesma transação do agregado;
- suportar 1xN handlers in-process por evento, com retries independentes por handler;
- coordenar processamento entre múltiplas instâncias do `cmd/worker` via `FOR UPDATE SKIP LOCKED`;
- atender SLO de latência p95 menor que 1s;
- expor DLQ por handler e housekeeping automatizado de 90 dias;
- entregar em produção um ciclo completo de publish, poll, deliver, retry, DLQ e housekeeping, ainda que com handler dummy no primeiro deploy.

## Escopo Inicial
Inclui:
- Schema two-table no PostgreSQL: `outbox_events` (imutável, payload JSONB, metadados genéricos) e `outbox_deliveries` (linha por par evento+handler, com status, attempts, next_retry_at, last_error, dead_letter_at, claimed_at, claimed_by).
- Componente Publisher que insere `outbox_events` e N linhas em `outbox_deliveries` (uma por handler registrado) dentro da mesma transação do agregado.
- Dispatcher rodando como goroutine dentro do `cmd/worker`, com `time.Ticker` de 250ms, lendo `outbox_deliveries` via `SELECT ... FOR UPDATE SKIP LOCKED LIMIT $batch`.
- Registry estático de handlers (event_type para slice de handlers), resolvido em build time.
- Política de retry: 8 tentativas, backoff exponencial com jitter (base 1s, cap 5min), depois transição para `dead_letter` com `dead_letter_at` preenchido.
- Cron `github.com/robfig/cron/v3` rodando como segunda goroutine no `cmd/worker` com dois jobs: housekeeping `@daily` (apaga linhas processadas e em DLQ com mais de 90d) e reaper `@every 1m` (libera registros `claimed` com `claimed_at` anterior a now() menos 5min).
- Migrações via `golang-migrate` (já em uso no projeto).
- Métricas OpenTelemetry mínimas: contadores de pending, published, dlq e histograma de latência de entrega por subscription.
- Handler dummy de exemplo e um caso real cumprindo o ciclo completo end-to-end em produção.

Exclui:
- Broker externo (RabbitMQ, Kafka, NATS, SNS/SQS) no MVP.
- Integrações HTTP e webhooks externos no MVP, apenas handlers in-process.
- CDC, Debezium ou leitura via WAL.
- LISTEN/NOTIFY no MVP (decisão pendente de evolução condicionada a métricas reais).
- Ordenação global FIFO entre event_types distintos; ordem apenas por aggregate_id ou partition_key quando o producer especificar.
- Schema registry tipo Avro ou Protobuf; payloads ficam JSONB opacos com campo `event_version` controlado pelo producer.
- Particionamento por shard com advisory locks (Alternativa 4 do scorecard), descartado por over-engineering.
- Configurabilidade de retry policy por subscription no MVP; policy é global no início e vira evolução futura.

## Restrições
- PostgreSQL é o backend persistente fixo do produto e do Outbox; Outbox vive no mesmo schema e DB do agregado para preservar atomicidade transacional.
- Sem novas dependências de infraestrutura (sem broker, sem cache distribuído, sem coordenador externo).
- Dispatcher e cron rodam como goroutines do `cmd/worker` existente, hoje idle; sem novo binário.
- Scheduler periódico obrigatoriamente `github.com/robfig/cron/v3` (versão estável atual da família v3).
- Coordenação multi-instância obrigatoriamente via `FOR UPDATE SKIP LOCKED` (sem leader election externo, sem ZooKeeper ou etcd).
- Restrição dominante em conflito: simplicidade operacional vence sobre elegância ou otimização (Rodada 2.3).
- SLO formal: latência de entrega p95 menor que 1s; p99 ocasional entre 1s e 2s aceitável.
- Idempotência de handler é regra obrigatória de implementação (consequência do at-least-once).

## Hipóteses
- H1 confirmada: pgx/v5 e PostgreSQL suportam nativamente `FOR UPDATE SKIP LOCKED`; evidência em `go.mod` e testes com `testcontainers-go/modules/postgres`.
- H2 confirmada: `cmd/worker` está idle e pronto para hospedar dispatcher e cron sem conflito com cargas atuais (inspeção em `cmd/worker/worker.go`).
- H3 confirmada: volumetria alvo média (10 a 100 ev/s, ~1M/dia, p95 menor que 1s) declarada na Rodada 1.4, viabilizando polling 250ms sem necessidade de LISTEN/NOTIFY.
- H4 confirmada: modelo 1xN in-process declarado na Rodada 2.1, justificando `outbox_deliveries` por handler.
- H5 não validada: assumir que mais uma escrita por handler no caminho de publish não degrada throughput; validar com benchmark ou load test na techspec.
- H6 não validada: assumir que todos os handlers serão escritos idempotentes; mitigar via checklist de PR e revisão técnica.
- H7 não validada: assumir que retenção de 90d atende compliance; confirmar com Legal antes do deploy e registrar em ADR.
- H8 não validada: assumir que polling em torno de 4 qps por instância em fila vazia não pressiona o DB; monitorar `pg_stat_statements` após o deploy.

## Alternativas Avaliadas
### Alternativa 1 - Polling Single-Table com fan-out agregado
Resumo:
Uma tabela `outbox_events` com colunas de estado por linha (status, attempts, next_retry_at, claimed_at, claimed_by). Dispatcher faz polling com `SELECT ... FOR UPDATE SKIP LOCKED` e, para cada linha, executa todos os handlers registrados sequencialmente; marca `processed` somente quando todos os handlers retornam OK; falha de qualquer handler incrementa attempts e agenda retry. DLQ via `status='dead_letter'`. Housekeeping via `robfig/cron @daily`. Reaper via `@every 1m`.

Viabilidade:
Técnica: trivial, schema mínimo, código curto, padrões consagrados. Operacional: 1 tabela para inspecionar, baixo footprint, mas observabilidade granular por handler é ausente, debugging depende de logs estruturados. Financeira: custo mais baixo do conjunto (menos escritas, menos índices). Risco principal: se o handler B falha após A ter sucedido, A é reexecutado no retry, aceitável apenas com handlers idempotentes; a perda de visibilidade por subscription dificulta runbook em incidentes.

### Alternativa 2 - Polling Two-Table (events + deliveries)
Resumo:
`outbox_events` imutável após publish (id, event_type, event_version, aggregate_type, aggregate_id, partition_key, payload JSONB, headers JSONB, correlation_id, causation_id, created_at) e `outbox_deliveries` (id, event_id, subscription_name, status, attempts, next_retry_at, last_error, processed_at, dead_letter_at, claimed_at, claimed_by). Publish insere `outbox_events` e N linhas em `outbox_deliveries` (uma por handler registrado) na mesma transação do agregado. Dispatcher faz polling sobre `outbox_deliveries` com `FOR UPDATE SKIP LOCKED`, processa cada delivery independentemente. DLQ por delivery. Housekeeping `@daily` apaga eventos quando todas as deliveries estão finalizadas há mais de 90d. Reaper `@every 1m`.

Viabilidade:
Técnica: padrão maduro (idiomático em sistemas event-driven sérios em SQL), facilmente implementável com pgx e golang-migrate. Operacional: observabilidade granular por handler nativa no DB (métricas por subscription_name), retries isolados, runbook claro. Financeira: custo moderado, mais uma escrita por handler ativo no caminho de publish; índices adicionais em `outbox_deliveries(status, next_retry_at)`. Risco controlado: aumento de tráfego de escrita no DB requer monitoramento mas é proporcional ao número médio de handlers por evento (esperado 1 a 3 no início).

### Alternativa 3 - LISTEN/NOTIFY com Polling Fallback two-table
Resumo:
Mesmo schema da Alternativa 2. Após `tx.Commit()` o aplicativo emite `pg_notify('outbox','')`. Dispatcher mantém conexão dedicada via pgx em `LISTEN outbox` e acorda imediatamente para processar `outbox_deliveries`. Polling com `time.Ticker` de 5s atua como rede de segurança contra notificações perdidas (conexão caída, falha de pg_notify). Multi-instância: todas as réplicas recebem notify; vence quem pega a linha com `SKIP LOCKED`.

Viabilidade:
Técnica: viável com pgx (suporta LISTEN nativamente), mas dobra o caminho de delivery (notify mais poll) e exige cuidado para não emitir notify dentro da transação. Operacional: latência p95 cai para a faixa de 50ms a 200ms, mas adiciona conexão dedicada por instância, mais um caminho para depurar e mais um modo de falha (notify silenciosamente perdido). Financeira: custo de implementação alto vs. ganho de latência não exigido pelo SLO atual (p95 menor que 1s já atendido pela Alternativa 2). Risco: complexidade extra não justificada pelo escopo do MVP; melhor evolução futura quando métricas reais comprovarem necessidade.

### Alternativa 4 - Polling Particionado por hash de partition_key
Resumo:
Base na Alternativa 2 e coluna `partition_id smallint` calculada por `hash(partition_key) % N`. Pool de N workers, cada um processa apenas seu shard. Multi-instância usa `pg_advisory_xact_lock` por `partition_id` para garantir que apenas uma instância processa um partition por vez, preservando ordem por `aggregate_id` mesmo entre réplicas.

Viabilidade:
Técnica: viável mas envolve advisory locks, pool de workers e tunning de N, sensível a hotspots de partition_key. Operacional: aumenta superfície de modos de falha (lock contention, deadlocks, worker starvation), runbook mais denso, depuração mais complexa. Financeira: custo de implementação e operação alto. Risco principal: over-engineering, a Rodada 2.2 colocou ordenação global FIFO explicitamente fora de escopo e ordem por aggregate_id pode ser obtida no MVP via `ORDER BY id` dentro do batch sem advisory lock. Descartada por não justificar trade-off de complexidade no escopo aprovado.

## Trade-offs
- Alternativa 2 recomendada: aceita mais uma escrita transacional por handler no caminho de publish em troca de observabilidade granular, retries independentes e DLQ por delivery.
- Alternativa 2 recomendada: aceita latência média de aproximadamente 250ms (uma janela de Ticker) ao preferir simplicidade operacional sobre LISTEN/NOTIFY.
- Política de retry: aceita janela total de até aproximadamente 60min (8 tentativas, backoff exponencial com jitter, cap 5min) para reduzir pressão em DLQ durante manutenções curtas.
- SLO: aceita p99 ocasional entre 1s e 2s desde que p95 menor que 1s seja mantido, com plano de evolução para Alternativa 3 caso p99 degrade consistentemente.
- Idempotência de handler é ônus do desenvolvedor da subscription: documentar e fiscalizar via PR template.

## Riscos
- Risco: contenção de escrita extra no caminho de publish degradar latência de endpoints transacionais (H5).
  Impacto: aumento de p95 em APIs críticas de negócio.
  Probabilidade: média no início, alta se número de handlers por evento crescer rápido.
  Mitigação: benchmark obrigatório na techspec; índices adequados em `outbox_deliveries`; cap de handlers por evento monitorado em métricas; possível batching de insert.
- Risco: handler não idempotente causando side-effects duplicados em retry (H6).
  Impacto: cobranças duplicadas, notificações repetidas, integrações inconsistentes.
  Probabilidade: alta sem disciplina.
  Mitigação: regra obrigatória documentada na techspec; PR template com checklist; chave de idempotência baseada em `event_id` exposta no contrato do handler.
- Risco: housekeeping falhar silenciosamente e tabela crescer indefinidamente.
  Impacto: aumento de custo de storage, degradação de queries de polling por inchaço de índice.
  Probabilidade: baixa com cron monitorado, média sem alerta.
  Mitigação: métrica `outbox_deliveries.count` exposta via OTel; alerta de capacidade configurado em painel; runbook com query de purga manual.
- Risco: polling agressivo (em torno de 4 qps por instância em fila vazia) pressionar `pg_stat_statements` e CPU do DB (H8).
  Impacto: latência geral do DB.
  Probabilidade: baixa com 1 a 3 instâncias, média com mais de 5.
  Mitigação: monitorar via OTel e `pg_stat_statements`; permitir ajuste de intervalo do ticker via config; considerar polling adaptativo se métricas justificarem.
- Risco: retenção 90d conflitar com requisito regulatório de auditoria (H7).
  Impacto: violação de compliance, perda de evidência.
  Probabilidade: desconhecida, requer validação com Legal.
  Mitigação: validar antes do deploy; alternativa = arquivar para cold storage antes da purga em vez de delete físico.

## Custos
Estimativa relativa:
média

Drivers de custo:
- Implementação inicial: cerca de 2 a 3 sprints para entrega completa do ciclo end-to-end (publisher mais dispatcher mais cron mais métricas mais 1 handler real).
- Operação contínua: storage adicional proporcional ao volume vezes 90d (aproximadamente 90M deliveries em pico anual médio). Em tamanho médio de 1KB por linha, cerca de 90GB em pico, viável em Postgres.
- Manutenção: baixa, código localizado em um único pacote interno; ausência de dependências externas reduz custo de upgrade e oncall.
- Custo de oportunidade: time alocado nesta fundação não está em features de produto durante o esforço.

## Impactos Operacionais
- Deploy: nenhuma infraestrutura nova; apenas migrations `golang-migrate` e novo código no `cmd/worker` (já em produção como idle).
- Rollback: migrations reversíveis; em caso de problema com dispatcher, basta desabilitar via feature flag ou config sem afetar o caminho transacional do agregado (eventos seguem sendo gravados, apenas não entregues).
- Multi-instância: cada réplica do `cmd/worker` participa do polling com `SKIP LOCKED`; não há leader election; reaper protege contra crash com `claimed_at` antigo.
- Suporte e Oncall: novo runbook obrigatório cobrindo: como ver DLQ, como executar retry manual, como apagar deliveries específicas, como diagnosticar handler em loop.
- Documentação: registrar contrato do handler e regra de idempotência em `AGENTS.md` e `CLAUDE.md` para que novas features sigam o padrão.
- Capacidade do time: equipe atual confortável com Postgres, Go e cron; nenhuma curva de aprendizado significativa.

## Segurança
- Payload em JSONB pode carregar dados sensíveis (PII, segredos). Documentar regra de não incluir segredos no payload; recomendar campos cifrados quando necessário.
- Autorização do dispatcher: roda com a mesma credencial do `cmd/worker` no Postgres, sem nova superfície de ataque externa.
- Compliance: hipótese H7 (retenção 90d) precisa ser confirmada antes de deploy em ambientes regulados.
- Audit trail: `outbox_events` imutável funciona como log de eventos auditável durante 90d.
- Sem exposição de rede externa adicional, dispatcher é puramente interno.

## Observabilidade
- Métricas OpenTelemetry mínimas (SDK 1.44 já no `go.mod`):
  - `outbox.events.published` (counter, label: event_type).
  - `outbox.deliveries.pending` (gauge, label: subscription_name).
  - `outbox.deliveries.processed` (counter, label: subscription_name).
  - `outbox.deliveries.failed` (counter, label: subscription_name, error_class).
  - `outbox.deliveries.dlq` (counter, label: subscription_name).
  - `outbox.delivery.latency_ms` (histogram, label: subscription_name), mede da publicação à entrega bem-sucedida.
  - `outbox.poll.duration_ms` (histogram), mede custo do query de claim.
- Logs estruturados via `slog` com `event_id`, `subscription_name`, `attempt`, `correlation_id`.
- Tracing: propagar `traceparent` via campo `headers` do evento para que handlers continuem o trace do publisher.
- Painel sugerido para a próxima skill: latência por subscription, taxa de falha por subscription, depth de DLQ, depth de pending, idade do registro mais antigo em pending.
- Alertas mínimos: DLQ maior que 0 (warning), pending maior que N há mais de M min (critical), reaper detectando claims-stuck repetidos (warning).

## Escalabilidade
- Capacidade alvo: 10 a 100 ev/s sustentado, picos de 200 ev/s tolerados. Aproximadamente 1 a 3 handlers por evento, totalizando 30 a 600 deliveries/s.
- Gargalo principal previsto: query de polling (`SELECT ... FOR UPDATE SKIP LOCKED`). Mitigação: índice composto `(status, next_retry_at)` mais filtro por shard caso necessário no futuro; batching no fetch (LIMIT 100).
- Gargalo secundário: escrita transacional adicional em `outbox_deliveries` no caminho de publish. Mitigação: monitorar p95 do publish; introduzir batching ou COPY em V2 caso necessário.
- Escalabilidade horizontal: linear no número de réplicas do `cmd/worker` graças ao `SKIP LOCKED`; sem coordenação externa.
- Limite operacional: estimado entre 500 e 1000 ev/s antes de exigir mudança de transporte (broker externo) ou particionamento. Cenário de V2 com mais de 1000 ev/s pede revisão para Kafka ou NATS, manter contrato Publisher desacoplado já no MVP para facilitar essa migração.

## Alternativa Recomendada
Alternativa 2 - Polling Two-Table (events + deliveries)

## Justificativa
A Alternativa 2 vence o scorecard (total 36) ao equilibrar simplicidade operacional com observabilidade granular e confiabilidade, os três critérios mais ponderados pela natureza multi-propósito da fundação. Em comparação direta:
- Vs. Alternativa 1: paga mais 1 ponto de complexidade e mais 2 pontos de tempo de entrega, mas ganha mais 2 em confiabilidade e mais 3 em observabilidade. O ganho é decisivo porque o sucesso mínimo definido na Rodada 1.2 (DLQ mais housekeeping em produção) exige ver onde a fila está parando, atribuir status por delivery torna isso trivial; agregar no nível de evento esconde a causa.
- Vs. Alternativa 3: empata em confiabilidade e observabilidade, mas perde 1 ponto em complexidade, 1 em tempo de entrega e 1 em risco operacional sem ganhar nada que o SLO atual (p95 menor que 1s) já não obtenha com polling de 250ms. LISTEN/NOTIFY é evolução natural quando métricas reais demonstrarem necessidade, não antes.
- Vs. Alternativa 4: descartada por over-engineering frente ao escopo aprovado (FIFO global fora de escopo; ordem por aggregate é alcançável sem advisory lock).

A combinação Ticker 250ms mais `robfig/cron/v3` para housekeeping e reaper mais `FOR UPDATE SKIP LOCKED` mais retry 8 vezes com backoff exponencial e jitter mais DLQ por delivery atende todas as restrições confirmadas, respeita o limite de capacidade do time, mantém custo operacional baixo e deixa portas abertas para evoluções (LISTEN/NOTIFY em V2, broker externo em V3) sem reescrever publishers.

## Decisões Pendentes
- Catálogo inicial de event_types que serão publicados no MVP, precisa ser definido na techspec ou no épico subsequente (atualmente apenas o handler dummy está garantido).
- Caso de uso real de produção que acompanhará o handler dummy no primeiro deploy, alinhar com PO ou produto.
- Tamanho de batch ideal do dispatcher (default proposto: 100), calibrar via benchmark.
- Mecanismo de feature flag para desabilitar dispatcher em caso de incidente, decidir entre config Viper, env var ou flag no banco.
- Validação de retenção de 90d com Legal ou Compliance (H7) antes do deploy em ambientes regulados.
- Política de versionamento de payload (campo `event_version`): regras de evolução de schema serão tratadas em ADR separado na techspec.

## Próximo Passo Recomendado
technical-discovery-production com o objetivo de detalhar a arquitetura da Alternativa 2: interfaces Go (Publisher, Dispatcher, Registry, Handler, Storage), schema SQL final, migrations `golang-migrate`, integração no `cmd/worker` (bootstrap de goroutines), políticas de retry e backoff codificadas, contrato OTel completo (métricas, traces, logs), ADRs (escolha de schema two-table, ausência de LISTEN/NOTIFY no MVP, retenção 90d, idempotência obrigatória), estratégia de testes (unitários, integração com testcontainers Postgres, teste de concorrência multi-instância) e runbook operacional inicial.
