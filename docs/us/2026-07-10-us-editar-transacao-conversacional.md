# Editar Transação pela Conversa (paridade total de campos) — User Story

> Objetivo: permitir que o usuário edite pela conversa no WhatsApp uma transação já registrada, ajustando qualquer campo, com confirmação humana antes de gravar e reflexo correto no orçamento.
> Escopo confirmado com o solicitante: **Editar por conversa (paridade total)**. Criar transação entra como pré-condição já existente, não como entrega.
> Data de geração: 2026-07-10
> Nome do arquivo: `2026-07-10-us-editar-transacao-conversacional.md`
> Base: `internal/transactions`, `internal/budgets`, `internal/agents`, `internal/platform/{agent,workflow,tool,memory,scorer}`.

---

## Declaração

Como usuário do MeControla no WhatsApp, quero editar por conversa uma transação que já registrei — ajustando qualquer um dos seus campos (valor, descrição, data, categoria, forma de pagamento, cartão e parcelas, ou direção) com uma confirmação antes de gravar, para que meus lançamentos e o consumo do meu orçamento passem a refletir a realidade sem que eu precise apagar e recadastrar o lançamento.

## Contexto

- Problema: o fluxo de edição conversacional existe apenas de forma parcial. A tool `edit_entry` aceita somente `amountCents`, `description` e `occurredAt` (`internal/agents/application/tools/edit_entry.go:30-43`), enquanto o caso de uso de domínio `UpdateTransaction` suporta editar direção, forma de pagamento, categoria/subcategoria, cartão e parcelas, valor, descrição e data com `version` de controle otimista (`internal/transactions/application/usecases/update_transaction.go:53-160`; DTO `internal/transactions/application/dtos/input/raw_update_transaction.go:9-63`). Há uma lacuna funcional entre o que o agente oferece e o que o domínio já sabe fazer.
- Problema secundário (reflexo no orçamento): o módulo `internal/budgets` consome apenas `transactions.transaction.created.v1` e `transactions.transaction.deleted.v1` (`internal/budgets/module.go:144-145`). Não existe consumidor para `transactions.transaction.updated.v1`, apesar de o `transactions` publicar esse evento (`internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher.go`, `eventTypeTransactionUpdated = "transactions.transaction.updated.v1"`). Consequência atual: editar valor, data ou categoria de uma despesa não atualiza `budgets_expenses`, deixando o resumo mensal e os alertas de limite defasados.
- Resultado esperado: o usuário edita qualquer campo da transação por conversa; o sistema pede confirmação com o resumo do impacto; ao confirmar, grava via `UpdateTransaction` com controle otimista; o consumo do orçamento por categoria é recomputado; nada é afirmado como sucesso sem retorno real da ferramenta.
- Fonte: solicitação do usuário (editar transações via `internal/transactions` + `internal/budgets`, com paridade de campos e cobertura integral das possibilidades de conversa) e evidências da base de código listadas na seção Evidências.

## Confronto com o Codebase e Análise de Workflow

### O que já existe (reaproveitar, não recriar)

- **Substrato de workflow durável**: `workflow.Engine[S]` com `Start`/`Resume`, `Snapshot` persistido, estados fechados `RunStatus`/`StepStatus`/`SuspendReason` (`internal/platform/workflow/engine.go:27-31`, `internal/platform/workflow/step.go`). Resume por merge-patch sobre `Snapshot.State` (R-WF-KERNEL-001.7).
- **Padrão de slot-filling conversacional já pronto para CRIAR**: `pending_entry_workflow.go` preenche, um campo por vez, categoria → forma de pagamento → cartão → data → confirmação, com reprompt único (`maxReprompts = 1`, `internal/agents/application/workflows/pending_entry_decisions.go:29`) e expiração de 35 minutos (`PendingEntryStaleAfter = 35 * time.Minute`, `internal/agents/application/workflows/pending_entry_workflow.go:56`). Este é o molde a reusar para EDITAR.
- **Gate HITL de confirmação já pronto**: `destructive_confirm_workflow.go` com `OperationKind` fechado (inclui `OpEditEntry`) e `AwaitingKind` (`AwaitingNone`/`AwaitingConfirm`), TTL de 5 minutos (`confirmTTL`), executor `executeEditEntry` que chama `ledger.UpdateTransaction` com `UpdatePayload` + `Version` (`internal/agents/application/workflows/destructive_confirm_workflow.go:92-124,313-328`).
- **Binding para o domínio**: `transactionsLedgerAdapter.UpdateTransaction` delega a `txusecases.UpdateTransaction` e injeta o `auth.Principal` a partir da identidade inbound (`internal/agents/infrastructure/binding/transactions_ledger_adapter.go:112-146,57-70`).
- **Idempotência de escrita conversacional**: `IdempotentWrite.Execute` desduplica por `(wamid, itemSeq, operation)` e classifica o desfecho como `ToolOutcomeRouted`/`ToolOutcomeReconciled`/`ToolOutcomeReplay` (`internal/agents/application/usecases/idempotent_write.go:59-145`). Reforçado pela unicidade de origem no banco: `ON CONFLICT (origin_wamid, origin_item_seq, origin_operation) DO NOTHING` (`internal/transactions/infrastructure/repositories/postgres/transaction_repository.go:41-98`).
- **Resolução de categoria e cartão**: `classify_category` (`internal/agents/application/tools/classify_category.go`) e `resolve_card` (`internal/agents/application/tools/resolve_card.go`), reaproveitáveis quando a edição mexer em categoria ou migrar para cartão de crédito.
- **Consumo do orçamento por update**: `UpsertExpense.Execute` já suporta o caminho de atualização via `ExpectedVersion` e emite `MutationKindUpdate` (`internal/budgets/application/usecases/upsert_expense.go:150-218`); o `TransactionCreatedConsumer` já mapeia evento de transação → `UpsertExpense` (`internal/budgets/infrastructure/messaging/database/consumers/transaction_created_consumer.go:77-129`), servindo de molde para um consumidor de update.

### O que falta (entrega desta US)

1. **Edição conversacional com paridade de campos** reusando o workflow de slot-filling: hoje `edit_entry` só cobre 3 campos; falta cobrir categoria, forma de pagamento, cartão/parcelas e direção, com as mesmas invariantes de domínio.
2. **Consumidor `transactions.transaction.updated.v1` em `internal/budgets`** que chame `UpsertExpense` com os novos valores (competência, valor, subcategoria→root slug), fechando o reflexo no orçamento.
3. **Enriquecer o payload de `TransactionUpdated`** com `CategoryID`/`SubcategoryID` (`internal/transactions/domain/entities/events.go:26-35`), pois hoje o evento de update não carrega categoria e o orçamento não teria como recomputar o consumo por categoria após troca de categoria.

### Decisão de workflow (análise solicitada)

Editar transação **deve** usar o kernel de workflow (`workflow.Engine[S]`), não uma tool única de disparo. Justificativa ancorada no código: a edição pode exigir múltiplas rodadas (resolver nova categoria via `classify_category`, resolver cartão via `resolve_card` quando migrar para crédito, coletar parcelas, e confirmar) e precisa sobreviver entre mensagens do WhatsApp — exatamente o que o `pending_entry_workflow` já faz para criação com suspensão/resume durável. O gate final de gravação reusa o contrato do `destructive_confirm_workflow` (`OpEditEntry`), que já persiste o estado de espera antes de perguntar e retoma por merge-patch antes do parse (R-AGENT-WF-001.7). Não criar `switch case intent.Kind`: a edição entra como comportamento resolvido por registry (R-AGENT-WF-001.1).

## Possibilidades de Conversação

Cada possibilidade abaixo é ancorada em texto verbatim já presente no código; nenhuma resposta foi inventada. Onde a mensagem de edição ainda não existe, o alvo é reusar o texto equivalente do fluxo de criação/confirmação já implementado (indicado como "reuso").

### Gatilhos de intenção de edição (linguagem do usuário)

- "edita o último lançamento" / "edita o lançamento de 30 reais" / "muda a data do último gasto para ontem" / "corrige a descrição para almoço" / "troca a categoria para transporte" / "muda a forma de pagamento para pix" / "passa esse gasto para o crédito do nubank em 3x" / "na verdade isso foi receita, não despesa".

### Perguntas de esclarecimento (slot-filling) — verbatim existentes

- Categoria: `"Qual é a categoria deste lançamento?"` (`pending_entry_workflow.go:855`).
- Forma de pagamento: `"Como você pagou? Ex.: dinheiro, pix, débito, crédito, boleto, vale-refeição"` (`pending_entry_workflow.go:857`; espelha `mecontrola_agent.go:57`).
- Cartão: `"Qual cartão foi utilizado?"` (`pending_entry_workflow.go:859`).
- Data: `"Qual foi a data do lançamento?"` (`pending_entry_workflow.go:861`).
- Escolha entre categorias candidatas: `"Qual se encaixa melhor? 1. ... 2. ..."` (`pending_entry_workflow.go:812`).

### Reprompts (uma única vez, depois cancela) — verbatim existentes

- `"Não reconheci a categoria. Qual é a categoria deste lançamento?"` (`pending_entry_workflow.go:877`).
- `"Não reconheci a forma de pagamento. Como você pagou? Ex.: dinheiro, pix, débito, crédito, boleto, vale-refeição"` (`pending_entry_workflow.go:879`).
- `"Não reconheci o cartão. Qual cartão foi utilizado?"` (`pending_entry_workflow.go:881`).
- `"Não reconheci a data. Qual foi a data do lançamento?"` (`pending_entry_workflow.go:883`).
- Limite de reprompt: `maxReprompts = 1` (`pending_entry_decisions.go:29`).

### Confirmação (gate HITL) — verbatim existentes

- Resumo de confirmação (formato reusado do criar): `"Confirma? *{descrição}* {valor BRL} em *{Categoria > Subcategoria}* para {data} {segmento de pagamento}?"` (`buildConfirmSummary`, `pending_entry_workflow.go:889-909`). Segmento de pagamento: receita omite; crédito com parcelas exibe `"no crédito em {N}x"`; crédito à vista exibe `"no crédito à vista"`; demais exibem `"no {forma}"` (`confirmPaymentSegment`, `pending_entry_workflow.go:911-926`).
- Confirmação da tool de edição atual é relayada verbatim pelo campo `impactNote` quando `needsConfirmation=true` (`edit_entry.go:45-60`; protocolo verbatim em `mecontrola_agent.go:72-84`).
- Aceite reconhecido no fluxo de pendência: `sim | confirmar | confirma | ok | pode` (regex `reConfirmYes`, `pending_entry_decisions.go:121`).
- Cancelamento reconhecido: `não | nao | cancela | cancels | deixa pra lá | não registra` e frases de cancelamento como `^cancela(r)?$`, `^deixa pra lá$`, `^não registra(r)?$` (`pending_entry_decisions.go:115-122`).
- No gate HITL (`destructive_confirm`): aceite `sim, confirmar, confirmo, ok, pode, yes, s`; cancelamento `não, nao, cancelar, cancelo, no, n` (`destructive_confirm_workflow.go:226-238`); resposta ambígua: `"Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar a operação."` (`destructive_confirm_workflow.go:116`); sufixo de confirmação: `"\n\nResponda *sim* para confirmar ou *não* para cancelar."` (`destructive_confirm_workflow.go:202`).

### Encerramentos e falhas — verbatim existentes

- Cancelado pelo usuário: `"Tudo certo, o registro foi cancelado."` (`pending_entry_workflow.go:144,211,376`).
- Expiração da pendência: `"O registro expirou. Para registrar, envie a informação completa novamente."` (`pending_entry_workflow.go:135,202,305,382`).
- Categoria não identificada após reprompt: `"Não consegui identificar a categoria. O registro foi cancelado."` (`pending_entry_workflow.go:279`).
- Cartão não identificado após reprompt: `"Não consegui identificar o cartão. O registro foi cancelado."` (`pending_entry_workflow.go:181`).
- Erro de gravação (sem inventar sucesso): `"Não consegui registrar. Tente novamente em breve."` (`mecontrola_agent.go:45`).
- Múltiplas intenções na mesma mensagem: `"Percebi mais de um lançamento na mesma mensagem. Por segurança, registro um de cada vez — me manda o primeiro (ex.: \"gastei 30 no ônibus\") que eu já cuido dele. 🙂"` (`mecontrola_agent.go:19,69`).

### Formatação e anti-simulação — regras verbatim aplicáveis à edição

- Negrito de WhatsApp com asterisco único `*texto*`; asterisco duplo `**texto**` é proibido (`mecontrola_agent.go:25-31`).
- Proibido afirmar sucesso sem retorno real da ferramenta; `isReplay=true` confirma sem re-gravar; proibido chamar ferramenta de escrita duas vezes para a mesma operação na mesma mensagem (`mecontrola_agent.go:39-46`).
- Proibido vazar termos internos (`workflow`, `thread`, `run`, `correlation`, `sistema interno`, `usecase`) — categoria golden `no_internal_terms` (`golden/case.go:22`, `golden/cases_ambiguity_format.go`).

## Regras de Negócio

- RN-01: A edição só é gravada após confirmação humana explícita (aceite reconhecido). Enquanto não confirmada, nenhuma mutação ocorre no ledger (`destructive_confirm_workflow.go:92-124`; contrato R-AGENT-WF-001.7-A).
- RN-02: A gravação usa controle otimista por `version`. Se a `version` informada não bater com a atual, a atualização falha sem efeito colateral (`transaction_repository.go` `UpdateWithVersion` WHERE `version = expectedVersion`; DTO exige `Version > 0` em `raw_update_transaction.go`).
- RN-03: Invariantes de domínio permanecem nos smart constructors, nunca no adapter/tool: valor em centavos maior que zero; descrição de 1 a 500 caracteres; parcelas entre 1 e 24; forma de pagamento `credit_card` exige `cardId` e direção `outcome`; migração para fora de `credit_card` é bloqueada quando há parcelas em aberto (`internal/transactions/domain/commands/update_transaction.go`, `valueobjects/*`, guard em `update_transaction.go` persist).
- RN-04: A descrição não pode ser parafraseada; deve refletir o termo literal do usuário para não quebrar a busca de categoria (`mecontrola_agent.go:20-21`).
- RN-05: Editar categoria dispara re-resolução via `classify_category`; múltiplos candidatos viram lista numerada para escolha do usuário; após um reprompt sem resolução, a edição é cancelada com a mensagem verbatim de categoria não identificada.
- RN-06: Editar para `credit_card` exige resolver o cartão por apelido via `resolve_card` antes de gravar; cartão não encontrado leva a listar cartões e pedir escolha; `cardId` nunca é inventado (`mecontrola_agent.go` regra de cartão; `resolve_card.go`).
- RN-07: A pendência de edição expira em 35 minutos e o gate de confirmação em até 5 minutos; expirado, a operação é cancelada sem efeito e o run é concluído, sem deixar estado órfão (`pending_entry_workflow.go:56`, `destructive_confirm_workflow.go` `confirmTTL`).
- RN-08: Resposta ambígua na confirmação gera um único reprompt; a segunda resposta ambígua cancela a operação sem efeito (`maxReprompts = 1`).
- RN-09: A operação é idempotente por `(wamid, itemSeq, operation)`; reenvio da mesma mensagem não gera segunda mutação e devolve desfecho de replay (`idempotent_write.go`; `ON CONFLICT` no repositório de transações).
- RN-10: Ao gravar a edição, o módulo publica `transactions.transaction.updated.v1` com os meses afetados (antigos e novos) e, para refletir troca de categoria, com `CategoryID`/`SubcategoryID` no payload; o `internal/budgets` consome esse evento e atualiza `budgets_expenses` via `UpsertExpense` com `MutationKindUpdate`, mantendo resumo mensal e alertas de limite consistentes (`events.go:26-35` a enriquecer; `budgets/module.go:144-145` a estender; `upsert_expense.go:150-218`).
- RN-11: Nenhuma resposta afirma que a edição foi salva sem o retorno efetivo da gravação; falha de infraestrutura responde exatamente `"Não consegui registrar. Tente novamente em breve."` e o run é marcado como falho (anti-simulação `mecontrola_agent.go:39-46`; scorer `write_persistence_accuracy` exige desfecho `Routed`/`Reconciled`).
- RN-12: Uma edição por mensagem. Ao detectar mais de uma intenção de lançamento/edição na mesma mensagem, o agente responde a frase verbatim de múltiplos lançamentos e não chama ferramenta de escrita.

## Critérios de Aceite

```gherkin
Cenário: Editar o valor de uma despesa com confirmação
  Dado que tenho uma despesa de R$ 50,00 registrada em "Alimentação > Restaurante"
  Quando eu envio "muda o último gasto para 65 reais"
  Então o agente responde com um resumo de confirmação no formato "Confirma? *...* R$ 65,00 em *Alimentação > Restaurante* para <data> no <forma>?"
  E nenhuma gravação ocorre antes da minha resposta

Cenário: Confirmar a edição grava via controle otimista
  Dado que recebi o resumo de confirmação da edição de valor
  Quando eu respondo "sim"
  Então o sistema chama UpdateTransaction com a version atual da transação
  E responde confirmando a atualização sem parafrasear a descrição
  E publica o evento transactions.transaction.updated.v1 com os meses afetados

Cenário: Cancelar a edição não altera nada
  Dado que recebi o resumo de confirmação da edição
  Quando eu respondo "não"
  Então o sistema responde "Tudo certo, o registro foi cancelado."
  E a transação permanece com os valores originais e a mesma version

Cenário: Resposta ambígua na confirmação gera um único reprompt e depois cancela
  Dado que recebi o resumo de confirmação da edição
  Quando eu respondo "talvez"
  Então o sistema reprompt uma vez pedindo "sim" ou "não"
  Quando eu respondo novamente algo não reconhecido
  Então o sistema cancela a operação sem efeito e conclui o run

Cenário: Editar a categoria dispara re-resolução e reflete no orçamento
  Dado que tenho uma despesa em "Alimentação > Restaurante"
  Quando eu envio "troca a categoria desse gasto para transporte" e confirmo
  Então o sistema re-resolve a categoria via classify_category
  E grava a nova categoria na transação
  E o evento de update carrega CategoryID e SubcategoryID
  E o consumo do orçamento em "Transporte" passa a incluir esse valor e o de "Alimentação" deixa de incluí-lo

Cenário: Editar categoria com múltiplos candidatos pede escolha
  Dado que informo uma categoria ambígua ao editar
  Quando o classify_category retorna mais de um candidato
  Então o sistema apresenta "Qual se encaixa melhor? 1. ... 2. ..."
  E aguarda minha escolha antes de confirmar a edição

Cenário: Migrar a forma de pagamento para crédito exige resolver o cartão
  Dado que tenho uma despesa paga em pix
  Quando eu envio "passa esse gasto para o crédito do nubank em 3x"
  Então o sistema resolve o cartão "nubank" via resolve_card antes de confirmar
  E o resumo de confirmação exibe "no crédito em 3x"
  E ao confirmar, as parcelas e a fatura são recompostas pelo domínio

Cenário: Cartão não encontrado ao migrar para crédito
  Dado que peço para migrar um gasto para o crédito de um cartão que não existe
  Quando o resolve_card retorna que não encontrou o cartão
  Então o sistema lista meus cartões e pede que eu escolha
  E não grava a edição com um cardId inventado

Cenário: Conflito de version bloqueia a gravação
  Dado que a transação foi alterada por outro caminho e a version mudou
  Quando eu confirmo uma edição baseada na version antiga
  Então o UpdateTransaction falha por incompatibilidade de version
  E o sistema não afirma sucesso e o run é concluído como falho

Cenário: Transação alvo inexistente
  Dado que peço para editar um lançamento que não existe ou não é meu
  Quando o sistema tenta localizar a transação
  Então nenhuma edição é gravada
  E o sistema informa que não localizou o lançamento

Cenário: Falha de infraestrutura na gravação não inventa sucesso
  Dado que recebi o resumo de confirmação e respondi "sim"
  Quando a gravação falha por erro de infraestrutura
  Então o sistema responde exatamente "Não consegui registrar. Tente novamente em breve."
  E não afirma que o lançamento foi atualizado

Cenário: Reenvio idempotente da mesma edição
  Dado que confirmei e gravei uma edição vinculada a um wamid
  Quando a mesma mensagem de confirmação é reprocessada com o mesmo wamid e itemSeq
  Então o sistema reconhece replay e não aplica uma segunda mutação
  E responde de forma consistente com a edição já efetivada

Cenário: Pendência de edição expira
  Dado que iniciei uma edição e o sistema aguarda um dado meu
  Quando eu não respondo dentro de 35 minutos
  Então a pendência expira e o sistema responde "O registro expirou. Para registrar, envie a informação completa novamente."

Cenário: Editar a data de um lançamento
  Dado que tenho um gasto lançado com data de hoje
  Quando eu envio "muda a data desse gasto para ontem" e confirmo
  Então o sistema grava a nova data
  E, se a competência mudar, o evento de update carrega os meses afetados para o orçamento recomputar

Cenário: Reverter a direção de um lançamento
  Dado que registrei algo como despesa por engano
  Quando eu envio "na verdade isso foi receita" e confirmo
  Então o sistema atualiza a direção respeitando as invariantes de domínio
  E rejeita a mudança se ela violar uma invariante (por exemplo, crédito exige direção de despesa)

Cenário: Mais de uma edição na mesma mensagem é bloqueada
  Dado que envio "muda o gasto do mercado para 40 e o do ônibus para 10"
  Quando o agente detecta múltiplas intenções na mesma mensagem
  Então ele responde a frase de múltiplos lançamentos verbatim
  E não chama nenhuma ferramenta de escrita

Cenário: Resposta sem formatação inválida e sem termos internos
  Dado qualquer resposta do agente durante a edição
  Quando a mensagem é montada para o WhatsApp
  Então ela usa negrito com asterisco único e não contém asterisco duplo
  E não vaza termos internos como "workflow", "run", "thread" ou "usecase"
```

## Dados e Permissões

- Dados obrigatórios para gravar a edição: identificador da transação alvo, `version` atual (controle otimista), e os campos alterados dentre direção, forma de pagamento, valor em centavos, descrição, `categoryId`/`subcategoryId`, `cardId` e parcelas, e data (`raw_update_transaction.go:9-63`).
- Dados derivados/recompostos pelo domínio: `refMonth` a partir da data; parcelas e deltas de fatura para transações de cartão; snapshot de categoria (`update_transaction.go`, `transaction_workflow.go` `DecideUpdate`).
- Evento e reflexo no orçamento: `transactions.transaction.updated.v1` com `RefMonthsAffected` e (a incluir) `CategoryID`/`SubcategoryID`; `budgets_expenses` atualizado por `UpsertExpense` com `ExpectedVersion` e `MutationKindUpdate`.
- Perfis/permissões: usuário autenticado como dono da transação; o principal é resolvido a partir da identidade inbound do WhatsApp (`transactions_ledger_adapter.go:57-70`, `auth.Principal` com `Source = WhatsApp`). A transação editada precisa pertencer ao mesmo `user_id`; edição de lançamento de outro usuário é negada pela cláusula `WHERE user_id = ...` do repositório.

## Dependências

- Domínio de transações: `UpdateTransaction` e `TransactionWorkflow.DecideUpdate` já existentes (`internal/transactions/application/usecases/update_transaction.go`, `internal/transactions/domain/services/transaction_workflow.go`).
- Enriquecimento do evento `TransactionUpdated` com `CategoryID`/`SubcategoryID` (`internal/transactions/domain/entities/events.go:26-35`) e ajuste do produtor correspondente (`transaction_event_publisher.go`).
- Novo consumidor em `internal/budgets` para `transactions.transaction.updated.v1`, registrado em `internal/budgets/module.go` (hoje só há `created`/`deleted` em `module.go:144-145`), reusando `UpsertExpense` (`upsert_expense.go:150-218`) e o molde de `transaction_created_consumer.go`.
- Substrato de agente/workflow/tool: `workflow.Engine[S]`, `tool.NewTool`/`NewVerbatimTool`, `AgentRuntime`, `ThreadGateway`, `IdempotentWrite`, `classify_category`, `resolve_card` (já existentes; apenas consumidos).
- Governança obrigatória: R-AGENT-WF-001 (roteamento por registry, tool fina, estados fechados, LLM só nas call-sites sancionadas, Run auditável, pending step antes da confirmação), R-ADAPTER-001 (zero comentários, adapter fino), R-TXN-WORKFLOWS-001 (regra em `Decide*`, cardinalidade de métricas), R-DTO-VALIDATE-001 (input DTO com `Validate()`).

## Fora de Escopo

- Criar transação por conversa: já implementado (fluxo `pending_entry` + `register_expense`/`register_income`), tratado aqui apenas como pré-condição e molde de reuso.
- Excluir transação por conversa: coberto por `delete_entry` + `destructive_confirm`, fora desta entrega.
- Edição em massa ou por lote de múltiplas transações numa única mensagem.
- Edição via API HTTP (o handler `UpdateTransactionHandler` já existe e não é alvo desta história conversacional).
- Edição de templates de recorrência (coberta por `update_recurrence`/`delete_recurrence`).
- Reconciliação retroativa de faturas de cartão já fechadas além do que `DecideUpdate` já recompõe.

## Evidências

- Entrada: solicitação do usuário — editar transações considerando `internal/transactions` e `internal/budgets`, com paridade de campos, cobertura integral das possibilidades de conversa e sem inventar respostas; escopo confirmado como "Editar por conversa (paridade total)".
- Base de código:
  - Tool de edição parcial (3 campos): `internal/agents/application/tools/edit_entry.go:30-60`.
  - Use case de domínio completo: `internal/transactions/application/usecases/update_transaction.go:53-160`; DTO `internal/transactions/application/dtos/input/raw_update_transaction.go:9-63`.
  - Workflow de slot-filling reusável: `internal/agents/application/workflows/pending_entry_workflow.go:56,812,849-887,889-926`; decisões `internal/agents/application/workflows/pending_entry_decisions.go:29,115-122,254-277`.
  - Gate HITL: `internal/agents/application/workflows/destructive_confirm_workflow.go:92-124,202,226-238,313-328`.
  - Binding: `internal/agents/infrastructure/binding/transactions_ledger_adapter.go:57-70,112-146`.
  - Idempotência: `internal/agents/application/usecases/idempotent_write.go:59-145`; repositório `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go:41-98`.
  - Reflexo no orçamento (gaps): consumidores registrados só para created/deleted em `internal/budgets/module.go:144-145`; evento de update sem categoria em `internal/transactions/domain/entities/events.go:26-35`; caminho de update do orçamento em `internal/budgets/application/usecases/upsert_expense.go:150-218`.
  - Mensagens verbatim: prompt do agente em `mecontrola_agent.go:19,25-31,39-46,57,69` (pacote `agents` da camada application em `internal/agents`); `internal/agents/application/golden/fixtures_text.go`; enum de categorias golden `internal/agents/application/golden/case.go:10-22`.
- Inferências: o resumo de confirmação da edição reusa o formato de `buildConfirmSummary` (hoje usado na criação); o texto exato para o gate de edição a ser exibido segue o padrão verbatim de `impactNote`/`destructive_confirm` já existente, não um texto novo inventado.
- Não evidenciado: consumidor de `transactions.transaction.updated.v1` em `internal/budgets` (busca executada em `internal/budgets/` sem resultado); presença de `CategoryID`/`SubcategoryID` no struct `TransactionUpdated` (ausentes em `events.go:26-35`). Ambos tratados como itens de entrega/dependência, não como capacidade existente.

## Notas de Validação

- Cobertura de cenários: fluxo feliz (editar valor/categoria/data/pagamento/direção), fluxos alternativos (múltiplos candidatos de categoria, migração para crédito, competência alterada) e fluxos de erro/bloqueio (cancelamento, ambiguidade com reprompt único, expiração, conflito de version, alvo inexistente, falha de infraestrutura sem falso sucesso, idempotência, múltiplas intenções). Atende aos critérios de fluxo feliz, alternativo e erro exigidos.
- Gate de qualidade sugerido antes de concluir a implementação desta US: build, vet, test race e lint do projeto; suíte golden real-LLM cobrindo os novos cenários de edição com razão de acerto mínima por categoria; scorer `write_persistence_accuracy` verde para desfechos de escrita `Routed`/`Reconciled`.
- Rastreabilidade: cada regra de negócio e mensagem cita arquivo e linha; itens sem evidência foram declarados na subseção "Não evidenciado" e convertidos em dependências, sem afirmar existência sem prova.
