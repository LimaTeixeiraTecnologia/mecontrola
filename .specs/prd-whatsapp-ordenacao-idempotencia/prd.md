# Documento de Requisitos do Produto (PRD) — Ordenação e Idempotência do Fluxo WhatsApp do Agente

<!-- spec-version: 2 -->

- Origem: `docs/runs/2026-07-01-diagnostico-mensagens-fora-ordem-arquitetura-10k.md`
- Tipo: remediação de confiabilidade + preparação para escala (0 → 10.000 usuários ativos)
- Histórico: v1 (rascunho com questões em aberto) → v2 (decisões D-01..D-08 travadas; zero questões em aberto)

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
- **D-05 (SLO):** lag webhook→publicação do outbox **p95 < 5s** (alerta em > 30s sustentado);
  **duplicidade de lançamento = 0** (qualquer duplicata é bug e dispara alerta imediato).
- **D-06 (escala):** **escalonamento horizontal** de workers; o claim particionado suporta N workers
  sem quebrar a ordem por usuário; `default_pool_size`/`DB_MAX_CONNS` ajustados junto. Sem novo
  componente de infra.
- **D-07 (RF-17):** processar **todas** as mensagens de um webhook em lote (não só a primeira).
- **D-08 (RF-18):** ordenação FIFO por usuário por **timestamp da mensagem da Meta**, com `created_at`
  do outbox como desempate (robusto a skew entre server-1/2).

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
  concorrente (`order: start-first`, `max_parallelism: 1`, `stop-grace-period` suficiente para drain
  cooperativo), aproveitando o outbox reaper existente (`STUCK_AFTER=5m`).
- RF-14: O caminho crítico DEVE emitir spans de tracing em `whatsapp.handler.inbound`, no agent runtime
  e na chamada ao LLM, correlacionáveis por `run_id`/`thread_id`.
- RF-15: DEVE existir um contador `workflow_version_conflict_total` (ou equivalente) expondo corridas
  de concorrência, com cardinalidade controlada (sem `user_id`/`correlation_key` como label —
  R-TXN-004 / R-WF-KERNEL-001.4).
- RF-16: A versão de telemetria (`OTEL_SERVICE_VERSION`) DEVE refletir o binário efetivamente em
  execução, sem divergência de tag.

### P2 — Robustez incremental e tuning

- RF-17: A ingestão DEVE processar todas as mensagens de um webhook em lote (não apenas a primeira),
  preservando a ordem real do usuário.
- RF-18: A ordem FIFO por usuário DEVE usar o **timestamp da mensagem da Meta** como critério primário
  e o `created_at` do outbox como desempate, independente do agendamento de retry.
- RF-19: Os parâmetros de vazão/capacidade (tamanho de lote do dispatcher, `default_pool_size` do
  pgbouncer, `DB_MAX_CONNS`, número de réplicas de worker) DEVEM ser dimensionados por fase para que a
  serialização por usuário não cause contenção de conexões, suportando escalonamento horizontal.

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
- Nenhuma questão em aberto material permanece; SLOs numéricos definitivos de alerta (D-05) serão
  parametrizados na techspec a partir do baseline de produção, sem alterar os requisitos acima.
