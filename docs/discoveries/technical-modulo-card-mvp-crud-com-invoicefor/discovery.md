# DOSSIÊ DE DISCOVERY TÉCNICO

## Título
Módulo `internal/card` MVP CRUD + função pura `InvoiceFor` para `mecontrola` (Go)

## Resumo Executivo
Contexto:
O `mecontrola` precisa de um bounded context `internal/card` antes que o módulo de transações seja construído. O brainstorming decisório (`discoveries/brainstorm-modulo-crud-de-cartoes-de-credito-com-calculo-de-fatura/`) consolidou a direção: bounded context próprio, CRUD HTTP, função pura `InvoiceFor` em domínio, persistência Postgres com soft-delete, idempotência via header, observabilidade via logs+OTel, sem armazenamento de PAN/CVV, e robustez extrema por testes table-driven + property-based. Este discovery materializa esquema PostgreSQL, contrato HTTP, ADRs, plano de testes, observabilidade, segurança, custos e rollout.

Recomendação:
Avançar para `create-prd` consumindo este bundle. A arquitetura é viável com restrições conhecidas; nenhum bloqueador material persiste.

Status de viabilidade:
viável

## Necessidade e Objetivos
Problema atual:
O `mecontrola` não tem agregado de cartão, nem formaliza a regra "em qual fatura uma compra cai". Sem isso, o módulo de transações fica bloqueado e qualquer hack temporário em categoria/conta gera dívida que exige recalculo financeiro retroativo.

Objetivos de negócio:
- Permitir que o usuário cadastre seus cartões com nome, apelido, dia de fechamento e dia de vencimento.
- Garantir que uma compra futura seja atribuída à fatura correta de forma determinística e auditável.
- Preservar histórico financeiro: alterar o ciclo de um cartão NÃO migra transações antigas.

Objetivos técnicos:
- Entregar bounded context `internal/card` aderente ao Padrão Obrigatório de Módulo do `AGENTS.md`.
- Expor função pura `InvoiceFor(purchase, closingDay, dueDay, tz) Invoice` como porta interna Go e endpoint HTTP `GET /api/v1/cards/:id/invoices?for=<date>`.
- Atingir SLO 99.5% disponibilidade, p99 CRUD < 300ms, p99 InvoiceFor < 10ms.
- Cobertura por 50+ fixtures table-driven + property-based (`testing/quick`) cobrindo fev/29, dia 31, virada de ano, `due == closing`, `due > closing`, `due < closing`.
- Aderência total a R0–R7 do `AGENTS.md` (sem `init()`, sem `panic` em produção, `context.Context` em IO, `errors.Join`/`fmt.Errorf("ctx: %w", err)`, sem `clock.Clock`).

## Materiais de Apoio
- `discoveries/brainstorm-modulo-crud-de-cartoes-de-credito-com-calculo-de-fatura/decision-brief.md` — direção arquitetural fixada.
- `discoveries/brainstorm-modulo-crud-de-cartoes-de-credito-com-calculo-de-fatura/option-scorecard.md` — comparativo de 4 alternativas.
- `discoveries/brainstorm-modulo-crud-de-cartoes-de-credito-com-calculo-de-fatura/assumptions.md` — hipóteses confirmadas/pendentes.
- `AGENTS.md` — Padrão Obrigatório de Módulo, R0–R7, regras de handlers/consumers/jobs/producers.
- `internal/identity/` — referência de módulo (UUID v4, router, handler, repository pgx, errors).
- `internal/billing/` — referência com middleware customizado (`infrastructure/http/server/middleware/`).
- `internal/platform/observability/` — `Tracer`, `Logger`, helpers.
- `internal/platform/outbox/` — convenções de IDs e instance_id.
- Pesquisa oficial 2026 das bandeiras BR (Visa, Mastercard, Elo, Hipercard, Amex, JCB, Diners, Discover, UnionPay, Aura, Cabal, Banescard, Sorocred, Credz), BACEN Res. 4.658/2018, Resolução BCB 468/2025, IN BCB 621/2025, LGPD, PCI-DSS 4.0 — consolidada no brainstorm.
- Dependências do `go.mod`: `chi/v5`, `pgx/v5`, `google/uuid`, `JailtonJunior94/devkit-go v0.5.0`, `testcontainers-go`, `mockery v2`, `viper`, `cobra`.

## Escopo
Inclui:
- Bounded context `internal/card` com layout: `application/{usecases,dtos/input,dtos/output,interfaces,errors.go}`, `domain/{entities,valueobjects,services,interfaces}`, `infrastructure/{http/server/{handlers,middleware,router.go},repositories/postgres}`, `module.go`.
- Agregado `Card` com `id (UUID v4)`, `user_id`, `name`, `nickname`, `closing_day (1–31)`, `due_day (1–31)`, `created_at`, `updated_at`, `deleted_at`.
- Value objects `Nickname`, `CardName`, `BillingCycle{ClosingDay, DueDay}`.
- Domain service `BillingCycle.InvoiceFor(purchase time.Time, tz *time.Location) Invoice` em `domain/services/billing_cycle.go`.
- Repositório Postgres `cardRepository` em `infrastructure/repositories/postgres/`.
- Endpoints HTTP: `POST /api/v1/cards`, `GET /api/v1/cards`, `GET /api/v1/cards/{id}`, `PUT /api/v1/cards/{id}`, `DELETE /api/v1/cards/{id}`, `GET /api/v1/cards/{id}/invoices?for=<date>`.
- Middleware `RequireUser` (header `X-User-ID` UUID v4) em `infrastructure/http/server/middleware/`.
- Middleware genérico de idempotência em `internal/platform/idempotency/` (novo pacote) consumido pelo `card`.
- Migrations Postgres: `0010_create_platform_idempotency_keys.{up,down}.sql`, `0011_create_card_cards.{up,down}.sql`.
- Wiring em `cmd/server/server.go` registrando `cardModule.CardRouter`.
- Observabilidade: spans OTel `card.handler.<op>`, `card.usecase.<op>`, `card.repository.<op>`, `card.domain.invoice_for`; logs estruturados com `trace_id`, `user_id`, `card_id`, sem `name`/`nickname`.
- Plano de testes: unitários (domain + usecase com mockery), property-based (`testing/quick` no `InvoiceFor`), integração Postgres (testcontainers), contrato HTTP (golden files `testdata/`).
- Documento OpenAPI 3.1 em `internal/card/infrastructure/http/server/openapi.yaml`.
- ADRs em `docs/adrs/`: ADR-001 (BillingCycle domain service), ADR-002 (platform/idempotency), ADR-003 (RequireUser via X-User-ID), ADR-004 (algoritmo clamp + auto-detecção).

Exclui:
- Limite de crédito, saldo disponível.
- Bandeira (`brand`), últimos 4 dígitos (`last4`).
- Multi-titular / cartão adicional.
- Integração Open Finance / Belvo / Pluggy.
- Emissor (banco).
- Audit log de domínio (`card_audit_log`).
- Versionamento histórico de ciclos por cartão (`card_billing_cycles`).
- Métricas Prometheus dedicadas (logs + OTel suficientes no MVP).
- Recálculo retroativo automático de faturas.
- JWT/OIDC (header `X-User-ID` é transitório; fase 2 substituirá).
- Migração de `internal/billing` ou `internal/identity` para o novo pacote de idempotência (fica para refactor pós-MVP).

## Premissas e Restrições
Premissas:
- O ciclo de fatura é definido pelo emissor (banco), não pela bandeira (confirmado por Visa Core Rules, Mastercard Rules, BACEN Res. 4.658, BCB 468/2025).
- `mecontrola` é aplicação não-PCI e nunca armazenará PAN, CVV/CVC, trilha magnética ou PIN.
- A regra do usuário é monotônica: compra em data `D` cai no próximo `due_day` cujo `closing_date` correspondente seja `>= D`.
- Clamp `min(day, daysInMonth(year, month))` resolve fev/29 e meses de 30 dias sem spillover.
- Brasil aboliu horário de verão em 2019; a zona `America/Sao_Paulo` (-03 fixo) é estável para datas futuras, mas testes históricos incluem 2018 com DST.
- Usuário será identificado por header `X-User-ID` (UUID v4) até o módulo de autenticação ser construído.
- Volumetria projetada: 100k usuários × 3 cartões = 300k registros; 10k criações/dia.

Restrições:
- Stack Go com versão fixada em `go.mod` (deve ser respeitada; usar Generics R7 apenas se já permitidos).
- Arquitetura monolito modular: `domain` puro, `application` sem IO, `infrastructure` com IO concreto.
- Padrão Obrigatório de Módulo de `internal/identity`/`internal/billing` é mandatório.
- Migrations em `/migrations/NNNNN_*.{up,down}.sql` via `devkit-go/pkg/database/migration`.
- LGPD: dados pessoais (nome, apelido) com base legal "execução de contrato + legítimo interesse"; minimização aplicada.
- PCI-DSS 4.0: persistência permitida limitada a bandeira (se entrasse) + últimos 4 dígitos (se entrasse). MVP não persiste nem brand nem last4.
- SLO 99.5%, p99 CRUD < 300ms, p99 InvoiceFor < 10ms.

## Viabilidade Técnica
Status:
viável

Justificativa:
- A arquitetura proposta é isomórfica a `internal/identity` (módulo de referência); risco de aderência ao padrão é baixo.
- `InvoiceFor` é problema fechado de calendário gregoriano com clamp; complexidade ciclomática estimada < 8.
- Stack já em uso: `chi`, `pgx`, `google/uuid`, `testify/mock` + mockery, testcontainers para integração, `devkit-go/pkg/database/migration`.
- Zero nova dependência externa (pacote de idempotência usa `pgx` e `chi` existentes; `testing/quick` é stdlib).
- Sem requisito de auth pronto: o header transitório `X-User-ID` é mitigação técnica de prazo, documentada em ADR-003.

Bloqueadores:
- Nenhum bloqueador material identificado.

## Arquitetura Atual
- Monolito modular Go com `internal/identity`, `internal/billing`, `internal/platform`.
- Sem módulo de cartões.
- Sem módulo de transações (será o consumidor primário deste módulo no futuro).
- Sem middleware de autenticação genérico; sem middleware de idempotência genérico.
- `internal/billing` implementa idempotência local via tabelas específicas (`processed_events`, `kiwify_events`).
- Migrations em `/migrations/`, atual última 0009 (`create_identity_entitlements`).
- HTTP server centralizado em `cmd/server/server.go` que recebe routers via `srv.RegisterRouters(...)` (chi).

## Arquitetura Proposta
Componentes:
- `internal/card/domain/entities/card.go` — agregado `Card` com construtor `NewCard` e `Hydrate` (mesmo padrão de `internal/identity/domain/entities/user.go`).
- `internal/card/domain/valueobjects/nickname.go`, `card_name.go`, `billing_cycle.go` — VOs com validação no construtor; `BillingCycle.WithClosingDay`, `WithDueDay`.
- `internal/card/domain/services/billing_cycle.go` — função pura `InvoiceFor(purchase time.Time, cycle BillingCycle, tz *time.Location) Invoice` retornando `Invoice{ClosingDate, DueDate}`.
- `internal/card/domain/interfaces/repository.go` — interfaces consumidas pelos use cases (declaradas no consumidor; R6).
- `internal/card/application/usecases/create_card.go`, `list_cards.go`, `get_card.go`, `update_card.go`, `delete_card.go`, `invoice_for.go`.
- `internal/card/application/dtos/{input,output}/` — DTOs por use case.
- `internal/card/application/interfaces/repository_factory.go`, `card_repository.go`, `idempotency_repository.go` (consumida do platform).
- `internal/card/application/errors.go` — sentinels: `ErrCardNotFound`, `ErrNicknameConflict`, `ErrInvalidClosingDay`, `ErrInvalidDueDay`, `ErrInvalidName`, `ErrInvalidNickname`, `ErrInvalidPurchaseDate`.
- `internal/card/infrastructure/http/server/router.go` — `CardRouter.Register(r chi.Router)` montando `/api/v1/cards`.
- `internal/card/infrastructure/http/server/handlers/{create,list,get,update,delete,invoice_for}_handler.go` — handlers finos (validação de input, chamada de use case, mapeamento de erro→HTTP).
- `internal/card/infrastructure/http/server/middleware/require_user.go` — extrai `X-User-ID`, valida UUID v4, injeta `ctx`.
- `internal/card/infrastructure/repositories/postgres/card_repository.go` — pgx puro, `database.DBTX` injetado.
- `internal/card/infrastructure/repositories/postgres/repository_factory.go` — `interfaces.RepositoryFactory` retornando `CardRepository`.
- `internal/card/module.go` — `NewCardModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) CardModule` retornando struct com `CardRouter`, `RepositoryFactory`, use cases públicos consumidos por outros módulos (porta `CardLookup`).
- `internal/platform/idempotency/middleware.go`, `storage.go`, `storage_postgres.go` — pacote novo.
- Migrations: `/migrations/0010_create_platform_idempotency_keys.{up,down}.sql`, `/migrations/0011_create_card_cards.{up,down}.sql`.
- `cmd/server/server.go` registra `cardModule.CardRouter`.

Fluxo de alto nível:
1. Cliente envia `POST /api/v1/cards` com header `X-User-ID` e `Idempotency-Key`.
2. `RequireUser` valida UUID e injeta `user_id` no ctx; idempotency middleware verifica `idempotency_keys`.
3. Handler decodifica request → input DTO → `CreateCard.Execute(ctx, input)`.
4. Use case valida VOs (`Nickname`, `CardName`, `BillingCycle`), instancia `Card`, chama `repository.Insert(ctx, card)`.
5. Repositório persiste em `cards`, retorna `Card`; idempotency middleware grava resposta com `expires_at = now + 24h`.
6. Handler serializa resposta e responde `201 Created` com `Location: /api/v1/cards/{id}`.
7. Para `GET /api/v1/cards/{id}/invoices?for=<date>`: handler decodifica query `for` (ISO-8601 date), recupera `Card` (lookup leve), chama `BillingCycle.InvoiceFor(purchase, cycle, tzBR)`, responde com `{closing_date, due_date}`.

Decisão arquitetural:
Função pura `InvoiceFor` em `domain/services` (não método do agregado) — permite reuso por módulo de transações futuro sem hidratar agregado completo. Persistência denormalizada de `closing_date`/`due_date` será feita pelo agregado `Transaction` futuro, garantindo histórico imutável. Idempotência genérica em `internal/platform/idempotency/` desbloqueia evolução de billing/identity no futuro, sem acoplar o `card` ao schema de billing. Middleware `RequireUser` é mitigação transitória até JWT/OIDC.

## Dados e Integrações
Domínios de dados:
- `card.cards` — agregado de cartão (300k linhas projetadas).
- `platform.idempotency_keys` — chaves de idempotência compartilhadas (10k linhas/dia, TTL 24h → ~10k linhas em estado estacionário).
- `internal/card` referencia `users.id` por FK lógica; FK física postergada até existir leitura cross-module estável (verificar em `internal/identity/infrastructure/repositories/postgres/`).

Integrações:
- Internas: `internal/platform/observability` (Tracer, Logger), `internal/platform/database` (DBTX), `internal/platform/idempotency` (middleware), `internal/identity` (lookup futuro de usuário — porta a definir).
- Externas: nenhuma no MVP. Sem chamadas HTTP outbound. Sem fila/broker. Sem terceiros.

Consistência requerida:
forte

## Volumetria e Capacidade
Volume atual:
0 cartões (módulo inexistente).

Pico esperado:
- 300k cartões totais em 12 meses (100k usuários × 3 médios).
- 10k criações/dia (~120 RPS médio, pico ~1k RPS em janela de migração de usuários).
- Listagens: 50 RPS médio.
- Consulta de fatura (`/invoices?for=<date>`): pico 200 RPS quando módulo de transações for ativo (assumindo 100k transações/dia × 5% precisando consultar fatura via API).

Taxa de crescimento:
- 10k cards/dia inicialmente; estabiliza ~30k/mês após aquisição.

SLO alvo:
- Disponibilidade 99.5% (≈ 3h36min de downtime/mês).
- p99 CRUD < 300ms (medido no servidor).
- p99 `InvoiceFor` < 10ms (microsegundos reais + overhead OTel).
- p99 paginação `GET /cards` < 50ms com 100 itens por página.

Gargalos conhecidos:
- Postgres único compartilhado com identity/billing — risco baixo para 300k linhas; índice composto `(user_id, created_at DESC) WHERE deleted_at IS NULL` cobre listagem.
- Idempotency middleware grava 1 linha por POST/PUT/DELETE — 10k linhas/dia adicionais, vacuum padrão suficiente.
- `time.LoadLocation("America/Sao_Paulo")` é lock-free após primeiro carregamento; deve ser cacheado em variável de pacote inicializada por uma factory function (R0 proíbe `init()`; usar lazy `sync.Once` no construtor do domain service).

## Segurança e Compliance
Classificação dos dados:
- `users.id` — identificador (não-sensível isoladamente).
- `cards.name` — PII (dado pessoal LGPD).
- `cards.nickname` — PII (pode revelar hábitos/contexto pessoal; LGPD considera dado pessoal).
- `cards.closing_day`/`due_day` — não-sensível.
- Nenhum dado de cartão de pagamento (PAN, CVV, trilha) — fora de escopo PCI.

Autenticação e autorização:
- MVP: header `X-User-ID` (UUID v4) validado por middleware `RequireUser`. Documentado como transitório em ADR-003.
- Fase 2: substituir por JWT/OIDC ou mTLS; contrato não muda (continua injetando `user_id` no `ctx`).
- Autorização: todas as queries de `card_repository` aceitam e filtram por `user_id` recebido do ctx. Nenhuma operação cross-user permitida.

Gestão de segredos:
- Nenhum segredo dedicado ao `card`. Reusa DSN Postgres do `internal/platform`.
- Idempotência: chave é fornecida pelo cliente; armazenada em texto plano (não é segredo, é nonce).

Criptografia:
- TLS terminado pelo balanceador (responsabilidade infra existente).
- Em repouso: criptografia volumétrica do Postgres (responsabilidade de infra). Sem encryption-at-application para PII no MVP (LGPD permite quando há base legal e controles operacionais; ADR-001 registra premissa).

Auditoria e rastreabilidade:
- Spans OTel cobrem CRUD e InvoiceFor.
- Logs estruturados incluem `trace_id`, `user_id`, `card_id`, `operation`, `outcome`. NUNCA logam `name` ou `nickname`.
- Audit log dedicado em tabela própria fica fora do MVP (registrado em "Itens em Aberto" como item não-bloqueante).

Compliance/LGPD:
- Bases legais: execução de contrato (cartão como parte do serviço de finanças pessoais) + legítimo interesse (operação do sistema).
- Direitos do titular: implementação completa do "direito ao esquecimento" requer endpoint `DELETE /api/v1/cards/{id}` (já no escopo) + truncamento de PII em transações antigas — esse último fica para módulo de transações futuro.
- Minimização: apenas `name`, `nickname`, `closing_day`, `due_day` persistidos; nada além disso.
- Retenção: sem política formal de expurgo no MVP. Registrado em "Itens em Aberto".
- PCI-DSS 4.0: aplicação não-PCI; nenhum SAQ aplicável; controles compensatórios documentados via ADR.

## Confiabilidade e Resiliência
SLA/SLO:
- SLO disponibilidade 99.5%.
- SLO latência p99 CRUD < 300ms.
- SLO latência p99 `InvoiceFor` < 10ms.
- Error budget mensal: ≈ 3h36min de degradação aceitável.

RTO/RPO:
- RTO: 30 minutos (tempo de redeploy + restauração de pod).
- RPO: 15 minutos (intervalo de WAL shipping/backup do Postgres compartilhado — herdado da infra atual).

Estratégia de retry/idempotência:
- Middleware `internal/platform/idempotency/` consome header `Idempotency-Key` em POST/PUT/DELETE.
- Tabela `idempotency_keys(scope, key, user_id, request_hash, response_status, response_body, expires_at)` com `UNIQUE (scope, key, user_id)`.
- Retentativa com mesma chave retorna a resposta armazenada se `request_hash` coincide; retorna 409 Conflict se difere; retorna 425 Too Early se chave existe mas resposta ainda não foi persistida (caso raríssimo).
- TTL 24h; vacuum por job posterior (ou Postgres `pg_cron` se disponível). Para o MVP, sem job — a tabela cresce limitada (10k/dia × 1 dia = 10k linhas).
- `InvoiceFor` é puro e idempotente por natureza.

Degradação/contingência:
- Banco indisponível: 503 retornado por handler quando repositório falha; cliente faz retry com `Idempotency-Key`.
- Idempotency storage indisponível: handler responde 503 (não processa a operação) — preferimos falhar a aceitar request que pode duplicar.
- `time.LoadLocation` falhar: erro irrecuperável no startup; processo encerra com `os.Exit(1)` via main (não panic em produção, conforme R5.12).

Rollback:
- Código: revert do commit + redeploy.
- Migration `0011_create_card_cards.down.sql`: renomeia `cards` → `cards_archived_<timestamp>`, dropa apenas índices únicos. Dados preservados.
- Migration `0010_create_platform_idempotency_keys.down.sql`: idem.
- Runbook em `docs/runbooks/card-rollback.md` documenta passos.

## Observabilidade e Operação
Métricas:
- Não há métricas Prometheus dedicadas no MVP (decisão de Rodada 3).
- Latência e taxa de erro são derivadas dos spans OTel.

Logs:
- Estruturados JSON via `internal/platform/observability/Logger` (slog por trás).
- Campos: `trace_id`, `span_id`, `user_id`, `card_id`, `operation`, `outcome`, `duration_ms`, `error_kind`.
- Eventos: `card.create.started`, `card.create.completed`, `card.create.failed`, `card.list.served`, `card.update.completed`, `card.delete.completed`, `card.invoice_for.computed`, `card.idempotency.replay`, `card.auth.rejected`.
- NUNCA logam `name`/`nickname`/payload completo de request/response.

Traces:
- `card.handler.create`, `card.handler.list`, `card.handler.get`, `card.handler.update`, `card.handler.delete`, `card.handler.invoice_for`.
- `card.middleware.require_user`, `card.middleware.idempotency`.
- `card.usecase.create`, `card.usecase.list`, etc.
- `card.repository.pg.insert`, `card.repository.pg.list_by_user`, `card.repository.pg.get_by_id`, `card.repository.pg.update`, `card.repository.pg.soft_delete`.
- `card.domain.invoice_for` (atributos: `closing_day`, `due_day`, `purchase_year_month`).
- `span.RecordError(err)` em todo erro propagado.

Alertas:
- Alertas em error budget burn rate (1h e 6h windows) configurados no OTel/Grafana após estabilização (1ª semana sem alerta para evitar fadiga).
- Alerta crítico em > 1% de erro 5xx no handler `/cards` por 15min.
- Alerta crítico em > 5% de taxa de `card.idempotency.replay` (indica cliente bugado).

Dashboards/Runbooks:
- Dashboard "Card Module" em Grafana exibindo: latência por endpoint (do trace), taxa de erro, contagem de operações por outcome.
- Runbook `docs/runbooks/card-rollback.md` — passos de rollback de código e migration.
- Runbook `docs/runbooks/card-incident-triage.md` — playbook para falhas de InvoiceFor (verificar entrada, validar fixtures, abrir fixture de regressão).

## Performance e Escalabilidade
Latência alvo:
- p99 CRUD < 300ms.
- p99 `InvoiceFor` < 10ms.
- p99 listagem (`GET /cards` paginada) < 50ms para 100 itens.

Estratégia de escala:
- Horizontal: instâncias adicionais do `cmd/server`; stateless. Idempotência centralizada no Postgres garante consistência entre instâncias.
- Postgres compartilhado: índices cobrem queries hot path; sem necessidade de sharding em volume MVP.
- `time.LoadLocation("America/Sao_Paulo")` carregada uma única vez em variável de pacote inicializada por construtor lazy (`sync.Once`), evitando overhead por chamada.

Limites conhecidos:
- Idempotency storage: ~10k linhas/dia + janela 24h → ~10k linhas em estado estacionário; inserção ~120 TPS no pico. pgx + tabela com PK composta `(scope, key, user_id)` suporta com folga.
- `InvoiceFor`: stateless O(1); limite teórico = throughput do handler/middleware (estimado 5k RPS por instância).
- Tabela `cards`: 300k linhas — desprezível para Postgres.

Teste de carga:
- Cenário 1: 1k RPS de POST `/cards` por 10min com Idempotency-Key único — validar 99.5% e p99 < 300ms.
- Cenário 2: 200 RPS de `GET /cards/{id}/invoices?for=<date>` por 10min — validar p99 < 10ms.
- Cenário 3: replay de 30% das requests com mesma `Idempotency-Key` — validar 0 duplicação e p99 mantido.
- Ferramenta: `k6` ou `vegeta` (não estabelecido no repo; ADR-005 a definir). Execução pré-go-live em ambiente de homologação.

## Custos e Orçamento
Orçamento estimado:
~6 dias úteis de uma pessoa sênior Go (estimativa do brainstorm) + 1 dia adicional para `internal/platform/idempotency/` + 0.5 dia para middleware `RequireUser` + 0.5 dia para OpenAPI + 0.5 dia para load testing leve = **~8.5 dias**.

Drivers de custo:
- Domain + InvoiceFor + property-based tests: 2 dias.
- Pacote `internal/platform/idempotency/` + migration + testes: 1 dia.
- Repository Postgres + migration `cards` + testes de integração: 1.5 dias.
- Handlers HTTP + middleware `RequireUser` + DTOs + mapeamento de erro: 1 dia.
- Module wiring + registro em `cmd/server/server.go`: 0.5 dia.
- Observabilidade (spans + logs estruturados): 0.5 dia.
- Contrato OpenAPI 3.1 + golden files de contract test: 0.5 dia.
- ADRs (4 documentos) + runbooks: 0.5 dia.
- Load test em homologação: 0.5 dia.
- Revisão final + ajustes: 0.5 dia.

Guardrails de custo:
- Sem nova dependência externa (zero custo de licença/SaaS).
- Sem nova infraestrutura (Postgres compartilhado, sem novo cluster, sem nova fila).
- Custo de armazenamento desprezível: 300k linhas de cartões + 10k linhas de idempotência ≈ < 50 MB.

Plano de otimização:
- Se p99 `InvoiceFor` ultrapassar SLO: cachear `time.Location` em variável de pacote (já planejado).
- Se idempotency storage virar gargalo: introduzir cache LRU em memória por instância (em frente do storage Postgres) — fica como fase 2.
- Se listagem com cursor virar lenta para usuário com muitos cartões: avaliar índice parcial específico por `user_id`; volume previsto não exige.

## Riscos e Mitigações
- Risco: Bug em edge case raro de calendário (ex.: ano bissexto secular 2400; mês 31 com `closing_day=31` em fev de não-bissexto).
  Impacto: compra cai em fatura errada — quebra financeira para o usuário.
  Mitigação: 50+ fixtures table-driven cobrindo fev/28, fev/29, abr/jun/set/nov (30 dias), virada de ano, due==closing, due>closing, due<closing; property-based test `testing/quick` com 10k iterações em `(purchaseDate, closingDay, dueDay)` validando invariantes (fatura sempre no futuro, monotonicidade, idempotência).
  Dono: dev responsável + revisão obrigatória por segundo dev.

- Risco: Header `X-User-ID` falsificado (sem auth real).
  Impacto: usuário malicioso opera sob outro user_id.
  Mitigação: documentar limitação em ADR-003; deploy inicial em ambiente sem exposição pública direta (atrás de gateway autenticador). Antes de exposição pública geral, substituir por JWT/OIDC.
  Dono: equipe de produto + segurança.

- Risco: Idempotency storage acumula linhas indefinidamente sem job de vacuum.
  Impacto: crescimento de tabela, espaço em disco.
  Mitigação: TTL 24h documentado; criação de job de cleanup é item de fase 2 quando volume justificar. Monitoramento da tabela em dashboard.
  Dono: dev responsável.

- Risco: PII (`nickname` revelador) vazar em logs por descuido.
  Impacto: violação LGPD.
  Mitigação: helper `redactCardLogFields(card)` em handler; revisão obrigatória em PR; lint regex de PR procurando `nickname` em chamadas de logger.
  Dono: dev responsável + revisor.

- Risco: Drift entre `closing_date`/`due_date` calculado pela API e calculado pelo módulo de transações futuro (ambos chamam `InvoiceFor` — divergência seria bug).
  Impacto: usuário vê fatura X na API e transação cai em fatura Y.
  Mitigação: `InvoiceFor` é única fonte da verdade exposta por `CardLookup` (porta Go); módulo de transações importa o pacote `card.domain.services` (permitido pelas regras de fronteira do `AGENTS.md`); contract test verifica determinismo.
  Dono: dev card + dev transação no futuro.

- Risco: Inserção concorrente com mesmo `(user_id, nickname)` por race condition entre `SELECT` e `INSERT`.
  Impacto: violação de unicidade.
  Mitigação: confiar no `UNIQUE INDEX` parcial Postgres; capturar `pgerrcode.UniqueViolation` no repositório e retornar `ErrNicknameConflict`. Handler mapeia para 409 Conflict.
  Dono: dev responsável.

## Trade-offs e Decisões
Alternativas consideradas:
- Algoritmo: função pura vs pré-computação materializada vs híbrida vs RRULE (avaliadas e descartadas no brainstorm — scorecard 26/31/28 vs 44).
- Idempotência: tabela local por módulo vs pacote genérico em platform — escolhido pacote genérico.
- ID generation: UUID v4 vs UUID v7 vs ULID vs BIGSERIAL — escolhido v4 (aderência à convenção).
- Paginação: cursor vs offset vs sem paginação — escolhido cursor opaco.
- Auth: header `X-User-ID` vs body vs JWT já no MVP — escolhido header transitório.
- PBT engine: `testing/quick` vs `go test -fuzz` vs `gopter` — escolhido `testing/quick`.
- Métricas: apenas OTel vs Prometheus dedicado — escolhido apenas OTel.

Decisão tomada:
Alternativa A do brainstorm (função pura `InvoiceFor` + clamp + testes exaustivos) operacionalizada por: header `X-User-ID`, idempotência genérica em platform, UUID v4, cursor base64, `testing/quick`, OTel-only, deploy direto, rollback preservando dados.

Trade-off aceito:
- +1 dia para construir `internal/platform/idempotency/` em troca de evitar dívida técnica imediata.
- Auth transitória (`X-User-ID`) em troca de bloquear o MVP até auth real; mitigado por exposição controlada.
- Sem métricas Prometheus dedicadas em troca de simplicidade; mitigado por traces OTel suficientes para SLO 99.5%.
- Histórico imutável (mudança de ciclo não recalcula) em troca de integridade contábil.

## Plano de Entrega e Rollout
Fases:
- F1 (dias 1–2): domain + InvoiceFor + property-based tests + ADR-001 e ADR-004.
- F2 (dia 3): pacote `internal/platform/idempotency/` + migration `0010` + testes unitários e integração.
- F3 (dias 4–5): repository Postgres + migration `0011` + testes de integração com testcontainers.
- F4 (dia 6): handlers HTTP + middleware `RequireUser` + DTOs + mapeamento de erro + ADR-002 e ADR-003.
- F5 (dia 7): module wiring + registro em `cmd/server/server.go` + logs/spans completos.
- F6 (dia 8): OpenAPI 3.1 + contract tests + runbooks + load test em homologação.
- F7 (dia 8.5): revisão final, ajustes, merge.

Migração:
- Sem dados legados para migrar (módulo novo).
- Migrations 0010 e 0011 aplicadas via pipeline padrão de deploy (devkit-go migrator).
- Backward-compatibility: nenhuma rota existente é alterada; novos endpoints adicionados sob `/api/v1/cards`.

Feature flags/canary:
- Sem feature flag por usuário.
- Sem canary — deploy direto após smoke test em homologação.
- "Flag" implícita: as rotas só existem após `srv.RegisterRouters(cardModule.CardRouter)` ser chamado em `cmd/server/server.go`. Rollback = revert do registro.

Critério de rollback:
- Taxa de erro 5xx > 5% por 15 minutos no endpoint `/cards/*`.
- Latência p99 CRUD > 1s por 15 minutos.
- Qualquer incidente de divergência financeira reportado por usuário.
- Procedimento: revert do commit em `cmd/server/server.go` + redeploy. Migrations `down` aplicadas apenas em incidente catastrófico (preservam dados via rename).

## Decomposição em Épicos e Features

### Epic 01 - Domínio e cálculo de fatura
Objetivo: Entregar a função pura `InvoiceFor` com cobertura exaustiva e VOs/entities mínimos do agregado `Card`, sem qualquer IO.
Feature 01: Value objects `Nickname`, `CardName`, `BillingCycle` com validação e testes table-driven.
Feature 02: Domain service `BillingCycle.InvoiceFor(purchase, cycle, tz)` com clamp e auto-detecção `closing_day` vs `due_day`.
Feature 03: Property-based tests com `testing/quick` validando invariantes de monotonicidade, idempotência e clamp.
Feature 04: ADR-001 (domain service estrutura) e ADR-004 (algoritmo de clamp).

### Epic 02 - Plataforma de idempotência
Objetivo: Pacote genérico reutilizável `internal/platform/idempotency/` com storage Postgres e middleware Chi.
Feature 01: Migration `0010_create_platform_idempotency_keys.{up,down}.sql` (down preserva via rename).
Feature 02: `idempotency.Storage` interface + impl Postgres com `pgerrcode.UniqueViolation` handling.
Feature 03: Middleware `idempotency.Middleware(scope string, storage Storage)` para chi.
Feature 04: Testes unitários (mock storage) + integração com testcontainers.

### Epic 03 - Persistência do agregado Card
Objetivo: Repositório Postgres para `Card`, com migrations e índice parcial de unicidade.
Feature 01: Migration `0011_create_card_cards.{up,down}.sql` com `UNIQUE INDEX cards_user_nickname_active`.
Feature 02: `cardRepository` em pgx puro com `Insert`, `GetByID`, `ListByUser`, `UpdateByID`, `SoftDeleteByID`.
Feature 03: `RepositoryFactory` em `application/interfaces` + impl em `infrastructure/repositories/postgres/repository_factory.go`.
Feature 04: Testes de integração com testcontainers cobrindo concorrência (UniqueViolation) e cursor de paginação.

### Epic 04 - HTTP CRUD do cartão
Objetivo: Endpoints HTTP completos com middleware `RequireUser` e idempotência.
Feature 01: Middleware `RequireUser` extraindo `X-User-ID` (UUID v4) + ADR-003.
Feature 02: Handlers `Create`, `List`, `Get`, `Update`, `Delete` finos, com mapeamento de erro→HTTP via `errors.Is`.
Feature 03: Handler `InvoiceFor` consumindo o domain service e respondendo `GET /cards/{id}/invoices?for=<date>`.
Feature 04: DTOs input/output + sentinels em `application/errors.go`.
Feature 05: `CardRouter.Register(r chi.Router)` + middleware `idempotency.Middleware("card")` em POST/PUT/DELETE.

### Epic 05 - Wiring, observabilidade e contrato público
Objetivo: Plugar o módulo no servidor, instrumentar e publicar contrato OpenAPI 3.1.
Feature 01: `module.go` com `NewCardModule(cfg, o11y, mgr) CardModule` (sem error; sem init complexo) + porta `CardLookup`.
Feature 02: Registro em `cmd/server/server.go` com log "card module wired".
Feature 03: Spans OTel em handler/usecase/repository/domain e helper `redactCardLogFields` para logs sem PII.
Feature 04: `openapi.yaml` com schemas e respostas; contract tests via golden files.
Feature 05: ADR-002 (platform/idempotency) + runbooks `card-rollback.md` e `card-incident-triage.md`.

## Itens em Aberto
- Política formal de retenção/expurgo de cartões soft-deleted (atual: indefinido; sugestão: 5 anos para alinhar com prática contábil BR).
- Decisão sobre FK física `cards.user_id → users.id` (atual: FK lógica; verificar se identity expõe convenção de FK cross-module).
- Job de limpeza de `idempotency_keys` expirados (atual: sem job; aceitável até ~30 dias de operação).
- Endpoint de recálculo manual de faturas (`POST /cards/{id}/recompute-invoices?from=<date>`) — adiado para fase 2.
- Ferramenta de load test (k6 vs vegeta) — escolha fica para ADR-005 durante F6.
- Audit log de domínio (`card_audit_log`) — adiado para fase 2.
- Versionamento histórico de ciclos por cartão (`card_billing_cycles`) — adiado para fase 2.
- Substituição de `X-User-ID` por JWT/OIDC — depende do épico de autenticação ainda não planejado.
