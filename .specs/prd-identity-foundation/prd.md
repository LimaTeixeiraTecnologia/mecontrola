# Documento de Requisitos do Produto (PRD) — Identity Foundation

<!-- spec-version: 1 -->

## Visão Geral

O módulo `internal/identity/` é a fundação de identidade do MeControla (SaaS de controle financeiro via WhatsApp). Esta funcionalidade entrega o agregado `User`, os Value Objects de telefone e e-mail, o repositório canônico e a função pura `IsEntitled` — e elimina o drift entre o scaffold atual e as decisões do brainstorm `consolidacao-core`.

Hoje, `internal/identity/` contém apenas placeholders `doc.go` que declaram responsabilidades de "JWT/refresh, RBAC e audit de acesso" — **conflitantes** com a direção consolidada (canal WhatsApp é o autenticador; sem RBAC no MVP). Sem esta fundação, qualquer feature de cobrança (E2) ou onboarding (E3) nasce sem `User` para vincular `Subscription`, sem `WhatsAppNumber` normalizado para deduplicar, e replica normalização de telefone em três lugares diferentes.

Esta é a **fatia bloqueadora** do roadmap. Sem ela, E2 (billing-pipeline) e E3 (onboarding-magic-token) não podem rodar em paralelo. Resolvendo agora, o time desbloqueia frentes paralelas de cobrança e ativação na semana seguinte.

**Stakeholders impactados:**
- Time de engenharia (E2 e E3 dependem desta entrega).
- Suporte (precisa de mecanismo de soft delete e admin manual desde dia 1 para responder a LGPD).
- Produto (sem `User`, não há funil ponta-a-ponta para medir).

## Objetivos

- **OB-01:** Disponibilizar o agregado `User` e seus Value Objects (`WhatsAppNumber`, `Email`) prontos para uso por E2 e E3, sem reinvenção de normalização ou validação em outros módulos.
- **OB-02:** Materializar a função pura `IsEntitled(sub, now) bool` em `internal/identity/domain` com cobertura testável das 6 transições de status canônicas (`TRIALING`, `ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED`).
- **OB-03:** Eliminar o drift de governança: remover menção a RBAC/JWT em `doc.go`, `README.md` e `AGENTS.md` do módulo, alinhando com a Arquitetura Inegociável.
- **OB-04:** Garantir conformidade LGPD mínima via soft delete obrigatório em `users` (campo `deleted_at` + filtragem em todas as queries de leitura).
- **OB-05:** Estabelecer fronteiras de import enforçadas por `depguard` para que qualquer regressão de fronteira cross-module quebre CI.

### Métricas de sucesso

- **MS-01:** Cobertura de testes unitários **100%** em `WhatsAppNumber.New`, `Email.New` e `IsEntitled` (verificado por `go test -cover`).
- **MS-02:** Lint `depguard` verde no CI sem nenhuma violação dentro de `internal/identity/`.
- **MS-03:** Grep por `JWT`, `RBAC` ou `role` em `internal/identity/**/*.go` retorna apenas referências em testes ou em comentários de histórico explícito (`// removed: RBAC out of scope`).
- **MS-04:** Smoke E2E mínimo com Postgres real (testcontainers ou docker-compose) cobrindo: upsert por `whatsapp_number`, soft delete + filtragem, registro de histórico de número.
- **MS-05:** Build passa em pre-commit hook `ai-spec check-spec-drift .specs/prd-identity-foundation/tasks.md` quando a techspec e as tasks derivadas existirem.

## Histórias de Usuário

- **HU-01 — Engenheiro do módulo billing (E2):** Como engenheiro implementando o `BillingEventProcessor`, quero receber um `UserID` estável (UUID) ao chamar `UserRepository.UpsertByWhatsAppNumber(ctx, number)` para que eu possa vincular a `Subscription` recebida do webhook Kiwify sem inventar normalização de telefone no meu módulo.
- **HU-02 — Engenheiro do módulo onboarding (E3):** Como engenheiro implementando o handler `ATIVAR`, quero converter um número cru do WhatsApp em um `WhatsAppNumber` validado para que eu rejeite no construtor qualquer formato inválido (sem +55, sem 9, com caracteres extras) antes de tocar no banco.
- **HU-03 — Engenheiro do módulo billing (E2, entitlement):** Como engenheiro implementando o `EntitlementService.Check`, quero usar a função pura `IsEntitled(sub, now) bool` exportada por `identity/domain` para que eu tenha decisão determinística testável sem mock e sem depender do estado do meu cache.
- **HU-04 — Pessoa de suporte:** Como pessoa de suporte respondendo a um pedido de exclusão de dados (LGPD), quero ter um caminho técnico claro de soft delete (`UserRepository.SoftDelete(ctx, userID)`) que retire o usuário das queries normais para que eu cumpra a obrigação legal sem hard-delete imediato.
- **HU-05 — Administrador do produto:** Como administrador (uma das duas contas com `is_admin = true`), quero ter um marcador booleano em `users` para que eu seja reconhecido pelos painéis administrativos futuros sem depender de tabela de roles ou JWT.
- **HU-06 — Engenheiro de plataforma:** Como engenheiro responsável pela governança do repositório, quero que `depguard` impeça qualquer arquivo dentro de `internal/identity/domain` de importar `internal/identity/application` ou `internal/identity/infrastructure` para que regressões de fronteira hexagonal sejam detectadas no CI, não no code review.

## Funcionalidades Core

### F-01 — Agregado `User` em `internal/identity/domain`

**O que faz:** Define o tipo `User` com identificador UUID imutável, número de WhatsApp normalizado, e-mail opcional, flag `is_admin`, status (`ACTIVE`/`BLOCKED`/`DELETED`), timestamps de criação/atualização e marcador de soft delete. Encapsula invariantes (não pode existir sem `WhatsAppNumber` válido).

**Por que é importante:** É a entidade central que vincula assinatura, mensagens e auditoria. Sem agregado claro, cada módulo cria seu próprio "objeto user" e a coerência se perde.

**Como funciona em alto nível:** Construtor `NewUser(id, number, options...) (*User, error)` recebe VOs já validados; mutações passam por métodos de domínio (`MarkAsAdmin`, `SoftDelete`, `UpdateEmail`); sem setters mecânicos.

### F-02 — Value Object `WhatsAppNumber`

**O que faz:** Tipo imutável que representa um número de WhatsApp BR no formato E.164 (`+5511988887777`). Construtor único `NewWhatsAppNumber(input string) (WhatsAppNumber, error)` aceita formatos comuns (com/sem `+55`, com/sem `9` nono dígito, com formatação tipo `(11) 98888-7777`) e retorna erro determinístico para entrada inválida.

**Por que é importante:** Garante em tempo de compilação que nenhum número não normalizado entra em port, repo ou evento. Elimina a classe de bugs "telefone digitado diferente, dois usuários para a mesma pessoa". Centraliza a regra de normalização BR em um único lugar.

**Como funciona em alto nível:** Suporte exclusivamente BR no MVP (sem parâmetro `region`); valida tamanho (10/11/12/13 dígitos após limpeza), garante prefixo `55`, injeta o `9` nono dígito quando ausente em celular pós-2012, e formata como `+55DDXNNNNNNNN`. Falha com erro tipado para qualquer formato fora desse contrato.

### F-03 — Value Object `Email`

**O que faz:** Tipo imutável que representa um e-mail válido. Construtor único `NewEmail(input string) (Email, error)` aceita string, valida formato (mínimo: presença de `@` e domínio com TLD), normaliza para lowercase, e retorna erro tipado se inválido.

**Por que é importante:** Email é nullable no MVP (sem fluxo de login), mas quando aparecer (admin web, recibo Kiwify) já está validado e normalizado. Custo marginal hoje, ganho imediato amanhã.

**Como funciona em alto nível:** Tipo simples com método `String() string` e comparação por igualdade. Sem dependência externa de validação RFC completa — apenas as validações estritamente necessárias para uso interno.

### F-04 — Função pura `IsEntitled`

**O que faz:** Função pura `IsEntitled(sub *Subscription, now time.Time) bool` que decide se um usuário tem direito de uso **agora**, dada a sua subscription canônica. Sem I/O, sem cache, sem efeito colateral.

**Por que é importante:** É a fonte única de verdade da regra de acesso. Testável sem banco, sem Redis, sem mock. Reutilizada pelo `EntitlementService` (em E2) e por qualquer override administrativo futuro.

**Como funciona em alto nível:** Recebe a `Subscription` (do domínio) e o tempo atual. Para cada status canônico (`TRIALING`, `ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED`), aplica a regra de acesso descrita no decision-brief. `nil` retorna `false`. Cobertura 100% obrigatória.

> **Nota de fronteira:** O tipo `Subscription` consumido por `IsEntitled` é declarado em `internal/identity/domain` como **contrato mínimo** (status + period_end + grace_period_end). O agregado `Subscription` completo vive em `internal/billing/domain` (Épico E2) e implementa esse contrato. Isso preserva a regra "domain não importa application/infrastructure" sem criar import cíclico cross-module.

### F-05 — Port `UserRepository` + implementação Postgres

**O que faz:** Define a interface `UserRepository` em `internal/identity/application` com operações canônicas: `UpsertByWhatsAppNumber`, `FindByID`, `FindByWhatsAppNumber`, `SoftDelete`, `LinkNewNumber` (adiciona registro em histórico). Implementação Postgres em `internal/identity/infrastructure`.

**Por que é importante:** Permite que E2 e E3 dependam de uma abstração estável; troca de banco ou strategy de persistência fica isolada na infrastructure.

**Como funciona em alto nível:** Repo respeita soft delete em todas as queries de leitura (filtra `WHERE deleted_at IS NULL`); upsert é idempotente por `whatsapp_number`; insert em `user_whatsapp_history` registra mudanças de número quando aplicável.

### F-06 — Tabela `users` + tabela `user_whatsapp_history`

**O que faz:** Define o schema canônico em Postgres: `users (id, whatsapp_number, display_name, email, is_admin, status, created_at, updated_at, deleted_at)` com PK UUID e índice único em `whatsapp_number`; `user_whatsapp_history (id, user_id, number, active, linked_at, unlinked_at, reason)` para registrar mudanças.

**Por que é importante:** Schema é a contraparte persistente do agregado; sem ele, o repositório não funciona. Índice único garante invariante de unicidade no banco, não só em código.

**Como funciona em alto nível:** Migration Postgres versionada (ferramenta a definir na techspec — typicamente `migrate` ou `goose`). Schema inclui constraints mínimas e índices estritamente necessários para o MVP.

### F-07 — Eliminação de drift no scaffold

**O que faz:** Reescreve `internal/identity/domain/doc.go`, `internal/identity/README.md` e `internal/identity/AGENTS.md` para refletir as decisões do brainstorm: sem RBAC, sem JWT, sem sessions; responsabilidade declarada é "agregado `User`, VOs, port `UserRepository` e regra pura `IsEntitled`".

**Por que é importante:** O scaffold atual contradiz as decisões consolidadas. Sem corrigir, PRDs futuros vão herdar "RBAC" do README do módulo por inércia.

**Como funciona em alto nível:** Atualização textual nos três arquivos, removendo menção a JWT/RBAC/audit de acesso e adicionando referência ao bundle `consolidacao-core` como fonte das decisões. README ganha tabela de Value Objects e seção sobre `IsEntitled`.

### F-08 — Fronteiras enforçadas por `depguard`

**O que faz:** Atualiza `.golangci.yml` (regras `depguard`) para que `internal/identity/domain` não possa importar `application` nem `infrastructure`, e que `application` não possa importar `infrastructure` ou bibliotecas concretas de I/O.

**Por que é importante:** Documentar fronteiras sem enforcement automatizado leva a regressão silenciosa. Lint quebra build, code review livre de "ah, esqueci de checar import".

**Como funciona em alto nível:** Regras `depguard` por pacote com lista de imports permitidos/proibidos; CI executa `golangci-lint run` na pipeline padrão; violação = build vermelho.

## Requisitos Funcionais

- **RF-01:** O módulo `internal/identity/domain` DEVE expor o tipo `User` com identificador `UserID` baseado em UUID v4 imutável, construído via `NewUserID(string) (UserID, error)`.
- **RF-02:** O módulo `internal/identity/domain` DEVE expor o Value Object `WhatsAppNumber` imutável, construído exclusivamente via `NewWhatsAppNumber(input string) (WhatsAppNumber, error)`.
- **RF-03:** O construtor `NewWhatsAppNumber` DEVE aceitar entradas brasileiras nos formatos: dígitos sem código de país (10 ou 11 dígitos), com código de país `55` (12 ou 13 dígitos), com `+55`, com formatação humana (espaços, parênteses, hífen). Saída sempre no formato E.164 `+55DDXNNNNNNNN` (13 caracteres após o `+`).
- **RF-04:** O construtor `NewWhatsAppNumber` DEVE injetar o nono dígito (`9`) em celulares pós-2012 (entrada com 12 dígitos começando em `55`) e DEVE rejeitar com erro tipado entradas que não casem nenhum dos formatos suportados.
- **RF-05:** O módulo `internal/identity/domain` DEVE expor o Value Object `Email` imutável, construído via `NewEmail(input string) (Email, error)`, com validação mínima (presença de `@`, domínio com TLD) e normalização para lowercase.
- **RF-06:** O agregado `User` DEVE expor o campo `IsAdmin bool` mutável apenas via método `MarkAsAdmin(bool)`, sem tabela de roles ou estrutura de permissões.
- **RF-07:** O agregado `User` DEVE suportar soft delete via método `SoftDelete(at time.Time)` que define `DeletedAt`; usuários soft-deletados DEVEM ser invisíveis a queries normais de leitura.
- **RF-08:** A tabela `users` em Postgres DEVE conter as colunas: `id UUID PRIMARY KEY`, `whatsapp_number TEXT NOT NULL UNIQUE`, `display_name TEXT`, `email TEXT UNIQUE` (nullable), `is_admin BOOLEAN NOT NULL DEFAULT false`, `status TEXT NOT NULL DEFAULT 'ACTIVE'`, `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `deleted_at TIMESTAMPTZ` (nullable).
- **RF-09:** A tabela `users` DEVE ter índice único em `whatsapp_number` enforçado em nível de banco.
- **RF-10:** A tabela `user_whatsapp_history` em Postgres DEVE conter: `id UUID PRIMARY KEY`, `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `number TEXT NOT NULL`, `active BOOLEAN NOT NULL`, `linked_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `unlinked_at TIMESTAMPTZ` (nullable), `reason TEXT` (nullable).
- **RF-11:** O módulo `internal/identity/application` DEVE expor a interface `UserRepository` com operações: `UpsertByWhatsAppNumber(ctx, WhatsAppNumber) (*User, error)`, `FindByID(ctx, UserID) (*User, error)`, `FindByWhatsAppNumber(ctx, WhatsAppNumber) (*User, error)`, `SoftDelete(ctx, UserID) error`, `LinkNewNumber(ctx, UserID, WhatsAppNumber, reason string) error`.
- **RF-12:** A implementação Postgres de `UserRepository` em `internal/identity/infrastructure` DEVE filtrar `WHERE deleted_at IS NULL` em todas as queries de leitura, exceto em operação administrativa explícita (fora de escopo deste PRD).
- **RF-13:** O módulo `internal/identity/domain` DEVE expor a função pura `IsEntitled(sub *Subscription, now time.Time) bool` cobrindo as 6 transições canônicas (`TRIALING`, `ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED`) e o caso `sub == nil` (retorna `false`).
- **RF-14:** O tipo `Subscription` consumido por `IsEntitled` DEVE ser declarado em `internal/identity/domain` como contrato mínimo (campos `Status`, `CurrentPeriodEnd`, `GracePeriodEnd`) — não como agregado completo, que vive em `internal/billing/domain` (Épico E2).
- **RF-15:** Os arquivos `internal/identity/domain/doc.go`, `internal/identity/README.md` e `internal/identity/AGENTS.md` DEVEM ser atualizados para remover menção a "JWT/refresh, RBAC e audit de acesso" e declarar responsabilidade como "agregado `User`, Value Objects (`WhatsAppNumber`, `Email`), port `UserRepository`, função pura `IsEntitled`".
- **RF-16:** O arquivo `.golangci.yml` DEVE configurar regras `depguard` que (a) proíbem `internal/identity/domain` de importar `internal/identity/application` ou `internal/identity/infrastructure`; (b) proíbem `internal/identity/application` de importar `internal/identity/infrastructure`; (c) proíbem `internal/identity/infrastructure` de importar diretamente módulos `internal/billing/*`, `internal/onboarding/*` ou `internal/finance/*`.
- **RF-17:** Os testes unitários DEVEM cobrir 100% de `NewWhatsAppNumber`, `NewEmail` e `IsEntitled` (verificado por `go test -cover ./internal/identity/...`).
- **RF-18:** O teste de integração `internal/identity/infrastructure/postgres_integration_test.go` (ou equivalente) DEVE validar com Postgres real (testcontainers ou docker-compose): (a) upsert idempotente por `whatsapp_number`; (b) soft delete + invisibilidade em query de leitura; (c) registro em `user_whatsapp_history` quando `LinkNewNumber` é chamado.
- **RF-19:** Logs do módulo `identity` que referenciem `whatsapp_number` ou `email` DEVEM aplicar mascaramento (ex.: `+5511***88887777` → `+5511***888***777`) antes da emissão. Mascaramento é aplicado em ponto único reutilizável.

## Restrições Técnicas de Alto Nível

- **RT-01 — Stack obrigatória:** Go (versão declarada em `go.mod`) e Postgres. Sem novos serviços ou dependências de infraestrutura.
- **RT-02 — Arquitetura hexagonal canônica:** `domain/` puro (sem imports de I/O), `application/` com ports, `infrastructure/` com adapters. Imposta por `.golangci.yml` (`depguard`).
- **RT-03 — Sem RBAC, sem JWT, sem sessions:** decisão inegociável do bundle `consolidacao-core`. Qualquer demanda futura abre PRD novo precedido de brainstorm próprio.
- **RT-04 — 1 user = 1 subscription ativa:** regra inegociável; não suportar plano família/equipe nem multi-conta neste PRD.
- **RT-05 — LGPD:** soft delete obrigatório; mascaramento de PII em logs; janela de 30 dias para anonimização efetiva (mecanismo neste PRD; runbook completo em PRD futuro).
- **RT-06 — Identidade do canal:** o módulo `identity` confia no `from` autenticado pelo WhatsApp Business API; não implementa autenticação adicional na entrada de mensagens. Painéis administrativos web (quando existirem) usarão magic link por email — fora deste PRD.
- **RT-07 — Outbox vs events.Bus:** este PRD não emite eventos persistentes. Caso surja necessidade futura (ex.: `identity.user.soft_deleted` para reconciliação cross-module), seguir a regra dual definida em `AGENTS.md` seção "Outbox vs events.Bus" — outbox para durabilidade, Bus para fan-out intra-processo.
- **RT-08 — Convenções de qualidade:** seguir Uber Go Style Guide PT-BR (`.agents/skills/go-implementation/references/`) e governança transversal (`.claude/rules/governance.md`).
- **RT-09 — Performance:** as operações de leitura por `whatsapp_number` DEVEM ser servidas em < 5ms p99 (índice único garante isso); este PRD não impõe outras metas de latência.

## Fora de Escopo

- **FE-01:** Implementação do módulo `internal/billing/` (Épico E2) — `Subscription`, webhook ingress, `BillingEventProcessor`, `EntitlementService`, máquina de estados, `BillingProvider`, adapter Kiwify, reconciliação. Esta fundação **declara** o contrato `Subscription` mínimo consumido por `IsEntitled`, mas não implementa o agregado completo.
- **FE-02:** Implementação do módulo `internal/onboarding/` (Épico E3) — `SignupToken`, `/api/checkout-session`, thank-you page, handler `ATIVAR`, outreach, fallback E.164. Este PRD entrega `WhatsAppNumber` que será consumido por E3.
- **FE-03:** Caso de uso "trocar número de WhatsApp" via comando admin ou via fluxo WhatsApp. Este PRD entrega apenas schema + método `LinkNewNumber` no repositório; caso de uso completo (handler/CLI/fluxo) entra em PRD próprio quando demanda real aparecer.
- **FE-04:** Runbook completo de exclusão LGPD e job automatizado de anonimização para dados com `deleted_at > 30 dias`. Este PRD entrega apenas o mecanismo de soft delete + filtragem; runbook e job ficam para PRD pós-MVP.
- **FE-05:** Painel administrativo web e mecanismo de autenticação por magic link. `is_admin bool` está no schema, mas não há interface ou auth implementadas neste PRD.
- **FE-06:** Suporte multi-país no construtor `NewWhatsAppNumber`. MVP é exclusivamente BR; assinatura sem parâmetro `region`.
- **FE-07:** Override administrativo de entitlement (tabela `entitlement_overrides`). É decisão do bundle e entrará em E2 ou em PRD próprio de operações de suporte.
- **FE-08:** Métricas, alertas e dashboards de operação para o módulo identity. Telemetria básica (logs estruturados com mascaramento) está no escopo; instrumentação de métricas por módulo ficará para techspec ou PRD futuro quando E2/E3 também precisarem.
- **FE-09:** Migração do scaffold de `internal/finance/` — está intencionalmente fora deste PRD; `finance/` continua como controle pessoal e não recebe alteração aqui.
- **FE-10:** Reconciliação cross-module (Épico E4 pós-MVP). Não emite nem consome eventos persistentes.

## Suposições e Questões em Aberto

- **SQ-01 (suposição):** A ferramenta de migration Postgres a ser usada (`golang-migrate`, `goose`, `atlas` ou outra) será decidida na techspec; nenhuma é pressuposto deste PRD.
- **SQ-02 (suposição):** O suporte a testes de integração com Postgres real será via `testcontainers-go` ou `docker-compose`; decisão fica para techspec.
- **SQ-03 (suposição):** Os 1-2 administradores iniciais terão `is_admin = true` setado via script de seed ou migration; este PRD não define o mecanismo, apenas a coluna.
- **SQ-04 (questão em aberto, não bloqueante):** H7 do bundle `consolidacao-core` — propagação de `?s={token}` no webhook Kiwify — **não afeta** este PRD, mas precisa ser validado antes de E2 fechar.
- **SQ-05 (questão em aberto, não bloqueante):** H8 do bundle — volume MVP esperado — **não afeta** este PRD, mas pode demandar revisão de índices Postgres em escala > 5k subs.
- **SQ-06 (questão em aberto, não bloqueante):** H9 do bundle — tamanho do time — afeta apenas paralelismo de E2 e E3 após E1.
- **SQ-07 (suposição):** Logs estruturados (formato JSON) já estão padronizados em `internal/platform/observability` ou similar; este PRD apenas exige mascaramento de PII, não define formato.
- **SQ-08 (suposição):** O `User` agregado não publica Domain Events neste PRD; quando surgir caso real (ex.: `user.created` para fan-out de boas-vindas), evento entra em PRD próprio respeitando o contrato Outbox vs Bus do `AGENTS.md`.
