# Documento de Requisitos do Produto (PRD) — Ordenação e Idempotência do Fluxo WhatsApp do Agente

<!-- spec-version: 3 -->

- Origem: `docs/runs/2026-07-01-diagnostico-mensagens-fora-ordem-arquitetura-10k.md`
- Tipo: remediação de confiabilidade + preparação para escala (0 → 10.000 usuários ativos)
- Histórico: v1 (rascunho com questões em aberto) → v2 (decisões D-01..D-08 travadas) →
  v3 (auditoria PRD×techspec×ADRs + verificação contra código e produção; decisões D-09..D-20
  travadas: contradição de deploy resolvida, traceparent obrigatório, SLOs travados, idempotência
  natural de domínio + defesa em profundidade, dead-letter de poison, gate de carga sintética;
  zero questões em aberto, zero ressalvas)

## Visão Geral

O fluxo conversacional do WhatsApp do MeControla apresenta **mensagens fora de ordem, respostas
incoerentes, reinício repetido de onboarding e confirmações de lançamento sem efeito real**. O
diagnóstico de 2026-07-01 (usuário `06edc407-…`) provou, com evidência de código e de produção, que a
causa não é a lógica do agente em si, mas a **ausência de serialização por usuário** no pipeline (2
servers + 2 workers processando o mesmo outbox em paralelo, sem ordem por conversa), somada a
**escrita não-idempotente sob redelivery**, **advisory lock desligado e incompatível com o pgbouncer
em `pool_mode=transaction`**, **TOCTOU no reinício de onboarding** e **confirmação de escrita
"alucinada"** (o agente diz "registrado com sucesso" sem persistir). Um deploy storm foi apenas o
gatilho que expôs essas fraquezas permanentes.

Esta funcionalidade elimina essas fraquezas para que **uma conversa (thread) seja sempre processada
por um único executor por vez, em ordem, com idempotência ponta-a-ponta e confirmações honestas**,
preparando a arquitetura para crescer de 0 a 10.000+ usuários ativos sem reescrita estrutural entre
fases — usando apenas os componentes já existentes (outbox Postgres), sem introduzir Kafka/NATS,
sharding ou cache distribuído.

## Objetivos

- **Ordem garantida por usuário:** dois eventos inbound do mesmo usuário nunca são processados
  concorrentemente; a resposta N+1 nunca precede a N.
- **Idempotência ponta-a-ponta:** redelivery de evento (deploy, retry, reprocesso) nunca gera
  lançamento duplicado nem resposta contraditória.
- **Confirmação honesta:** o agente só confirma "registrado com sucesso" quando o dado foi
  efetivamente persistido; zero sucesso alucinado; zero mensagem vazia enviada.
- **Onboarding sem loop:** reduzir a taxa de `onboarding_error` de ~68% (janela do incidente) para
  < 2%; nenhum reinício de onboarding para usuário que já respondeu/concluiu.
- **Deploy sem storm:** releases não drenam runs em massa nem disparam reprocesso concorrente.
- **Observabilidade do caminho crítico:** o percurso `webhook → agente → LLM → envio` é rastreável
  (traces) e as corridas de concorrência são mensuráveis (métrica de conflito).
- **Escala:** suportar 10.000 usuários ativos por escalonamento horizontal de workers, sem reescrita
  estrutural entre as fases 0–500 / 500–2.000 / 2.000–10.000.

Métricas-chave a acompanhar: `onboarding_error` %, taxa de duplicidade de escrita (lançamentos
duplicados por `wamid`), divergência confirmação-vs-`agents_write_ledger`, lag de publicação do
outbox (p95), `workflow_version_conflict_total`, cobertura de traces do caminho inbound, taxa de
mensagens outbound vazias (deve ser 0).

## Histórias de Usuário

- Como **usuário do WhatsApp**, quero que minhas respostas sejam entendidas na ordem em que envio,
  para que a conversa faça sentido e o onboarding não recomece.
- Como **usuário do WhatsApp**, quero que "gastei 150 no pix" seja registrado de fato quando o bot
  disser "registrado com sucesso", para que meu relatório reflita a realidade.
- Como **usuário do WhatsApp**, quero concluir o onboarding uma única vez, sem repetir objetivo/renda,
  para não desistir por frustração.
- Como **usuário do WhatsApp**, nunca quero receber uma mensagem vazia do bot.
- Como **operador da plataforma**, quero fazer deploy sem gerar reprocesso em massa e respostas
  duplicadas para os usuários ativos naquele momento.
- Como **operador/engenheiro**, quero rastrear o caminho inbound→agente→LLM→envio e medir conflitos
  de concorrência, para diagnosticar incidentes em minutos, não horas.
- Como **negócio**, quero crescer de 0 para 10.000 usuários ativos sem que a qualidade de conversa
  degrade e sem reescrever a arquitetura a cada patamar.

## Funcionalidades Core

1. **Serialização por usuário via claim particionado.** O outbox entrega no máximo 1 evento inbound
   "em voo" por `aggregate_user_id`; o próximo evento do usuário só é reivindicado após o anterior
   concluir. Garante ordem sem segurar conexão de banco durante o LLM.
2. **Idempotência ponta-a-ponta ligada por padrão.** `agents_write_ledger` como fonte de verdade;
   redelivery reconhece trabalho já feito e não repete escrita nem resposta.
3. **Confirmação condicionada à persistência.** A tool de escrita retorna resultado tipado; o agente
   só afirma sucesso quando a tool confirma gravação; caso contrário responde honestamente.
4. **Onboarding resiliente a concorrência e rastreável.** Reinício só quando legítimo; conflito de run
   ativo é tratado como retomada; turnos de onboarding passam a ser persistidos em `platform_messages`.
5. **Deploy seguro.** Atualização sem storm nem drain em massa de runs em andamento.
6. **Observabilidade do caminho crítico.** Traces ponta-a-ponta e métrica de conflito de concorrência;
   versão de telemetria correta.
7. **Robustez de ingestão.** Processar todas as mensagens de um webhook em lote e preservar a ordem
   real do usuário (timestamp da Meta).

## Decisões Travadas (resolvem as questões em aberto da v1)

- **D-01 (RF-01/02):** serialização por **claim particionado** — no máximo 1 evento em voo por
  `aggregate_user_id`; próximo só após o anterior concluir. Não segura lock durante o LLM. Se algum
  lock advisory for usado como auxiliar de reserva, DEVE ser de **escopo de transação**
  (`pg_advisory_xact_lock`), nunca de sessão. Travas presas por crash são limpas pelo reaper existente
  (`STUCK_AFTER=5m`).
- **D-02 (RF-11):** onboarding passa a **persistir turnos** (inbound+outbound) em `platform_messages`,
  tornando empty-reply e fora-de-ordem diagnosticáveis.
- **D-03 (RF-06/07):** confirmação honesta via **resultado tipado da tool** (persistido/duplicado/erro);
  sem query extra de releitura.
- **D-04 (RF-12):** configuração de **rolling deploy seguro** faz parte do escopo deste PRD.
- **D-05 (SLO — travado, com justificativa):** lag webhook→publicação do outbox **p95 < 5s**
  (headroom ≈1.6× sobre o round-trip do LLM ~3s), **alerta em > 30s sustentado**;
  **duplicidade de lançamento = 0** (absoluto — qualquer duplicata é bug e dispara alerta imediato);
  **onboarding_error < 2%** (baseline de 68% na janela do incidente). Estes números são metas
  **verificadas** pelo gate de carga sintética (D-20), não parametrização adiada.
- **D-06 (escala):** **escalonamento horizontal** de workers; o claim particionado suporta N workers
  sem quebrar a ordem por usuário; `default_pool_size`/`DB_MAX_CONNS` ajustados junto. Sem novo
  componente de infra.
- **D-07 (RF-17):** processar **todas** as mensagens de um webhook em lote (não só a primeira).
- **D-08 (RF-18):** ordenação FIFO por usuário por **timestamp da mensagem da Meta**, com `created_at`
  do outbox como desempate (robusto a skew entre server-1/2).

### Decisões v3 (auditoria + verificação contra código e produção)

- **D-09 (RF-13):** deploy usa **`order: stop-first`** + `stop_grace_period: 30s` (shutdown do app
  ≈15s) + **gate de CI anti-storm**. Resolve a contradição da v2 (RF-13 dizia `start-first`, que o
  ADR-004 rejeita por exigir 2 tasks transitórias no nó único). Ataca a causa (storm), não a janela
  por-serviço; Caddy roteia para o outro server durante o update.
- **D-10 (RF-14):** propagação do `traceparent` (W3C) no `metadata` (JSONB) do `outbox_events` é
  **obrigatória**, não opcional — sem ela o hop assíncrono server→worker quebra o trace e a correlação
  fim-a-fim por `run_id`/`thread_id` não é atingível.
- **D-11 (D-05):** SLOs **travados agora** com justificativa (ver D-05); a antiga "Suposição Residual"
  de parametrização adiada é **removida**.
- **D-12 (rastreabilidade):** o relatório de diagnóstico de origem é **recriado e versionado** no
  caminho citado, consolidando as evidências de código (file:line) e de produção (ledger vazio,
  1 usuário, 118 eventos na janela do incidente).
- **D-13 (RF-17):** um webhook com N mensagens gera **1 evento outbox por mensagem** (cada com seu
  `wamid` e o timestamp da Meta); `item_seq` permanece como índice de escrita dentro do turno de uma
  mensagem. Formalizado no **ADR-005**.
- **D-14 (RF-01):** colisão do índice único em voo (`SQLSTATE 23505`) no claim particionado é
  **capturada e adiada** para o próximo tick (o `UPDATE ... FROM claimable` é atômico por statement;
  a colisão é rara sob `FOR UPDATE SKIP LOCKED` + `NOT EXISTS`). Não é erro fatal.
- **D-15 (RF-11):** turnos de onboarding são persistidos na **mesma thread** do agente
  (`resourceId=userID, threadId=peer`), para histórico único e diagnóstico de empty-reply.
- **D-16 (RF-02):** D-01 permanece como **guardrail** (se algum lock advisory for reintroduzido, DEVE
  ser xact-scoped); o design atual usa **zero lock** (claim particionado + UNIQUE do ledger).
- **D-17 (RF-04/20) [corrigido v3 — falso-positivo removido]:** a idempotência de **escrita de
  domínio** JÁ é garantida por **chave natural** nos módulos consumidores — verificado em produção:
  `transactions_origin_uk` e `transactions_card_purchases_origin_uk` já existem, com `origin` cabeado
  ponta-a-ponta nas 3 tools e reconciliação (`Reconciled`) no conflito. **Não é trabalho novo** (não
  requer migration nova); o requisito é **preservar** essa proteção, mapear o conflito para
  `ToolOutcomeReconciled`/replay, e exigir `origin`+UNIQUE em qualquer nova tool de escrita. O
  `agents_write_ledger` continua registro de replay do agente. *(A alegação anterior da v3 de "duplo
  write de domínio catastrófico" era falso-positivo por leitura incompleta do schema.)*
- **D-18 (RF-21) [rebaixado v3]:** o timeout de **LLM/tool ≪ `STUCK_AFTER`** (ex.: 90s < 5m) é
  **hardening de coerência** — impede re-pick concorrente pelo reaper que geraria 2ª resposta fora de
  ordem — não correção de integridade financeira (D-17 já cobre). A reserva `ledger-first` é opcional
  (redundante com a chave natural do domínio).
- **D-19 (RF-22):** um evento inbound poison (falha permanente) faz **head-of-line blocking** no FIFO
  do usuário; mitiga-se com `max_attempts`/backoff dos eventos inbound dimensionados para dead-letter
  (`status=4`) rápido (~1 turno de conversa), preservando FIFO estrito; alerta em `status=4 > 0`.
- **D-20 (RF-23):** a escala (10k) e os SLOs são validados por **gate de carga sintética por fase**
  (prova CA-01 e CA-08 nas fronteiras 500/2.000/10.000) **antes** de captar usuários reais — produção
  hoje tem 1 usuário, sem baseline; os SLOs de D-05 são metas verificadas por esse gate.

## Requisitos Funcionais

### P0 — Correção crítica (bloqueia captação de usuários)

- RF-01: O processamento de eventos inbound (`agents.whatsapp.inbound.v1`) DEVE ser serializado por
  `aggregate_user_id`, de modo que dois eventos do mesmo usuário nunca executem concorrentemente,
  mesmo com 2+ workers ativos, via **claim particionado** (no máximo 1 evento em voo por usuário).
- RF-02: A serialização NÃO DEVE segurar conexão de banco/transação aberta durante a chamada ao LLM.
  Qualquer lock advisory auxiliar DEVE ser de escopo de transação (`pg_advisory_xact_lock`); é
  PROIBIDO lock de escopo de sessão (`pg_advisory_lock`), inseguro sob pgbouncer `pool_mode=transaction`.
- RF-03: Dentro de um mesmo usuário, os eventos inbound DEVEM ser processados em ordem (FIFO por
  usuário); a resposta a uma mensagem posterior nunca é enviada antes da resposta à anterior.
- RF-04: A idempotência de escrita por `(wamid, item_seq, operation)` DEVE estar ativa por padrão (sem
  depender de flag de ambiente), com `agents_write_ledger` como fonte de verdade.
- RF-05: Um evento inbound reprocessado (redelivery por deploy/retry) com o mesmo `message_id` NÃO
  DEVE produzir segunda escrita nem resposta duplicada/contraditória para a mesma ação.
- RF-06: O agente só DEVE emitir confirmação de sucesso após a tool correspondente retornar resultado
  tipado indicando persistência efetiva; em falha, DEVE responder de forma honesta, nunca sucesso.
- RF-07: A tool de escrita DEVE ser um adaptador fino que propaga o resultado real da persistência
  (persistido/duplicado/erro) ao agente (R-AGENT-WF-001.2 e R-ADAPTER-001), sem regra de negócio.
- RF-08: O sistema NUNCA DEVE enviar mensagem outbound de conteúdo vazio ao WhatsApp; saída vazia do
  LLM DEVE ser convertida em resposta honesta de fallback (nunca um envio em branco).
- RF-20: A idempotência da **escrita de domínio** (a mutação real) É garantida por **chave natural**
  no módulo consumidor e DEVE ser **preservada**. Estado verificado em produção (2026-07-02): já
  existem `transactions_origin_uk` e `transactions_card_purchases_origin_uk`
  (UNIQUE `(origin_wamid, origin_item_seq, origin_operation) WHERE origin_wamid IS NOT NULL`), com o
  `origin` cabeado ponta-a-ponta (tool → `RawTransaction`/`RawCardPurchase` → `SetOrigin` → persistido)
  para as 3 tools do agente (expense/income/card); o usecase já devolve `Reconciled` no conflito. Logo,
  o duplo `write()` de domínio sob corrida já é **prevenido e reconciliado** independente de lock/claim.
  Requisito: o conflito da chave natural DEVE mapear para `ToolOutcomeReconciled`/replay (nunca
  `usecaseError` nem confirmação de sucesso falsa), e **toda nova tool de escrita** DEVE carregar
  `origin` e ter UNIQUE natural equivalente no alvo.
- RF-21: **Hardening de coerência** (não correção de dinheiro — RF-20 já cobre a integridade): impor
  **timeout de contexto na chamada de LLM/tool estritamente menor que `STUCK_AFTER`**, para o worker
  original concluir/liberar antes de o reaper resetar o evento (`status=2→1`) e evitar re-pick
  concorrente que gere 2ª resposta fora de ordem. Reserva ledger-first é opcional (redundante com a
  chave natural do domínio).

### P1 — Estabilidade e visibilidade

- RF-09: Violação do índice `workflow_runs_active_key_uidx (workflow, correlation_key)` ao iniciar um
  onboarding DEVE ser tratada como **retomada (resume)** do run ativo, não como erro genérico.
- RF-10: A resolução onboarding-vs-agente DEVE ser atômica sob a serialização por usuário (RF-01),
  eliminando a janela TOCTOU entre "checar run ativo / working memory" e "iniciar run".
- RF-11: Os turnos de onboarding (inbound e outbound) DEVEM ser persistidos em `platform_messages`,
  com a mesma semântica de ordem do agente, para rastreabilidade e diagnóstico.
- RF-12: Um usuário que já concluiu o onboarding (marcador de conclusão presente) NÃO DEVE ter o
  onboarding reiniciado; a mensagem segue para o agente.
- RF-13: O rolling deploy DEVE ser configurado para não drenar runs em massa nem disparar reprocesso
  concorrente: **`order: stop-first`**, `max_parallelism: 1`, `stop_grace_period: 30s` (≥ shutdown
  cooperativo do app, ≈15s) e **gate de CI anti-storm** que serializa/consolida releases, aproveitando
  o outbox reaper existente (`STUCK_AFTER=5m`). É PROIBIDO `order: start-first` (exigiria 2 tasks
  transitórias no nó único — ver ADR-004).
- RF-14: O caminho crítico DEVE emitir spans de tracing em `whatsapp.handler.inbound`, no agent runtime
  e na chamada ao LLM, correlacionáveis por `run_id`/`thread_id`.
- RF-15: DEVE existir um contador `workflow_version_conflict_total` (ou equivalente) expondo corridas
  de concorrência, com cardinalidade controlada (sem `user_id`/`correlation_key` como label —
  R-TXN-004 / R-WF-KERNEL-001.4).
- RF-16: A versão de telemetria (`OTEL_SERVICE_VERSION`) DEVE refletir o binário efetivamente em
  execução, sem divergência de tag.
- RF-22: Um evento inbound com falha permanente (poison) NÃO DEVE bloquear indefinidamente o FIFO do
  usuário: `max_attempts`/backoff dos eventos inbound DEVEM ser dimensionados para dead-letter
  (`status=4`) rápido (≈1 turno de conversa), preservando FIFO estrito; DEVE haver alerta em
  `status=4 > 0`. O reaper (`STUCK_AFTER`) permanece como rede de segurança para leases órfãos.

### P2 — Robustez incremental e tuning

- RF-17: A ingestão DEVE processar todas as mensagens de um webhook em lote (não apenas a primeira,
  hoje `ExtractFirstMessage`), gerando **1 evento outbox por mensagem** (cada com seu `wamid` e o
  timestamp da Meta), preservando a ordem real do usuário (ver ADR-005).
- RF-18: A ordem FIFO por usuário DEVE usar o **timestamp da mensagem da Meta** como critério primário
  e o `created_at` do outbox como desempate, independente do agendamento de retry.
- RF-19: Os parâmetros de vazão/capacidade (tamanho de lote do dispatcher, `default_pool_size` do
  pgbouncer, `DB_MAX_CONNS`, número de réplicas de worker) DEVEM ser dimensionados por fase para que a
  serialização por usuário não cause contenção de conexões, suportando escalonamento horizontal.
- RF-23: A escala (0 → 10.000) e os SLOs (D-05) DEVEM ser validados por um **gate de carga sintética
  por fase** que prove CA-01 e CA-08 nas fronteiras 500 / 2.000 / 10.000 **antes** de captar usuários
  reais — produção hoje tem 1 usuário, sem baseline; o custo do `NOT EXISTS` por usuário e o
  dimensionamento de pool só são verificáveis sinteticamente.

## Experiência do Usuário

- Conversa coerente: cada resposta reflete a última mensagem do usuário, na ordem correta.
- Onboarding único e linear: sem repetição de objetivo/renda; sem "voltar ao início".
- Confiança no registro: quando o bot confirma um lançamento, ele existe no relatório; quando falha,
  o bot diz honestamente e o usuário pode repetir. Nunca uma mensagem vazia.
- Sem regressão de latência perceptível: a serialização por usuário não deve tornar a resposta mais
  lenta do que o round-trip do LLM (hoje p95 ~3s) para o caminho de um único usuário.

## Restrições Técnicas de Alto Nível

- **Fronteiras arquiteturais inegociáveis:** respeitar R-WF-KERNEL-001 (kernel de workflow genérico,
  sem domínio/SQL fora do adapter/LLM), R-AGENT-WF-001 (Thread/Run/WorkingMemory/PendingStep como
  primitivos de plataforma; tool fina; estados como tipos fechados), R-ADAPTER-001 (adaptadores finos,
  zero comentários em Go de produção) e a skill `go-implementation`.
- **Sem novos componentes de infraestrutura:** proibido introduzir Kafka/NATS, sharding de banco ou
  cache distribuído — a solução usa o outbox Postgres existente + claim particionado. Escala até 10k
  por escalonamento horizontal de workers.
- **Compatibilidade com pgbouncer `pool_mode=transaction`:** nada pode segurar conexão durante o LLM;
  qualquer lock DEVE ser transação-scoped.
- **Cardinalidade de métricas controlada:** sem `user_id`/`correlation_key`/`category_id` como label.
- **Meta de escala:** 0 → 10.000 usuários ativos, com plano evolutivo em três fases sem reescrita
  estrutural entre elas.
- **Sensibilidade de dado financeiro:** perda silenciosa de lançamento é inaceitável; idempotência,
  confirmação honesta e rastreabilidade têm precedência.

## Critérios de Sucesso Mensuráveis (aceite)

- CA-01 (RF-01/02/03): teste de integração enviando N mensagens rápidas do mesmo usuário com 2 workers
  ativos comprova zero execução concorrente por usuário e ordem FIFO preservada nas respostas; nenhum
  passo segura conexão de banco durante o LLM.
- CA-02 (RF-04/05): reprocessar o mesmo evento inbound (simular redelivery) resulta em **1** linha em
  `agents_write_ledger` e **0** lançamentos/respostas duplicados.
- CA-03 (RF-06/07/08): para toda confirmação de sucesso existe a linha persistida correspondente
  (divergência = 0); saída vazia do LLM nunca vira envio em branco (taxa de outbound vazio = 0).
- CA-04 (RF-09/10/11/12): teste de concorrência de onboarding não gera `onboarding_error`; usuário
  concluído não reinicia; turnos de onboarding aparecem em `platform_messages`; `onboarding_error` em
  produção < 2%.
- CA-05 (RF-13): ensaio de rolling deploy sob carga sintética de conversas não produz respostas
  duplicadas nem lag de publicação de outbox p95 ≥ 5s.
- CA-06 (RF-14/15/16): traces cobrem webhook→agente→LLM→envio; `workflow_version_conflict_total`
  presente e observável; `OTEL_SERVICE_VERSION` == tag do binário.
- CA-07 (RF-17/18): webhook com múltiplas mensagens processa todas na ordem do timestamp da Meta.
- CA-08 (D-05/RF-19): sob carga sintética escalada, lag p95 < 5s e 0 duplicidade mantidos ao adicionar
  réplicas de worker; pool de conexões não satura.
- CA-09 (RF-20/21): teste que força dupla execução do mesmo `(origin_wamid, origin_item_seq,
  origin_operation)` — inclusive simulando reset do reaper durante um `write()` lento — resulta em
  **1** mutação de domínio (a chave natural `transactions_origin_uk`/`card_purchases_origin_uk` rejeita
  a 2ª, retornando `Reconciled`), o outcome mapeia para `reconciled`/replay (nunca `usecaseError` nem
  sucesso falso), e o timeout de LLM dispara antes do `STUCK_AFTER` (evita a 2ª resposta fora de ordem).
- CA-10 (RF-22): um evento inbound poison vai a dead-letter (`status=4`) dentro do orçamento de
  `max_attempts` sem bloquear indefinidamente os eventos seguintes do usuário; alerta de `status=4`
  dispara; FIFO das mensagens não-poison é preservado.

## Fora de Escopo

- Reescrita do agente ou do onboarding conversacional (apenas correção de concorrência/idempotência/
  confirmação/persistência de turnos; a lógica de negócio permanece).
- Introdução de broker externo (Kafka/NATS/SQS), sharding de banco ou cache distribuído.
- Separar o dispatcher de inbound em serviço dedicado — considerado só se métricas provarem contenção
  na fase 2.000–10.000 (não é requisito desta iniciativa).
- Otimização de custo/latência do LLM e seleção de modelo por classe de tarefa.
- Multi-turn ao LLM (não-goal do MVP, já decidido).
- Hardening de segredos em texto plano nos service specs (item de hardening separado; registrado como
  risco no relatório de diagnóstico).
- Implementação de código: esta sessão entrega apenas o PRD; techspec/tasks e código são etapas
  posteriores.

## Suposições Residuais

- A topologia base 2 servers + 2 workers é mantida; a solução escala adicionando réplicas de worker
  (D-06), não reduzindo instâncias.
- O timestamp da mensagem da Meta (RF-18) tem granularidade suficiente para ordenar turnos humanos;
  o `created_at` do outbox cobre empates dentro do mesmo segundo.
- Os SLOs de D-05 estão **travados** com justificativa e são verificados pelo gate de carga sintética
  (D-20/RF-23); não há parametrização adiada. A escala de 10k é forward-looking (produção tem 1
  usuário) e sua prova é sintética, não observação de baseline de produção.
- Nenhuma questão em aberto material permanece; nenhuma ressalva pendente.
