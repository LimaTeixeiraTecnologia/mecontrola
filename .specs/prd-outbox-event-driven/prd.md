# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 4 -->

## Visão Geral

O MeControla é um monólito Go organizado em módulos de domínio (identity, conversation, agent, finance, notifications, telemetry). Hoje a única forma de propagar eventos entre módulos é o eventbus in-process `events.Bus` (ADR-003), que descarta mensagens quando o canal do subscriber está cheio e não sobrevive a crash do processo — adequado para sinais voláteis, inadequado para side-effects críticos (notificações, projeções, integrações futuras) que precisam ser entregues **depois** do commit transacional do agregado, mesmo diante de falha, deploy, reinício ou réplica concorrente.

Sem uma fundação canônica para entrega assíncrona com garantia, cada nova feature tende a inventar sua própria solução (goroutine fire-and-forget, chamada HTTP inline na transação, cronjob específico). Esse padrão acumula dívida estrutural difícil de migrar e expõe o produto a perda silenciosa de eventos quando o caminho ad-hoc falha após o commit.

Esta funcionalidade entrega a fundação **Outbox transacional** do MeControla: um novo Publisher opt-in que persiste o evento na mesma transação do agregado e um Dispatcher que entrega aos handlers internos com garantia at-least-once, retries automáticos, Dead Letter Queue (DLQ) por handler, housekeeping de retenção e suporte a múltiplas réplicas do worker — tudo sem broker externo, usando apenas o PostgreSQL que já é o backend persistente do produto. Os destinatários primários são os próprios desenvolvedores do MeControla (que ganham um caminho confiável e padronizado) e o time de oncall/SRE (que ganha observabilidade granular para operar a plataforma).

Decisões de discovery a montante:

- Brainstorming decisório (status `done`): `docs/discoveries/brainstorm-event-driven-outbox-foundation/decision-brief.md`.
- Discovery técnico (status `done`): `docs/discoveries/technical-outbox-event-driven-foundation/discovery.md`.

## Objetivos

- **OB-01 — Padronizar o caminho de side-effects críticos**: oferecer ao desenvolvedor uma única forma canônica e documentada de publicar eventos que exigem garantia, eliminando incentivo a soluções ad-hoc. Métrica de sucesso: a partir da disponibilização, 100% dos novos side-effects críticos identificados em PR review usam `outbox.Publisher`; soluções ad-hoc são rejeitadas no review.
- **OB-02 — Garantir entrega at-least-once para eventos críticos**: nenhum evento publicado dentro de uma transação que comitou pode ser perdido por crash, reinício ou falha temporária de handler. Métrica de sucesso: zero perda de eventos em testes de injeção de falha (kill -9 do worker durante processamento, queda simulada do DB, etc.); 100% das deliveries terminam em `processed` ou `dead_letter` (nenhuma fica órfã indefinidamente).
- **OB-03 — Operar dentro do SLO declarado**: latência de entrega p95 < 1s e p99 < 2s por subscription, sustentado para volumetria média do primeiro ano (10–100 eventos/s, ~1M deliveries/dia). Métrica: histograma `outbox.delivery.latency_ms` por subscription medido em produção após go-live.
- **OB-04 — Oferecer observabilidade operacional desde o primeiro deploy**: o time de oncall precisa diagnosticar em < 5 min se a fila está crescendo, qual handler está falhando, qual subscription está em DLQ e qual a latência por subscription. Métrica: dashboard ativo no go-live com 6 painéis padrão; runbook publicado antes de ativar o dispatcher em produção.
- **OB-05 — Entregar com risco controlado**: rollout em duas etapas (deploy com flag off + ativação com flag on após smoke test) sem necessidade de reverter migração de schema em caso de problema. Métrica: zero incidente de produto causado por ativação do dispatcher no primeiro deploy; rollback (flag off) executável em < 2 min.
- **OB-06 — Preservar a fundação existente**: a entrega não revoga o `events.Bus` (ADR-003) nem força migração de eventos voláteis para o Outbox. Métrica: ADR-003 permanece ativa; documentação explicita "quando usar `events.Bus` vs. `outbox.Publisher`".
- **OB-07 — Manter custo total de operação baixo**: zero infraestrutura nova (sem broker, sem coordenador externo, sem novo binário). Métrica: CPU/IO médio do PostgreSQL não cresce mais que 15% sobre baseline pré-deploy em janela de 14 dias.

## Histórias de Usuário

Persona primária — **Desenvolvedor backend do MeControla** (consumidor da plataforma):

- US-01: Como desenvolvedor implementando um caso de uso que precisa notificar outros módulos após commit, quero **publicar um evento dentro do `UnitOfWork[T].Do` existente** (recebendo `database.DBTX` como qualquer outro repositório) sem me preocupar com retry, DLQ ou crash do worker, para que o side-effect seja entregue mesmo que o processo morra logo após o commit.
- US-02: Como desenvolvedor escrevendo um novo handler, quero **registrar minha subscription declarativamente no bootstrap** (mapeamento `event_type → handler`), para que o Dispatcher passe a entregar automaticamente sem cabo solto, e quero que o sistema **valide no startup** que não há duplicidade ou inconsistência.
- US-03: Como desenvolvedor revisando um PR, quero ter **critérios claros e documentados** de quando usar `events.Bus` (volátil) versus `outbox.Publisher` (persistente), para decidir corretamente sem precisar perguntar a cada review.
- US-04: Como desenvolvedor escrevendo testes do meu use case, quero **testar o caminho de publish sem subir um DB** (via mock da camada de Storage), para manter testes unitários rápidos; e quero **testar o ciclo completo** (publish → entrega → handler) em testes de integração com Postgres real.

Persona secundária — **Engenheiro de oncall / SRE** (operador da plataforma):

- US-05: Como engenheiro de oncall recebendo alerta de DLQ, quero **abrir um dashboard e ver imediatamente qual subscription está falhando, qual o erro, há quantas tentativas e quantos eventos foram para DLQ**, para diagnosticar em segundos sem ter que rodar query manual.
- US-06: Como engenheiro de oncall durante incidente de PostgreSQL ou problema no Dispatcher, quero **desligar o Dispatcher via `OUTBOX_DISPATCHER_ENABLED=false` sem reverter código nem afetar publicações em curso**, para conter o problema e religar quando o ambiente estabilizar, sabendo que nenhum evento será perdido.
- US-07: Como engenheiro de oncall reagindo a uma demanda de LGPD (direito de eliminação), quero **um runbook documentado com a query manual** para purgar eventos de um `aggregate_id` específico antes da janela de 90d, para atender o requisito sem improvisar.
- US-08: Como engenheiro de oncall identificando um evento legitimamente travado em DLQ (handler corrigido, payload válido), quero **um procedimento claro para re-enfileirar manualmente uma delivery** (resetar status, attempts, next_retry_at), para reprocessar sem deletar e perder histórico.

Persona terciária — **Tech lead / arquiteto da plataforma**:

- US-09: Como tech lead avaliando se a fundação suporta um novo módulo de produto, quero **uma seção do AGENTS.md descrevendo o contrato do Publisher, a regra obrigatória de idempotência de handler e os limites operacionais conhecidos** (volumetria suportada, quando promover para LISTEN/NOTIFY ou broker externo), para decidir com base em fato e não em intuição.

Casos de borda relevantes:

- US-10 (worker crash): se uma réplica do worker for morta enquanto processa um batch, as deliveries em estado `claimed` precisam ser automaticamente recuperadas e re-entregues em até 5 minutos sem intervenção humana.
- US-11 (handler em loop transitório): se um handler falhar consistentemente por causa de dependência temporariamente indisponível (ex.: provider externo em manutenção de 30 min), o sistema deve continuar tentando dentro da janela de retry (até ~60 min totais) sem ir prematuramente para DLQ.
- US-12 (handler com erro permanente): se o handler retornar erro marcado como permanente (ex.: payload inválido por mudança de schema não tratada), a delivery deve ir imediatamente para DLQ sem desperdiçar tentativas.
- US-13 (réplicas concorrentes): duas ou mais réplicas do worker rodando simultaneamente jamais devem processar a mesma delivery duas vezes — a coordenação acontece transparentemente, sem leader election externo.

## Funcionalidades Core

- **FC-01 — `outbox.Publisher` (Publisher opt-in transacional)**: API Go invocada dentro de um `UnitOfWork[T].Do(ctx, fn)` ativo do agregado, recebendo o `database.DBTX` exposto pelo UoW canônico do projeto (ADR-002). Consulta o Registry para descobrir os handlers registrados para aquele `event_type` e insere o evento + uma linha de delivery por handler na mesma transação. O commit do agregado é a fronteira de durabilidade — eventos só existem se o estado do agregado também existe (atomicidade SQL nativa). Importância: este é o ponto que dá a garantia at-least-once. Como funciona em alto nível: sem retry, sem rede, sem mágica — apenas dois `INSERT` na transação do chamador.

- **FC-02 — `outbox.Registry` (registro estático de subscriptions)**: estrutura preenchida no bootstrap do `cmd/worker` que mapeia `events.EventName → []Handler` (reutilizando os tipos canônicos de `internal/infrastructure/events/`: `events.EventID` em formato ULID e `events.EventName` no padrão `<modulo>.<acao>` com módulos validados). Validada no startup quanto a duplicidade e consistência. Importância: torna o conjunto de subscriptions explícito, auditável e testável; impede que um handler "esquecido" ou "duplicado" passe despercebido; mantém uma única fonte de verdade para nomenclatura/identidade de evento entre `events.Bus` e `outbox.Publisher`. Como funciona: lista declarativa em código, lida uma vez no boot, exposta como contrato imutável para Publisher e Dispatcher.

- **FC-03 — `outbox.Dispatcher` (worker de entrega com polling e retry)**: goroutine rodando dentro do `cmd/worker` que acorda em intervalo configurável, faz claim de um lote de deliveries pendentes usando `FOR UPDATE SKIP LOCKED`, executa o handler de cada uma com timeout, e marca o resultado. Em sucesso, marca `processed`. Em erro transitório, calcula próximo retry com backoff exponencial e jitter. Após N falhas, transita para DLQ. Operacionalmente, o Dispatcher e o Cron (FC-05/FC-06) são expostos ao runtime como um único **`outbox.Subsystem` agregador** que implementa a interface `runtime.Subsystem` (`Start(ctx)`/`Stop(ctx)`/`Name()`) definida em `internal/infrastructure/runtime/app.go` — um único ponto de orquestração e shutdown ordenado, simétrico ao `serverSubsystem` já existente. Importância: é o motor que materializa a garantia at-least-once. Como funciona em alto nível: ciclo lock-execute-mark, sem coordenação externa, escalando horizontalmente com o número de réplicas do worker.

- **FC-04 — DLQ por delivery (granularidade por subscription)**: cada par (evento, handler) tem seu próprio registro de delivery, com `status`, `attempts`, `last_error`, `dead_letter_at`. Falha de um handler não bloqueia os outros nem corrompe a contabilidade de retry dos demais. Importância: observabilidade e operação granular — oncall sabe exatamente qual subscription quebrou, sem precisar correlacionar logs. Como funciona: tabela de deliveries separada da tabela de eventos; cada handler tem ciclo de vida próprio.

- **FC-05 — Housekeeping automatizado (retenção de 90 dias)**: job diário do scheduler interno apaga deliveries finalizadas (`processed` e `dead_letter`) com mais de 90 dias e propaga delete para eventos órfãos. Importância: garante que a tabela não cresce indefinidamente em produção; satisfaz a janela de retenção mínima esperada por compliance no produto. Como funciona em alto nível: job agendado `@daily` via scheduler interno do worker, com métrica de quantas linhas foram apagadas em cada execução.

- **FC-06 — Reaper de claims-stuck (recuperação automática de crash)**: job de alta frequência (`@every 1m`) detecta deliveries em estado `claimed` há mais de 5 minutos (indicativo de worker morto antes de marcar resultado) e as libera para reprocessamento. Importância: garante self-healing automático após crash; oncall não precisa intervir manualmente para recuperar deliveries travadas. Como funciona: simples `UPDATE` periódico, instrumentado com métrica de quantas linhas foram liberadas.

- **FC-07 — Feature flag global do Dispatcher**: chave de configuração `OUTBOX_DISPATCHER_ENABLED` (env var booleana, default `true`) que controla se o Dispatcher inicia o loop de polling. Quando `false`, o Publisher continua escrevendo (eventos seguem persistidos), apenas a entrega fica suspensa até o flag voltar. Importância: kill-switch operacional para incidentes; permite degradação controlada sem reverter código nem afetar o caminho transacional do agregado. Como funciona: novo grupo `OutboxConfig` com `mapstructure:",squash"` agregado a `configs.Config` (consistente com `DBConfig`/`O11yConfig`), lido no bootstrap via Viper + restart do worker para aplicar mudança (sem live-reload no MVP — decisão consolidada coerente com o padrão atual de `configs.LoadConfig`; live-reload via `viper.WatchConfig` fica como evolução futura).

- **FC-08 — Observabilidade OpenTelemetry completa**: catálogo de métricas (counters + gauges + histograms) com label por `subscription_name`, traces propagados via `headers.traceparent` ligando publisher ao handler, logs estruturados via `slog` com redaction de payload. Acompanhado de dashboard sugerido (queries Prometheus) e runbook inicial. Importância: torna a plataforma operável desde o primeiro deploy; é o que diferencia "feature em produção" de "feature operável em produção". Como funciona: instrumentação codificada dentro de cada componente do pacote, exposta via o pipeline OTel já presente no projeto.

- **FC-09 — Documentação canônica e governança**: ADR nova ("Outbox transacional como Publisher opt-in") coexistindo com a ADR-003 e atualização em `AGENTS.md`/`CLAUDE.md` descrevendo o contrato, a regra obrigatória de idempotência de handler e os critérios de quando usar `events.Bus` versus `outbox.Publisher`. Importância: produto interno só é produto se o consumidor entender quando e como usar. Como funciona: arquivos Markdown na governança existente, sujeitos à mesma esteira de revisão dos demais ADRs.

- **FC-10 — Handler dummy + suite de testes ponta a ponta**: a primeira entrega inclui um handler dummy registrado no Registry e uma suíte de testes (unitários com mock, integração com testcontainers Postgres, concorrência com múltiplos dispatchers paralelos) provando que o ciclo completo funciona em produção sem depender de evento de negócio real. Importância: provar a fundação em produção antes de comprometer um caso de uso real reduz risco; demonstra padrão de teste para futuras subscriptions. Como funciona: handler trivial que apenas registra métrica e log; suíte de testes obrigatória no pipeline de CI.

## Requisitos Funcionais

Requisitos do **publisher e contrato de eventos**:

- RF-01: O sistema DEVE oferecer uma API `outbox.Publisher.Publish(ctx, tx database.DBTX, evt outbox.Event)` que recebe a transação exposta pelo `UnitOfWork[T].Do` canônico do projeto (ADR-002, `internal/infrastructure/database/uow.go`) e persiste, na mesma transação, um registro do evento e uma linha de delivery por handler registrado para aquele `event_type`. O Publisher NÃO DEVE expor `pgx.Tx` diretamente na sua superfície pública.
- RF-02: O sistema DEVE rejeitar o publish (retornando `outbox.ErrHandlerNotRegistered`) quando o `event_type` informado não tiver nenhum handler registrado no Registry e a política configurada exigir pelo menos um — sem inserir registros parciais.
- RF-03: O sistema DEVE reutilizar `events.EventID` (ULID) e `events.EventName` (formato `<modulo>.<acao>` com módulos validados) como tipos canônicos de identidade e nomenclatura, sem criar duplicações em `internal/infrastructure/outbox/`. Cada evento DEVE registrar metadados mínimos: `event_id`, `event_type` (via `events.EventName`), `event_version`, `aggregate_type`, `aggregate_id`, `partition_key` opcional, `payload` (JSON), `headers` (incluindo `traceparent`, `correlation_id`, `causation_id`), `occurred_at`.
- RF-04: O sistema DEVE garantir que duas chamadas de publish com o mesmo `event_id` não produzam deliveries duplicadas para a mesma subscription (idempotência do publish via constraint de unicidade `(event_id, subscription_name)`).
- RF-05: O sistema DEVE permitir que o desenvolvedor escolha explicitamente entre `events.Bus` (volátil) e `outbox.Publisher` (persistente) sem que um dependa do outro — ambos coexistem como caminhos publicáveis independentes.

Requisitos do **registry e subscriptions**:

- RF-06: O sistema DEVE oferecer um Registry estático em que o desenvolvedor registra subscriptions no bootstrap do `cmd/worker` declarando `subscription_name`, `event_type` e `handler`.
- RF-07: O sistema DEVE validar no startup que não existem duas subscriptions com o mesmo `subscription_name` registradas para o mesmo `event_type` e DEVE falhar o startup do worker se houver inconsistência.
- RF-08: O sistema DEVE permitir que um mesmo `event_type` tenha múltiplos handlers (cardinalidade 1×N) e DEVE entregar cada evento a todos os handlers registrados com ciclo de vida independente.

Requisitos do **dispatcher e entrega**:

- RF-09: O sistema DEVE rodar o Dispatcher como goroutine dentro do `cmd/worker` existente, sem novo binário e sem alterar o caminho do `cmd/server`. O Dispatcher e os jobs de Cron (housekeeping + reaper) DEVEM ser orquestrados por um único `outbox.Subsystem` que implementa `runtime.Subsystem` (`Start(ctx)`/`Stop(ctx)`/`Name()` retornando `"outbox"`), registrado em `bootstrapper.buildSubsystems(ModeWorker)` em `internal/infrastructure/runtime/bootstrap.go`. `Stop(ctx)` DEVE drenar handlers em execução respeitando o contexto recebido antes de retornar.
- RF-10: O sistema DEVE fazer claim de deliveries pendentes usando `SELECT ... FOR UPDATE SKIP LOCKED` em lote configurável (default 100), respeitando a ordem `next_retry_at <= now()` e `ORDER BY id`.
- RF-11: O sistema DEVE executar cada handler com um timeout configurável (default proposto: definido na techspec) e tratar timeout como falha transitória.
- RF-12: O sistema DEVE aplicar retry com backoff exponencial e jitter quando o handler retornar erro transitório: base 2s, cap 5min, até 15 tentativas; após esgotar, transitar para DLQ. A janela total resultante (~46min com jitter) cobre o cenário declarado em US-11 (manutenções transitórias de provedores externos por até ~30min).
- RF-13: O sistema DEVE oferecer sentinels de erro exportados no pacote `outbox` — no mínimo `outbox.ErrPermanent` (handler sinaliza falha terminal; delivery transita imediatamente para DLQ sem consumir tentativas), `outbox.ErrHandlerNotRegistered` (publish para `event_type` sem handler) e `outbox.ErrDispatcherDisabled` (operações dependentes do loop quando `OUTBOX_DISPATCHER_ENABLED=false`), consumíveis via `errors.Is` / `errors.As`. Por se tratar de caminho assíncrono interno, esses sentinels NÃO precisam de mapeamento para Problem Details / RFC 7807 (ADR-004 cobre o caminho HTTP, não o assíncrono); falhas que cheguem ao HTTP via use case são mapeadas pelo `internal/infrastructure/errors` existente.
- RF-14: O sistema DEVE garantir que múltiplas réplicas do worker rodando simultaneamente jamais processam a mesma delivery duas vezes — sob carga concorrente, cada delivery em estado `pending` é entregue a exatamente um Dispatcher por vez.
- RF-15: O sistema DEVE registrar em cada delivery: `status` (`pending`/`claimed`/`processed`/`dead_letter`), `attempts`, `next_retry_at`, `last_error`, `processed_at`, `dead_letter_at`, `claimed_at`, `claimed_by` (identificador da réplica).

Requisitos de **DLQ**:

- RF-16: O sistema DEVE expor consultas e métricas que permitam ao oncall identificar, em qualquer momento, quantas deliveries estão em DLQ por `subscription_name` e qual o último erro de cada uma.
- RF-17: O sistema DEVE documentar (runbook) o procedimento de re-enfileiramento manual de deliveries do DLQ (resetar `status`, `attempts`, `next_retry_at`) — automação fica fora do MVP.

Requisitos de **housekeeping e reaper**:

- RF-18: O sistema DEVE rodar um job diário (frequência `@daily`) que apaga deliveries em `processed` ou `dead_letter` com idade superior à janela de retenção configurada (default 90 dias) e propaga delete para eventos sem deliveries restantes.
- RF-19: O sistema DEVE rodar um job de alta frequência (default `@every 1m`) que libera deliveries em estado `claimed` há mais de 5 minutos, retornando-as para `pending`.
- RF-20: O sistema DEVE expor métricas das execuções de housekeeping e reaper (contadores de linhas afetadas) e DEVE permitir ao oncall confirmar via dashboard que ambos estão executando regularmente.

Requisitos de **observabilidade**:

- RF-21: O sistema DEVE emitir métricas OpenTelemetry com label `subscription_name` (e demais labels relevantes) para: eventos publicados, deliveries pendentes (gauge), deliveries processadas, deliveries em falha, deliveries em DLQ, latência de entrega (histograma), duração do poll (histograma), tamanho do batch claimado (histograma), linhas liberadas pelo reaper, linhas apagadas pelo housekeeping.
- RF-22: O sistema DEVE propagar contexto de tracing via `headers.traceparent` do evento, de modo que um trace iniciado no Publisher possa ser continuado no Handler através do Dispatcher.
- RF-23: O sistema DEVE emitir logs estruturados via `slog` em transições relevantes (startup, processed, failed, dlq, reaper, housekeeping) contendo `event_id`, `event_type`, `subscription_name`, `attempt`, `correlation_id` quando aplicável.
- RF-24: O sistema NÃO DEVE incluir o `payload` bruto do evento em logs em nenhuma circunstância.
- RF-25: O sistema DEVE entregar, junto com o código, um dashboard sugerido (queries equivalentes a Prometheus listadas na techspec ou no runbook) e um runbook inicial cobrindo: desligar/religar Dispatcher, inspecionar DLQ, re-enfileirar delivery, purgar evento por demanda LGPD, diagnosticar pending crescente.

Requisitos de **operação e rollout**:

- RF-26: O sistema DEVE oferecer a feature flag `OUTBOX_DISPATCHER_ENABLED` (env var booleana, default `true`) lida no bootstrap via novo grupo `OutboxConfig` adicionado a `configs.Config` com `mapstructure:",squash"`, consistente com o padrão flat SCREAMING_SNAKE usado por `DBConfig`/`O11yConfig`. Quando `false`, o Dispatcher não inicia o loop de polling e o Publisher continua operando normalmente. Demais chaves de configuração do Outbox DEVEM seguir o mesmo padrão flat SCREAMING_SNAKE, com os defaults consolidados abaixo (validados nas decisões consolidadas):
  - `OUTBOX_DISPATCHER_ENABLED` (bool, default `true`)
  - `OUTBOX_DISPATCHER_TICK_INTERVAL` (duration, default `500ms`)
  - `OUTBOX_DISPATCHER_BATCH_SIZE` (int, default `50`)
  - `OUTBOX_DISPATCHER_HANDLER_TIMEOUT` (duration, default `10s`)
  - `OUTBOX_RETRY_MAX_ATTEMPTS` (int, default `15`)
  - `OUTBOX_RETRY_BASE_BACKOFF` (duration, default `2s`)
  - `OUTBOX_RETRY_MAX_BACKOFF` (duration, default `5m`)
  - `OUTBOX_HOUSEKEEPING_RETENTION_DAYS` (int, default `90`)
  - `OUTBOX_REAPER_INTERVAL` (string cron-spec, default `@every 1m`)
  - `OUTBOX_REAPER_STUCK_AFTER` (duration, default `5m`)
  - `OUTBOX_HOUSEKEEPING_SCHEDULE` (string cron-spec, default `@daily`)

  A flag `OUTBOX_DISPATCHER_ENABLED` é lida apenas no boot do worker; mudar o valor exige restart (sem live-reload no MVP, consistente com `configs.LoadConfig` atual).
- RF-27: O sistema DEVE ser rolado em produção em duas etapas: deploy 1 aplica migração e código com flag `false`; deploy 2 ativa a flag após smoke test em staging.
- RF-28: O sistema DEVE oferecer uma migração `0002_outbox.up.sql` idempotente (`CREATE TABLE IF NOT EXISTS`, índices condicionais) e uma `0002_outbox.down.sql` que reverte a estrutura para uso em caso extremo.
- RF-29: O sistema DEVE permitir rollback operacional (desligar o Dispatcher em incidente) em menos de 2 minutos via mudança da flag e restart do worker, sem necessidade de revert de código ou migração.

Requisitos de **segurança e compliance**:

- RF-30: O sistema DEVE documentar e impor (via revisão de código) a regra de que o `payload` do evento não pode conter senhas, tokens, chaves criptográficas ou outros segredos da aplicação.
- RF-31: O sistema DEVE garantir que `payload` nunca aparece em logs (RF-24 reforça) e que campos de auditoria/log são limitados a um allowlist canônico (`event_id`, `event_type`, `subscription_name`, `attempt`, `correlation_id`, `error_class`).
- RF-32: O sistema DEVE atender LGPD quanto ao direito de eliminação por meio de procedimento manual documentado no runbook (purge por `aggregate_id` antes da janela de 90d); automação fica fora do MVP.

Requisitos de **testes e validação**:

- RF-33: O sistema DEVE entregar suíte de testes unitários do Dispatcher cobrindo regras de retry, backoff, transição para DLQ e timeout de handler, usando mock da camada de Storage.
- RF-34: O sistema DEVE entregar suíte de testes de integração com testcontainers Postgres cobrindo o ciclo completo (publish → claim → handler → mark) end-to-end.
- RF-35: O sistema DEVE entregar teste de concorrência com pelo menos 3 dispatchers paralelos no mesmo Postgres processando massa pré-populada de deliveries e provando empiricamente zero double-processing.
- RF-36: O sistema DEVE entregar benchmark básico medindo p95 do publish com 1, 10, 100 e 1000 eventos/s e throughput sustentado do Dispatcher drenando massa pré-populada.

Requisitos de **documentação e governança de revisão**:

- RF-37: O sistema DEVE incluir uma ADR nova ("Outbox transacional como Publisher opt-in") referenciando explicitamente a ADR-003 e estabelecendo critérios de quando cada Publisher prevalece. A ADR DEVE ser salva como `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md` (D-12), seguindo a convenção dos `adr-001` até `adr-015` atuais.
- RF-38: O sistema DEVE atualizar `AGENTS.md` raiz e `CLAUDE.md` documentando o contrato do `outbox.Publisher`, a regra obrigatória de idempotência de handler (chave canônica `event_id`) e o critério de escolha entre `events.Bus` (volátil) e `outbox.Publisher` (persistente). DEVE também criar `internal/infrastructure/outbox/AGENTS.md` seguindo o padrão por-módulo já estabelecido (`internal/identity/AGENTS.md`, `internal/finance/AGENTS.md`, etc.) com referência ao README do pacote.

Requisitos de **arquitetura runtime e ciclo de vida**:

- RF-39: O Outbox DEVE expor um único agregador `outbox.Subsystem` que implementa `runtime.Subsystem` (`Start(ctx) error`, `Stop(ctx) error`, `Name() string` retornando `"outbox"`) e que internamente compõe Dispatcher (goroutine com ticker) e Cron (housekeeping + reaper via `robfig/cron/v3`). `Stop(ctx)` DEVE: (i) cancelar o ticker do Dispatcher e o scheduler do Cron, (ii) aguardar handlers em execução terminarem respeitando o `ctx`, (iii) retornar erros acumulados via `errors.Join`. O Subsystem DEVE respeitar `OUTBOX_DISPATCHER_ENABLED=false` não iniciando o loop de polling, ainda que a estrutura seja construída e registrada (visibilidade para observabilidade).

Requisitos de **enforcement de revisão**:

- RF-40: O sistema DEVE criar `.github/PULL_REQUEST_TEMPLATE.md` (hoje inexistente — o repositório tem apenas `.github/copilot-instructions.md`) contendo, no mínimo, uma seção condicional "Outbox / Event Handler" com checklist obrigatório quando o PR introduzir ou alterar uma `outbox.Subscription`:
  - [ ] Handler é idempotente (executar 2× com mesmo `event_id` produz o mesmo resultado final).
  - [ ] `event_id` é usado como chave de idempotência (ex.: tabela de deduplicação, upsert, no-op se já aplicado).
  - [ ] `Subscription{Name, EventType, Handler}` foi registrada no Registry no bootstrap do `cmd/worker`.
  - [ ] Payload do evento NÃO contém senhas, tokens ou segredos.
  - [ ] Critério explícito de quando usar `outbox.Publisher` vs `events.Bus` foi avaliado.
  O template DEVE também conter seções genéricas mínimas (descrição, tipo de mudança, testes, breaking changes) para servir como template padrão do repositório, beneficiando PRs futuros além do Outbox.

## Experiência do Usuário

Esta funcionalidade é predominantemente de plataforma interna — não há UI voltada ao usuário final do produto. A "experiência do usuário" se materializa para dois personas técnicos:

**Desenvolvedor backend** (consumidor da API Go):

- Fluxo "publicar evento": dentro de um use case existente, obter a `pgx.Tx` do `UnitOfWork`, chamar `outbox.Publisher.Publish(ctx, tx, evt)`, fazer `tx.Commit()`. Sem código adicional de retry, sem `go func()`, sem chamadas HTTP inline. O godoc da função descreve o contrato em uma frase.
- Fluxo "criar nova subscription": declarar `Subscription{Name, EventType, Handler}` no bootstrap do `cmd/worker`. Erro de duplicidade ou inconsistência é capturado no startup, não em runtime. PR template lembra de declarar idempotência do handler.
- Fluxo "testar localmente": rodar `go test ./internal/infrastructure/outbox/...` para suíte unitária com mock; rodar `go test -tags=integration ./...` para suíte completa com testcontainers. Tempo alvo da suíte unitária: < 5s; integração: < 60s.
- Critério de "boa experiência": desenvolvedor consegue implementar e testar uma nova subscription crítica em < 30 min de trabalho ativo, lendo apenas o godoc do pacote e a seção de `AGENTS.md`.

**Engenheiro de oncall / SRE** (operador da plataforma):

- Fluxo "responder a alerta de DLQ": abrir dashboard, identificar `subscription_name` em DLQ, abrir painel de logs filtrando por essa subscription, ler `last_error`, decidir entre fix-and-replay ou descartar. Tempo alvo de diagnóstico: < 5 min.
- Fluxo "incidente do Dispatcher": setar `outbox.dispatcher.enabled=false` em config, restartar worker, confirmar pelo dashboard que poll parou e publish continua. Tempo alvo: < 2 min de "decisão" até "Dispatcher off".
- Fluxo "demanda LGPD": consultar runbook, executar query manual de purge por `aggregate_id`, registrar no canal de audit. Tempo alvo: < 15 min do recebimento da demanda à confirmação de purge.
- Critério de "boa experiência": runbook cobre todos os cenários previstos e contém exemplos copy-paste de queries SQL.

Acessibilidade: não aplicável (sem UI).

## Restrições Técnicas de Alto Nível

- **R-01 (integração existente)**: a funcionalidade integra-se ao `cmd/worker` atual (idle) sem novo binário e reaproveita as fundações já estabelecidas:
  - `internal/infrastructure/database/Manager` (`devkit-go/pkg/database/manager`, ADR-007/ADR-002) para acesso ao Postgres;
  - `internal/infrastructure/database/UnitOfWork[T]` (`devkit-go/pkg/database/uow`, ADR-002) que expõe `database.DBTX` aos use cases — Publisher recebe esse tipo;
  - `internal/infrastructure/observability/Provider` (`devkit-go/pkg/observability/otel`) para instrumentação OTel já configurada;
  - `internal/infrastructure/runtime/{App,Subsystem}` (`runtime.Subsystem` com `Start/Stop/Name`) — Outbox implementa essa interface e é registrado em `buildSubsystems(ModeWorker)` que hoje retorna `[]Subsystem{}` vazio;
  - `internal/infrastructure/events` (`events.EventID`, `events.EventName`) como fonte de verdade dos tipos de evento — coexistindo com o `events.Bus` (ADR-003) sem revogá-lo.

  O `devkit-go` (`github.com/JailtonJunior94/devkit-go`) NÃO oferece helper de Outbox/at-least-once; a implementação é nova e fica localizada em `internal/infrastructure/outbox/`. Dependências NOVAS introduzidas pela entrega: `github.com/robfig/cron/v3` (apenas).
- **R-02 (Postgres-only e atomicidade)**: o Outbox deve viver no mesmo schema/DB do agregado para preservar atomicidade transacional do publish. Backends alternativos (Redis Streams, banco dedicado, MongoDB) estão fora do escopo da entrega.
- **R-03 (sem broker externo)**: a entrega NÃO pode introduzir RabbitMQ, Kafka, NATS, SNS/SQS ou similares. Substrato deve permitir migração futura para broker sem reescrever publishers, mas a infraestrutura no MVP é exclusivamente Postgres.
- **R-04 (scheduler obrigatório)**: jobs periódicos (housekeeping, reaper) devem usar `github.com/robfig/cron/v3` (versão estável atual da família v3) — restrição declarada no prompt original e preservada em todas as rodadas de discovery.
- **R-05 (coordenação multi-instância)**: a coordenação entre réplicas do worker deve ser feita exclusivamente via `FOR UPDATE SKIP LOCKED` no Postgres — sem leader election externo, ZooKeeper, etcd ou advisory locks.
- **R-06 (volumetria alvo)**: a solução deve suportar 10–100 eventos/s sustentados e picos de 200 ev/s no primeiro ano (~1M deliveries/dia). Volumetria > 500 ev/s sustentado dispara re-discovery e fica explicitamente fora do escopo do MVP.
- **R-07 (SLO formal)**: latência de entrega p95 < 1s e p99 < 2s, mensurável por subscription via histograma `outbox.delivery.latency_ms`. p99 ocasional acima de 2s aceitável durante incidente, não como regime permanente.
- **R-08 (segurança baseline)**: payload em texto claro no Postgres (sem criptografia adicional no MVP); regra obrigatória de "sem segredos no payload"; logs nunca expõem payload bruto. Reforço de criptografia em repouso fica fora do escopo até requisito regulatório explícito.
- **R-09 (LGPD por retenção)**: ciclo de vida do dado pessoal voluntariamente incluído no payload é fechado pela retenção de 90 dias do housekeeping; direito de eliminação por demanda é atendido via runbook manual no MVP.
- **R-10 (sem prazo rígido)**: a entrega deve priorizar qualidade do design e cobertura de testes; estimativa 2–3 sprints com folga (~80–120h-eng + ~10h DBA/SRE + ~5h Legal/Compliance). Não há prazo externo forçando corte de escopo.
- **R-11 (governança)**: qualquer mudança em código Go deve carregar `agent-governance` + `go-implementation` conforme `CLAUDE.md` e `AGENTS.md`. ADR nova deve coexistir com ADR-003 e seguir a esteira existente de ADRs do projeto (`adr-001` até `adr-015`).
- **R-12 (rollout em duas etapas)**: ativação em produção exige flag off no deploy 1 + smoke test em staging com flag on + ativação em produção no deploy 2 em horário de baixa carga. Big-bang está fora do escopo de operação aceitável.

## Fora de Escopo

- **FE-01 — Broker externo**: nenhuma integração com RabbitMQ, Kafka, NATS, SNS/SQS, NATS JetStream ou similares. Substrato fica preparado para migração futura (contrato `Publisher` desacoplado), mas a entrega é Postgres-only.
- **FE-02 — Integrações HTTP/webhooks externos como handler**: o MVP cobre exclusivamente handlers in-process (funções Go registradas no Registry). Handlers que façam chamadas HTTP a sistemas externos não são bloqueados, mas a política de retry e DLQ é genérica — não há suporte específico a 429/5xx remotos, circuit breaker por endpoint externo ou rate limiting por destino.
- **FE-03 — CDC / Debezium / leitura via WAL**: não há leitura de replicação lógica do Postgres. Dispatcher faz polling SQL convencional.
- **FE-04 — LISTEN/NOTIFY no MVP**: usado polling com `time.Ticker` de 250ms. LISTEN/NOTIFY fica como evolução condicional disparada por métrica de p99 > 2s consistente.
- **FE-05 — Ordenação global FIFO entre `event_types` distintos**: ordem só é garantida dentro de um lote de claim (`ORDER BY id`) e para o mesmo `aggregate_id`. Não há FIFO global, nem por `partition_key`.
- **FE-06 — Schema registry (Avro / Protobuf)**: payloads JSONB são opacos para o Outbox; versionamento é responsabilidade do producer/handler via campo `event_version`.
- **FE-07 — Particionamento por advisory lock**: alternativa 4 do scorecard de brainstorming, descartada por over-engineering vs. escopo aprovado.
- **FE-08 — Configurabilidade de retry policy por subscription**: política global no MVP (8 tentativas, base 1s, cap 5min). Configurar por subscription fica como evolução futura.
- **FE-09 — Substituição ou deprecação do `events.Bus` (ADR-003)**: ambos coexistem; o desenvolvedor escolhe.
- **FE-10 — Acoplamento a módulo de negócio na primeira entrega**: nenhum evento real de produto é publicado no MVP. A entrega é validada com handler dummy ponta a ponta. Qual evento real será o primeiro a usar Outbox em produção fica como decisão pendente da próxima sprint.
- **FE-11 — Automação de re-enfileiramento do DLQ**: no MVP, re-enfileirar uma delivery do DLQ é procedimento manual via SQL documentado em runbook. CLI/API dedicado fica como evolução futura.
- **FE-12 — Purge LGPD automatizado**: no MVP, purge por demanda de direito de eliminação é procedimento manual via SQL documentado em runbook. Endpoint/CLI fica como evolução futura.
- **FE-13 — Criptografia em repouso ou envelope encryption por evento**: baseline padrão (texto claro + política de não-segredos). Reforço fica condicionado a requisito regulatório explícito.
- **FE-14 — Cold storage para auditoria além de 90 dias**: retenção é 90d com delete físico. Arquivamento para cold storage antes da purga é evolução futura caso compliance exija.
- **FE-15 — Polling adaptativo / batching de insert**: otimizações de performance reservadas para V1.5 se benchmarks/metrics indicarem necessidade. MVP usa `time.Ticker` fixo (default 250ms) e insert linha-a-linha.
- **FE-16 — Live-reload de configuração**: mudança de `outbox.dispatcher.enabled` requer restart do worker. Live-reload de Viper fica como evolução futura.
- **FE-17 — Suporte a outros bancos relacionais**: PostgreSQL é fixo. MySQL/SQL Server/Oracle não estão no escopo (mesmo que parte da arquitetura seja portável em tese).

## Suposições e Decisões Consolidadas

Suposições assumidas (com base nos discoveries aprovados):

- **S-01**: o `cmd/worker` continuará sendo o lar correto para workers de plataforma e tolerará duas novas goroutines de longa duração (Dispatcher + Cron) sem impacto em outras cargas — atualmente está idle.
- **S-02**: o Postgres em produção tem capacidade ociosa para receber polling de ~2 qps por instância em fila vazia (tick=500ms) e tráfego de escrita adicional (+1 linha por handler ativo na transação de publish) sem degradar > 15% sobre baseline de CPU/IO — validar com `pg_stat_statements` em janela de 14 dias pós-deploy.
- **S-03**: retenção de 90 dias atende às exigências regulatórias atuais do produto — premissa assumida pelo escopo do PRD aprovado (D-01); ajuste fica condicionado a sinalização explícita de Legal/Compliance.
- **S-04**: o número médio de handlers por evento permanecerá em 1–3 no primeiro ano; cenários acima de 5 disparam revisão (batching de insert no caminho de publish).
- **S-05**: o time aceita a regra obrigatória de idempotência por `event_id` e a fiscalizará via PR template (a ser criado por esta entrega — RF-40) + code review — sem ferramenta automatizada de detecção de não-idempotência no MVP.
- **S-06**: a coexistência de `events.Bus` e `outbox.Publisher` é viável e será comunicada por documentação (ADR + AGENTS.md) sem necessidade de tooling adicional para guiar a escolha.

Decisões consolidadas (resolvem o conjunto Q-01 a Q-06 originais + ambiguidades detectadas no confronto v3 com o codebase):

- **D-01 (retenção 90d / LGPD)**: retenção de 90 dias considerada aprovada pelo escopo do PRD (substitui Q-01). Tratada como premissa documentada em AGENTS.md/runbook; revisão fica condicionada a sinalização posterior de Legal/Compliance.
- **D-02 (primeiro evento real)**: MVP entrega exclusivamente handler dummy (FC-10/FE-10). A escolha do primeiro evento real de produto a usar `outbox.Publisher` fica para sprint subsequente, sob responsabilidade de PO/produto + tech lead da área escolhida (substitui Q-02).
- **D-03 (defaults numéricos finais)**: `OUTBOX_DISPATCHER_TICK_INTERVAL=500ms`, `OUTBOX_DISPATCHER_BATCH_SIZE=50`, `OUTBOX_DISPATCHER_HANDLER_TIMEOUT=10s`, `OUTBOX_RETRY_MAX_ATTEMPTS=15`, `OUTBOX_RETRY_BASE_BACKOFF=2s`, `OUTBOX_RETRY_MAX_BACKOFF=5m`, `OUTBOX_HOUSEKEEPING_RETENTION_DAYS=90`, `OUTBOX_REAPER_INTERVAL=@every 1m`, `OUTBOX_REAPER_STUCK_AFTER=5m`, `OUTBOX_HOUSEKEEPING_SCHEDULE=@daily` (substitui Q-03). Justificativa do conjunto: `tick=500ms` mantém o orçamento p95 < 1s declarado em R-07; `attempts=15` com `base=2s, cap=5m` produz janela total ~46min (com jitter) compatível com US-11 (~60min); `reaper=@every 1m` + `stuck_after=5m` cumpre o RTO de 5min de US-10.
- **D-04 (pin do cron/v3)**: `github.com/robfig/cron/v3` pinado em `v3.0.1` no `go.mod` inicial (substitui Q-04). Dependabot já configurado (`.github/dependabot.yml`, grupo `go-deps`, semanal) promove minor/patch automaticamente.
- **D-05 (mecanismo da feature flag)**: `OUTBOX_DISPATCHER_ENABLED` lida via Viper no boot do `cmd/worker`; mudança exige restart do worker — sem live-reload no MVP, consistente com o padrão atual de `configs.LoadConfig` (substitui Q-05). RF-29 (rollback < 2min) atendido pelo ciclo `kubectl rollout restart` / equivalente Fly.io.
- **D-06 (alocação/capacidade)**: confirmação da alocação (engenheiro responsável, sprints reservadas) é responsabilidade do PO/produto, fora do escopo do PRD (substitui Q-06). Estimativa do discovery (80–120h-eng) mantida como referência.
- **D-07 (namespace do schema)**: tabelas `outbox_events` e `outbox_deliveries` vivem no schema `public`, consistente com `health_probe` (migração `0001_init.up.sql`). Schema dedicado fica como evolução futura caso isolamento por grants seja exigido.
- **D-08 (geração de EventID)**: `outbox.Event.ID` é fornecido pelo caller (use case), reutilizando `events.NewEventID` existente. Publisher apenas valida não-vazio e unicidade — não gera ULID automaticamente. Garante simetria com `events.Bus` e habilita testes determinísticos.
- **D-09 (escopo de unicidade de `Subscription.Name`)**: a chave de unicidade é o par `(Name, EventType)` (RF-07 literal). Permite reuso do mesmo `Name` entre `event_types` distintos quando semanticamente justificado (ex.: `notifier` em múltiplos módulos). Documentação deve recomendar nomes globalmente distintos para facilitar dashboards e labels métricos.
- **D-10 (`partition_key`)**: coluna `partition_key TEXT NULL` criada na migração `0002_outbox.up.sql` sem uso pelo Dispatcher do MVP (consistente com FE-05). Reservada para particionamento/LISTEN-NOTIFY futuro sem migration adicional. Sem índice no MVP.
- **D-11 (identificador `claimed_by`)**: cada réplica do `cmd/worker` calcula `claimed_by = fmt.Sprintf("%s-%d", hostname, os.Getpid())` no startup (`os.Hostname()` + `os.Getpid()`). Persistido em `outbox_deliveries.claimed_by`. Hostname + pid garante unicidade mesmo se dois pods compartilharem hostname e é legível em dashboards/queries operacionais.
- **D-12 (ADR — número e local)**: a nova ADR vive em `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md`, seguindo a numeração sequencial existente (`adr-001` a `adr-015`) — substitui a referência genérica de RF-37.

---

**Documentos a montante consumidos por este PRD**:

- Brainstorming decisório aprovado: `docs/discoveries/brainstorm-event-driven-outbox-foundation/`.
- Discovery técnico aprovado: `docs/discoveries/technical-outbox-event-driven-foundation/`.

**Próximo passo recomendado**: gerar a especificação técnica via `create-technical-specification` consumindo este PRD + o discovery técnico, que registrará o `spec-hash` deste arquivo para detecção de drift futuro.

---

## Histórico de Versões

- **v4 (2026-06-02)**: rodada de sanação de questões em aberto via múltipla escolha confrontando codebase + discoveries.
  - Substituída a seção "Suposições e Questões em Aberto" por "Suposições e Decisões Consolidadas" com 12 decisões fixadas (D-01 a D-12).
  - RF-12 ajustado: política de retry passa a `base=2s, cap=5min, attempts=15` (janela total ~46min) para cumprir US-11 (~60min). Substitui o conjunto sugerido pelo discovery (`base=1s, attempts=8`) cuja aritmética (~4.25min) era incompatível com US-11.
  - RF-26 / FC-07: defaults numéricos explicitados linha-a-linha — `OUTBOX_DISPATCHER_TICK_INTERVAL=500ms`, `BATCH_SIZE=50`, `HANDLER_TIMEOUT=10s`, `RETRY_MAX_ATTEMPTS=15`, `RETRY_BASE_BACKOFF=2s`, `RETRY_MAX_BACKOFF=5m`, `HOUSEKEEPING_RETENTION_DAYS=90`, `REAPER_INTERVAL=@every 1m`, `REAPER_STUCK_AFTER=5m`, `HOUSEKEEPING_SCHEDULE=@daily`. Flag lida apenas no boot via Viper; restart obrigatório para alterar (sem live-reload no MVP).
  - RF-37: ADR fixada como `adr-016-outbox-publisher-opt-in.md` (D-12).
  - S-02: ajustada para refletir `tick=500ms` (~2 qps em fila vazia, antes ~4 qps com tick=250ms).
  - Decisões agregadas refletem confronto com codebase: schema `public` (D-07), `EventID` fornecido pelo caller (D-08), `Subscription.Name` único por `(Name, EventType)` (D-09), `partition_key` NULLável reservada sem índice (D-10), `claimed_by = hostname+pid` (D-11), pin `robfig/cron/v3@v3.0.1` (D-04).
- **v3 (2026-06-02)**: segunda passada de confronto com o codebase.
  - RF-09 / FC-03 / novo RF-39: explicita que Outbox implementa `runtime.Subsystem` (`Start`/`Stop`/`Name`) como agregador único de Dispatcher + Cron, registrado em `buildSubsystems(ModeWorker)`.
  - RF-02 / RF-13: error sentinels exportados (`outbox.ErrPermanent`, `outbox.ErrHandlerNotRegistered`, `outbox.ErrDispatcherDisabled`); sem mapeamento Problem Details (ADR-004 cobre HTTP, não caminho assíncrono interno).
  - R-01: lista explicitamente o que vem de devkit-go (database, uow, manager, observability) e confirma que devkit-go não oferece helper de Outbox; única dep nova é `robfig/cron/v3`.
  - RF-38: além do `AGENTS.md`/`CLAUDE.md` raiz, exige `internal/infrastructure/outbox/AGENTS.md` por consistência com o padrão por-módulo já estabelecido.
  - RF-37: ancora a ADR nova no diretório de ADRs do PRD foundation (`.specs/prd-mecontrola-foundation/adr-016-*`).
  - Novo RF-40: criação de `.github/PULL_REQUEST_TEMPLATE.md` (hoje inexistente) com checklist condicional de Outbox/handler + seções genéricas reaproveitáveis.
  - Q-04 / S-05: notas ajustadas refletindo Dependabot já configurado e o novo RF-40 como fonte de enforcement do PR template.
- **v2 (2026-06-02)**: confronto com codebase real e ajustes de fidelidade.
  - RF-01 / FC-01 / US-01: contrato do Publisher passa a receber `database.DBTX` (compatível com `UnitOfWork[T].Do`, ADR-002), não `pgx.Tx`.
  - RF-03 / FC-02: reutilização explícita de `events.EventID` e `events.EventName` como tipos canônicos; sem duplicação em `internal/infrastructure/outbox/`.
  - RF-26 / FC-07 / US-06: feature flag e demais chaves de configuração migradas para padrão flat `SCREAMING_SNAKE` consistente com `DBConfig`/`O11yConfig` (`OUTBOX_DISPATCHER_ENABLED`, `OUTBOX_RETENTION_DAYS`, etc.). Novo grupo `OutboxConfig` agregado a `configs.Config` via `mapstructure:",squash"`.
  - R-01: atualizada para listar `UnitOfWork[T]`, `database.DBTX` e o slot vazio de `ModeWorker` em `internal/infrastructure/runtime/bootstrap.go` como pontos de integração concretos.
  - Q-05: reescrita refletindo o padrão real do projeto (Viper carregada no boot, sem live-reload por default).
- **v1 (2026-06-02)**: emissão inicial consumindo brainstorming + discovery técnico aprovados.
