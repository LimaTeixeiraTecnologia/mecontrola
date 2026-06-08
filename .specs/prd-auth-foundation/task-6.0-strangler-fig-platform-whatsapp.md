# Tarefa 6.0: Strangler Fig atômico — internal/platform/whatsapp + migra onboarding + ADR-002

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

**Esta tarefa MUST ser entregue em PR único atômico (RF-28).** Cria o pacote compartilhado `internal/platform/whatsapp` com `signature/`, `payload/` e `dedup/`, reescreve `internal/onboarding` para consumir o novo pacote, **deleta** os arquivos antigos no mesmo PR e atualiza `prd-onboarding-magic-token` com spec-version bump documentando a migração.

<requirements>
- RF-04: migração via Strangler Fig sem regressão observável do onboarding.
- RF-05: HMAC SHA-256 com `secretCurrent+secretNext` e status `valid/invalid/rotated` mantidos.
- RF-28: PR único atômico — cria novo + migra onboarding + deleta antigo.
- RF-31: bump de spec-version em `.specs/prd-onboarding-magic-token/prd.md` referenciando ADR-002.
</requirements>

## Subtarefas

- [ ] 6.1 Criar `internal/platform/whatsapp/signature/raw_body_buffer.go` (migrado de onboarding, mesma lógica) + `_test.go` (suíte movida).
- [ ] 6.2 Criar `internal/platform/whatsapp/signature/hmac.go` (migrado de `meta_signature.go`) + `_test.go` (suíte movida intacta).
- [ ] 6.3 Criar `internal/platform/whatsapp/signature/compose.go` com `func Compose(secretCurrent, secretNext, metrics) func(http.Handler) http.Handler` que monta `RawBodyBuffer ∘ HMACMiddleware` na ordem correta + `compose_test.go` validando ordem.
- [ ] 6.4 Criar `internal/platform/whatsapp/payload/types.go` (migrado de `meta_models.go`) + `parser.go` com `ExtractFirstMessage(raw) (Message, error)` (migrado de `whatsapp_inbound_handler.go::extractFirstMessage`) + `MaskMobile` helper + `_test.go`.
- [ ] 6.5 Criar `internal/platform/whatsapp/dedup/repository.go` (porta `MessageRepository.InsertIfAbsent(ctx, wamid) (bool, error)`) + `postgres/repository.go` (adapter Postgres reusando tabela `meta_processed_messages`).
- [ ] 6.6 Reescrever `internal/onboarding/infrastructure/http/server/router.go` para usar `whatsapp.signature.Compose(...)`.
- [ ] 6.7 Reescrever `internal/onboarding/infrastructure/http/server/handlers/whatsapp_inbound_handler.go` para consumir `whatsapp.payload.Parser` e `whatsapp.dedup.MessageRepository`.
- [ ] 6.8 **Deletar** arquivos antigos no MESMO COMMIT: `internal/onboarding/.../middleware/meta_signature.go` + `_test.go`, `raw_body_buffer.go`, `handlers/meta_models.go`.
- [ ] 6.9 Atualizar `.specs/prd-onboarding-magic-token/prd.md` (spec-version bump) com nota referenciando ADR-002 e listando arquivos migrados.
- [ ] 6.10 Confirmar `.specs/prd-auth-foundation/adr-002-strangler-fig-onboarding-whatsapp.md` está consistente com a execução (já existe).
- [ ] 6.11 Criar `task onboarding:smoke` em `Taskfile.yml` se ausente, cobrindo fluxo ATIVAR end-to-end com HMAC real.

## Detalhes de Implementação

Ver ADR-002 (`adr-002-strangler-fig-onboarding-whatsapp.md`) para escopo completo + plano de implementação + critérios de aceitação. Ver techspec `## Componentes modificados` para lista de arquivos.

## Critérios de Sucesso

- PR único atômico com todos os movimentos (`git mv` preserva blame quando possível).
- Suíte completa de testes verde (unit + integration de onboarding + nova suíte de whatsapp).
- `task onboarding:smoke` verde em staging.
- Métrica `meta_signature_status_total{status='valid'}` sobe ao mesmo ritmo pré-migração após deploy.
- Zero regressão em `onboarding_activation_total`.
- Spec-version bump no PRD de onboarding aplicado no mesmo PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Suíte de testes movida intacta (HMAC, raw_body_buffer, parser)
- [ ] Compose() testado validando ordem fixa
- [ ] Integration test do webhook de onboarding pré-merge
- [ ] `task onboarding:smoke` pós-deploy em staging

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/whatsapp/signature/raw_body_buffer.go` + `_test.go` (criar — movido)
- `internal/platform/whatsapp/signature/hmac.go` + `_test.go` (criar — movido)
- `internal/platform/whatsapp/signature/compose.go` + `_test.go` (criar)
- `internal/platform/whatsapp/payload/types.go` + `parser.go` + `_test.go` (criar — movido)
- `internal/platform/whatsapp/dedup/repository.go` (criar — porta)
- `internal/platform/whatsapp/dedup/postgres/repository.go` (criar — adapter)
- `internal/onboarding/infrastructure/http/server/middleware/meta_signature.go` + `_test.go` (**deletar**)
- `internal/onboarding/infrastructure/http/server/middleware/raw_body_buffer.go` (**deletar**)
- `internal/onboarding/infrastructure/http/server/handlers/meta_models.go` (**deletar**)
- `internal/onboarding/infrastructure/http/server/handlers/whatsapp_inbound_handler.go` (reescrever)
- `internal/onboarding/infrastructure/http/server/router.go` (reescrever)
- `.specs/prd-onboarding-magic-token/prd.md` (atualizar — spec-version bump)
- `.specs/prd-auth-foundation/adr-002-strangler-fig-onboarding-whatsapp.md` (já existe — confirmar)
- `Taskfile.yml` (atualizar — `onboarding:smoke` se ausente)
