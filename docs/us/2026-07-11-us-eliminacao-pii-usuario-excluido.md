# US-IDN-LGPD-001: Eliminação (anonimização) da PII de usuários excluídos após a janela de reanimação

## Resumo e decisão de escopo
História única de habilitação com resultado observável de conformidade. Fecha um gap real de
proteção de dados no módulo `internal/identity`: quando um usuário é excluído, o fluxo faz apenas
*soft-delete* e anonimiza a tabela `auth_events`, mas a PII central do titular
(`users.whatsapp_number`, `users.email`, `users.display_name`, `user_identities.external_id` e
`user_whatsapp_history.number`) permanece em texto claro **indefinidamente**, sem nenhum job ou
use case que a elimine após o fim da janela de reanimação de 30 dias. Esta US traz o restante da
PII à paridade com o padrão de eliminação já aplicado a `auth_events`, disparado por um job de
housekeeping automático (espelho de `auth_events_housekeeping`), anonimizando in-place assim que o
usuário deixa de ser reanimável. Escopo fechado a um único job de eliminação; não altera o
soft-delete, a reanimação nem os demais módulos.

## Confronto com o Codebase
Investigação executada em `internal/identity` e nos schemas de migração; grep no repositório inteiro.

- Soft-delete não toca a PII: `internal/identity/infrastructure/repositories/postgres/user_repository.go:314-344` executa `UPDATE users SET status='DELETED', deleted_at=$2, updated_at=$2 WHERE id=$3 AND deleted_at IS NULL` — `whatsapp_number`, `email` e `display_name` são retidos.
- Use case de exclusão só publica evento: `internal/identity/application/usecases/mark_user_deleted.go:37-74` marca deletado e publica `user.deleted`; não anonimiza PII de usuário.
- O evento `user.deleted` é consumido apenas para anonimizar `auth_events`: `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go:68-75` chama `AnonymizeUserAuthEvents`, cujo alcance é a tabela `auth_events` (`internal/identity/application/usecases/anonymize_user_auth_events.go:58` → `repo.AnonymizeByUserID`).
- Único housekeeping existente é de `auth_events`: `internal/identity/infrastructure/jobs/handlers/auth_events_housekeeping_job.go:29-41` (`@monthly`) e `internal/identity/application/usecases/cleanup_auth_events.go:51-104` (retenção 180 dias, lote, cancelamento de contexto, métricas). Não há job equivalente para PII de `users`.
- Janela de reanimação é de 30 dias e depois `CanReanimate` retorna false, porém a PII continua no banco: `internal/identity/domain/policies.go:5` (`ReanimationWindow = 30 * 24h`) e `internal/identity/domain/entities/user.go:104-109`.
- A porta `UserRepository` não oferece operação de anonimização/purga de excluídos: `internal/identity/application/interfaces/user_repository.go:21-30` expõe `MarkDeleted` e `Reanimate`, mas nenhuma operação de eliminação de PII em lote.
- PII e formato das colunas (base para a regra de anonimização): `migrations/000001_initial_schema.up.sql` — `users.whatsapp_number TEXT NOT NULL`, `email TEXT NULL`, `display_name TEXT NULL`, sem CHECK de formato; índice único ativo é parcial (`users_whatsapp_number_active_uniq_idx ... WHERE deleted_at IS NULL`), logo linhas excluídas estão isentas de colisão de unicidade. `user_identities.external_id TEXT NOT NULL CHECK (length(external_id) > 0)` com `unlinked_at`. `user_whatsapp_history.number TEXT NOT NULL`.
- Precedente de anonimização in-place por `user_id`: `internal/identity/application/interfaces/auth_events_repository.go:14` (`AnonymizeByUserID`), reforçando anonimizar-em-lugar como padrão do módulo.
- Wiring de job e consumer segue o Padrão Obrigatório de Módulo em `internal/identity/module.go:121-167` e `AGENTS.md:173-205`.
- Config do módulo: `configs/config.go:93-96` (`IdentityConfig` com schedule/batch/retention de auth_events) é o molde para novos parâmetros de eliminação.
- Grep em `internal/` por `DELETE FROM users` ou anonimização de PII de `users` fora de `auth_events`: sem resultados.

## Declaração
Como Encarregado de Proteção de Dados da MeControla, quero que a PII de usuários excluídos seja
anonimizada automaticamente pelo sistema assim que eles deixam de ser reanimáveis, para cumprir o
direito de eliminação da LGPD e não reter dados pessoais além do prazo necessário.

## Contexto
- Problema: a exclusão de usuário faz soft-delete e anonimiza somente `auth_events`; a PII em
  `users`, `user_identities` e `user_whatsapp_history` fica retida em texto claro sem prazo de
  expurgo. Não existe job, use case nem porta de repositório que a elimine, mesmo depois de o
  usuário deixar de ser reanimável (janela de 30 dias). O direito de eliminação fica cumprido
  apenas em parte.
- Resultado esperado: um job de housekeeping periódico varre usuários com `status=DELETED` cujo
  `deleted_at` ultrapassou a janela de reanimação e anonimiza in-place a PII dessas linhas em
  `users`, `user_identities` e `user_whatsapp_history`, preservando `user_id` como chave estável
  para as referências de outros módulos. A operação é idempotente, processada em lote, observável
  por métrica sem PII, e não afeta usuários ativos nem usuários ainda reanimáveis. A PII do titular
  fica à paridade com a anonimização já aplicada a `auth_events`.
- Fonte: análise do módulo `internal/identity` solicitada pelo usuário em 2026-07-11, com decisões
  de gatilho (job automático), forma (anonimizar in-place) e marco de retenção (fim da janela de
  reanimação) confirmadas pelo usuário; evidências em `## Confronto com o Codebase`.

## Regras de Negócio
- RN-01: Elegibilidade — só é elegível para eliminação o usuário com `status=DELETED` cujo
  `now - deleted_at` seja maior que `domain.ReanimationWindow` (30 dias). Usuário ativo e usuário
  excluído ainda dentro da janela de reanimação nunca são elegíveis.
- RN-02: A eliminação é anonimização in-place, não exclusão física da linha: a linha e o `user_id`
  são preservados como chave estável para as FKs de outros bounded contexts (transactions, budgets,
  cards, billing), evitando quebra de integridade referencial.
- RN-03: Campos anonimizados por usuário elegível:
  - `users.whatsapp_number` → pseudônimo determinístico e não reversível que satisfaça `NOT NULL`
    (a coluna não tem CHECK de formato; a linha excluída está isenta do índice único ativo parcial).
  - `users.email` → `NULL`.
  - `users.display_name` → `NULL`.
  - `user_identities.external_id` → pseudônimo que satisfaça `CHECK (length(external_id) > 0)`, e a
    identidade ainda ativa DEVE ser marcada como desvinculada (`unlinked_at`).
  - `user_whatsapp_history.number` → pseudônimo que satisfaça `NOT NULL`.
- RN-04: Idempotência — reexecutar a eliminação sobre um usuário já anonimizado é no-op; a métrica
  de usuários anonimizados conta apenas as linhas efetivamente alteradas na execução.
- RN-05: Processamento em lote com tamanho configurável e respeito ao cancelamento de contexto,
  na mesma forma de `CleanupAuthEvents`, para não bloquear o worker em bases grandes.
- RN-06: Observabilidade — a execução expõe um contador de usuários anonimizados e um histograma de
  duração, sem PII nos labels das métricas.
- RN-07: Agendamento configurável via `IdentityConfig`, com default seguro; o job é entregue ao
  `WorkerManager` como `worker.Job` pelo adapter de `internal/platform/worker`, seguindo o Padrão
  Obrigatório de Módulo.
- RN-08: A anonimização de `auth_events` disparada pelo evento `user.deleted` permanece inalterada;
  esta regra cobre a lacuna das demais tabelas de PII e a completa após a janela de reanimação.
- RN-09: A eliminação não pode competir com reanimação legítima: como a reanimação só é permitida
  dentro de 30 dias (`CanReanimate`) e a eliminação só ocorre depois de 30 dias, os dois caminhos
  nunca disputam a mesma linha.
- RN-10: A operação é escopada por registro (por `user_id` elegível); nunca altera usuários ativos
  nem PII de terceiros.

## Critérios de Aceite
```gherkin
Cenário: usuário excluído além da janela de reanimação tem a PII anonimizada
  Dado um usuário com status DELETED cujo deleted_at ocorreu há 31 dias
  E que possui whatsapp_number, email e display_name preenchidos em users
  E uma identidade ativa em user_identities e um registro em user_whatsapp_history
  Quando o job de eliminação de PII de usuários excluídos é executado
  Então o whatsapp_number em users é substituído por um pseudônimo não reversível
  E o email e o display_name em users passam a ser nulos
  E o external_id em user_identities é pseudonimizado e a identidade fica desvinculada
  E o number em user_whatsapp_history é pseudonimizado
  E o user_id da linha permanece inalterado

Cenário: usuário excluído ainda dentro da janela de reanimação é preservado
  Dado um usuário com status DELETED cujo deleted_at ocorreu há 10 dias
  Quando o job de eliminação de PII de usuários excluídos é executado
  Então a PII desse usuário permanece intacta em users, user_identities e user_whatsapp_history
  E o usuário continua reanimável

Cenário: usuário ativo nunca é afetado
  Dado um usuário com status ACTIVE e deleted_at nulo
  Quando o job de eliminação de PII de usuários excluídos é executado
  Então a PII desse usuário permanece intacta

Cenário: reexecução é idempotente sobre usuário já anonimizado
  Dado um usuário excluído cuja PII já foi anonimizada por uma execução anterior
  Quando o job de eliminação de PII de usuários excluídos é executado novamente
  Então nenhuma linha adicional é alterada para esse usuário
  E o contador de usuários anonimizados não incrementa por causa dele

Cenário: cancelamento de contexto interrompe o lote sem corromper dados
  Dado que há vários usuários elegíveis para eliminação
  E que o contexto de execução é cancelado no meio do processamento em lote
  Quando o job de eliminação de PII de usuários excluídos observa o cancelamento
  Então a execução retorna erro de contexto cancelado
  E as linhas já anonimizadas permanecem anonimizadas e consistentes
```

## Dados e Permissões
- Dados obrigatórios: `users` (`id`, `status`, `deleted_at`, `whatsapp_number`, `email`,
  `display_name`); `user_identities` (`user_id`, `external_id`, `unlinked_at`);
  `user_whatsapp_history` (`user_id`, `number`); e `domain.ReanimationWindow`.
- Perfis/permissões: job interno agendado, executado pelo `WorkerManager`; sem exposição HTTP e sem
  input de usuário final. O gatilho é temporal (idade do `deleted_at`), não uma requisição externa.

## Dependências
- `domain.ReanimationWindow` (`internal/identity/domain/policies.go:5`) — reuso do marco de 30 dias
  como limite de elegibilidade.
- Nova operação na porta `UserRepository`
  (`internal/identity/application/interfaces/user_repository.go:21`) para selecionar e anonimizar em
  lote usuários excluídos-expirados; hoje inexistente (só `MarkDeleted` e `Reanimate`).
- Novo use case espelhando `CleanupAuthEvents` e novo job handler espelhando
  `AuthEventsHousekeepingJob`, com wiring em `internal/identity/module.go` conforme o Padrão
  Obrigatório de Módulo (`AGENTS.md:173-205`).
- Novo parâmetro em `IdentityConfig` (`configs/config.go:93`) para schedule e tamanho de lote da
  eliminação.
- Adapter Postgres: nova instrução de anonimização no `userRepository`
  (`internal/identity/infrastructure/repositories/postgres/user_repository.go`).

## Fora de Escopo
- Anonimização de `auth_events`, já implementada (`auth_events_consumer.go:68`,
  `anonymize_user_auth_events.go`).
- Exclusão física de linhas ou cascata para dados financeiros (`transactions`, `budgets`, `cards`) e
  de `billing`; esta US preserva `user_id` e não remove registros de negócio.
- Endpoint sob demanda para o titular solicitar eliminação imediata; o gatilho definido é o job
  automático pós-janela.
- Alteração do valor da janela de reanimação de 30 dias.
- Eliminação de PII fora do módulo `internal/identity`.

## Evidências
- Entrada: pedido do usuário em 2026-07-11 para analisar `internal/identity`, identificar um gap
  real e produzir uma única história de usuário; decisões de gatilho, forma e retenção confirmadas
  pelo usuário via perguntas de múltipla escolha.
- Base de código: `internal/identity/infrastructure/repositories/postgres/user_repository.go:314-344`
  (soft-delete sem tocar PII); `internal/identity/application/usecases/mark_user_deleted.go:37-74`;
  `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go:68-75`;
  `internal/identity/application/usecases/anonymize_user_auth_events.go:58`;
  `internal/identity/application/usecases/cleanup_auth_events.go:51-104`;
  `internal/identity/infrastructure/jobs/handlers/auth_events_housekeeping_job.go:29-41`;
  `internal/identity/domain/policies.go:5`; `internal/identity/domain/entities/user.go:104-109`;
  `internal/identity/application/interfaces/user_repository.go:21-30`;
  `internal/identity/application/interfaces/auth_events_repository.go:14`;
  `migrations/000001_initial_schema.up.sql` (DDL e índices de `users`, `user_identities`,
  `user_whatsapp_history`); `configs/config.go:93-96`; `internal/identity/module.go:121-167`.
- Inferências: o enquadramento no direito de eliminação da LGPD (Art. 18) é interpretação de
  conformidade a partir do comportamento observado; o repositório não cita um documento de política
  de retenção, então o marco de 30 dias foi adotado por reuso do conceito de reanimação já modelado.
- Não evidenciado: nenhum job, use case ou operação de repositório que anonimize ou remova a PII de
  `users`, `user_identities` ou `user_whatsapp_history` — o grep por `DELETE FROM users` e por
  anonimização de PII de `users` fora de `auth_events` não retornou resultados.

## Notas de Validação
- Validado por `python3 .agents/skills/user-stories/scripts/validar-historias-usuario.py` sobre este
  arquivo (saída de sucesso).
- Skills aplicadas na análise: `go-implementation`, `domain-modeling-production`,
  `design-patterns-mandatory` e `user-stories`.
- Cobertura de cenários: fluxo feliz (anonimização pós-janela), alternativos (dentro da janela,
  usuário ativo, reexecução idempotente) e erro (cancelamento de contexto no lote).
