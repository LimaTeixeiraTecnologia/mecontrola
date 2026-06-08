# DOSSIÊ DE DISCOVERY TÉCNICO

## Título
Módulo de orçamentos mensais por categoria production-ready

## Resumo Executivo
Contexto:
O `mecontrola` precisa de um novo bounded context para configurar orçamentos mensais, registrar despesas por API e evento, calcular uso por categorias fixas e preparar alertas. O working tree atual possui `identity`, `billing`, Postgres, outbox, workers e observabilidade, mas ainda não possui `budgets`, `categories` ou `transactions` implementados.

Recomendação:
Criar `internal/budgets` no monólito modular, usando despesas canônicas como fonte de verdade e calculando totais diretamente por competência. Escritas via API são síncronas; eventos internos são idempotentes e ordenados por `occurred_at` mais `event_id`. Alertas são best-effort após commit via outbox/worker.

Status de viabilidade:
Viável com restrições: depende de categorias fixas do sistema, aceita RPO de até 15 minutos, não preserva valores anteriores de despesas editadas/excluídas e exige validar o SLO de leitura calculada com teste de carga.

## Necessidade e Objetivos
Problema atual:
Não existe capacidade para planejar e acompanhar orçamento mensal por categoria. Sem uma fonte canônica de despesas e regras de idempotência, entradas por API e evento podem duplicar ou produzir totais incorretos.

Objetivos de negócio:
- Permitir orçamento único por usuário e competência mensal.
- Exibir valor planejado, gasto, percentual utilizado e total mensal.
- Permitir gastos acima das metas sem bloquear despesas.
- Preparar alertas de 80% e 100% para futura entrega via agente LLM/WhatsApp.
- Permitir recorrência de orçamento por até 12 meses futuros.

Objetivos técnicos:
- Garantir estado financeiro atual correto por transações Postgres e idempotência.
- Responder escritas e consultas críticas com p95 até 300 ms.
- Reutilizar Postgres, outbox, workers e OpenTelemetry existentes.
- Evitar acumulados persistidos; calcular totais a partir das despesas canônicas.
- Operar com telemetria completa e retenção de 24 meses.

## Materiais de Apoio
- `AGENTS.md`, fonte canônica de arquitetura e governança.
- Bundle decisório `docs/discoveries/brainstorms/brainstorm-modulo-de-orcamentos-mensais-por-categoria/`.
- Bundle decisório de categorias em `docs/discoveries/brainstorms/brainstorm-crud-de-categorias-e-subcategorias-de-despesas/`.
- `go.mod`, com Go 1.26.4, Chi, pgx/Postgres e OpenTelemetry.
- `internal/platform/outbox` e `internal/platform/worker`.
- Padrões reais de composição e eventos em `internal/billing` e `internal/identity`.

## Escopo
Inclui:
- Novo bounded context `internal/budgets`.
- Orçamentos mensais em estado rascunho ou ativado, únicos por usuário e competência.
- Orçamento rascunho automático quando uma despesa chega sem orçamento mensal.
- Recorrência de até 12 meses, com atualização apenas de meses futuros não ativados.
- Categorias fixas do sistema, validadas por interface declarada por budgets.
- Distribuição exatamente igual a 100% para ativação.
- Valores em centavos BRL, percentuais em basis points e half-even.
- Despesas canônicas criadas, atualizadas e excluídas por API e evento.
- Tombstones idempotentes após exclusão física.
- Consultas mensais calculadas diretamente das despesas.
- Alertas persistidos após commit, limites de 80%/100% e máximo de 10 por usuário/mês.
- Jobs de expurgo mensal e sinalização de rascunhos não ativados.

Exclui:
- Categorias personalizadas ou removíveis em budgets.
- Histórico dos valores anteriores de despesas editadas/excluídas.
- Acumulados financeiros persistidos ou read-model assíncrono.
- Broker, cache ou serviço dedicado novo.
- Envio real por WhatsApp e implementação de agente LLM.
- Fechamento automático de competências.
- Valores ou percentuais em ponto flutuante.

## Premissas e Restrições
Premissas:
- Até 10 mil usuários ativos, 100 despesas por usuário/mês e pico de 10 escritas/s.
- Categorias aceitas por budgets são fixas, governadas pelo sistema e sempre existentes.
- Produtores internos fornecem `source`, `external_transaction_id`, `event_id` e `occurred_at` válidos.
- Postgres e a plataforma existente possuem capacidade para o volume inicial.

Restrições:
- Fluxo permitido `infrastructure -> application -> domain`.
- Interfaces cross-module são declaradas pelo consumidor; budgets não importa implementação de categories.
- Nenhuma infraestrutura gerenciada nova no MVP.
- Integridade financeira prevalece sobre prazo e alertas.
- Orçamento ativado é imutável, mas despesas de competências passadas permanecem editáveis.
- Retenção de orçamentos, despesas e tombstones limitada a 24 meses.

## Viabilidade Técnica
Status:
Viável com restrições.

Justificativa:
O volume selecionado cabe em Postgres com índices por usuário, competência e categoria. Totais calculados evitam divergência de projeções. A plataforma já possui outbox, workers, shutdown coordenado e observabilidade. A solução exige testes de carga para comprovar p95 de 300 ms e contrato rígido de eventos para preservar idempotência.

Bloqueadores:
- O contrato executável de categorias fixas ainda não existe no working tree.
- O formato final do contrato canônico de eventos precisa ser especificado antes da implementação.
- A infraestrutura atual precisa demonstrar backup/restauração compatíveis com RPO de 15 minutos e RTO de 4 horas.

## Arquitetura Atual
- Monólito modular Go com bounded contexts `internal/identity` e `internal/billing`.
- Postgres acessado via pgx e abstrações existentes de database/uow.
- Outbox Postgres com retries, dispatcher e housekeeping em `internal/platform/outbox`.
- Jobs e consumers coordenados pelo `WorkerManager`.
- Observabilidade OpenTelemetry já disponível.
- Não existem módulos implementados de budgets, categories ou transactions.

## Arquitetura Proposta
Componentes:
- `internal/budgets/domain`: orçamento mensal, alocações, recorrência, despesa canônica, tombstone e invariantes monetárias.
- `internal/budgets/application`: use cases de orçamento, recorrência, mutação de despesas, consultas, expurgo e contratos consumidos.
- `internal/budgets/infrastructure/http/server`: API síncrona.
- `internal/budgets/infrastructure/messaging/database/consumers`: entrada canônica de eventos e avaliação assíncrona de alertas.
- `internal/budgets/infrastructure/messaging/database/producers`: publicação de intenções de alerta via outbox.
- `internal/budgets/infrastructure/jobs/handlers`: expurgo mensal e sinalização de rascunhos.
- `internal/budgets/infrastructure/repositories/postgres`: persistência e consultas agregadas.
- `BudgetsModule`: DI manual, routers, jobs, consumers e adapters reais.

Fluxo de alto nível:
1. API ou consumer valida identidade, categoria fixa, competência e valores.
2. API aplica mutação e confirma somente após commit; consumer serializa eventos por usuário.
3. Postgres aplica unicidade por `(user_id, source, external_transaction_id)` e regras temporais por `occurred_at` + `event_id`.
4. Consulta mensal soma despesas canônicas existentes por competência e categoria.
5. Após commit, evento outbox dispara avaliação best-effort dos limiares.
6. Jobs expurgam dados com mais de 24 meses e sinalizam rascunhos no fim do mês.

Decisão arquitetural:
Usar registro canônico reconciliável sem acumulado persistido dentro do monólito modular. Postgres é a autoridade transacional; alertas e tarefas operacionais são assíncronos e não bloqueiam o núcleo financeiro.

## Dados e Integrações
Domínios de dados:
- `budget_months`: usuário, competência, status, total em centavos, recorrência e timestamps.
- `budget_allocations`: orçamento, categoria fixa, basis points e valor planejado calculado deterministicamente.
- `budget_recurrences`: identidade da série, configuração e horizonte máximo de 12 meses.
- `budget_expenses`: despesa canônica atual, origem, ID externo, categoria, competência, valor em centavos, último `occurred_at` e `event_id`.
- `budget_expense_tombstones`: identidade externa excluída, operação e expiração, sem valores financeiros.
- `budget_alerts`: intenção de alerta, limiar, despesa, competência, status e deduplicação.
- `budget_pending_events`: eventos update/delete aguardando criação por até 24 horas.

Integrações:
- Interface síncrona declarada por budgets para validar categorias fixas.
- API REST interna do módulo budgets.
- Contrato canônico interno de eventos, governado por budgets.
- Outbox e worker existentes para alertas e tarefas operacionais.
- Provider futuro para agente LLM/WhatsApp, fora do MVP.

Consistência requerida:
Forte para orçamento, despesa, tombstone e consultas após commit. Eventual e best-effort para alertas, sinais operacionais e processamento de eventos pendentes.

## Volumetria e Capacidade
Volume atual:
Capacidade nova sem tráfego produtivo; dimensionamento inicial para até 10 mil usuários ativos e até 1 milhão de despesas por mês.

Pico esperado:
10 escritas por segundo; consultas mensais limitadas a aproximadamente 100 despesas por usuário/competência.

Taxa de crescimento:
Hipótese conservadora de até 100 despesas adicionais por usuário ativo por mês, limitada pela retenção de 24 meses.

SLO alvo:
99,9% de disponibilidade mensal e p95 até 300 ms para escritas confirmadas e consultas mensais.

Gargalos conhecidos:
- Soma por competência/categoria sem acumulado persistido.
- Serialização de eventos por usuário.
- Índices e expurgo de até 24 meses de despesas.
- Avaliação repetida de alertas acima do limiar.

## Segurança e Compliance
Classificação dos dados:
Dados financeiros pessoais e metadados de comportamento, classificados como sensíveis ao negócio e sujeitos a proteção reforçada.

Autenticação e autorização:
Toda API deve autenticar o usuário e toda operação deve filtrar/autorizar por `user_id`. Consumers validam origem e contrato antes de aplicar eventos.

Gestão de segredos:
Budgets não cria segredos próprios no MVP. Credenciais de banco, observabilidade e futuros providers seguem a configuração segura existente, sem valores em código ou logs.

Criptografia:
Usar criptografia em trânsito e em repouso oferecida pela plataforma/Postgres. Payloads de eventos e alertas não devem expor dados além do necessário.

Auditoria e rastreabilidade:
Logs/traces registram operação, origem, IDs, competência e resultado, sem valores anteriores. Tombstones preservam idempotência, não auditoria financeira completa.

Compliance/LGPD:
Aplicar minimização, isolamento por usuário, mascaramento em logs e expurgo após 24 meses. A ausência de histórico reduz capacidade forense e é risco aceito.

## Confiabilidade e Resiliência
SLA/SLO:
SLO de 99,9% e p95 de 300 ms para o núcleo; alertas não participam do SLO financeiro.

RTO/RPO:
RTO até 4 horas e RPO até 15 minutos. A perda potencial de operações confirmadas em desastre é risco residual explícito.

Estratégia de retry/idempotência:
Unicidade por usuário/origem/ID externo; tombstone impede replay após exclusão. Eventos update/delete sem create aguardam 24 horas. Regressões por `occurred_at` são ignoradas; empates usam `event_id`.

Degradação/contingência:
Falha de alertas, jobs ou provider futuro não bloqueia API nem mutações financeiras. Consultas usam dados transacionais confirmados. Falha do Postgres bloqueia novas confirmações.

Rollback:
Migrations aditivas e rollback por deploy/configuração, desligando routers/consumers/jobs sem apagar dados. Não usar rollback destrutivo de schema.

## Observabilidade e Operação
Métricas:
- `budgets_operation_total{operation,source,outcome}`.
- `budgets_operation_latency_seconds{operation}`.
- `budgets_duplicate_total{source}` e `budgets_temporal_regression_total{source}`.
- `budgets_pending_events_total{status}` e idade máxima.
- `budgets_alerts_total{threshold,status}` e `budgets_alerts_discarded_total`.
- `budgets_draft_unactivated_total` e `budgets_expurge_total{entity,outcome}`.

Logs:
- Logs estruturados com `user_id` mascarado/adequado, `budget_id`, `expense_id`, `source`, `external_transaction_id`, `event_id`, competência e outcome.
- Nunca registrar payload financeiro completo ou valores anteriores.

Traces:
- Spans nos use cases, transações Postgres, consultas agregadas, publicação outbox, consumers e jobs.
- Correlação entre API/evento, despesa canônica e alerta.

Alertas:
- SLO/latência do núcleo, taxa de erro, duplicidades anormais, eventos pendentes acima de 24 horas, regressões temporais, falha de expurgo e descarte de alertas.

Dashboards/Runbooks:
- Dashboard do núcleo financeiro, dashboard de eventos e alertas, runbook de evento fora de ordem, runbook de restauração e runbook de rollback geral.

## Performance e Escalabilidade
Latência alvo:
p95 até 300 ms para criar/editar/excluir e consultar resumo mensal.

Estratégia de escala:
Índices compostos por usuário/competência/categoria e usuário/origem/ID externo; paginação; queries agregadas Postgres; expurgo mensal em lotes. Escalar verticalmente Postgres/workers antes de introduzir nova infraestrutura.

Limites conhecidos:
- Até 10 mil usuários ativos e 10 escritas/s no MVP.
- Até 100 despesas por usuário/mês como hipótese.
- Retenção de 24 meses.
- Máximo de 10 alertas persistidos por usuário/mês.
- Recorrência máxima de 12 meses futuros.

Teste de carga:
Validar escrita concorrente por usuário, consulta mensal com 100 despesas, criação recorrente de 12 meses, consumer fora de ordem e expurgo em lotes. Falhar o gate de release se p95 ultrapassar 300 ms no perfil definido.

## Custos e Orçamento
Orçamento estimado:
Custo incremental de infraestrutura baixo, usando Postgres, outbox, workers e observabilidade existentes; custo de engenharia relativo alto devido ao núcleo completo e aos testes de integridade.

Drivers de custo:
- Armazenamento de despesas, tombstones, eventos pendentes e telemetria por 24 meses.
- Queries agregadas mensais e índices Postgres.
- Worker/outbox para alertas e eventos.
- Testes de concorrência, idempotência, carga, backup e restauração.

Guardrails de custo:
- Nenhuma nova infraestrutura gerenciada.
- Retenção máxima de 24 meses e expurgo mensal em lotes.
- Máximo de 10 alertas por usuário/mês.
- Sem cache, broker externo, read-model ou serviço dedicado no MVP.

Plano de otimização:
- Medir queries antes de criar projeções.
- Ajustar índices a partir de `EXPLAIN ANALYZE`.
- Monitorar tamanho de tabelas e duração do expurgo.
- Reavaliar acumulados/read-model apenas se o p95 não for atingido.

## Riscos e Mitigações
- Risco: perda de operação confirmada dentro do RPO de 15 minutos.
  Impacto: total financeiro incorreto após desastre.
  Mitigação: validar backups/restauração, monitorar lag e revisar RPO antes de ampliar criticidade.
  Dono: plataforma/operação.
- Risco: ausência de histórico de edições e exclusões.
  Impacto: investigação retroativa limitada.
  Mitigação: logs operacionais, tombstones e decisão explícita de escopo.
  Dono: produto/budgets.
- Risco: leitura calculada ultrapassar p95 de 300 ms.
  Impacto: quebra de SLO.
  Mitigação: índices, teste de carga, retenção e gate para futura projeção.
  Dono: equipe budgets.
- Risco: produtor emitir `occurred_at` incorreto.
  Impacto: evento legítimo ignorado ou estado incorreto.
  Mitigação: contrato governado por budgets, validação, métricas e correção por novo evento.
  Dono: produtores internos/budgets.
- Risco: alertas descartados pelo limite de 10.
  Impacto: usuário não recebe todos os sinais.
  Mitigação: métrica/log e alertas fora do SLO financeiro.
  Dono: produto/operação.
- Risco: liberação geral sem canary.
  Impacto: raio de falha amplo.
  Mitigação: testes completos, migrations aditivas, smoke test e rollback por deploy/configuração.
  Dono: engenharia/operação.

## Trade-offs e Decisões
Alternativas consideradas:
- Acumulado direto persistido.
- Registro canônico com total calculado.
- Ledger imutável com projeções.
- Event sourcing completo.

Decisão tomada:
Registro canônico de despesas com total calculado por competência no Postgres, API síncrona e eventos internos idempotentes.

Trade-off aceito:
Menor complexidade operacional e ausência de divergência de projeção em troca de queries agregadas; ausência de histórico em troca de edição/exclusão física; alertas best-effort em troca de preservar o núcleo financeiro.

## Plano de Entrega e Rollout
Fases:
- Fundação: domínio, schema, categorias fixas, orçamento mensal, recorrência e rascunho automático.
- Núcleo financeiro: despesas canônicas, API, eventos, tombstones e ordenação temporal.
- Leitura e alertas: resumo mensal calculado, limiares, outbox e consumers.
- Operação: expurgo, sinais de rascunho, telemetria, carga, backup/restauração e runbooks.

Migração:
Criar tabelas e índices por migrations aditivas. Não migrar dados legados inexistentes. Expurgo inicia somente após validação em ambiente produtivo.

Feature flags/canary:
Não haverá feature flag nem canary por decisão explícita. A liberação será geral e controlada após testes, migrations, smoke test e validação operacional.

Critério de rollback:
Taxa de erro crítica, total incorreto, falha de idempotência, regressão temporal indevida ou p95 acima do limite sustentado. Rollback desliga entradas/workers por deploy/configuração e preserva dados.

## Decomposição em Épicos e Features
### Epic 01 - Fundação de orçamentos mensais
Objetivo: estabelecer o bounded context, modelo mensal e configuração confiável.
Feature 01: Criar e consultar orçamento mensal rascunho/ativado.
Feature 02: Configurar alocações fixas com basis points, half-even e ativação em 100%.
Feature 03: Criar e atualizar recorrência por até 12 meses.
Feature 04: Criar e completar rascunho automático.

### Epic 02 - Despesas canônicas e entradas idempotentes
Objetivo: garantir mutações corretas por API e evento.
Feature 01: Criar, editar e excluir despesas via API síncrona.
Feature 02: Consumir contrato canônico de create/update/delete.
Feature 03: Aplicar idempotência, tombstones e ordenação temporal.
Feature 04: Operar eventos pendentes por 24 horas.

### Epic 03 - Consultas, metas e alertas
Objetivo: oferecer leitura mensal correta e sinais best-effort.
Feature 01: Consultar resumo mensal calculado por categoria.
Feature 02: Calcular planejado, gasto e percentual utilizado.
Feature 03: Avaliar alertas de 80%/100% após commit.
Feature 04: Aplicar deduplicação e limite de 10 alertas.

### Epic 04 - Operação, retenção e hardening
Objetivo: tornar budgets operável sob os SLOs definidos.
Feature 01: Expurgar dados e tombstones após 24 meses.
Feature 02: Sinalizar rascunhos não ativados no fim do mês.
Feature 03: Implementar telemetria, dashboards e runbooks.
Feature 04: Validar carga, concorrência, backup, restauração e rollback.

## Itens em Aberto
- Especificar o catálogo exato e os IDs estáveis das categorias fixas do sistema.
- Fechar o schema e payload versionado do contrato canônico de eventos.
- Definir a ordem determinística usada para distribuir centavos restantes entre categorias.
- Definir autenticação/autorização concreta dos produtores internos de eventos.
- Confirmar capacidade real de backup/restauração para RPO de 15 minutos e RTO de 4 horas.
- Definir contrato futuro do provider de agente LLM/WhatsApp sem incluí-lo no MVP.
