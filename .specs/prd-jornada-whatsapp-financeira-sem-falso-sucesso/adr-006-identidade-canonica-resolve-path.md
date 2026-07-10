# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Vínculo canônico idempotente de identidade e coluna resolve_path em auth_events
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** time de identidade / plataforma
- **Relacionados:** PRD (RF-20, RF-21), `techspec.md`, US-001

## Contexto

`EstablishPrincipal` (`internal/identity/application/usecases/establish_principal.go`) resolve o
principal de um inbound WhatsApp por dois caminhos, ambos dentro de uma única transação
(`uow.Do`, L148):

1. **Canônico (path `identity`):** consulta `user_identities` via
   `UserIdentityRepository.TryFindActive`.
2. **Legado (path `legacy`):** fallback em `users.whatsapp_number` via
   `lookupUserIDByWhatsApp` (L222), acionado quando não há vínculo em `user_identities`.

Ao resolver pelo caminho legado, o use case retorna o principal, mas **não** cria o vínculo
canônico em `user_identities` e **não** registra em `auth_events` qual caminho foi usado. O único
sinal do caminho é a métrica `auth_resolve_path_total` (label `path`: `identity` / `legacy` /
`miss`), volátil e não auditável por usuário.

Consequências do gap:

- Usuários legados permanecem indefinidamente no fallback, sem migração forward.
- Não há trilha de auditoria persistente indicando por qual caminho cada estabelecimento de
  principal ocorreu.

Restrições relevantes do código e do schema:

- `LinkChannelToUser.Execute` (L54) abre a **própria** `uow.Do` (L106); chamá-lo de dentro de
  `resolvePrincipal` aninharia transações.
- `UserIdentityRepository.Insert` (postgres, L169-203) mapeia `UniqueViolation` para o sentinela
  `application.ErrUserIdentityAlreadyLinked`.
- `user_identities` tem índice UNIQUE parcial `(channel, external_id) WHERE unlinked_at IS NULL`,
  garantindo unicidade do vínculo ativo mesmo sob concorrência.
- `auth_events` (migration `000001`, L158-191): coluna `reason TEXT NULL` com a constraint
  `auth_events_reason_check` (L173-190) que **exige** `reason IN (<enum de falha>)` quando
  `kind = 'failed'` e **exige** `reason IS NULL` quando `kind != 'failed'`. Logo, um evento
  `principal_established` com `reason` não-nulo **viola** o CHECK — a coluna `reason` não pode
  carregar o caminho de resolução.

Decisão de produto D-05: criar o vínculo canônico forward (idempotente) no fallback legado e
registrar o caminho em `auth_events`, **sem** job de backfill histórico.

## Decisão

### 1. Vínculo canônico na mesma transação (sem uow aninhada)

Criar o vínculo dentro de `EstablishPrincipal.resolvePrincipal`, reutilizando a `tx` já aberta pelo
`uow.Do` da própria `EstablishPrincipal`. **Não** chamar `LinkChannelToUser.Execute` (evita
transação aninhada, alternativa (b) rejeitada).

Novo método privado:

```
ensureIdentityLink(ctx, tx, userID, channel, externalID, now)
```

Comportamento:

- Usa `u.factory.UserIdentityRepository(tx)` e chama `InsertIfAbsent` (não `Insert` direto).
- `InsertIfAbsent` executa a inserção dentro de um `SAVEPOINT` e, ao capturar
  `pgerrcode.UniqueViolation`, faz `ROLLBACK TO SAVEPOINT` e retorna `(inserted=false, nil)` — no-op
  idempotente. Esta é a implementação materializada; ela **substitui** a prescrição original de
  chamar `Insert` e tratar `application.ErrUserIdentityAlreadyLinked` como no-op no chamador. Motivo:
  no Postgres, uma `UniqueViolation` levantada dentro da transação já aberta pelo `uow.Do` envenenaria
  toda a transação (abortando inclusive o `publisher.Publish` e a jornada), violando a própria
  invariante "nunca abortar a jornada" deste ADR. O `SAVEPOINT` é o padrão canônico do Postgres para
  recuperar de violação de constraint mid-transaction sem contaminar a transação externa. O
  `Insert` original permanece inalterado para os consumidores existentes (ex.: `LinkChannelToUser`),
  que abrem transação própria.
- Qualquer outro erro **não** aborta a jornada: o principal legado já foi resolvido; emite log de
  nível `warn` e continua. A criação do vínculo é um efeito forward oportunista, não uma
  pré-condição do estabelecimento do principal.
- Idempotência e segurança sob concorrência são garantidas pelo índice UNIQUE parcial
  `(channel, external_id) WHERE unlinked_at IS NULL`: chamadas concorrentes convergem para um único
  vínculo, e a perdedora recebe o `ROLLBACK TO SAVEPOINT` tratado como no-op.

O `ensureIdentityLink` é acionado apenas no caminho `legacy` (quando o vínculo canônico ainda não
existe). No caminho `identity` o vínculo já está presente.

### 2. Coluna aditiva `resolve_path` em `auth_events`

Como `auth_events_reason_check` proíbe `reason` não-nulo quando `kind != 'failed'`, o caminho de
resolução **não** pode reusar `reason`. Adicionar coluna nova aditiva via migration `000008`:

```sql
ALTER TABLE auth_events
  ADD COLUMN IF NOT EXISTS resolve_path TEXT NULL;

ALTER TABLE auth_events
  ADD CONSTRAINT auth_events_resolve_path_check
  CHECK (resolve_path IS NULL OR resolve_path IN ('identity','legacy','backfill'));
```

A constraint `auth_events_reason_check` permanece **intacta** — os dois eixos (motivo de falha vs.
caminho de resolução) ficam em colunas separadas.

Propagação no código:

- `authEventPayload` ganha campo `ResolvePath *string` (`omitempty`).
- `newAuthEventOutbox` recebe o path e o repassa ao payload.
- O repositório persiste o valor na nova coluna `resolve_path`.

### 3. Tipo fechado `AuthResolvePath` (DMMF state-as-type)

Criar em `internal/identity/domain` o tipo fechado:

```
type AuthResolvePath string

const (
    AuthResolvePathIdentity AuthResolvePath = "identity"
    AuthResolvePathLegacy   AuthResolvePath = "legacy"
    AuthResolvePathBackfill AuthResolvePath = "backfill"
)
```

Com `IsValid()`, `String()` e `Parse*`. `lookupUserIDByWhatsApp` passa a retornar `AuthResolvePath`
em vez de strings soltas. A métrica `auth_resolve_path_total` consome `path.String()` (o label
`path` continua sendo um conjunto fechado, respeitando cardinalidade controlada).

### 4. Sem backfill (D-05)

Não há job de backfill histórico. O valor `backfill` permanece no enum apenas para
compatibilidade/futuro; em runtime somente `identity` ou `legacy` são emitidos.

## Alternativas Consideradas

### (a) Estender `auth_events_reason_check` para aceitar `reason IN ('identity','legacy','backfill')` quando `kind = 'principal_established'`

- **Descrição:** reusar a coluna `reason` existente para carregar o caminho de resolução, relaxando
  o CHECK.
- **Vantagens:** não adiciona coluna nova.
- **Desvantagens:** polui a semântica de `reason`, hoje exclusiva de motivo de falha; mistura dois
  eixos ortogonais numa mesma coluna; `DROP CONSTRAINT` + `ADD CONSTRAINT` é mais arriscado sob
  carga (revalida toda a tabela) que um `ADD COLUMN` aditivo.
- **Motivo da rejeição:** perda de clareza semântica e maior risco operacional na migration.

### (b) Chamar `LinkChannelToUser.Execute` de dentro de `resolvePrincipal`

- **Descrição:** reaproveitar o use case de vínculo existente.
- **Vantagens:** reuso direto de lógica já testada.
- **Desvantagens:** `LinkChannelToUser.Execute` abre a própria `uow.Do` (L106); chamá-lo dentro da
  transação de `EstablishPrincipal` aninharia transações.
- **Motivo da rejeição:** transação dentro de transação, comportamento não suportado com segurança.

### (c) `ON CONFLICT DO NOTHING` no `Insert` compartilhado

- **Descrição:** alterar o `Insert` do repositório para ignorar conflito.
- **Vantagens:** idempotência direta no SQL.
- **Desvantagens:** mudaria a semântica do `Insert` da qual `LinkChannelToUser` depende — hoje ele
  sinaliza o conflito (via sentinela) para permitir o re-read do vínculo existente.
- **Motivo da rejeição:** quebraria o contrato de `LinkChannelToUser`. Em vez disso, tratamos o
  sentinela `ErrUserIdentityAlreadyLinked` como no-op **dentro** de `ensureIdentityLink`.

## Consequências

### Benefícios Esperados

- Migração forward automática: cada estabelecimento por caminho legado cria o vínculo canônico,
  reduzindo progressivamente a dependência do fallback `users.whatsapp_number`.
- Trilha de auditoria persistente por evento (`resolve_path` em `auth_events`), complementando a
  métrica volátil.
- Estados de fronteira tipados (`AuthResolvePath`), tornando valores inválidos irrepresentáveis e
  mantendo o label de métrica com cardinalidade controlada.
- Migration puramente aditiva, com risco operacional baixo.

### Trade-offs e Custos

- Escrita adicional (`Insert` do vínculo) no caminho legado, dentro da transação já aberta — custo
  marginal, incorrido apenas até o usuário migrar.
- Nova coluna em `auth_events`, sempre nullable, sem impacto em consumidores existentes.
- O valor `backfill` fica no enum sem emissor em runtime (dívida de compatibilidade consciente,
  D-05).

### Riscos e Mitigações

- **Risco:** migration bloqueante sob carga. **Mitigação:** `ADD COLUMN IF NOT EXISTS` de coluna
  nullable é operação de metadados (não reescreve a tabela); aplicar com `lock_timeout` curto e
  `ADD CONSTRAINT ... CHECK` aditivo; `IF NOT EXISTS` garante reentrância.
- **Risco:** transação aninhada. **Mitigação:** reuso da `tx` existente via
  `u.factory.UserIdentityRepository(tx)`; `LinkChannelToUser` não é chamado.
- **Risco:** corrida entre requests concorrentes criando vínculos duplicados. **Mitigação:** índice
  UNIQUE parcial `(channel, external_id) WHERE unlinked_at IS NULL`; a perdedora recebe o sentinela
  tratado como no-op.
- **Risco:** falha na criação do vínculo derrubar a jornada do usuário. **Mitigação:**
  `ensureIdentityLink` nunca aborta o estabelecimento — erro não-sentinela vira `warn` + continue;
  o principal legado já foi resolvido antes da tentativa de vínculo.
- **Rollback:** ver Plano de Implementação (migration `down` remove coluna e constraint).

## Plano de Implementação

1. Criar tipo fechado `AuthResolvePath` (com `IsValid`/`String`/`Parse*`) em
   `internal/identity/domain`.
2. Criar migration `000008` (`up`: `ADD COLUMN IF NOT EXISTS resolve_path` +
   `ADD CONSTRAINT auth_events_resolve_path_check`; `down`: `DROP CONSTRAINT` + `DROP COLUMN`).
3. Estender `authEventPayload` com `ResolvePath *string` (`omitempty`), `newAuthEventOutbox` e o
   repositório para persistir `resolve_path`.
4. Alterar `lookupUserIDByWhatsApp` para retornar `AuthResolvePath`; alimentar
   `auth_resolve_path_total` com `path.String()`.
5. Adicionar método privado `ensureIdentityLink(ctx, tx, userID, channel, externalID, now)` em
   `EstablishPrincipal`, acionado no caminho `legacy`, chamando `UserIdentityRepository.InsertIfAbsent`
   (SAVEPOINT + `ROLLBACK TO SAVEPOINT` na `UniqueViolation`, retornando no-op idempotente sem
   contaminar a transação externa) e tratando demais erros como `warn` + continue.
6. Propagar o caminho resolvido ao evento `principal_established` (via `resolve_path`).

- **Dependências:** repositório `UserIdentityRepository` e `factory` já disponíveis na `tx`; schema
  `auth_events` da migration `000001`.
- **Sequência recomendada:** 1 → 2 → 3 → 4 → 5 → 6.
- **Conclusão:** vínculo canônico criado no fallback legado e `resolve_path` gravado em
  `auth_events`, cobertos por testes de integração (abaixo).

## Monitoramento e Validação

Testes de integração obrigatórios:

- **Resolução legada cria vínculo + registra path:** inbound resolvido por
  `users.whatsapp_number` cria exatamente **1** linha ativa em `user_identities`
  (`unlinked_at IS NULL`) e grava `auth_events.resolve_path = 'legacy'`.
- **Idempotência:** 2ª chamada do mesmo usuário resolve agora por `user_identities`
  (`resolve_path = 'identity'`) **sem** criar 2ª linha em `user_identities`.
- **Concorrência:** 2 requests simultâneos do mesmo número resultam em **1** único vínculo ativo
  (perdedora no-op via sentinela).

Sinais em produção:

- `auth_resolve_path_total{path="legacy"}` deve decrescer ao longo do tempo (migração forward);
  `{path="identity"}` deve crescer.
- Contagem de `auth_events` com `resolve_path = 'legacy'` como proxy de usuários ainda não migrados.
- Logs `warn` de falha em `ensureIdentityLink` (não deve haver volume relevante).

Critério de revisão/reversão: se a taxa de `warn` em `ensureIdentityLink` indicar contenção ou
falha sistêmica, ou se a migração forward não reduzir o caminho `legacy`.

## Impacto em Documentação e Operação

- Documentação técnica: `techspec.md` (RF-20, RF-21) e dicionário de eventos `auth_events`
  (nova coluna `resolve_path`).
- Observabilidade: painel/consulta de `auth_resolve_path_total` já existente; adicionar
  acompanhamento de `resolve_path` em `auth_events`.
- Runbook de identidade: registrar que o fallback legado agora cria vínculo canônico forward e que
  não há job de backfill (D-05).

## Conformidade Arquitetural

- **Sem novo padrão GoF:** método privado na mesma transação + coluna aditiva + tipo fechado; nenhum
  novo padrão estrutural/comportamental introduzido.
- **Adapter fino:** o repositório apenas persiste; a lógica (fallback, no-op no sentinela, decisão
  de não abortar) vive no use case `EstablishPrincipal`.
- **Zero comentários em Go** (R-ADAPTER-001.1) em todo o código de produção introduzido.
- **State-as-type:** `AuthResolvePath` é tipo fechado; nenhum estado de fronteira como string solta.
- **Cardinalidade controlada:** label `path` de `auth_resolve_path_total` permanece um enum fechado.

## Revisão Futura

- Revisar quando o caminho `legacy` cair a um volume residual: avaliar remoção do fallback
  `users.whatsapp_number` e, se necessário, reavaliar a decisão D-05 (backfill histórico) para
  migrar a cauda remanescente antes da remoção.
- Revisar se `auth_events.resolve_path` precisar de novos valores além de
  `identity`/`legacy`/`backfill`.
