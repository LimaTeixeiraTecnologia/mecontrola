# Editar Cartão pela Conversa (WhatsApp) — User Story única, pronta para desenvolvimento

> Fonte: pedido do usuário (criar/editar cartão em `internal/agents` + `internal/card`) confrontado com a base de código real do repositório `mecontrola`.
> Data de geração: 2026-07-10
> Nome do arquivo: `2026-07-10-us-editar-cartao-conversacional.md`

## Resumo e decisão de escopo

O pedido original cobria criar e editar cartão. Ao confrontar a base de código, a **criação já está implementada e robusta**: workflow dedicado `card-create-confirm`, confirmação humana (HITL), TTL de 15 minutos, escrita idempotente, reaper e mensagens determinísticas (`internal/agents/application/workflows/card_create_confirm_workflow.go:26-191`). O trabalho de desenvolvimento pendente e os defeitos estão na **edição**. Por isso, atendendo ao pedido de "uma única US robusta e pronta para desenvolvimento", esta história foca em **editar cartão pela conversa**, espelhando o padrão comprovado da criação. A criação entra como referência e dependência, não como trabalho a construir.

Esta única US consolida todas as possibilidades de conversação de edição (identificação, confirmação, aceitar, cancelar, ambíguo, expirar, pendência, sucesso, replay idempotente, erros de domínio, versão divergente e no-false-success) em critérios de aceite verificáveis.

## Confronto com o Codebase

Comportamento **atual** (evidenciado) da edição:

- Tool `update_card` exige `cardId` e `version` (`internal/agents/application/tools/update_card.go:20-26,49`).
- Alterar apenas `nickname`/`bank` executa a escrita direto, sem confirmação e sem idempotência (`update_card.go:98-116`).
- Alterar `due_day` pede confirmação e reusa o workflow compartilhado `destructive-confirm` (TTL 5 min) (`update_card.go:118-170`, `internal/agents/application/workflows/destructive_confirm_workflow.go:19,64-126`).
- Gate de confirmação compartilhado trata sim/não/ambíguo/expira (`destructive_confirm_workflow.go:83-126,200-242`) e executa via `executeUpdateCard` (`destructive_confirm_workflow.go:355-371`).
- Identificação: `resolve_card` (`internal/agents/application/tools/resolve_card.go:20-23,53`), `list_cards` (`list_cards.go:18-28`), `get_card` (`get_card.go:20-59`).
- Módulo de suporte: use case `internal/card/application/usecases/update_card.go`, serviço puro `internal/card/domain/services/decide_update_card.go`, value objects e erros de domínio (`internal/card/domain/valueobjects/*`, `internal/card/domain/errors.go`), repositório com incremento de versão (`internal/card/infrastructure/repositories/postgres/card_repository.go`).

Gaps e defeitos **confirmados** que esta US fecha:

1. `update_card` exige `version`, mas `resolve_card`/`list_cards`/`get_card` não expõem `version` — o agente não tem como obtê-lo sem inventar (proibido). (`update_card.go:22,49` vs. `resolve_card.go:20-23,53`, `list_cards.go:18-28`, `get_card.go:20-59`)
2. Provável defeito: ao confirmar mudança de vencimento, o payload serializado omite `DueDay`, então o novo vencimento pode não ser persistido. (`update_card.go:118-122` + `destructive_confirm_workflow.go:363-369`)
3. Assimetria de robustez: edição de apelido/banco muta sem confirmação e sem idempotência; criação sempre confirma e é idempotente.
4. TTL divergente (edição 5 min vs. criação 15 min).
5. Sem cobertura golden/real-LLM para edição (`internal/agents/application/golden/cases_card.go` só cobre leitura/registro — relatado na exploração).

## Análise de possibilidade de Workflow (pedido explícito)

A criação já é um workflow durável dedicado; a edição não. Como editar cartão tem exatamente as mesmas necessidades de produção (confirmação humana antes de mutar, retomada durável, TTL, idempotência por `wamid`, mensagem final determinística), a decisão-alvo confirmada é: **a edição vira um workflow dedicado `card-update-confirm`, simétrico ao de criação**, consumindo o kernel `internal/platform/workflow` (regra de ouro da skill `mastra`: consumir o substrato, não recriá-lo). Isso elimina a mutação direta sem confirmação, corrige o payload de `due_day`, alinha o TTL e fecha o gap de idempotência.

---

## Declaração
Como usuário do MeControla no WhatsApp, quero editar apelido, banco ou dia de vencimento de um cartão já cadastrado conversando com o agente, confirmando antes de qualquer alteração, para que meus cartões fiquem corretos com segurança, sem mudanças silenciosas e sem que o agente afirme algo que não foi realmente gravado.

## Contexto
- Problema: hoje a edição é assimétrica e frágil — apelido/banco mudam sem confirmação e sem idempotência, a alteração de vencimento pode não persistir por um defeito de payload, e o agente não consegue obter a versão exigida sem inventá-la.
- Resultado esperado: uma capacidade de edição robusta pela conversa, com identificação segura do cartão (incluindo versão), confirmação humana para toda alteração, persistência idempotente, recálculo correto do fechamento quando o vencimento muda e mensagens determinísticas de sucesso e erro.
- Fonte: pedido do usuário e confronto com a base de código (`internal/agents/application/tools/update_card.go`, `internal/agents/application/workflows/destructive_confirm_workflow.go`, `internal/card/application/usecases/update_card.go`).

## Regras de Negócio
- Identificação segura: antes de editar, o agente resolve o cartão por apelido (`resolve_card`) ou por lista (`list_cards`)/detalhe (`get_card`), e obtém o `cardId` e a `version` atuais; o agente nunca inventa `cardId` nem `version` (prompt do agente `mecontrola_agent.go:40,95`, pacote `agents` da camada application em `internal/agents`).
- Estado-alvo obrigatório: `resolve_card`, `list_cards` e `get_card` passam a expor o campo `version` (aditivo; a entidade já mantém `Version` em `internal/card/domain/entities/card.go`), fechando o gap que hoje impede obter a versão para `update_card` (`update_card.go:22,49`).
- Confirmação universal: qualquer alteração (`nickname`, `bank` e/ou `due_day`) exige confirmação humana explícita antes de mutar, removendo a execução direta atual de apelido/banco (`update_card.go:98-116`).
- Campos editáveis: `nickname`, `bank`, `due_day`. `closing_day` permanece derivado e é recalculado pelo módulo card quando `bank` ou `due_day` mudam (`internal/card/application/usecases/update_card.go`, `internal/card/domain/services/purchase_day.go`); não é editável isoladamente. Não existe conceito de limite de crédito no domínio.
- Nota de impacto: alterar o dia de vencimento exibe "A alteração do dia de vencimento pode impactar parcelas em aberto." (`update_card.go:128`).
- Correção de defeito: o payload de confirmação DEVE incluir `due_day`, para que a execução após o aceite persista o novo vencimento (`update_card.go:118-122` hoje omite `DueDay`; `executeUpdateCard` desserializa o payload em `destructive_confirm_workflow.go:363-369`).
- Semântica de confirmação (espelha o gate existente): aceite reconhece "sim", "confirmar", "confirmo", "ok", "pode", "yes", "s"; cancelamento reconhece "não", "nao", "cancelar", "cancelo", "no", "n" (`destructive_confirm_workflow.go:226-242`). Resposta ambígua re-pergunta uma vez ("Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar a operação.") e cancela na segunda (`destructive_confirm_workflow.go:108-124`).
- Persistência do estado de espera antes de perguntar: o estado de confirmação é salvo no snapshot do kernel antes de devolver a pergunta, e o resume aplica merge-patch antes do parse (padrão do workflow durável, `card_create_confirm_workflow.go:38-49`).
- TTL alinhado ao de criação (15 minutos), substituindo os 5 minutos atuais do gate compartilhado (`card_create_decisions.go:9` vs. `destructive_confirm_workflow.go:19`).
- Idempotência: a escrita de edição usa `IdempotentWriter` com `operation="update_card"`, `resourceKind="card"` e `wamid` original, espelhando a criação (`card_create_confirm_workflow.go:126`); reenvio do mesmo `wamid` não aplica a edição duas vezes.
- Pendência: confirmação já aberta bloqueia novo pedido com "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outra operação." (`update_card.go:148-155`).
- Mensagens determinísticas: sucesso responde "✅ Cartão atualizado com sucesso." (`destructive_confirm_workflow.go:219-220`); cancelamento responde "🚫 Operação cancelada conforme solicitado." (`destructive_confirm_workflow.go:102-106`).
- Erros de domínio classificados com mensagem determinística: apelido em uso ao renomear (unique index `cards_user_nickname_active_uniq_idx`, `internal/card/domain/errors.go:7`), vencimento inválido, cartão não encontrado e versão divergente (lock otimista). Erro não relacionado a regra de negócio marca o Run como falho, sem afirmar sucesso.
- No-false-success: o agente nunca afirma que atualizou o cartão sem o retorno real da ferramenta, repassa textos determinísticos verbatim e nunca menciona termos de infraestrutura ("workflow", "pendência", "sistema interno") (`mecontrola_agent.go:41,74,84`).

## Critérios de Aceite
```gherkin
Cenário: Identificar o cartão pelo apelido retorna identificador e versão
  Dado que o usuário quer editar o cartão chamado "Nubank"
  Quando o agente resolve o cartão pelo apelido
  Então o resultado indica que o cartão foi encontrado e fornece o identificador e a versão atual

Cenário: Apelido não encontrado oferece a lista de cartões
  Dado que o usuário cita um apelido que não corresponde a nenhum cartão ativo
  Quando o agente resolve o cartão pelo apelido
  Então o resultado indica que não foi encontrado e o agente oferece listar os cartões para o usuário escolher

Cenário: Alterar apelido pede confirmação antes de mutar
  Dado que o usuário pede para renomear um cartão já identificado
  Quando o agente aciona a edição de cartão
  Então o sistema devolve uma pergunta de confirmação e não altera o cartão antes da resposta

Cenário: Alterar vencimento pede confirmação com nota de impacto
  Dado que o usuário pede para mudar o dia de vencimento de um cartão identificado
  Quando o agente aciona a edição de cartão
  Então a resposta contém "A alteração do dia de vencimento pode impactar parcelas em aberto." e pede confirmação com sim ou não

Cenário: Usuário confirma e a edição é efetivada
  Dado que existe uma confirmação de edição pendente
  Quando o usuário responde "sim"
  Então a alteração é persistida e a resposta é exatamente "✅ Cartão atualizado com sucesso."

Cenário: Confirmar alteração de vencimento persiste o novo dia e recalcula o fechamento
  Dado que o usuário confirmou a alteração do vencimento para um novo dia
  Quando o sistema executa a atualização a partir do estado persistido
  Então o dia de vencimento gravado é o novo dia informado, não o anterior
  E o dia de fechamento é recalculado de acordo com o banco

Cenário: Usuário cancela a edição
  Dado que existe uma confirmação de edição pendente
  Quando o usuário responde "não"
  Então a resposta é exatamente "🚫 Operação cancelada conforme solicitado."
  E o cartão permanece inalterado

Cenário: Resposta ambígua repergunta uma vez e depois cancela
  Dado que existe uma confirmação de edição pendente
  Quando o usuário responde algo que não é sim nem não pela primeira vez
  Então a resposta é exatamente "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar a operação."
  E se o usuário responder de forma ambígua novamente a operação é cancelada com "🚫 Operação cancelada: resposta não reconhecida."

Cenário: Confirmação pendente bloqueia novo pedido
  Dado que já existe uma confirmação de edição pendente para o usuário
  Quando o usuário solicita outra operação de cartão
  Então a resposta é exatamente "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outra operação."

Cenário: Expiração após quinze minutos não altera o cartão
  Dado que existe uma confirmação de edição pendente há mais de quinze minutos
  Quando o usuário envia qualquer mensagem
  Então a edição é encerrada sem efeito e a mensagem segue o fluxo normal do agente

Cenário: Reenvio idempotente não aplica a edição duas vezes
  Dado que a mesma mensagem de confirmação de edição já foi processada com sucesso
  Quando a atualização é executada novamente com o mesmo identificador de mensagem
  Então a alteração não é aplicada de novo e a resposta confirma de forma determinística que o cartão já estava atualizado

Cenário: Renomear para apelido já em uso é rejeitado com mensagem clara
  Dado que o usuário pede para renomear um cartão com um apelido já usado por outro cartão ativo dele
  Quando a edição é confirmada e executada
  Então a resposta informa de forma determinística que já existe um cartão com esse apelido
  E o cartão não é alterado

Cenário: Versão divergente orienta nova tentativa
  Dado que a versão enviada na edição não corresponde à versão atual do cartão
  Quando a atualização é executada
  Então a resposta orienta o usuário a tentar novamente com os dados atuais
  E o cartão não é alterado

Cenário: Falha transitória não afirma sucesso
  Dado que a persistência da edição falha por erro não relacionado a regra de negócio
  Quando o sistema executa a atualização
  Então o agente não afirma que o cartão foi atualizado
  E a execução é registrada como falha
```

## Dados e Permissões
- Dados obrigatórios: `cardId`, `version`, ao menos um entre `nickname` (1..32), `bank` (não-vazio) e `dueDay` (1..31); `wamid` (identificador da mensagem) para idempotência; identidade do usuário derivada de `req.ResourceID` (`update_card.go:88-96`).
- Perfis/permissões: usuário final autenticado no canal WhatsApp; toda leitura e escrita é restrita aos cartões do próprio usuário (`UpdateCard`, `GetByIDForUser` filtram por `user_id`; resume só para o mesmo recurso/thread que abriu a confirmação).

## Dependências
- Kernel de workflow `internal/platform/workflow` (`Engine[S]`, suspend/resume por merge-patch) e runtime Thread→Run que injeta `agent.InboundRequest` no contexto.
- `IdempotentWriter` (`internal/agents/application/workflows/pending_entry_workflow.go`) e binding `internal/agents/infrastructure/binding/card_manager_adapter.go` delegando a `UpdateCard` do módulo card.
- Módulo `internal/card`: use case `application/usecases/update_card.go`, serviço `domain/services/decide_update_card.go` e `purchase_day.go`, value objects e erros de domínio, repositório com incremento de `version`.
- Padrão de referência a espelhar: workflow de criação `card_create_confirm_workflow.go` e continuer `card_create_confirm_continuer.go`.
- Ordem de resume no consumer WhatsApp (`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`), que deve incluir o novo resume de edição antes do agente, análogo ao da criação.

## Fora de Escopo
- Criação de cartão (já implementada; entra apenas como referência de padrão).
- Exclusão de cartão (feita via `delete_entry` com `targetKind="card"`, `destructive_confirm_workflow.go:331-337`).
- Tornar `closing_day` editável diretamente e introduzir limite de crédito.
- Contrato REST de edição em `internal/card` (`PUT /cards/{id}`), que já existe como adapter e não é o canal-alvo desta história.

## Evidências
- Entrada: pedido do usuário para uma única US robusta e pronta para desenvolvimento sobre editar cartão pela conversa, cobrindo todas as possibilidades, sem inventar resposta.
- Base de código: `internal/agents/application/tools/update_card.go:20-26,49,88-170` (contrato, confirmação por `due_day`, execução direta, pendência, montagem do payload); `internal/agents/application/workflows/destructive_confirm_workflow.go:19,64-126,200-242,355-371` (gate de confirmação e execução do update); `internal/agents/application/tools/{resolve_card.go:20-23,53,list_cards.go:18-28,get_card.go:20-59}` (identificação sem `version`); `internal/agents/application/workflows/card_create_confirm_workflow.go:26-191` e `card_create_decisions.go:9` (padrão de referência: idempotência, TTL, no-false-success); prompt do agente `mecontrola_agent.go:40-46,74,84,95` no módulo `internal/agents` (regras de não inventar e no-false-success); `internal/card/domain/errors.go:7`, `internal/card/domain/entities/card.go` (unique de apelido e `Version`).
- Inferências: o workflow dedicado `card-update-confirm`, a confirmação universal (inclusive apelido/banco), a exposição aditiva de `version` nas ferramentas de identificação, a correção do payload de `due_day` e o alinhamento de TTL são propostas de estado-alvo, espelhadas do padrão comprovado de criação. Estão separadas do comportamento atual nesta seção.
- Não evidenciado: hoje não existe `operation="update_card"` no caminho idempotente, nem `isCardUpdateDomainError`, nem workflow `card-update-confirm`, nem `version` nos outputs de identificação, nem caso golden/real-LLM de edição de cartão — são artefatos a criar. O agendamento do reaper e a cadeia exata de resume no consumer vieram do relatório de exploração, não de leitura linha a linha.

## Notas de Validação
- A história cobre fluxo feliz (identificar → confirmar → sucesso), variações válidas (apelido não encontrado → listar; alteração de vencimento com recálculo; replay idempotente) e erros/bloqueios (cancelar, ambíguo, pendência, expiração, apelido em uso, versão divergente, falha transitória sem falso sucesso). Os ramos de conversa de edição estão representados em critérios de aceite verificáveis.
- Robustez para desenvolvimento: consumir o kernel de workflow (não recriar), estado como tipo fechado (DMMF state-as-type), regra em `Decide*` puro, validação só em smart constructors, tools como adapters finos e zero comentários em `.go` (go-implementation, mastra, domain-modeling-production, design-patterns-mandatory). Nenhum novo padrão GoF é necessário além dos já usados (Workflow/Step, Adapter, Factory Function).
- Prontidão de teste: além de testes unit/integration análogos aos da criação, incluir caso golden/real-LLM de edição para fechar a lacuna de cobertura comportamental, com gate por categoria conforme prática do repositório.

## Notas Técnicas para Desenvolvimento

- Criar `internal/agents/application/workflows/card_update_confirm_workflow.go` (ID `card-update-confirm`), com estado fechado `CardUpdateState` (campos: `cardId`, `version`, `nickname`/`bank`/`dueDay` opcionais, `wamid`, `awaiting`, `reprompt`, `suspendedAt`, `status`), espelhando `card_create_state.go` e `card_create_confirm_workflow.go`.
- Reescrever a tool `update_card` para sempre iniciar o workflow (remover a execução direta de `update_card.go:98-116`) e incluir `dueDay` no payload/estado (corrige `update_card.go:118-122`).
- Adicionar campo `version` (aditivo) aos outputs de `resolve_card`, `list_cards` e `get_card`, propagando o `Version` da entidade do módulo card.
- Implementar `isCardUpdateDomainError` classificando `ErrNicknameConflict`, `ErrInvalidNickname`, `ErrInvalidDueDay`, `ErrInvalidBank`, `ErrCardNotFound` e conflito de versão, com mensagens determinísticas (molde: `cardCreateDomainErrorMessage`, `card_create_confirm_workflow.go:176-191`).
- Envolver a escrita com `IdempotentWriter.Execute(..., "update_card", "card", writeFn, isCardUpdateDomainError)` usando o `wamid` original; marcar `StepStatusFailed` em erro não-domínio (molde: `executeCreateCard`).
- Registrar o resume de edição na cadeia do `whatsapp_inbound_consumer.go` e adicionar reaper/housekeeping para runs suspensos, análogo ao de criação; alinhar TTL a 15 minutos.
- Métrica de Run com labels de cardinalidade controlada (`agent_id`, `channel`, `workflow`, `status`, `outcome`), sem `user_id`/`card_id` (R-AGENT-WF-001.5).
