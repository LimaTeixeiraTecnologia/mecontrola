# Tarefa 6.0: Identidade canônica com resolve_path

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Garantir que a resolução do principal de um inbound WhatsApp funcione pelo caminho canônico de
`user_identities` e que, quando o fallback legado (`users.whatsapp_number`) for usado com sucesso,
o sistema crie de forma idempotente o vínculo canônico em `user_identities` e registre o caminho de
resolução (`identity` / `legacy` / `backfill`) na trilha de `auth_events`. A correção é forward-only,
dentro da mesma transação já aberta por `EstablishPrincipal`, sem uow aninhada e sem derrubar a
jornada por falha na criação do vínculo.

Escopo, decisões firmes e alternativas rejeitadas estão em `techspec.md` e em
`adr-006-identidade-canonica-resolve-path.md` desta pasta — **referenciar, não duplicar**.

<requirements>
- RF-20: resolução pelo caminho canônico de `user_identities`; ao usar o legado
  `users.whatsapp_number` com sucesso, criar/garantir de forma idempotente o vínculo canônico em
  `user_identities`.
- RF-21: a trilha de `auth_events` deve indicar se a resolução da principal foi por identidade
  canônica (`identity`), backfill (`backfill`) ou legado (`legacy`).
- Vínculo criado dentro de `EstablishPrincipal.resolvePrincipal`, reutilizando a `tx` já aberta pelo
  `uow.Do` da própria `EstablishPrincipal` (ADR-006 seção 1); **não** chamar
  `LinkChannelToUser.Execute` (abre uow própria — aninharia tx).
- `AuthResolvePath` como tipo fechado (DMMF state-as-type) com `IsValid`/`String`/`Parse*`; nunca
  string livre (ADR-006 seção 3).
- Coluna aditiva `resolve_path` em `auth_events` persistida via `authEventPayload.ResolvePath`
  (`omitempty`) e `newAuthEventOutbox` (ADR-006 seção 2).
- Idempotência e segurança sob concorrência garantidas pelo índice UNIQUE parcial
  `(channel, external_id) WHERE unlinked_at IS NULL`; sentinela `ErrUserIdentityAlreadyLinked`
  tratado como no-op.
- Zero comentários em Go de produção (R-ADAPTER-001.1); adapter/repo apenas persiste, lógica no use
  case; label `path` de `auth_resolve_path_total` permanece enum fechado (cardinalidade controlada).
</requirements>

## Dependências

- **DEPENDE da Tarefa 7.0**: a coluna `auth_events.resolve_path` (migration `000008`) deve existir
  antes que o repositório persista o valor. A migration aditiva e sua constraint
  `auth_events_resolve_path_check` são entregues pela Tarefa 7.0 (ver ADR-006 seção 2 e Plano de
  Implementação, passo 2).

## Subtarefas

- [ ] 6.1 Criar o tipo fechado `AuthResolvePath` em `internal/identity/domain` com constantes
  `AuthResolvePathIdentity` / `AuthResolvePathLegacy` / `AuthResolvePathBackfill` e métodos
  `IsValid()`, `String()` e `Parse*` (ADR-006 seção 3).
- [ ] 6.2 Alterar `lookupUserIDByWhatsApp` para retornar `AuthResolvePath` em vez de string solta;
  `auth_resolve_path_total` passa a consumir `path.String()` mantendo o label `path` como enum
  fechado.
- [ ] 6.3 Adicionar o método privado `ensureIdentityLink(ctx, tx, userID, channel, externalID, now)`
  em `EstablishPrincipal`, acionado somente no caminho `legacy`, usando
  `u.factory.UserIdentityRepository(tx).Insert`; tratar `ErrUserIdentityAlreadyLinked` como no-op;
  qualquer outro erro **não** aborta a jornada (log `warn` + continue) — ADR-006 seção 1.
- [ ] 6.4 Estender `authEventPayload` com `ResolvePath *string` (`omitempty`) e `newAuthEventOutbox`
  para receber e repassar o path; propagar o caminho resolvido ao evento `principal_established`.
- [ ] 6.5 Persistir `resolve_path` no repositório postgres de `auth_events` (coluna nova entregue
  pela Tarefa 7.0), sem introduzir lógica de domínio no adapter.

## Detalhes de Implementação

Ver `adr-006-identidade-canonica-resolve-path.md` (Decisão seções 1-4, Plano de Implementação passos
1, 3, 4, 5, 6 — o passo 2 da migration pertence à Tarefa 7.0) e `techspec.md` (RF-20, RF-21).
Pontos firmes que **não** devem ser reinterpretados:

- Reuso da `tx` já aberta por `EstablishPrincipal` (L148 `uow.Do`); alternativa (b)
  `LinkChannelToUser.Execute` rejeitada por aninhar transação.
- `Insert` do repositório permanece com semântica atual (sinaliza conflito via
  `ErrUserIdentityAlreadyLinked`); alternativa (c) `ON CONFLICT DO NOTHING` rejeitada por quebrar o
  contrato de `LinkChannelToUser`.
- `reason` de `auth_events` **não** pode carregar o path (`auth_events_reason_check` proíbe `reason`
  não-nulo quando `kind != 'failed'`); daí a coluna separada `resolve_path` — alternativa (a)
  rejeitada.
- `backfill` fica no enum sem emissor em runtime (D-05, sem job de backfill histórico); só
  `identity` ou `legacy` são emitidos.

## Critérios de Sucesso

- Resolução legada cria exatamente **1** linha ativa em `user_identities` (`unlinked_at IS NULL`) e
  grava `auth_events.resolve_path = 'legacy'`.
- 2ª chamada do mesmo usuário resolve por `user_identities` (`resolve_path = 'identity'`) **sem**
  criar 2ª linha em `user_identities`.
- 2 requests concorrentes do mesmo número resultam em **1** único vínculo ativo (perdedora no-op via
  sentinela), sem falhar a jornada.
- Falha não-sentinela em `ensureIdentityLink` gera `warn` + continue; o principal legado permanece
  resolvido.
- `go build`, `go vet`, `go test -race -count=1` verdes no escopo alterado; `golangci-lint run` limpo
  quando disponível; gates de governança (R-ADAPTER-001, R-DTO-VALIDATE-001) retornam limpos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — tipo fechado `AuthResolvePath` (state-as-type) e evento `auth`
  idempotente com trilha `resolve_path`.
- `mastra` — não aplicável ao core, mas `EstablishPrincipal` alimenta a resolução de principal do
  inbound do agente.
- `postgresql-production-standards` — `Insert` idempotente com UNIQUE parcial e persistência da
  coluna `resolve_path` conforme documentação oficial PostgreSQL.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite + mockery): caminho `legacy` chama `Insert` 1x;
  `ErrUserIdentityAlreadyLinked` ⇒ no-op sem falhar a jornada; caminho `identity` não chama `Insert`;
  `AuthResolvePath.IsValid`/`Parse` puros (válido/ inválido).
- [ ] Testes de integração (Postgres): vínculo criado + `auth_events` com o path correto;
  idempotência (2ª chamada ⇒ `resolve_path='identity'` sem 2ª linha); concorrência (2 requests ⇒ 1
  vínculo, sem falhar jornada).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Riscos

- **Não aninhar uow**: reutilizar a `tx` de `EstablishPrincipal` via
  `u.factory.UserIdentityRepository(tx)`; nunca chamar `LinkChannelToUser.Execute`.
- **Não falhar a jornada por erro de vínculo**: `ensureIdentityLink` nunca aborta o estabelecimento
  do principal — erro não-sentinela vira `warn` + continue.

## Arquivos Relevantes

- `internal/identity/application/usecases/establish_principal.go` — `resolvePrincipal`,
  `ensureIdentityLink`, `lookupUserIDByWhatsApp`.
- `internal/identity/application/usecases/auth_event_payload.go` — `authEventPayload.ResolvePath`,
  `newAuthEventOutbox`.
- `internal/identity/domain/` — novo tipo fechado `AuthResolvePath`.
- `internal/identity/infrastructure/postgres/` — repositório de `auth_events` persistindo
  `resolve_path` (coluna entregue pela Tarefa 7.0).
