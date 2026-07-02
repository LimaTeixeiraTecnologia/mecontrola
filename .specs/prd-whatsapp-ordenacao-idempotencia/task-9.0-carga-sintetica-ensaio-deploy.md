# Tarefa 9.0: Gate de carga sintética por fase + ensaio de rolling deploy

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Provar a escala (0 → 10.000) e os SLOs de D-05 por um gate de carga sintética por fase (fronteiras
500 / 2.000 / 10.000) e por um ensaio de rolling deploy sob carga, dimensionando os parâmetros de
vazão/capacidade e validando os traces/versão do caminho crítico (RF-19, RF-23; CA-05, CA-06, CA-08).

<requirements>
- RF-23: gate de carga sintética por fase que prova CA-01 (zero execução concorrente por usuário, FIFO) e CA-08 (lag p95 < 5s, 0 duplicidade, pool não satura) nas fronteiras 500 / 2.000 / 10.000 ANTES de captar usuários reais — produção tem 1 usuário, sem baseline; a prova é sintética.
- RF-19: dimensionar por fase o tamanho de lote do dispatcher, `default_pool_size` do pgbouncer, `DB_MAX_CONNS` e o número de réplicas de worker para que a serialização por usuário não cause contenção de conexões (escalonamento horizontal).
- CA-05: ensaio de rolling deploy (`docker service update` com `order: stop-first` + `stop_grace_period: 30s`) sob carga sintética de conversas → sem respostas duplicadas nem lag de publicação p95 ≥ 5s.
- CA-06: traces cobrem `webhook → agente → LLM → envio`; `workflow_version_conflict_total`/`resumed_on_conflict` presentes e observáveis; `OTEL_SERVICE_VERSION` == tag do binário.
- CA-08: sob carga escalada, lag p95 < 5s e 0 duplicidade mantidos ao adicionar réplicas de worker; pool de conexões não satura.
- Se o `NOT EXISTS` por usuário mostrar contenção, acionar a evolução para partição por hash (ADR-001, fase 2.000–10.000) — registrar como achado, não implementar aqui.
</requirements>

## Subtarefas

- [ ] 9.1 Gerador de conversas sintéticas (testcontainers + carga paramétrica) parametrizado por fase (500 / 2.000 / 10.000).
- [ ] 9.2 Provar CA-01 (FIFO, zero concorrência por usuário) e CA-08 (lag p95 < 5s, 0 duplicidade, pool não satura) em cada fronteira.
- [ ] 9.3 Dimensionar RF-19 (batch do dispatcher, `default_pool_size`, `DB_MAX_CONNS`, réplicas) por fase e registrar os valores.
- [ ] 9.4 Ensaio de rolling deploy sob carga (CA-05): `stop-first` + `stop_grace_period: 30s`, sem duplicatas e lag p95 < 5s.
- [ ] 9.5 Validar CA-06: traces fim-a-fim visíveis no Tempo, `resumed_on_conflict` observável, `OTEL_SERVICE_VERSION` == binário.
- [ ] 9.6 Registrar achados de contenção (gatilho de partição por hash) se houver.

## Detalhes de Implementação

Ver techspec §Abordagem de Testes (§Testes E2E: gate de carga sintética por fase, CA-05/CA-08/RF-23),
ADR-001 §Monitoramento e §Revisão Futura, ADR-004 §Decisão (deploy seguro, métricas). Depende de 7.0
(observabilidade/deploy) e 8.0 (suíte de integração verde).

## Critérios de Sucesso

- CA-08 verde nas 3 fronteiras (lag p95 < 5s, 0 duplicidade, pool não satura).
- CA-05 verde (ensaio de deploy sem duplicatas, lag p95 < 5s).
- CA-06 verde (traces fim-a-fim, versão == binário, `resumed_on_conflict` observável).
- Parâmetros RF-19 dimensionados e documentados por fase.
- SLOs de D-05 verificados (não parametrização adiada).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `otel-grafana-dashboards` — os SLOs (lag p95, duplicidade, outbound vazio) e a validação de traces/versão (CA-06) são verificados via painéis Grafana/otel-lgtm; a skill cobre esses dashboards.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Esta tarefa É o gate de carga/E2E: executar o gerador sintético nas 3 fronteiras e o ensaio de deploy,
coletando lag p95, duplicidade e cobertura de traces.

## Rollback

Gate e ensaio são de validação; nenhum rollback funcional. Falha do gate bloqueia a captação de
usuários reais (é a própria trava de segurança de RF-23). Contenção detectada → aciona ADR-001 (partição
por hash) como iniciativa futura.

## Done-when

- CA-05, CA-06, CA-08 verdes; SLOs de D-05 verificados nas 3 fronteiras.
- Parâmetros RF-19 registrados por fase.
- Achados de contenção (se houver) documentados com gatilho de evolução.

## Arquivos Relevantes
- Gerador de carga sintética (novo, sob `//go:build integration`/e2e)
- `deployment/compose/compose.swarm.yml` (parâmetros de deploy do ensaio)
- `configs/config.go` (OutboxConfig: batch, pool), infra pgbouncer (`default_pool_size`, `DB_MAX_CONNS`)
- Dashboards Grafana (otel-lgtm): lag p95, duplicidade, traces fim-a-fim
