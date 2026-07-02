# Prompt Enriquecido - Revisao criteriosa do PRD `whatsapp-ordenacao-idempotencia`

> Artefato gerado pela skill `prompt-enricher` em 2026-07-02.
> Este arquivo e um PROMPT para execucao posterior por um agente. Ele nao executa review, bugfix, SSH, merge nem qualquer alteracao por si so.

---

## 1. Prompt original

Use `@.claude/skills/review/` de forma criteriosa e sem flexibilizacao, validando estritamente contra `.specs/prd-whatsapp-ordenacao-idempotencia`.

Criterios obrigatorios:

- Todos os criterios de aceite atendidos (implementados).
- DoD 100% atendido (implementado).
- 0 gaps.
- 0 lacunas.
- 0 falsos positivos.
- 0 ressalvas.
- Todas as regras de negocio atendidas (implementadas).

Caso encontre qualquer problema, use `@.claude/skills/bugfix/` e repita o ciclo `review -> bugfix -> review` ate obter `APPROVED`, sem falsos positivos e em conformidade total com a especificacao.

Dispare subagentes especializados quando agregarem qualidade a revisao.

E obrigatorio para encerrar o ciclo e o veredito de realmente pronto para `main` utilizar `ssh root@187.77.45.48`, consultar logs, tracing, metricas e DB para fechar todas as lacunas, ressalvas e gaps, e entao concluir o merge para `main`.

---

## 2. Prompt enriquecido

Execute a skill `@.claude/skills/review/` de forma estrita, deterministica e sem flexibilizacao para revisar o estado atual do repositorio contra a especificacao canonica em `.specs/prd-whatsapp-ordenacao-idempotencia/`.

### Objetivo inegociavel

Encerrar o ciclo apenas quando houver evidencia suficiente para um veredito final de `APPROVED` e `PRONTO PARA MAIN`, com:

- 100% dos criterios de aceite implementados e comprovados.
- 100% do DoD das 9 tarefas implementado e comprovado.
- 0 gaps.
- 0 lacunas.
- 0 falsos positivos.
- 0 ressalvas.
- 100% das regras de negocio implementadas e comprovadas.

`APPROVED_WITH_REMARKS` nao encerra o ciclo. Qualquer finding, risco nao fechado, criterio nao comprovado ou validacao faltante impede o encerramento.

### Escopo obrigatorio de leitura

Leia integralmente, antes do veredito:

- `AGENTS.md`
- `@.claude/skills/review/SKILL.md`
- `@.claude/skills/bugfix/SKILL.md` quando houver findings acionaveis
- `.specs/prd-whatsapp-ordenacao-idempotencia/prd.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/techspec.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/tasks.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/task-1.0-migration-indices-claim.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/task-2.0-claim-particionado.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/task-3.0-ingestao-lote-timestamp-meta.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/task-4.0-confirmacao-honesta.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/task-5.0-idempotencia-default-reconciled-timeout.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/task-6.0-onboarding-start-resume.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/task-7.0-observabilidade-deploy-deadletter.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/task-8.0-testes-integracao-testcontainers.md`
- `.specs/prd-whatsapp-ordenacao-idempotencia/task-9.0-carga-sintetica-ensaio-deploy.md`
- Todos os ADRs e `*_execution_report.md` do mesmo diretorio quando impactarem o criterio avaliado

### Fonte de verdade

A fonte de verdade e o conjunto `PRD + techspec + ADRs + tasks + execution reports + codigo real + evidencias locais + evidencias de producao`.

Nao aprove por inferencia.
Nao aceite checklist marcada sem reproducao.
Nao aceite execucao reportada sem evidencia atual.
Nao aceite criterio "provavelmente atendido".

### Requisitos funcionais que DEVEM ser confrontados 1:1

Confronte e evidencie todos os RFs do PRD, sem pular nenhum:

- RF-01 a RF-08: serializacao por usuario, sem lock de sessao, FIFO, idempotencia default, redelivery sem duplicidade, confirmacao honesta, tool adapter fino, sem outbound vazio.
- RF-09 a RF-16: onboarding start/resume idempotente, eliminacao de TOCTOU, persistencia de turnos em `platform_messages`, onboarding nao reinicia indevidamente, deploy seguro `stop-first`, tracing correlacionavel, metrica de conflito, `OTEL_SERVICE_VERSION` coerente.
- RF-17 a RF-19: ingestao em lote do webhook, ordenacao pelo timestamp da Meta com desempate adequado, tuning de capacidade por fase.
- RF-20 a RF-23: preservacao da chave natural de idempotencia de dominio, timeout `LLM/tool < STUCK_AFTER`, poison message indo para dead-letter dentro do orcamento, gate de carga sintetica nas fronteiras 500 / 2.000 / 10.000.

### Criterios de aceite que DEVEM estar verdes

Todos os criterios CA do PRD precisam estar comprovadamente verdes:

- CA-01: duas ou mais workers com N mensagens rapidas do mesmo usuario sem execucao concorrente e com FIFO preservado.
- CA-02: redelivery do mesmo inbound gera 1 registro no ledger e 0 duplicidade de escrita/resposta.
- CA-03: sem sucesso alucinado e sem outbound vazio; validacao com LLM real e obrigatoria quando aplicavel.
- CA-04: onboarding concorrente resulta em 1 run ativo, retomada da segunda execucao e 0 `onboarding_error`.
- CA-05: rolling deploy sob carga sem respostas duplicadas nem lag p95 fora da meta.
- CA-06: traces fim a fim, conflitos observaveis e `OTEL_SERVICE_VERSION` igual ao binario.
- CA-07: webhook com multiplas mensagens processa todas na ordem correta.
- CA-08: carga sintetica em 500 / 2.000 / 10.000 com lag p95 < 5s, 0 duplicidade e pool sem saturacao.
- CA-09: corrida forcada no mesmo `origin` produz 1 mutacao de dominio, retorna `Reconciled` e nao gera falso sucesso.
- CA-10: poison inbound vai para dead-letter (`status=4`) sem bloquear indefinidamente os seguintes.

### Regras de negocio obrigatorias

Valide explicitamente, com evidencia, que as seguintes regras de negocio estao implementadas:

1. O outbox processa no maximo 1 inbound em voo por `aggregate_user_id`.
2. A ordem por usuario e FIFO usando `occurred_at` derivado do timestamp da Meta, com desempate seguro.
3. Nao ha lock de sessao (`pg_advisory_lock`) segurando conexao sob `pgbouncer` em `pool_mode=transaction`.
4. A chamada ao LLM e a tool nao mantem transacao/conexao aberta durante o processamento longo.
5. O agente so confirma sucesso quando a persistencia de fato ocorreu.
6. Resposta vazia nunca e enviada; ha fallback honesto.
7. O conflito da chave natural de dominio mapeia para `ToolOutcomeReconciled`/replay, nunca para sucesso falso.
8. O onboarding concorrente vira resume, nao erro generico.
9. Os turnos de onboarding passam a existir em `platform_messages`.
10. O deploy seguro usa `order: stop-first`, `max_parallelism: 1`, `stop_grace_period: 30s` e gate anti-storm.
11. O tracing cobre `webhook -> agente -> LLM -> envio`, preservando correlacao por `run_id`/`thread_id`.
12. O tuning e a carga sintetica por fase existem e fecham o argumento de escala ate 10k.

### Confronto obrigatorio por tarefa

Para cada tarefa `1.0` a `9.0`:

- abrir a task file correspondente;
- abrir o execution report correspondente;
- confrontar cada item de `## Criterios de Sucesso` e DoD com o codigo e com as validacoes reais;
- marcar cada item apenas como `atendido`, `nao atendido` ou `nao verificavel`;
- tratar `nao verificavel` como lacuna aberta, nunca como aprovacao.

Matriz minima de cobertura:

- `1.0` -> RF-01
- `2.0` -> RF-01, RF-02, RF-03
- `3.0` -> RF-17, RF-18
- `4.0` -> RF-06, RF-07, RF-08
- `5.0` -> RF-04, RF-05, RF-20, RF-21
- `6.0` -> RF-09, RF-10, RF-11, RF-12
- `7.0` -> RF-13, RF-14, RF-15, RF-16, RF-22
- `8.0` -> CA-01, CA-02, CA-03, CA-04, CA-07, CA-09, CA-10
- `9.0` -> RF-19, RF-23, CA-05, CA-06, CA-08

### Procedimento de execucao

1. Carregar o contexto minimo e determinar o diff real contra `origin/main` ou a base apropriada.
2. Rodar `review` com confronto explicito de todos os RFs, CAs, DoD e regras de negocio acima.
3. Produzir findings com severidade canonica antes de qualquer veredito.
4. Se existir qualquer problema, gerar a lista canonica de bugs e acionar `@.claude/skills/bugfix/`.
5. Rodar nova rodada de `review` apos cada remediacao, revisando pelo menos o delta da correcao e revalidando os criterios afetados.
6. Repetir `review -> bugfix -> review` ate `APPROVED` limpo.
7. So depois disso validar producao por SSH e decidir se esta realmente pronto para `main`.

### Validacoes minimas obrigatorias

Executar e registrar evidencias, no minimo:

- `go build ./...`
- `go vet ./...`
- `go test -race -count=1` no escopo alterado
- `golangci-lint run` no escopo proporcional quando disponivel
- testes de integracao da tarefa 8.0
- gate de carga sintetica da tarefa 9.0
- validacao com `RUN_REAL_LLM=1` e credenciais `OPENROUTER_*` quando o criterio exigir comportamento real do agente/LLM

Se a baseline do repositorio estiver quebrada, separar claramente falha preexistente de regressao introduzida.

### Validacao obrigatoria em producao antes do veredito final

E obrigatorio usar `ssh root@187.77.45.48` antes de concluir `PRONTO PARA MAIN`.

Coletar evidencia objetiva e fechar todas as lacunas em:

- DB/Postgres: schema, indices, rows relevantes, seeds, `agents_write_ledger`, `platform_messages`, `outbox_events`, constraints de origem em `transactions` e `transactions_card_purchases`.
- Logs/Loki: erros, retries, poison events, mensagens vazias, onboarding errors, reprocessos, sinais de storm.
- Tracing/Tempo: cadeia `webhook -> agente -> LLM -> envio`, correlacao por `run_id`/`thread_id`, spans ausentes ou quebrados.
- Metricas/Prometheus-Grafana: `workflow_version_conflict_total`, lag de publicacao, duplicidade, outbound vazio, dead-letter, `OTEL_SERVICE_VERSION`, saturacao de pool.
- Deploy/Swarm: `order: stop-first`, `stop_grace_period`, replicas, comportamento de rolling update, ausencia de storm.

Regras operacionais:

- consultas em producao devem ser read-only, salvo instrucao explicita contraria;
- nao mascarar lacuna de producao com "nao reproduzi agora";
- producao limpa e parte do criterio de encerramento;
- se producao contradisser a spec ou o codigo local, abrir finding e reabrir o ciclo.

### Uso de subagentes

Dispare subagentes especializados quando isso aumentar a qualidade da revisao. Paralelizacao sugerida:

- subagente para `internal/platform/outbox` e migration do claim particionado;
- subagente para `internal/agents` e runtime/tool outcomes;
- subagente para onboarding/workflow/platform messages;
- subagente para observabilidade/deploy/carga sintetica;
- subagente para validacao em producao via SSH;
- subagente para busca global de residuos e regressao cruzada.

Consolide tudo em um unico veredito canonico.

### Formato de saida esperado do agente executor

Retorne, no minimo:

- `verdict`
- `files_reviewed`
- `refs_loaded`
- `findings`
- `residual_risks`
- `validations_run`
- tabela RF-01..RF-23 com status e evidencia
- tabela CA-01..CA-10 com status e evidencia
- tabela por tarefa 1.0..9.0 com DoD e criterios de sucesso
- evidencias de producao consultadas
- estado final: `PRONTO PARA MAIN` ou `NAO PRONTO PARA MAIN`

Mapeamento final:

- qualquer evidencia faltante -> `BLOCKED`
- qualquer finding `critical` ou `high` -> `REJECTED`
- qualquer finding `medium` ou `low` -> continuar ciclo, sem encerrar
- zero findings, zero lacunas, zero ressalvas, zero riscos residuais abertos, tudo comprovado -> `APPROVED`

### Proibicoes

- Nao aprovar por amostragem parcial.
- Nao considerar execution report como prova suficiente.
- Nao tratar `APPROVED_WITH_REMARKS` como aceitavel.
- Nao omitir producao do veredito final.
- Nao chamar algo de falso positivo sem prova material.
- Nao encerrar com risco residual aberto.

---

## 3. Justificativas do enriquecimento

- Explicitei RF-01..RF-23 e CA-01..CA-10 para impedir revisao generica e reduzir falso positivo.
- Amarrei o fluxo as 9 tarefas e aos execution reports para tornar o DoD auditavel item a item.
- Transformei "0 gaps / 0 ressalvas / pronto para main" em condicoes operacionais objetivas.
- Preservei o objetivo original de review rigoroso e de ciclo `review -> bugfix -> review`, sem executar nada nesta etapa.
- Mantive a validacao obrigatoria em producao via SSH, mas como instrucao ao agente executor do prompt, nao como acao deste artefato.
