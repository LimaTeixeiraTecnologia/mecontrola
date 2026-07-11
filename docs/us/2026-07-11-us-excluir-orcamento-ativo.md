# US-01: Excluir/Resetar o Orçamento Ativo de um Mês

> Fonte: pedido do usuário ("analisar `internal/budgets`, identificar gap/lacuna e criar UMA ÚNICA US") + confronto direto com a base de código em `/Users/jailtonjunior/Git/mecontrola`.
> Data de geração: 2026-07-11
> Nome do arquivo: `2026-07-11-us-excluir-orcamento-ativo.md`
> Módulo Go: `github.com/LimaTeixeiraTecnologia/mecontrola`

---

## Decisões Confirmadas (rodada de múltipla escolha, 2026-07-11)

| # | Decisão | Escolha |
|---|---------|---------|
| D-01 | Canal | **Ambos**: HTTP (estende `DELETE /api/v1/budgets/{competence}`) **e** WhatsApp com confirmação humana (HITL) |
| D-02 | Despesas committadas | **Preservar** as despesas do mês; remover apenas o plano (orçamento, alocações, estados de threshold e alertas de threshold da competência) |
| D-03 | Semântica de exclusão | **Hard delete** do orçamento, espelhando o comportamento atual de exclusão de rascunho (sem novo estado terminal) |
| D-04 | Pós-exclusão | **Manter o auto-draft atual**: uma nova transação naquela competência recria um rascunho automático via `CreateOrAutoDraftForExpense.EnsureExists` |

---

## Declaração

Como assinante do MeControla que administra o próprio orçamento mensal, quero excluir por completo o orçamento ativo de uma competência pela API ou por conversa no WhatsApp com confirmação, para zerar o planejamento daquele mês e recomeçar do rascunho sem perder as despesas já registradas.

## Contexto

- Problema: hoje, uma vez que o orçamento de um mês é ativado, o assinante fica preso a ele. É possível editar (fluxo já especificado em `.specs/prd-editar-orcamento-conversacional/`), mas **não existe nenhum caminho para removê-lo**. A exclusão só funciona para rascunho: `internal/budgets/application/usecases/delete_draft_budget.go:64` rejeita orçamento ativo com `entities.ErrBudgetAlreadyActive`, e o `BudgetState` possui apenas `Draft` e `Active` (`internal/budgets/domain/entities/budget.go:23-27`), sem estado de arquivamento ou soft-delete.
- Resultado esperado: o assinante consegue apagar o orçamento ativo (e também o rascunho) de uma competência, mantendo as despesas do mês; o mês volta a ficar sem orçamento até uma recriação manual ou até a chegada de uma nova transação, que recria um rascunho automático como já ocorre hoje.
- Fonte: pedido do usuário e auditoria da base de código realizada em 2026-07-11 (mapa completo de capacidades HTTP, eventos, jobs e ferramentas conversacionais do módulo `internal/budgets` e do consumidor conversacional `internal/agents`).

## Regras de Negócio

- RN-01: A exclusão remove fisicamente o orçamento da competência informada, independentemente do estado (`Draft` ou `Active`), unificando e ampliando o caminho de exclusão que hoje aceita apenas `Draft` (`internal/budgets/application/usecases/delete_draft_budget.go:57-73`).
- RN-02: A exclusão remove o plano da competência: o orçamento, suas alocações por categoria (`internal/budgets/domain/entities/allocation.go`), os estados de threshold e os alertas de threshold daquela competência. As despesas committadas da competência são preservadas intactas (elas têm origem em transações via `TransactionCreatedConsumer` → `UpsertExpense` e valor próprio de histórico).
- RN-03: Após a exclusão, se uma nova transação chegar para aquela competência, um rascunho automático é recriado pelo caminho vigente `internal/budgets/application/usecases/create_or_auto_draft_for_expense.go:26-37` (`NewAutoDraftBudget`). A exclusão zera o mês, mas não desativa o auto-draft.
- RN-04: A operação é idempotente por competência: excluir uma competência que não possui orçamento (nunca criado, ou já excluído) é tratado como sucesso silencioso, sem erro ao usuário.
- RN-05: A exclusão é estritamente escopada ao usuário autenticado. A competência é resolvida junto ao `userID` do requisitante; nunca é permitido excluir orçamento de outro usuário (proteção contra IDOR).
- RN-06: No canal WhatsApp, por ser operação destrutiva, a exclusão só é efetivada após confirmação humana explícita ("sim"/"não"), reutilizando o substrato de confirmação já existente (`ConfirmState` + `OperationKind` fechado em `internal/agents/application/workflows/confirm_state.go:46-104`; workflow durável `internal/agents/application/workflows/destructive_confirm_workflow.go:38`). Um novo valor fechado de operação (ex.: `OpDeleteBudget`) é adicionado ao enum e ao mapa `buildExecMap` (registry por operação, sem `switch case intent.Kind`), espelhando `OpDeleteRecurrence`.
- RN-07: No canal WhatsApp, a confirmação é durável e retomável: o estado de espera é persistido antes de perguntar; a retomada aplica merge-patch sobre o snapshot do kernel antes de qualquer parse; o replay pela mesma mensagem (`wamid`) não executa a exclusão duas vezes, herdando o contrato do fluxo destrutivo atual (`ContinueDestructiveConfirm`, `destructive_confirm_workflow.go:172`).
- RN-08: Conflito de escrita concorrente na mesma competência (ex.: edição em andamento) resulta em `interfaces.ErrBudgetConflict` e a exclusão falha de forma explícita, sem remoção parcial.
- RN-09: A remoção do plano ocorre em uma única transação de banco (unidade de trabalho), de modo que ou tudo é removido (orçamento, alocações, estados de threshold e alertas da competência) ou nada é.

## Critérios de Aceite

```gherkin
Cenário: Excluir orçamento ativo via HTTP preservando despesas
  Dado que o assinante autenticado possui um orçamento ativo na competência "2026-07"
  E que existem despesas committadas nessa competência
  Quando ele envia DELETE para "/api/v1/budgets/2026-07"
  Então o orçamento, as alocações, os estados de threshold e os alertas de threshold da competência são removidos
  E as despesas committadas da competência permanecem inalteradas
  E a resposta indica sucesso (sem conteúdo de erro)

Cenário: Excluir orçamento em rascunho via HTTP continua funcionando
  Dado que o assinante autenticado possui um orçamento em rascunho na competência "2026-08"
  Quando ele envia DELETE para "/api/v1/budgets/2026-08"
  Então o rascunho é removido
  E a resposta indica sucesso

Cenário: Exclusão idempotente de competência sem orçamento
  Dado que o assinante autenticado não possui orçamento na competência "2026-09"
  Quando ele envia DELETE para "/api/v1/budgets/2026-09"
  Então a resposta indica sucesso silencioso
  E nenhum plano é alterado

Cenário: Excluir orçamento ativo por conversa exige confirmação humana
  Dado que o assinante conversa pelo WhatsApp e possui orçamento ativo em "2026-07"
  Quando ele pede "apaga meu orçamento de julho"
  Então o agente responde pedindo confirmação explícita antes de aplicar
  E o estado de espera é persistido de forma durável antes da pergunta
  Quando o assinante responde "sim"
  Então o plano da competência é removido, as despesas são preservadas e o agente confirma a exclusão

Cenário: Cancelamento da exclusão por conversa não remove nada
  Dado que o assinante recebeu a pergunta de confirmação de exclusão do orçamento de "2026-07"
  Quando ele responde "não"
  Então nenhum plano é removido
  E o agente confirma que a exclusão foi cancelada

Cenário: Recriação automática de rascunho após exclusão
  Dado que o orçamento ativo de "2026-07" foi excluído
  Quando uma nova transação committada chega para a competência "2026-07"
  Então um rascunho automático é recriado para a competência
  E as despesas anteriores da competência continuam preservadas

Cenário: Bloqueio de exclusão cruzada entre usuários
  Dado que o usuário A está autenticado
  E que existe um orçamento ativo pertencente ao usuário B na competência "2026-07"
  Quando o usuário A tenta excluir "/api/v1/budgets/2026-07"
  Então nenhum orçamento do usuário B é removido
  E a operação é tratada apenas no escopo do usuário A

Cenário: Conflito concorrente aborta a exclusão sem remoção parcial
  Dado que há uma edição concorrente em andamento no orçamento ativo de "2026-07"
  Quando o assinante tenta excluir a mesma competência
  Então a operação falha com conflito de orçamento
  E nenhum registro do plano é removido parcialmente
```

## Dados e Permissões

- Dados obrigatórios: `userID` do assinante autenticado; `competence` no formato `YYYY-MM` (validado por `internal/budgets/domain/valueobjects/competence.go`). No canal WhatsApp, a competência é derivada da mensagem via `MonthReference` + `DecideCompetence` (`internal/budgets/domain/valueobjects/month_reference.go`), e a identidade vem do inbound (`internal/platform/agent` → `agent.InboundIdentityFromContext`).
- Perfis/permissões: apenas o próprio assinante autenticado. HTTP protegido pelo middleware `gatewayAuth` já aplicado ao roteador (`internal/budgets/infrastructure/http/server/router.go`). Conversa protegida pela identidade resolvida no runtime de agentes; escrita idempotente por `wamid`.

## Dependências

- Domínio/aplicação (`internal/budgets`): ampliar o caminho de exclusão para aceitar orçamento ativo. Depende de novo comportamento no `BudgetRepository` para excluir por competência independentemente do estado (hoje só há `DeleteDraft` em `internal/budgets/application/interfaces/budget_repository.go:23`) e de rotinas de remoção do plano associado.
- Repositórios de plano: os contratos atuais não expõem remoção em massa por competência — `ThresholdStateRepository` só possui `GetCurrentlyCrossed` (`internal/budgets/application/interfaces/threshold_state_repository.go:15`) e `AlertRepository` só possui `Insert`/`ListForUser` (`internal/budgets/application/interfaces/alert_repository.go:13-15`). A remoção de estados de threshold e alertas da competência exige novos comandos de exclusão nesses repositórios (adapter Postgres em `internal/budgets/infrastructure/repositories/postgres/`).
- Unidade de trabalho: reutilizar `uow.UnitOfWork` (`internal/platform/database/uow`) para atomicidade (RN-09), como já feito em `delete_draft_budget.go`.
- HTTP: o handler e a rota já existem (`DELETE /api/v1/budgets/{competence}` em `router.go:61` → `DeleteBudgetHandler`); a mudança é passar a aceitar estado ativo, preservando o comportamento de rascunho.
- Conversacional (`internal/agents`): novo valor fechado de operação no enum `OperationKind` (`confirm_state.go:46`) e entrada correspondente no `buildExecMap` do `destructive_confirm_workflow.go:38`, além de uma nova ferramenta (ex.: `BuildDeleteBudgetTool`) espelhando `internal/agents/application/tools/delete_recurrence.go:30`. Reutiliza o kernel de workflow durável (`internal/platform/workflow`) e o substrato de confirmação, sem reimplementar HITL/TTL/replay.
- Governança aplicável: R-ADAPTER-001 (adaptadores finos, zero comentários), R-AGENT-WF-001 (roteamento por registry, tool fina sem regra/SQL, estados fechados, confirmação persistida antes do parse), R-WF-KERNEL-001 (kernel genérico sem domínio), R-TXN-004 (cardinalidade controlada de métricas), R-DTO-VALIDATE-001 (input DTO com `Validate()`).

## Fora de Escopo

- Editar total, ajustar porcentagem de categoria ou refazer a distribuição do orçamento: já coberto por `.specs/prd-editar-orcamento-conversacional/` e por `docs/us/2026-07-10-us-editar-criar-orcamento-conversacional.md`.
- Excluir despesas individuais ou transações: coberto por `DeleteExpense` (`internal/budgets/application/usecases/delete_expense.go`) e pelo fluxo de exclusão de lançamento do agente.
- Introduzir estado terminal `Cancelado`/`Arquivado` no `BudgetState` (D-03 optou por hard delete).
- Suprimir o auto-draft após exclusão (D-04 mantém o comportamento vigente de recriação automática).
- Expor via HTTP os casos de uso `EditCategoryPercentage` e `SuggestAllocation` (hoje sem handler): fora do escopo desta história.
- Alertas proativos e templates de notificação: cobertos por `.specs/prd-alertas-proativos/` e `docs/us/2026-07-07-us-alertas-proativos.md`.

## Evidências

- Entrada: pedido do usuário para auditar `internal/budgets`, identificar um gap real e produzir uma única história de usuário pronta para desenvolvimento.
- Base de código:
  - `internal/budgets/application/usecases/delete_draft_budget.go:64` — exclusão hoje rejeita orçamento ativo com `ErrBudgetAlreadyActive`.
  - `internal/budgets/domain/entities/budget.go:23-27` — `BudgetState` possui apenas `Draft` e `Active`; sem arquivamento/soft-delete de orçamento.
  - `internal/budgets/application/interfaces/budget_repository.go:19-28` — repositório expõe `DeleteDraft`, mas nenhum caminho de exclusão para orçamento ativo.
  - `internal/budgets/infrastructure/http/server/router.go:61` — rota `DELETE /api/v1/budgets/{competence}` mapeada para `DeleteBudgetHandler` (hoje só rascunho).
  - `internal/budgets/application/usecases/create_or_auto_draft_for_expense.go:26-37` — auto-draft recriado ao chegar despesa/transação (base de RN-03/D-04).
  - `internal/budgets/application/interfaces/threshold_state_repository.go:15` e `internal/budgets/application/interfaces/alert_repository.go:13-15` — sem remoção por competência (base das dependências de plano).
  - `internal/agents/application/workflows/confirm_state.go:46-104` — enum fechado `OperationKind` com `OpDeleteEntry`, `OpDeleteCard`, `OpDeleteRecurrence` etc.; ponto de extensão para `OpDeleteBudget`.
  - `internal/agents/application/workflows/destructive_confirm_workflow.go:38` e `:172` — `buildExecMap` (registry por operação) e `ContinueDestructiveConfirm` (retomada por confirmação).
  - `internal/agents/application/tools/delete_recurrence.go:30` — ferramenta destrutiva que inicia o workflow durável de confirmação; molde para a ferramenta de exclusão de orçamento.
  - `docs/us/2026-07-10-us-editar-criar-orcamento-conversacional.md` (D-01) — a história de edição exclui explicitamente "excluir/resetar", confirmando que esta capacidade não está coberta.
- Inferências: a remoção de estados de threshold e alertas da competência (RN-02) foi inferida como parte de "remover o plano" para manter consistência (evitar alertas órfãos referenciando orçamento inexistente); a granularidade final (caso de uso unificado vs. novo caso de uso dedicado) é decisão de especificação técnica.
- Não evidenciado: não há, na base de código, comando de exclusão de orçamento ativo, ferramenta conversacional de exclusão de orçamento, evento de "orçamento excluído", nem rotina de exclusão por competência em `ThresholdStateRepository`/`AlertRepository` — buscas por `Delete`, `OpDeleteBudget` e por remoção em massa por competência não retornaram implementação.

## Notas de Validação

- Cobertura de cenários: fluxo feliz (exclusão ativa via HTTP e via WhatsApp), variações válidas (rascunho, idempotência, recriação de auto-draft), e caminhos de erro/bloqueio (cancelamento na confirmação, escopo cruzado entre usuários, conflito concorrente).
- Sem marcadores pendentes; todas as afirmações técnicas apontam caminho e linha verificáveis na base de código.
- A história é uma fatia de valor independente, cabível em um sprint, com uma persona única (assinante) e critérios de aceite verificáveis em ambos os canais.
</content>
</invoke>
