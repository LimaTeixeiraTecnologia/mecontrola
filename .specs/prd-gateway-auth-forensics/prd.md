# Documento de Requisitos do Produto (PRD) — Gateway Authentication + Auth Event Forensics

<!-- spec-version: 1 -->

<!--
Histórico de versões:
- v1 (2026-06-12): escopo MVP do hardening pré go-live derivado de `docs/planos/2026-06-11-auditoria-seguranca-pre-golive.md` (itens B1 e A5) e da análise crítica `~/.claude/plans/analise-de-forma-criteriosa-shiny-book.md`. Recorta apenas o vetor identity/auth do plano de auditoria; os 8 itens restantes (B2, B3, B4, B5, B6, B7, A2/A4, A10) seguem direto para `create-tasks` referenciando o plano-fonte. Cravação inegociável de 7 decisões pendentes do B1 + 1 decisão pendente do A5. Modelagem DMMF aplicada a tipo e estado (smart constructors, discriminated union, workflow puro). Skill `go-implementation` obrigatória em toda execução derivada. Princípio inegociável: MVP robusto, eficiente, econômico, production-ready/proof, sem falso positivo.
-->

## Visão Geral

O middleware `InjectPrincipalFromHeader` (`internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go`) hoje aceita qualquer UUID enviado no header `X-User-ID` e cria `auth.Principal{Source: SourceHeader}` sem nenhuma prova de origem. A premissa "o header vem da LLM em rede interna" é apenas uma declaração — não é enforçada criptograficamente. Qualquer rota autenticada exposta publicamente sem o Caddy bloquear o header permite impersonação trivial. Esse é o único bloqueante de segurança classificado como **crítico** na auditoria pré go-live.

Adicionalmente, a tabela `mecontrola.auth_events` (módulo `internal/identity`) registra `auth.principal_established`, `auth.failed` e `auth.unknown_user` indexados por `user_id + occurred_at`, mas **não armazena `request_id` nem `client_ip`**. Sem esses dois campos, forense de incidente fica cega: não é possível correlacionar eventos com traces/logs nem identificar a origem real do request (apenas a borda do Caddy é conhecida).

Este PRD entrega:
1. **Gateway Authentication (B1)** — middleware `RequireGatewayAuth` HMAC-SHA256 com rotação `current/next` que enforça a fronteira de confiança LLM ↔ API. Aplicado **antes** de `InjectPrincipalFromHeader` em todos os routers que consomem o injetor. Defesa em profundidade: o Caddy strip-a `X-User-ID`/`X-Gateway-Auth` externos e o app Go valida a assinatura mesmo assim.
2. **Auth Event Forensics (A5)** — colunas `request_id` e `client_ip` em `mecontrola.auth_events`, populadas via `X-Request-Id` (já presente nos handlers via OTel) e `X-Forwarded-For` sanitizado (último hop do Caddy).

O escopo é **MVP cirúrgico**: nenhum JWT, nenhum KMS, nenhum cache distribuído de nonce. Rotação por env duplicada no mesmo padrão já usado por WhatsApp HMAC e Kiwify HMAC. Sem refactor dos routers que não usam `InjectPrincipalFromHeader` (webhooks têm HMAC próprio e não são alvo aqui). Sem RLS Postgres (fica pós go-live conforme decisão da auditoria).

### Volumetria-alvo e SLO do MVP

- **Usuários ativos**: 500 a 5.000 (mesma volumetria do `prd-auth-foundation`).
- **Requisições autenticadas via gateway**: pico esperado ~5 req/s (LLM intermedia WhatsApp inbound + comandos do agente).
- **Inserts em `auth_events`**: ~7.000/dia em estado estacionário, ~210k/mês — sem mudança vs. baseline atual, apenas duas colunas a mais por linha.
- **Latência**: overhead do `RequireGatewayAuth` p99 < 2 ms (constant-time HMAC + parse de timestamp + match em discriminated union). Sem IO no caminho síncrono além do `auth_events` insert que já existe via outbox.
- **Disponibilidade**: SLO mensal de 99,5% no endpoint `/api/v1/cards*` (e demais rotas com gateway), mesmo nível do baseline.

### Itens Fora de Escopo

- **JWT RS256 + JWKS**: pós go-live, quando houver > 1 gateway/integrador ou app móvel direto (item da seção 6 do plano-fonte).
- **RLS Postgres**: pós go-live (defesa em profundidade segunda onda).
- **Cache distribuído de nonces (Redis)**: aceitamos janela de replay de 60s dentro do mesmo socket TCP no MVP; mitigação documentada em ADR.
- **Rotação automática de secret via KMS/Vault**: rotação manual com env duplicada (`current`/`next`) seguindo o padrão WhatsApp/Kiwify já em produção.
- **Cobertura dos webhooks WhatsApp/Kiwify**: esses endpoints têm HMAC próprio dedicado; o gateway HMAC **não** se aplica a eles. Tabela explícita de rotas em RF-12.
- **Migração do `Source: SourceHeader` para outro valor**: o campo permanece `header`; a prova de origem fica no middleware externo que valida HMAC do gateway antes da injeção.
- **Itens B2 (timestamp WhatsApp), B3 (Caddyfile), B4 (backup restore), B5 (ufw), B6 (CORS guard), B7 (rate limit WhatsApp), A2/A4 (headers globais), A10 (rate limit por user)** do plano-fonte: vão direto para `create-tasks` separado, fora deste PRD.

## Objetivos

- **OBJ-01**: enforçar criptograficamente a fronteira de confiança LLM ↔ API, eliminando a impersonação trivial via `X-User-ID` em rotas autenticadas. Sem este enforcement, o modelo de auth atual é uma string declarada, não uma garantia.
- **OBJ-02**: manter o adapter middleware **fino** (R-ADAPTER-001.2): apenas parse de header + delegação para workflow puro de domínio. Toda lógica de verificação fica em `domain/services` do módulo `identity` sob a forma de função pura testável sem mock.
- **OBJ-03**: aplicar modelagem DMMF para tipo e estado do contrato de gateway: smart constructors para os value objects (`GatewaySignature`, `GatewayTimestamp`), discriminated union para o resultado (`GatewayAuthResult = Valid | Rotated | InvalidSignature | StaleTimestamp | MissingHeader`), workflow `VerifyGatewayRequest` puro. Inegociável.
- **OBJ-04**: habilitar forense de incidente correlacionando `auth_events` com traces OTel e logs Loki via `request_id` (X-Request-Id) e identificando a borda real via `client_ip` (X-Forwarded-For sanitizado).
- **OBJ-05**: respeitar o Padrão Obrigatório de Módulo e as Regras Estritas R0–R7 + R-ADAPTER-001 da skill `go-implementation`. Zero comentários em arquivos `.go` de produção. Zero abstração de tempo (regra de memória: `time.Now().UTC()` inline).
- **OBJ-06**: produzir rollout sem downtime usando o padrão `current`/`next` já estabelecido em WhatsApp HMAC e Kiwify HMAC. Sem soft-launch parcial: cutover atômico Caddy ↔ LLM ↔ app com janela de aceite dupla durante a transição.
- **OBJ-07**: tornar inviável por gate de revisão (grep automatizado em CI) que uma rota com `InjectPrincipalFromHeader` no chain seja servida sem `RequireGatewayAuth` à frente.

### Métricas de Sucesso

- **M-01**: 100% das rotas que hoje montam `InjectPrincipalFromHeader` (`internal/card/infrastructure/http/server/router.go` e quaisquer futuras) passam a executar `RequireGatewayAuth` antes do injetor. Verificado por grep no CI.
- **M-02**: 0 (zero) requisição em produção retorna 200 OK em rota autenticada sem header `X-Gateway-Auth` válido. Verificado por métrica Prometheus `identity_gateway_auth_total{result}` no dashboard "Auth Module".
- **M-03**: p99 do overhead do middleware `RequireGatewayAuth` ≤ 2 ms, medido em microbenchmark Go (`testing.B`) com input realista (UUID + timestamp + HMAC válido) e em produção via histograma OTel.
- **M-04**: discriminated union `GatewayAuthResult` cobre as 5 variantes explicitadas em OBJ-03 e o switch no middleware tem cobertura exaustiva validada por `golangci-lint` (`exhaustive` enabled).
- **M-05**: 0 ocorrência de `bool + error` paralelos ou de `Result[T]`/`Either` customizado introduzido na codebase neste PR. Validado por revisão.
- **M-06**: 100% das inserções em `auth_events` pós-rollout populam `request_id` e `client_ip` quando esses campos chegam no request (best-effort: NULL aceito quando o Caddy não injeta, registrado em log estruturado para investigação).
- **M-07**: 0 (zero) `panic` ou `init()` nos novos pacotes; 0 comentário em `.go` de produção (validado pelo grep R-ADAPTER-001.1).
- **M-08**: ADR-001 (canonicalização HMAC), ADR-002 (rotação `current/next`), ADR-003 (replay window de 60s sem cache de nonce), ADR-004 (ordem na chain de middlewares), ADR-005 (rollout cutover), ADR-006 (política de erro 401 + métrica), ADR-007 (tabela de rotas que pulam o gateway), ADR-008 (sanitização de `X-Forwarded-For`) publicados em `.specs/prd-gateway-auth-forensics/` e referenciados na techspec.
- **M-09**: gate de revisão automatizado prova ausência de bypass — toda rota que adicione `InjectPrincipalFromHeader` no chain DEVE ter `RequireGatewayAuth` acima dela. Implementado como script em `deployment/scripts/` chamado pelo `task lint`.

## Histórias de Usuário

- **US-01 — Atacante externo não impersona usuário legítimo**
  Como operador do MeControla, quero que qualquer requisição vinda da internet pública com header `X-User-ID` arbitrário e sem `X-Gateway-Auth` válido seja rejeitada com 401 antes de qualquer use case ser invocado, para que impersonação trivial seja impossível mesmo se a regra do Caddy falhar.

- **US-02 — LLM legítima opera sem fricção**
  Como integração interna (LLM intermediária), quero assinar minhas requisições com HMAC-SHA256 usando o secret compartilhado dentro da janela de timestamp aceita (±60s), para que minhas chamadas autenticadas sejam processadas sem custo adicional perceptível (overhead < 2ms p99).

- **US-03 — Operador rotaciona secret sem downtime**
  Como operador do MeControla, quero rotacionar `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` e `IDENTITY_GATEWAY_SHARED_SECRET_NEXT` aproveitando a janela dupla de aceite, para que a troca aconteça sem queda do canal LLM ↔ API e com visibilidade via métrica `identity_gateway_auth_total{result="rotated"}`.

- **US-04 — Responder a incidente em minutos, não em horas**
  Como responsável por forense do MeControla, quero correlacionar uma linha em `auth_events` com o trace OTel completo da requisição via `request_id` e identificar o IP real do client via `client_ip`, para reconstruir uma cadeia de eventos suspeita em < 15 minutos a partir de um único alerta.

- **US-05 — Reviewer bloqueia regressão de segurança em PR**
  Como reviewer do MeControla, quero que o CI bloqueie qualquer PR que adicione `InjectPrincipalFromHeader` em um router sem `RequireGatewayAuth` à frente, para que o erro humano não introduza bypass silencioso da fronteira de confiança.

- **US-06 — Compatibilidade com webhooks preservada**
  Como mantenedor do MeControla, quero que webhooks `/api/v1/whatsapp` e `/api/v1/kiwify` continuem operando exclusivamente com seu HMAC próprio (Meta SHA-256, Kiwify SHA-1), para que a introdução do gateway HMAC não exija mudança contratual com provedores externos.

## Requisitos Funcionais

### B1 — Gateway Authentication

- **RF-01**: O sistema MUST validar o header `X-Gateway-Auth` em toda rota que monte `InjectPrincipalFromHeader` no chain. A validação ocorre via middleware `RequireGatewayAuth` posicionado **antes** do injetor.
- **RF-02**: O middleware MUST aceitar dois headers: `X-Gateway-Auth` (HMAC-SHA256 em hex lowercase) e `X-Gateway-Timestamp` (unix epoch segundos como string decimal).
- **RF-03**: O sistema MUST rejeitar com **HTTP 401** sem body detalhado quando: a) qualquer dos dois headers está ausente; b) `X-Gateway-Timestamp` não é inteiro válido; c) `|now - timestamp| > 60s`; d) HMAC não confere com nenhum dos dois secrets (`current`, `next`). Nenhum detalhe do motivo é vazado ao client.
- **RF-04**: A mensagem canônica do HMAC MUST seguir o formato decidido em ADR-001 (não decidido neste PRD; deve ser cravado na techspec). O PRD fixa **apenas** que a canonicalização deve incluir no mínimo: `X-User-ID` (UUID canonical lowercase) + `X-Gateway-Timestamp` (string como recebida). Inclusão adicional de método HTTP, path ou hash do body fica para a ADR.
- **RF-05**: O sistema MUST aceitar HMAC válido tanto com `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` quanto com `IDENTITY_GATEWAY_SHARED_SECRET_NEXT`. Match com `current` resulta em `GatewayAuthResult.Valid`; match com `next` resulta em `GatewayAuthResult.Rotated`. Ambos passam, distinção apenas para métrica.
- **RF-06**: O sistema MUST validar não-vazio de `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` em `Environment=production` no boot via `Config.Validate()`. Boot falha com mensagem clara. `_NEXT` opcional. Pelo menos um secret de 32 bytes (256 bits) de entropia.
- **RF-07**: A verificação HMAC MUST usar `hmac.Equal` (constant-time). Implementação inline com `crypto/hmac` + `crypto/sha256`. Sem dependência externa nova.
- **RF-08**: O sistema MUST registrar evento em `auth_events` para toda falha do gateway com `reason` ∈ {`gateway_missing_header`, `gateway_invalid_timestamp`, `gateway_stale_timestamp`, `gateway_invalid_signature`}. Eventos são publicados via outbox transacional (padrão já estabelecido em `prd-auth-foundation`).
- **RF-09**: O sistema MUST emitir métrica Prometheus `identity_gateway_auth_total{result}` com `result` ∈ {`valid`, `rotated`, `missing_header`, `invalid_timestamp`, `stale_timestamp`, `invalid_signature`}. Sem `user_id` como label (cardinalidade controlada).
- **RF-10**: O sistema MUST emitir histograma OTel `identity_gateway_auth_duration_seconds` para overhead do middleware. Buckets: 0.0001, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.05.
- **RF-11**: Toda implementação MUST aplicar modelagem DMMF inegociável:
  - Smart constructor `NewGatewaySignature(hex string) (GatewaySignature, error)` que valida tamanho e charset.
  - Smart constructor `NewGatewayTimestamp(raw string, now time.Time) (GatewayTimestamp, error)` que parseia, valida janela e rejeita fora dela. **Único** ponto de uso de `time.Now().UTC()` é no caller (`r.Context()` chain ou middleware), passado por argumento — não abstrair em `Clock` (regra de memória).
  - Discriminated union `GatewayAuthResult` como sealed interface ou tipo enum: `Valid`, `Rotated`, `InvalidSignature`, `StaleTimestamp`, `MissingHeader`. Match exaustivo no middleware (`switch` com `default` proibido — usar listagem explícita).
  - Workflow puro `VerifyGatewayRequest(req VerifyRequest, secrets SecretPair, now time.Time) GatewayAuthResult` em `internal/identity/domain/services/`. Sem IO, sem context. Testável sem mock.
- **RF-12**: O sistema MUST manter a seguinte tabela de rotas (mantida na techspec, ADR-007) classificando aplicação do gateway:

| Rota | Gateway HMAC? | Mecanismo de auth atual |
|---|---|---|
| `/api/v1/cards*` | SIM | `InjectPrincipalFromHeader` |
| Futuras rotas com `InjectPrincipalFromHeader` | SIM | idem |
| `/api/v1/whatsapp/*` | NÃO | HMAC-SHA256 Meta próprio |
| `/api/v1/kiwify/*` | NÃO | HMAC-SHA1 Kiwify próprio |
| `/api/v1/onboarding/*` | NÃO (avaliar caso a caso) | rate-limit + token mágico |
| `/healthz`, `/readyz`, `/metrics` | NÃO | nenhum (interno) |

- **RF-13**: O adapter middleware MUST permanecer fino conforme R-ADAPTER-001.2: parse de header → invocar workflow puro → match no resultado → 401 ou next. **Proibido**: cálculo HMAC inline, branching sobre estado de domínio, leitura de configuração além do `SecretPair` injetado no construtor.

### A5 — Auth Event Forensics

- **RF-14**: O sistema MUST adicionar duas colunas em `mecontrola.auth_events`:
  - `request_id TEXT NULL` (formato livre, ULID/UUID; consumido do header `X-Request-Id` quando presente, ou do `trace_id` OTel como fallback).
  - `client_ip INET NULL` (sanitizado de `X-Forwarded-For` conforme ADR-008; NULL aceito quando ausente).
- **RF-15**: A migration MUST seguir o padrão golang-migrate já estabelecido no repositório, com `up` e `down` ambos preservando dados (no `down`, apenas `DROP COLUMN`).
- **RF-16**: O use case `EstablishPrincipal` (e variantes `Failed`, `UnknownUser`) MUST receber `request_id` e `client_ip` como entrada e persistir. O extractor desses campos fica em adapter de entrada (middleware/handler), não no use case.
- **RF-17**: A sanitização de `X-Forwarded-For` MUST seguir ADR-008: o app **confia apenas na borda do Caddy**; a sanitização pega o **último IP** da lista (mais próximo ao Caddy), valida com `net.ParseIP` e rejeita `nil`. Sem trust em cadeias de proxy arbitrárias.
- **RF-18**: 0 (zero) `request_id` ou `client_ip` aparece em log estruturado em texto livre como concatenação de mensagem. Esses campos vão em campos estruturados `slog.String("request_id", ...)` e `slog.String("client_ip", ...)`.

### Cross-Cutting

- **RF-19**: Toda implementação MUST carregar `.claude/skills/go-implementation/SKILL.md` (Etapas 1 a 5, R0–R7, R-ADAPTER-001) e `agent-governance/references/domain-modeling.md` (DMMF). Skill `go-implementation` declarada como obrigatória em toda task derivada.
- **RF-20**: Toda implementação MUST passar `task lint && task test && task vulncheck`. Gate adicional: grep R-ADAPTER-001.1 retorna vazio para arquivos novos.
- **RF-21**: Toda implementação MUST passar gate de revisão M-09 (script que prova ausência de bypass).
- **RF-22**: O rollout MUST seguir o plano de ADR-005 (cutover atômico Caddy → LLM → app). Sem soft-launch parcial em produção. Janela dupla de aceite (`current` + `next`) durante a transição.
- **RF-23**: 0 (zero) introdução de dependência externa nova em `go.mod`. Implementação usa exclusivamente `crypto/hmac`, `crypto/sha256`, `encoding/hex`, `net`, `strconv` e libs já presentes (devkit-go, observability).

## Riscos e Mitigações

- **R-01**: Erro de canonicalização HMAC entre app Go e LLM cliente. **Mitigação**: ADR-001 cravado antes de qualquer código, com vetor de teste fixo (input → expected hex) reproduzível em Go e em Python (linguagem provável do client LLM).
- **R-02**: Replay dentro da janela de 60s permite reexecução de requisição de mutação. **Mitigação**: aceito no MVP conforme ADR-003 — mutações já são idempotentes via `Idempotency-Key` em `platform_idempotency_keys`. Documentado como risco residual a ser endereçado pós go-live com cache de nonce em Redis (item da segunda onda do plano-fonte).
- **R-03**: Rotação manual de secret causa downtime se operador confundir ordem `current/next`. **Mitigação**: runbook em `docs/runbooks/gateway-auth-rotation.md` com checklist + script idempotente que valida ambos secrets antes do reload.
- **R-04**: PR futuro adiciona `InjectPrincipalFromHeader` sem `RequireGatewayAuth`. **Mitigação**: gate de revisão M-09 implementado como script de CI (`task lint:auth-bypass`).
- **R-05**: Drift entre PRD e techspec. **Mitigação**: spec-hash do PRD injetado no topo da techspec; `create-tasks` e `execute-task` detectam drift downstream.

## Requisitos Não-Funcionais

- **NRF-01**: Overhead p99 do middleware ≤ 2 ms (cf. M-03).
- **NRF-02**: 0 allocations evitáveis no hot path: reusar buffers, evitar `fmt.Sprintf` para canonicalização (usar `bytes.Buffer` ou concat direto).
- **NRF-03**: Cobertura de teste ≥ 95% em `domain/services/verify_gateway_request.go` (workflow puro). Teste unitário sem mock. Tabela de teste cobrindo todas as 5 variantes do `GatewayAuthResult` + rotação + canonicalização.
- **NRF-04**: Cobertura de teste ≥ 85% no middleware (adapter). Teste de integração HTTP cobrindo: 200 com gateway válido, 401 com cada motivo de falha, evento `auth_events` registrado com `reason` correto.
- **NRF-05**: Observabilidade: span OTel `auth.require_gateway_auth` com atributos `result`, `rotated`, `latency_ms`. Sem `user_id` em atributo de span (mantém padrão de cardinalidade do `prd-auth-foundation`).
- **NRF-06**: Documentação: runbook de rotação + diagrama de chain de middleware em `docs/runbooks/gateway-auth.md`.

## Dependências e Suposições

- **DEP-01**: Caddy reverse proxy em produção configurado para fazer strip de `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp` externos antes do upstream — coberto por **B3 do plano-fonte** (fora deste PRD, mas pré-requisito de produção).
- **DEP-02**: LLM intermediária implementa assinatura HMAC conforme ADR-001. **Suposição**: o operador da LLM tem capacidade de alterar o cliente HTTP para incluir os dois headers. Sem essa capacidade, este PRD não pode ir para produção.
- **DEP-03**: `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` provisionado no host Hostinger via `.env` (mesmo padrão de outros secrets) com entropia ≥ 256 bits gerada por `openssl rand -hex 32`.
- **DEP-04**: Padrão de outbox + `auth_events` já implementado em `prd-auth-foundation` continua válido. Este PRD apenas adiciona duas colunas e quatro valores novos de `reason`.

## Fluxos de Sucesso e Falha

### Fluxo de Sucesso (request da LLM com HMAC válido)

1. LLM gera `timestamp = now_unix()`.
2. LLM calcula `signature = hex(hmac_sha256(secret_current, canonical(user_id, timestamp)))`.
3. LLM envia `POST /api/v1/cards` com `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp`.
4. Caddy roteia para app Go.
5. `RequireGatewayAuth` parseia headers, invoca `VerifyGatewayRequest` puro, recebe `GatewayAuthResult.Valid`.
6. Métrica `identity_gateway_auth_total{result="valid"}` incrementa. Span OTel `auth.require_gateway_auth` fechado com `result=valid`.
7. `InjectPrincipalFromHeader` cria `auth.Principal{UserID: ..., Source: SourceHeader}` no context.
8. Handler executa use case normalmente. `EstablishPrincipal` registra `auth.principal_established` com `request_id` e `client_ip`.

### Fluxo de Falha (HMAC inválido)

1. Request chega sem header ou com HMAC errado.
2. `RequireGatewayAuth` invoca `VerifyGatewayRequest`, recebe e.g. `GatewayAuthResult.InvalidSignature`.
3. Middleware responde **401** com body `{"error":"unauthorized"}` (sem detalhe do motivo).
4. Use case `RecordGatewayAuthFailure` publica em outbox evento `auth.failed` com `reason="gateway_invalid_signature"`, `request_id`, `client_ip`.
5. Métrica `identity_gateway_auth_total{result="invalid_signature"}` incrementa.
6. Span OTel marcado com `result=invalid_signature`.
7. **Sem** rate-limit por falha neste PRD (fica para A10/segunda onda).

## Plano de Rollout (cravado em ADR-005)

1. **Pré-deploy**: provisionar `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` no host. Configurar LLM cliente para enviar headers em modo "shadow" (assina mas servidor ainda não enforça).
2. **Deploy do app Go** com `RequireGatewayAuth` em modo **enforce**. Janela dupla (`current` + `next`) ativa desde o boot.
3. **Caddy** atualizado para strip dos headers externos no mesmo deploy.
4. **Validação E2E**: smoke test do plano-fonte seção 9, itens 1 e 5. Curl externo deve retornar 401.
5. **Rollback**: revert do deploy app + Caddy. LLM continua enviando headers (idempotente em ambiente sem enforcement).

Sem soft-launch parcial: ou o cutover é atômico ou não vai para produção.

## Mapeamento Requisito → Decisão (preenchido pela techspec)

| RF | ADR | Implementação | Teste |
|---|---|---|---|
| RF-01–RF-13 | ADR-001, 002, 003, 004, 005, 006, 007 | `domain/services/verify_gateway_request.go` + `infrastructure/http/server/middleware/require_gateway_auth.go` | `verify_gateway_request_test.go` + `require_gateway_auth_test.go` |
| RF-14–RF-18 | ADR-008 | migration + `application/auth/establish_principal.go` | `establish_principal_test.go` + integração HTTP |
| RF-19–RF-23 | governance global | task lint + revisão | CI |

## Critério de Aceitação Global

O PRD é considerado entregue quando:
- Todos os 23 RFs estão implementados e validados pelas 9 métricas M-01–M-09.
- 8 ADRs publicadas e referenciadas na techspec.
- Smoke test do plano-fonte seção 9 itens 1, 7 (sem o restante, que é fora de escopo) executa green.
- Gate M-09 verde no CI.
- Runbook de rotação testado em staging.
