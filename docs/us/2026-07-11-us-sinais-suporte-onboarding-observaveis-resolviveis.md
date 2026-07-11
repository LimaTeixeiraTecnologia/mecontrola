# US-001: Sinais de suporte do onboarding observáveis e resolvíveis (recuperar clientes pagantes encalhados)

## Declaração
Como operador de suporte do MeControla (o time que opera o produto e atende clientes), quero enxergar a fila de sinais de suporte abertos por tipo e registrar a resolução de cada caso, para recuperar clientes pagantes que ficaram encalhados fora do fluxo automático de ativação e fechar cada caso sem perder receita nem cliente.

## Contexto
- Problema: o onboarding grava três tipos de `support_signal` que representam clientes de alto valor fora do fluxo feliz — `paid_without_token` (pagou sem token de funil), `orphan_expired_subscription` (token PAID expirou sem ativação) e `token_reuse_attempt` (reuso de token consumido). Hoje esses sinais são **write-only**: são inseridos no banco, mas nenhum código os lê, conta ou resolve. O único rastro operacional é uma linha de log `WARN` e contadores acumulados que nunca decrementam. O operador não tem como saber quantos casos estão abertos, quais são, nem marcar um caso como resolvido depois de atender o cliente.
- Resultado esperado: para cada tipo de sinal existe uma métrica gauge de "sinais abertos" (que dispara/decrementa conforme casos surgem e são resolvidos), habilitando alerta; e o operador consegue, por procedimento operacional (job/SQL, sem nova API HTTP), carimbar `resolved_at`/`resolved_by`/`notes` de um sinal, fazendo o gauge daquele tipo cair. Zero cliente pagante fica silenciosamente perdido.
- Fonte: análise da base de código do módulo `internal/onboarding` (solicitação do usuário: identificar gap/lacuna e produzir UMA US), mais decisões do usuário — superfície "Gauge + resolução via job/SQL" e escopo "Visibilidade + resolução manual".

## Regras de Negócio
- RN-01: Um sinal é **aberto** enquanto `resolved_at IS NULL` e **resolvido** quando `resolved_at` está preenchido; a entidade já modela isso via `IsResolved()` (base de código: `internal/onboarding/domain/entities/support_signal.go:63`).
- RN-02: Os tipos de sinal são um conjunto fechado de exatamente três valores — `orphan_expired_subscription`, `paid_without_token`, `token_reuse_attempt` — validados por `SupportSignalKind` e pela `CHECK` da tabela; a leitura e o gauge devem cobrir os três e nenhum valor livre.
- RN-03: O gauge de sinais abertos consulta **exclusivamente** registros com `resolved_at IS NULL`, agrupando por `kind`, aproveitando o índice parcial já existente `support_signals_kind_open_idx` (`ON (kind, occurred_at) WHERE resolved_at IS NULL`); não pode varrer a tabela inteira.
- RN-04: Registrar resolução carimba `resolved_at`, `resolved_by` e `notes`; a operação é idempotente — resolver um sinal que já está resolvido não altera `resolved_at` já gravado nem produz efeito duplicado.
- RN-05: Resolver um sinal é ato exclusivamente de fechamento de caso (visibilidade + resolução manual): **não** reativa, reprocessa ou vincula o cliente automaticamente. Recuperação automática do cliente está fora de escopo.
- RN-06: A superfície de resolução é operacional (job de manutenção e/ou SQL documentado em runbook), sem introduzir endpoint HTTP público ou admin novo; o `PublicRouter` do onboarding permanece inalterado.
- RN-07: A métrica de sinais abertos usa cardinalidade controlada — único rótulo permitido é `kind` (conjunto fechado de três valores); proibido rótulo de alta cardinalidade como `id`, `external_sale_id`, `customer_mobile` ou `token_hash_prefix`, herdando R-TXN-004.
- RN-08: A leitura de sinais abertos expõe apenas dados já mascarados na escrita (mobile/e-mail já são gravados mascarados nos payloads); nenhuma etapa desta história pode re-expor PII em claro.

## Critérios de Aceite
```gherkin
Cenário: Sinal aberto aparece no gauge e é fechado após resolução
  Dado um cliente que pagou sem token de funil e gerou um support_signal do tipo "paid_without_token" com resolved_at nulo
  Quando o gauge de sinais abertos é coletado
  Então onboarding_support_signals_open{kind="paid_without_token"} reflete esse sinal na contagem de abertos
  E quando o operador registra a resolução do sinal com seu identificador e uma nota
  Então resolved_at, resolved_by e notes ficam preenchidos
  E na próxima coleta o gauge{kind="paid_without_token"} decrementa em uma unidade

Cenário: Contagem por tipo é independente entre os três kinds
  Dado sinais abertos dos tipos "orphan_expired_subscription", "paid_without_token" e "token_reuse_attempt"
  Quando o gauge de sinais abertos é coletado
  Então cada série onboarding_support_signals_open{kind=...} conta apenas os sinais abertos daquele tipo
  E resolver um sinal de um tipo não altera a contagem dos outros tipos

Cenário: Resolução é idempotente
  Dado um sinal já resolvido com resolved_at definido em um instante T
  Quando o operador executa novamente a resolução do mesmo sinal
  Então resolved_at permanece igual a T
  E o gauge daquele tipo não muda

Cenário: Consulta de abertos ignora resolvidos e id inexistente não altera estado
  Dado um sinal já resolvido e uma tentativa de resolver um id que não existe
  Quando a consulta de sinais abertos é executada
  Então o sinal resolvido não reaparece na lista/contagem de abertos
  E a tentativa de resolver o id inexistente não altera nenhuma linha e é reportada explicitamente (erro ou no-op registrado), sem falha silenciosa
```

## Dados e Permissões
- Dados obrigatórios para resolver um sinal: `id` do sinal, `resolved_by` (identificador do operador), `notes` (motivo/ação tomada); o sistema carimba `resolved_at` com o instante UTC da resolução.
- Dados obrigatórios para o gauge: contagem de sinais com `resolved_at IS NULL` agrupada por `kind`.
- Perfis/permissões: operação restrita ao operador/time que administra o produto, executada por caminho operacional (job de manutenção ou SQL em runbook), fora de qualquer superfície pública; não há tabela de papéis de suporte no produto hoje, então a autorização é a de acesso operacional ao ambiente (mesma que roda jobs/migrations).

## Dependências
- Tabela `mecontrola.support_signals` com colunas `resolved_at`, `resolved_by`, `notes` e índice parcial `support_signals_kind_open_idx` — já existem (`migrations/000001_initial_schema.up.sql:366-382`); nenhuma migração de schema nova é necessária para os campos de resolução.
- Registro de métrica no módulo — há precedente direto de gauge alimentado por contagem de repositório (`registerMetrics` usa `CountPaidUnconsumed`), o ponto de extensão para o novo gauge já está estabelecido (`internal/onboarding/module.go:357-375`).
- Produção contínua de sinais pelos três produtores existentes, que já funcionam (`handle_paid_without_token.go:58`, `expire_tokens.go:102`, `consume_magic_token.go:183`) — esta história é puramente do lado da leitura/resolução, não altera a escrita.

## Fora de Escopo
- Recuperação/reativação automática do cliente `paid_without_token` (bind por mobile quando ele mandar mensagem) — decisão explícita do usuário por "resolução manual".
- Qualquer endpoint HTTP público ou API admin autenticada nova para listar/resolver sinais — decisão explícita por "job/SQL".
- Correções de outros gaps do módulo identificados na análise, que são histórias distintas: swallow silencioso de erro no beacon de jornada (`record_journey_beacon_handler.go` e `record_journey_timestamp.go` retornando `nil` em erro de validação), ausência de spans internos em `BindAndConsume` e cap de tentativas de outreach.
- Dedup por `(subscription_id, token_id)` do evento `SubscriptionBound`.
- Painel Grafana e regra de alerta concretos — esta história entrega o gauge que os habilita, mas a criação do dashboard/alerta é trabalho de observabilidade separado.

## Evidências
- Entrada: solicitação para analisar `internal/onboarding`, identificar gap/lacuna e criar UMA US com as skills obrigatórias; decisões do usuário sobre superfície (gauge + resolução via job/SQL) e escopo (visibilidade + resolução manual).
- Base de código:
  - Sinais são write-only: interface só tem `Insert` — `internal/onboarding/application/interfaces/support_signal_repository.go:11-13`; adapter Postgres só tem `Insert` — `internal/onboarding/infrastructure/repositories/postgres/support_signal_repository.go:24-44`.
  - Três produtores confirmados: `internal/onboarding/application/usecases/handle_paid_without_token.go:58` (`paid_without_token`), `internal/onboarding/application/usecases/expire_tokens.go:102` (`orphan_expired_subscription`), `internal/onboarding/application/usecases/consume_magic_token.go:183` (`token_reuse_attempt`).
  - Colunas de resolução e acessores sem uso: `resolved_at/resolved_by/notes` e `IsResolved()` — `internal/onboarding/domain/entities/support_signal.go:36-63`.
  - Tipos fechados: `internal/onboarding/domain/valueobjects/support_signal_kind.go` (três kinds).
  - Schema + índice parcial de "abertos" nunca consultado: `migrations/000001_initial_schema.up.sql:366-382` (colunas `resolved_at/resolved_by/notes`, `CHECK` dos três kinds e `support_signals_kind_open_idx ... WHERE resolved_at IS NULL`).
  - Precedente de gauge alimentado por contagem de repositório: `internal/onboarding/module.go:357-375` (`registerMetrics` → `Gauge` → `repo.CountPaidUnconsumed`); a assinatura `CountPaidUnconsumed` está em `internal/onboarding/application/interfaces/magic_token_repository.go:27` e `internal/onboarding/infrastructure/repositories/postgres/magic_token_repository.go:330`.
- Inferências:
  - Persona "operador de suporte" = o time/founder que opera o MeControla — inferido do contexto do produto (bot financeiro operado por time enxuto); não há tabela de papéis de suporte na base.
  - A intenção original de uma trilha de resolução é inferida do conjunto colunas `resolved_*` + índice parcial de abertos; o design deixa a intenção clara, mas nenhum código consumidor foi construído.
- Não evidenciado (busca executada e sem achado):
  - Nenhum leitor/consumer/job/endpoint dos `support_signals`: busca por `select|list|find|resolve|count|pending|unresolved` sobre `support_signal`/`SupportSignal` retornou apenas os acessores da entidade; e não há `CountOpen`/`MarkResolved`/`ResolveSignal`/`FindOpenSignals` em toda a árvore `internal`.
  - Nenhuma superfície HTTP admin ou autenticada no repositório (busca por `AdminRouter/InternalRouter/BackofficeRouter/PrivateRouter` sem resultado de código real).
  - Nenhum alerta/runbook/dashboard referenciando `support_signals`, `paid_without_token` ou `orphan_expired` (apenas diagramas PlantUML de fluxo em `docs/diagrams/`), busca em `docs/` executada.
  - Nenhum PRD de onboarding em `.specs/` (diretório não existe).

## Notas de Validação
- Os três cenários mínimos exigidos estão cobertos: fluxo feliz (sinal aberto → gauge → resolução → decremento), fluxo alternativo (independência por kind e idempotência de resolução) e fluxo de erro/bloqueio (id inexistente não altera estado; resolvido não reaparece nos abertos).
- História de habilitação com resultado observável (gauge decrementável + resolução persistida), não uma tarefa puramente técnica — o valor é recuperar clientes pagantes e fechar casos.
- Sem marcadores pendentes; nome de métrica `onboarding_support_signals_open` é o entregável proposto (não afirmado como já existente) e segue o padrão de nomenclatura das métricas atuais do módulo.
- Cardinalidade da métrica restrita ao rótulo fechado `kind`, em conformidade com R-TXN-004.
