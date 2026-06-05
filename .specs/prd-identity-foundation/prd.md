# Documento de Requisitos do Produto (PRD) — E1: Identity Foundation

<!-- spec-version: 1 -->

> **Origem:** Épico **E1 — `identity-foundation`** (`docs/epics/epic-01-identity-foundation.md`).
> **Discovery base:** `docs/discoveries/discovery-identity-entitlement.md`.
> **Posição no roadmap:** raiz do MVP. Bloqueia E2 (`billing-pipeline`) e E3 (`onboarding-magic-token`).
> **Próxima skill:** `create-technical-specification`.

> **Drift documental detectado:** `docs/epics/epic-01-identity-foundation.md` declara `status: prd_done` e cita `.specs/prd-identity-foundation/prd.md` como artefato existente, mas o diretório `.specs/` estava vazio no working tree. Por governança do `AGENTS.md`, working tree prevalece. Este PRD materializa o artefato pela primeira vez. Após aprovação, atualizar o `status` do épico para o estado real (`prd_done` continua válido apenas porque agora existe efetivamente).

---

## Visão Geral

O módulo **Identity** é a fundação de identidade canônica do MeControla. Antes dele, não há como o backend dizer "quem é este usuário do WhatsApp" nem responder, em uma única linha de código de domínio, "este usuário tem direito ao serviço agora?". Sem essa fundação, nenhum dos dois próximos épicos (cobrança em E2 e onboarding em E3) consegue ancorar `Subscription` em um `User` estável.

A funcionalidade entrega o agregado `User` com identidade estável por UUID (independente de troca de número de WhatsApp), Value Objects imutáveis para `WhatsAppNumber` (normalização E.164 BR) e `Email`, atributo opcional `display_name` (alimentado pelo `profile.name` do webhook WhatsApp Business 2026, com mascaramento de PII), soft delete por LGPD, histórico de números e a função pura `IsEntitled(sub, now) (bool, Reason)` — a única fonte de verdade sobre direito de acesso, reutilizada por E2 sem cross-module.

Em termos de produto, este épico **não** entrega valor visível ao usuário final do WhatsApp. Entrega previsibilidade arquitetural, fronteiras hexagonais aplicadas e a interface mínima que os épicos seguintes consomem. Portanto, o critério de "valor" é técnico-operacional: destravar E2 e E3 com superfície mínima, testável e production-proof, sem antecipar abstrações que ainda não têm dor real.

### Por que agora

1. **Bloqueador objetivo do roadmap.** `docs/epics/README.md` ordena E1 → (E2 ∥ E3) → E4. E2 e E3 dependem de `User.id` como FK e de `IsEntitled` como gate.
2. **Estado atual é placeholder.** `internal/identity/` contém apenas `module.go` vazio e diretórios sem implementação. Tudo precisa ser modelado do zero.
3. **Conflito documental ativo.** Materiais legados (`doc.go`, `README.md` do módulo, partes de `AGENTS.md`) mencionam RBAC/JWT, ferramental incompatível com a decisão de produto "WhatsApp é o canal de autenticação". Limpar agora evita orientação errada nos próximos épicos.

---

## Objetivos

1. **Destravar E2 e E3** entregando contrato mínimo e estável de `User` e `IsEntitled` consumível por billing e onboarding sem cross-module.
2. **Aplicar fronteiras hexagonais** no módulo `internal/identity/` (domain sem dependências de application/infrastructure), enforçadas por linter (`depguard`).
3. **Conformar com LGPD desde o dia 1** via soft delete obrigatório e mascaramento de PII em logs em ponto único reutilizável.
4. **Evitar abstrações prematuras**: sem RBAC, sem JWT, sem sessions, sem `is_admin`, sem multi-país. Construir a abstração na segunda dor, não na primeira.
5. **Reduzir drift documental**: remover menções a RBAC/JWT em `internal/identity/doc.go`, README do módulo e `AGENTS.md`.

### Métricas de sucesso (mensuráveis)

- **M-01:** cobertura de teste = 100% em `NewWhatsAppNumber`, `NewEmail` e `IsEntitled` (`go test -cover`). Para `IsEntitled`, a cobertura inclui obrigatoriamente o caso `sub == nil` e cada uma das 11 transições enumeradas em RF-12.
- **M-02:** zero violações `depguard` em `internal/identity/` no CI.
- **M-03:** zero ocorrências de `JWT`, `RBAC`, `role`, `is_admin` em `internal/identity/**/*.go` (exceto comentários históricos explícitos durante a limpeza, idealmente vazio).
- **M-04:** smoke E2E com Postgres real verde para upsert por `whatsapp_number`, soft delete invisível em queries subsequentes e gravação em `user_whatsapp_history`.
- **M-05:** `ai-spec check-spec-drift .specs/prd-identity-foundation/tasks.md` verde quando tasks for gerada.

---

## Histórias de Usuário

Como módulo backend, os "usuários" deste épico são desenvolvedores e os módulos `billing` (E2) e `onboarding` (E3) que consumirão a API interna.

- **Como** módulo `billing` (E2), **quero** receber um `User.ID` UUID estável associado a uma `Subscription`, **para que** mudanças de número de WhatsApp do usuário não quebrem a vinculação de cobrança.
- **Como** módulo `billing` (E2), **quero** chamar `IsEntitled(sub, now) (bool, Reason)` como função pura no domínio de identity, **para que** a decisão de gate seja determinística, testável sem I/O e reaproveitada sem replicar regra de negócio — e para que o `Reason` retornado alimente `Decision.Reason` no cache de entitlement e o copy de bloqueio nos handlers de WhatsApp sem espalhar a regra.
- **Como** módulo `onboarding` (E3), **quero** fazer upsert de `User` por `WhatsAppNumber` em transação, **para que** o handler `ATIVAR` ative o usuário atomicamente vinculando assinatura.
- **Como** time de produto, **quero** que toda exclusão de usuário seja soft delete, **para que** consigamos atender requisições LGPD sem perder histórico financeiro vinculado.
- **Como** time de operações, **quero** logs de identity sem PII em claro, **para que** o envio a sistemas de observabilidade (Datadog/Loki) não viole LGPD.
- **Como** desenvolvedor, **quero** que `internal/identity/domain` não compile se importar `application` ou `infrastructure`, **para que** acidentes arquiteturais sejam detectados antes do code review.

---

## Funcionalidades Core

### F-01. Agregado `User` com identidade estável

- **O que faz:** representa a identidade canônica de uma conta MeControla com PK UUID v4, número de WhatsApp normalizado, email opcional, `display_name` opcional, `status` (`ACTIVE | DELETED`) e timestamps padrão.
- **Por que é importante:** sem PK estável, qualquer troca de número de WhatsApp quebra FKs de subscription, histórico financeiro e métricas.
- **Como funciona em alto nível:** instanciação por construtor que exige VOs `WhatsAppNumber` e (opcional) `Email`. `display_name` é opcional, alimentado por integrações futuras (webhook WhatsApp Business `profile.name`, formulário de checkout, painel admin). Não existe atributo de autorização (RBAC/JWT/sessions/`is_admin`) no agregado, nem estado `BLOCKED` (qualquer suspensão administrativa abre PRD próprio).

### F-02. Value Object `WhatsAppNumber`

- **O que faz:** valor imutável que representa um número de WhatsApp já normalizado em E.164 BR (`+55DDD9NNNNNNNN`).
- **Por que é importante:** elimina ambiguidade entre "11988887777", "11 98888-7777", "(11) 98888-7777" e `+5511988887777`. Toda API interna trafega o VO, nunca `string`.
- **Como funciona em alto nível:** construtor normaliza, valida formato BR (DDD + 9 + 8 dígitos) e rejeita inputs inválidos. Comparação por igualdade de valor.

### F-03. Value Object `Email`

- **O que faz:** valor imutável de email com validação básica de formato.
- **Por que é importante:** email é opcional no MVP (recibos/admin futuro), mas precisa ser íntegro quando presente.
- **Como funciona em alto nível:** construtor valida e normaliza (lowercase). Igualdade por valor.

### F-04. Soft delete por LGPD

- **O que faz:** marca `deleted_at` em vez de remover linhas; todas as queries filtram `WHERE deleted_at IS NULL`.
- **Por que é importante:** atende requisições LGPD ("apague meus dados") preservando integridade referencial e histórico financeiro até janela de anonimização (E4).
- **Como funciona em alto nível:** método de domínio `MarkDeleted(now)` que muda `status` para `DELETED` e popula `deleted_at`.
- **Reanimação por janela temporal:** `UpsertByWhatsAppNumber` que encontra linha soft-deletada decide pelo intervalo `now - deleted_at`:
  - **≤ 30 dias** (antes da anonimização programada em E4): reanima a conta original — limpa `deleted_at`, recoloca `status = ACTIVE`, mantém o mesmo `UUID` e preserva FK histórica de `subscriptions` e finance. **PII é zerada e recoletada** (conformidade LGPD — "esquecimento" efetivo): `Email` e `display_name` são setados para `NULL` se o caller não fornecer; se fornecer, recebe os novos valores. A reanimação é registrada em audit log (mecanismo concreto fica para E4). Reanimação é tratada como "re-consentimento implícito do canal WhatsApp".
  - **> 30 dias**: cria conta nova com UUID novo. Histórico financeiro do UUID antigo permanece intocado (assumido como já anonimizado por E4). O número antigo segue arquivado em `user_whatsapp_history`.
- A janela espelha a regra de anonimização de E4 — antes da anonimização, dados ainda existem e reanimar é seguro; depois, identidade nova é coerente com o "esquecimento" efetivo.

### F-05. Histórico de números de WhatsApp

- **O que faz:** registra mudanças de número via tabela `user_whatsapp_history` (número, ativo, `linked_at`, `unlinked_at`, motivo).
- **Por que é importante:** usuário pode portar/trocar número e o histórico permite reconciliação e suporte.
- **Como funciona em alto nível:** entrega **só schema + método de repositório**. O comando de troca em si (handler/UC) **não** entra no MVP de E1.

### F-06. Port `UserRepository`

- **O que faz:** interface em `internal/identity/application` que descreve as operações necessárias de persistência (upsert por número, busca por ID, busca por número, soft delete, registro de histórico).
- **Por que é importante:** mantém domain agnóstico de infra; permite mock em testes e troca de implementação.
- **Como funciona em alto nível:** interface declarada no consumidor (regra Go R6). Implementação Postgres separada.

### F-07. Implementação Postgres do `UserRepository`

- **O que faz:** materializa o port em `internal/identity/infrastructure/repositories/postgres`.
- **Por que é importante:** entrega persistência real para o smoke E2E e para os módulos consumidores.
- **Como funciona em alto nível:** queries SQL diretas, sem ORM, respeitando filtro `deleted_at IS NULL`. Operações dentro de transações fornecidas via `database.FromContext(ctx)` (padrão já usado em `internal/platform/outbox`).

### F-08. Função pura `IsEntitled(sub, now) (bool, Reason)`

- **O que faz:** decide direito de acesso a partir do contrato mínimo de `Subscription` e do instante de referência, retornando também o motivo da decisão.
- **Por que é importante:** é o único ponto de decisão de entitlement do produto. Pura para ser determinística, trivialmente testável e reutilizada por E2. O retorno `Reason` é consumido por E2 (cache `Decision.Reason`) e pelos handlers de WhatsApp para gerar copy de bloqueio sem replicar regra (espelha `handler.copyForBlocked` na discovery).
- **Como funciona em alto nível:** cobre as transições de status (`TRIALING | ACTIVE | PAST_DUE | CANCELED_PENDING | EXPIRED | REFUNDED`) usando apenas `status`, `period_end` e `grace_period_end`, e o caso `sub == nil` → `(false, "no_subscription")`. Sem I/O, sem cache, sem efeito colateral. Razões esperadas: `"no_subscription" | "active" | "trialing" | "canceled_pending" | "past_due_grace" | "expired" | "refunded" | "past_due_no_grace"`.

### F-09. Contrato mínimo `Subscription` em `identity/domain`

- **O que faz:** define em `identity/domain` o tipo mínimo que `IsEntitled` consome (`status`, `period_end`, `grace_period_end`).
- **Por que é importante:** permite implementar `IsEntitled` sem cross-module e antes de E2 entregar `Subscription` completa.
- **Como funciona em alto nível:** struct ou interface no domínio com apenas os três campos necessários. E2 implementa um tipo concreto que satisfaz o contrato.

### F-10. Mascaramento de PII em logs

- **O que faz:** ponto único reutilizável que mascara `WhatsAppNumber`, `Email` e `display_name` antes de qualquer log estruturado.
- **Por que é importante:** webhooks e fluxos futuros trafegarão PII; sair com mascaramento já estabelecido evita débito imediato.
- **Como funciona em alto nível:** utilitário/método sobre os VOs e atributos (ex.: `WhatsAppNumber.Masked() string`, `Email.Masked() string`, helper para `display_name` retornando primeira letra + asteriscos) e/ou helper exportado por `internal/identity/domain` consumível por `slog.Attr`.

### F-11. Fronteiras hexagonais enforçadas (`depguard`)

- **O que faz:** regras em `.golangci.yml` que impedem `internal/identity/domain` de importar `application` ou `infrastructure`, e impedem `application` de importar `infrastructure`.
- **Por que é importante:** acidentes arquiteturais aparecem no lint local antes do PR.
- **Como funciona em alto nível:** lista de regras `depguard` adicionadas ao `.golangci.yml` com mensagens explícitas.

### F-12. Limpeza documental de RBAC/JWT

- **O que faz:** remove menções a RBAC, JWT, roles e sessions em `internal/identity/doc.go`, README do módulo (se existir) e seções aplicáveis de `AGENTS.md`.
- **Por que é importante:** documentação contradiz decisão de produto. Manter ambiguidade gera retrabalho nos próximos épicos.
- **Como funciona em alto nível:** edição cirúrgica nos arquivos afetados, preservando o restante.

---

## Requisitos Funcionais

> Numerados como `RF-nn` para rastreabilidade por `check-spec-drift`.

- **RF-01:** O agregado `User` deve usar `UUID v4` como chave primária, gerado no ponto de criação via `internal/platform/id.UUIDGenerator` ou equivalente.
- **RF-02:** O agregado `User` **não** deve conter atributo de autorização: sem `is_admin`, sem `roles`, sem `permissions`, sem `password_hash`, sem `session_token`.
- **RF-03:** `WhatsAppNumber` deve ser Value Object imutável; o construtor `NewWhatsAppNumber(raw string)` deve normalizar para E.164 BR (`+55DDD9NNNNNNNN`) e rejeitar input inválido com erro tipado.
- **RF-04:** APIs internas do módulo Identity **nunca** devem aceitar `string` cru para número de WhatsApp; apenas `WhatsAppNumber`.
- **RF-05:** `Email` deve ser Value Object imutável; o construtor `NewEmail(raw string)` deve validar formato básico (presença de `@` e domínio plausível) e normalizar para lowercase. `Email` é opcional no agregado `User` **no MVP de E1**. A decisão de tornar email obrigatório (driver: emissão de NFS-e via Kiwify em E2 exige email do tomador) é **deferida ao PRD do E2**, que pode propor: (a) manter opcional e abrir fluxo de coleta posterior via WhatsApp; (b) tornar obrigatório no checkout (E3) sem alterar o agregado; (c) tornar obrigatório no agregado via migration coordenada. Ver S-07.
- **RF-06:** A tabela `users` deve incluir a coluna `deleted_at TIMESTAMPTZ NULL` e a coluna `status TEXT NOT NULL` com valores permitidos `ACTIVE | DELETED`. Default `status = 'ACTIVE'`. **Estado `BLOCKED` não está no MVP**: qualquer demanda de suspensão administrativa (fraude, ordem judicial, suporte) abre PRD próprio que expande o enum em migration dedicada. **Invariante obrigatória:** `status = 'DELETED'` ⇔ `deleted_at IS NOT NULL`. Enforce em dois pontos: (a) método de domínio `MarkDeleted(now)` sempre seta os dois campos juntos; (b) `CHECK constraint` Postgres: `CHECK ((status = 'DELETED') = (deleted_at IS NOT NULL))`. A redundância é aceita conscientemente como ponto de extensão futuro (`BLOCKED`, `SUSPENDED`, etc.) — quando o enum crescer, o CHECK precisará evoluir junto.
- **RF-07:** Toda query de leitura de `users` deve filtrar `WHERE deleted_at IS NULL`. Hard delete é proibido.
- **RF-08:** A tabela `users` deve ter `UNIQUE` em `whatsapp_number` (somente para linhas não deletadas, via índice parcial ou equivalente) e `UNIQUE` parcial em `email` quando `email IS NOT NULL`.
- **RF-08-bis (display_name):** A tabela `users` deve incluir a coluna `display_name TEXT NULL`. O agregado `User` expõe `display_name` como atributo opcional. O campo é considerado PII e deve passar pelo helper de mascaramento em qualquer log estruturado. Captura por integração (webhook WhatsApp Business `profile.name`, checkout, painel admin) é responsabilidade dos épicos consumidores; E1 entrega apenas o slot persistido e o helper de mascaramento. **Política de atualização — first-write-wins:** `UpsertByWhatsAppNumber` só popula `display_name` quando o valor atual no banco é `NULL`; chamadas posteriores com novo valor preservam o nome original. Atualização explícita exige fluxo administrativo dedicado (fora do MVP). Esta política reconcilia G7+G10: webhook WhatsApp envia `profile.name` em toda mensagem mas só a primeira persiste; estabilidade nominal é preservada e logs evitam churn. Sem tabela de histórico de nomes neste épico.
- **RF-08-ter (reanimação por janela temporal):** `UpsertByWhatsAppNumber` que recebe número já existente em linha com `deleted_at IS NOT NULL` deve comportar-se assim: (a) se `now - deleted_at <= 30 dias`, reanima a conta original limpando `deleted_at`, recolocando `status = ACTIVE`, mantendo o mesmo `UUID` e **zerando obrigatoriamente `email` e `display_name` (SET NULL)** antes de aplicar o novo input — se o caller fornecer novos valores, eles substituem; se não fornecer, as colunas permanecem `NULL` (LGPD: PII anterior não retorna por inércia); (b) se `now - deleted_at > 30 dias`, cria conta nova com UUID novo. A janela espelha a anonimização programada em E4. Comportamento é coberto por teste E2E (CA-04 estendido).
- **RF-09:** Deve existir a tabela `user_whatsapp_history` com colunas mínimas `id UUID PK`, `user_id UUID FK ON DELETE CASCADE`, `number TEXT`, `active BOOLEAN`, `linked_at TIMESTAMPTZ`, `unlinked_at TIMESTAMPTZ NULL`, `reason TEXT NULL`. Índices: `(user_id, active)` e `(number)`.
- **RF-10:** O port `UserRepository` deve ser declarado em `internal/identity/application` e cobrir, no mínimo: `UpsertByWhatsAppNumber(ctx, user) (User, error)`, `FindByID(ctx, id) (User, error)`, `FindByWhatsAppNumber(ctx, number) (User, error)`, `MarkDeleted(ctx, id, now) error`, `AppendWhatsAppHistory(ctx, userID, entry) error`. Erros tipados para "não encontrado" e violação de unicidade. **Semântica de idempotência:** `UpsertByWhatsAppNumber` é "touch garantido" — toda invocação atualiza `updated_at`, mesmo quando os demais campos não mudam. Justificativa: handler `ATIVAR` (E3) é reentrante e queremos `updated_at` representando "última interação do usuário"; quem precisar de dirty-tracking real deve usar comparação explícita pré-chamada ou eventos de domínio (não há no E1).
- **RF-11:** A implementação Postgres de `UserRepository` deve residir em `internal/identity/infrastructure/repositories/postgres` e usar a transação corrente via `database.FromContext(ctx)` (padrão do projeto).
- **RF-12:** `IsEntitled(sub Subscription, now time.Time) (bool, Reason)` deve ser função pura, declarada no pacote de domínio de identity, sem I/O, sem cache e sem efeito colateral. Deve cobrir, no mínimo, estes casos retornando o par `(entitled, reason)`:
  - `sub == nil` → `(false, "no_subscription")`
  - `ACTIVE` com `period_end > now` → `(true, "active")`
  - `ACTIVE` com `period_end <= now` → `(false, "expired")`
  - `TRIALING` com `period_end > now` → `(true, "trialing")`
  - `TRIALING` com `period_end <= now` → `(false, "expired")`
  - `PAST_DUE` com `grace_period_end > now` → `(true, "past_due_grace")`
  - `PAST_DUE` com `grace_period_end <= now` ou nil → `(false, "past_due_no_grace")`
  - `CANCELED_PENDING` com `period_end > now` → `(true, "canceled_pending")`
  - `CANCELED_PENDING` com `period_end <= now` → `(false, "expired")`
  - `EXPIRED` → `(false, "expired")`
  - `REFUNDED` → `(false, "refunded")`

  `Reason` é um tipo do domínio de identity (`type Reason string` com constantes nomeadas). Consumido por E2 para popular `Decision.Reason` no cache de entitlement e pelos handlers de WhatsApp para gerar copy de bloqueio.
- **RF-13:** O contrato `Subscription` consumido por `IsEntitled` deve ser declarado em `internal/identity/domain` e expor apenas `status`, `period_end`, `grace_period_end`. E2 fornecerá implementação concreta que satisfaça o contrato.
- **RF-14:** Deve existir helper reutilizável para mascaramento de PII em logs cobrindo `WhatsAppNumber`, `Email` e `display_name` (ex.: `WhatsAppNumber.Masked()` que retorna `+55 11 9****-7777`, `Email.Masked()` que retorna `j***@example.com`, mascaramento de `display_name` que retorna primeira letra + asteriscos, ex.: `J****`). Todo log estruturado do módulo Identity deve usar o helper.
- **RF-15:** `.golangci.yml` deve declarar regras `depguard` que impeçam: (a) imports de `application` e `infrastructure` por `internal/identity/domain`; (b) imports de `infrastructure` por `internal/identity/application`. Mensagens de violação devem citar a fronteira hexagonal.
- **RF-16:** Migration SQL deve criar `users` e `user_whatsapp_history` via runner `golang-migrate` já presente em `cmd/migrate`. Numeração contínua a partir do último arquivo em `migrations/` (próximo número após `000001_outbox_events`).
- **RF-17:** `internal/identity/doc.go` (se existir com conteúdo divergente) e qualquer README do módulo Identity devem ser atualizados para refletir a ausência de RBAC/JWT/sessions/`is_admin`. Trechos correspondentes em `AGENTS.md` devem ser ajustados.
- **RF-18:** O módulo deve expor um construtor `NewIdentityModule(...)` em `internal/identity/module.go` seguindo o Padrão Obrigatório de Módulo de `AGENTS.md`, ainda que o MVP de E1 entregue apenas dependências internas (repositório, helper de PII). Routers, jobs e consumers ficam ausentes nesta versão (entram em E2/E3).

---

## Restrições Técnicas de Alto Nível

> Apenas restrições e considerações. Soluções de desenho ficam para a Especificação Técnica.

### Arquitetura e governança
- Padrão Obrigatório de Módulo de `AGENTS.md` (camadas `domain`, `application`, `infrastructure`).
- Fluxo `infrastructure → application → domain` em runtime, sem inversão.
- Regras Go R0–R7 (`CLAUDE.md`): sem `init()`, sem `panic` em produção, sem `var _ Interface = (*Type)(nil)`, sem abstração de tempo (usar `time.Now().UTC()` inline), `context.Context` em todo IO, interface no consumidor.
- Comunicação cross-module apenas por contrato explícito (interface no consumidor) ou evento/outbox; este épico não publica eventos (não há side-effect a propagar).

### Dados
- Postgres como armazenamento canônico (já em uso no projeto).
- Migrations via `golang-migrate` (runner em `cmd/migrate` já configurado).
- Soft delete obrigatório; hard delete proibido.
- BR-only no MVP: validação de número aceita apenas E.164 BR (`+55…`). Multi-país é fora de escopo.

### Domínio
- `User.ID` é UUID v4 estável; nunca derivado de número de WhatsApp.
- `WhatsAppNumber` e `Email` são Value Objects imutáveis.
- `IsEntitled` é pura.
- Regra global "1 user = 1 subscription ativa" — qualquer demanda de família/equipe abre brainstorm + PRD próprio.

### Segurança e privacidade (LGPD)
- PII (telefone, email) deve ser mascarada em logs antes de chegar em qualquer sink de observabilidade.
- Soft delete sustenta o requisito "direito ao esquecimento" no MVP; anonimização programada após 30 dias é responsabilidade de E4.
- Sem RBAC, sem JWT, sem sessions, sem `is_admin` — WhatsApp é o canal de autenticação garantido pelo provedor (Meta).

### Integração com outros módulos
- E2 (`billing-pipeline`) consumirá `User.ID` como FK em `subscriptions` e chamará `IsEntitled` por meio do contrato `Subscription` mínimo.
- E3 (`onboarding-magic-token`) executará upsert por `WhatsAppNumber` em transação atômica (`SELECT ... FOR UPDATE`) na ativação.
- Este épico **não** entrega `EntitlementService` (cache + invalidação) — esse serviço pertence a E2 e apenas reutiliza `IsEntitled`.

### Operação
- Sem métricas, alertas ou dashboards no MVP de E1: logs estruturados (`log/slog`) com mascaramento de PII são suficientes.
- Smoke E2E deve rodar contra Postgres real (testcontainers ou docker-compose) na pipeline CI.

### Reuso do que já existe no working tree
- `internal/platform/id.UUIDGenerator` para geração de UUID v4.
- `internal/platform/outbox` como referência de uso de `database.FromContext(ctx)` em repositórios transacionais.
- `cmd/migrate/migrate.go` para aplicação de migrations.
- `cmd/server/server.go` e `cmd/worker/worker.go` como pontos onde `NewIdentityModule(...)` será futuramente wirado (a wiring efetiva pode ficar parcial nesta versão, já que não há routers/jobs/consumers para registrar).

---

## Critérios de Aceite

- **CA-01:** Cobertura de teste = 100% (linhas e branches) em `NewWhatsAppNumber`, `NewEmail` e `IsEntitled`, verificada por `go test -cover ./internal/identity/...`. Para `IsEntitled`, os testes parametrizados cobrem todos os pares `(status, period_end/grace_period_end)` enumerados em RF-12 e o caso `sub == nil`.
- **CA-02:** Lint `depguard` verde no CI, sem nenhuma violação em `internal/identity/`.
- **CA-03:** `grep -RInE "JWT|RBAC|\\brole\\b|is_admin" internal/identity/` retorna no máximo comentários históricos explicitamente justificados (idealmente vazio).
- **CA-04:** Smoke E2E com Postgres real (testcontainers ou docker-compose) cobre: (a) upsert por `whatsapp_number` cria User na primeira execução e atualiza na segunda; (b) `MarkDeleted` torna o usuário invisível nas leituras subsequentes; (c) mudança de número registra entrada ativa em `user_whatsapp_history` e desativa a anterior; (d) upsert de número soft-deletado há ≤ 30 dias reanima a conta (mesmo UUID, `deleted_at = NULL`, `status = ACTIVE`) com `email` e `display_name` zerados antes da recoleta; (e) upsert de número soft-deletado há > 30 dias cria conta nova com UUID distinto; (f) `display_name` first-write-wins — segunda chamada com nome diferente preserva o primeiro nome persistido; (g) upsert sem mudança de campos atualiza `updated_at`; (h) tentativa de violar invariante `status='DELETED' ⇔ deleted_at IS NOT NULL` via SQL direto é rejeitada pelo CHECK constraint.
- **CA-05:** `ai-spec check-spec-drift .specs/prd-identity-foundation/tasks.md` passa quando o arquivo de tasks for gerado na skill seguinte.

---

## Riscos

- **R-01 (baixo):** ferramenta de migration. `golang-migrate` já está integrado em `cmd/migrate`; manter como padrão. Mitigação: nenhuma ação adicional, apenas reaproveitar o runner.
- **R-02 (médio):** smoke E2E exige Docker disponível no CI. Mitigação: confirmar que a pipeline (esteira) suporta testcontainers ou docker-compose antes de fechar a techspec; se não suportar, alternativa é executar smoke E2E em job manual local até a esteira receber Docker.
- **R-03 (médio):** drift documental do épico. `docs/epics/epic-01-identity-foundation.md` aponta para PRD inexistente. Mitigação: após este PRD ser aprovado, atualizar o arquivo de épico para refletir a realidade (`status` continua `prd_done`, mas agora coerente).
- **R-04 (baixo):** Conflito com a discovery sobre `is_admin`. `docs/discoveries/discovery-identity-entitlement.md` recomenda `is_admin bool`. Override explícito do usuário em 2026-06-05 revoga essa recomendação. Mitigação: este PRD declara `is_admin` proibido (RF-02 e Restrições); discovery permanece como histórico, sem alteração.
- **R-05 (baixo):** ambiguidade sobre identificadores BR no construtor de `WhatsAppNumber` (com/sem `9` na frente do celular, números fixos). Mitigação: a techspec deve fixar as regras (DDD + 9 + 8 dígitos para celular BR) e cobrir casos limítrofes com testes parametrizados.
- **R-06 (médio):** acoplamento de prazo entre RF-08-ter (reanimação ≤ 30 dias) e a janela de anonimização que E4 vai implementar. Se E4 escolher janela diferente (ex.: 60d), comportamento de reanimação fica incoerente — anonimização ainda não ocorreu mas E1 já cria UUID novo. Mitigação: techspec parametriza a janela (constante em domínio, default 30 dias) e E4 reafirma o valor; alterações futuras exigem migration coordenada.
- **R-07 (baixo):** `display_name` (RF-08-bis) entregue sem caller no MVP de E1; risco de coluna ociosa até E3. Mitigação: aceito explicitamente em S-06; primeiro caller é E3 (webhook ou checkout).
- **R-08 (médio):** invariante `status='DELETED' ⇔ deleted_at IS NOT NULL` (RF-06) pode ser violada por scripts ad-hoc ou migrations futuras descuidadas. Mitigação: CHECK constraint + teste CA-04(h) garantindo que SQL fora do método de domínio falha; novo estado no enum exige atualização coordenada do CHECK.
- **R-09 (baixo):** "touch garantido" em `UpsertByWhatsAppNumber` (RF-10) pode mascarar mudanças reais para quem usar `updated_at` como sinal de "campo mudou". Mitigação: documentar em S-09-bis; quem precisar de dirty-tracking real consulta antes de chamar ou aguarda eventos de domínio (não há no E1).

---

## Dependências e Bloqueios

- **Bloqueia:** E2 (`billing-pipeline`), E3 (`onboarding-magic-token`).
- **Bloqueado por:** nenhum (raiz do roadmap).
- **Pré-requisitos não técnicos:** nenhum.
- **Reuso obrigatório do working tree:**
  - `internal/platform/id` (geração de UUID v4).
  - `cmd/migrate` (runner de migrations).
  - Convenção `database.FromContext(ctx)` para acesso à transação (padrão do projeto, visível em `internal/platform/outbox/storage_postgres.go`).

---

## Fora de Escopo

- **Subscription completa, webhook Kiwify, `EntitlementService` (cache Redis + invalidação), máquina de estados de cobrança** → tudo isso é E2.
- **`SignupToken`, endpoint `/api/checkout-session`, thank-you page, handler `ATIVAR`, deep link `wa.me`, job de outreach** → tudo isso é E3.
- **Comando administrativo "trocar número de WhatsApp"** (UC + handler). Apenas o mecanismo (schema + método repo) entra no MVP.
- **Runbook LGPD de exclusão definitiva** e **job de anonimização após 30 dias** → E4 (`reconciliation-hardening`).
- **Painel admin web**, **magic link por email**, **override administrativo `entitlement_overrides`** → pós-MVP.
- **Multi-país de `WhatsAppNumber`** → pós-MVP.
- **Métricas, alertas e dashboards** (Prometheus, Grafana) → E4.
- **RBAC, JWT, sessions, `is_admin`** → não está no roadmap; abrir brainstorm + PRD próprio se houver demanda real no futuro.
- **Side-effects assíncronos** (publicação de evento `user_created` etc.) → não há consumidor no MVP; introduzir só quando E2/E3 demandarem.
- **Reabilitação de usuário soft-deletado** → fluxo administrativo fora do MVP.

> Riscos de implementação técnica detalhada (índices Postgres específicos, escolha entre `pgx`/`database/sql`, formato exato do helper de mascaramento) serão tratados na Especificação Técnica.

---

## Suposições e Questões em Aberto

### Suposições assumidas
- **S-01:** `golang-migrate` permanece como tool de migration; nenhuma migração para `goose` ou `atlas` será proposta nesta versão.
- **S-02:** A pipeline CI atual suporta (ou suportará antes do merge de E1) execução de testcontainers/docker-compose para o smoke E2E. Se não suportar, a techspec proporá fallback.
- **S-03:** A regra de normalização E.164 BR aceita exatamente DDD (2 dígitos) + `9` (celular) + 8 dígitos. Números fixos (sem `9`) **não** são aceitos no MVP — a base de usuários é celular WhatsApp por definição.
- **S-04:** Não há necessidade de publicar evento `user_created` ou similar no outbox neste épico; E2 e E3 farão lookup direto por `whatsapp_number` ou `user_id` em runtime.
- **S-05:** A janela de reanimação de 30 dias (RF-08-ter) é coerente com a janela de anonimização definida em E4. Se E4 mudar a janela, este PRD deve ser revisitado para manter o alinhamento.
- **S-06:** `display_name` (RF-08-bis) é entregue como slot persistido sem caller real no E1. O primeiro caller será o webhook do WhatsApp Business em E3 (mensagem de entrada) ou o checkout em E3 (formulário). Captura zero no MVP de E1 é aceitável.
- **S-07:** A obrigatoriedade do `Email` no agregado `User` é decisão deferida ao PRD do E2 (driver: emissão de NFS-e pela Kiwify exige email do tomador). E1 mantém email opcional. Caso E2 escolha tornar obrigatório no agregado, exige migration coordenada e backfill (default razoável: usar email de checkout Kiwify). Caso opte por obrigatório apenas no fluxo de checkout, E1 segue intocado.
- **S-08:** A redundância entre `status` e `deleted_at` (RF-06) é aceita por extensibilidade futura (`BLOCKED`/`SUSPENDED`/etc.). A invariante é defendida pelo CHECK constraint e pelo método de domínio `MarkDeleted(now)`. Toda evolução do enum exige atualizar o CHECK na mesma migration que adiciona o novo estado.
- **S-09:** A política `display_name = first-write-wins` (RF-08-bis) revoga a decisão anterior de last-write-wins (G7). Webhooks WhatsApp Business 2026 podem enviar `profile.name` em toda mensagem, mas apenas a primeira persiste. Atualização explícita do nome é fluxo administrativo fora do MVP. Se E3 perceber necessidade de UX para o usuário "renomear-se via WhatsApp", abre PRD próprio.

### Questões em aberto (resolver na techspec)
- **Q-01:** Formato exato do helper de mascaramento de PII — método no VO (`Masked()`) vs. helper externo (`mask.WhatsApp(v)`). Decidir na techspec.
- **Q-02:** `Subscription` mínima em `identity/domain` será **interface** (mais flexível para E2) ou **struct** (mais simples para `IsEntitled`)? Decidir na techspec, com viés para interface se E2 já estiver claro.
- **Q-03:** Tratamento de unicidade de `email` quando `email` for `NULL`: índice parcial `UNIQUE (email) WHERE email IS NOT NULL` é o padrão; confirmar na techspec.
- **Q-04:** Estrutura de erros tipados do repositório (`ErrUserNotFound`, `ErrWhatsAppNumberInUse`) — pacote e nomes definitivos a fixar na techspec.
- **Q-05:** `NewIdentityModule(...)` exporta o quê para `cmd/server` e `cmd/worker` no MVP de E1 (sem routers/jobs/consumers a registrar)? Decidir na techspec se a wiring efetiva fica como TODO ou se já injeta repo no contexto para futuros consumidores.
- **Q-06:** Tipo `Reason` em RF-12 — `type Reason string` com constantes nomeadas vs `type Reason int` com `String()` vs sealed iota. Decidir na techspec, com viés para `string` por interoperabilidade direta com cache JSON do `EntitlementService` em E2.
