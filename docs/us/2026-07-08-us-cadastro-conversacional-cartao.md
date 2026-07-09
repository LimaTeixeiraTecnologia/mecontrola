# US-01: Cadastro conversacional de cartão de crédito pelo WhatsApp

## Declaração
Como usuário ativo do MeControla no WhatsApp, quero cadastrar um novo cartão de crédito conversando com o agente, para registrar minhas compras no cartão sem sair da conversa nem precisar do app.

## Contexto
- Problema: o agente conversacional diário não possui capacidade de cadastrar cartão (o cadastro só existe no onboarding, em `internal/agents/application/workflows/onboarding_workflow.go:681`) e não há guardrail contra o pedido direto. No incidente de produção de 2026-07-08, o usuário `f56e1142-0960-4dd9-aa09-955aa519fee1` (+5511986896322) pediu "Quero cadastrar um cartão, XP, banco XP e o vencimento dia 1"; o LLM improvisou a confirmação "Vamos cadastrar seu cartão! Confirma? Cartão XP do banco XP com vencimento no dia 1?", o usuário respondeu "Sim" e recebeu "Não consegui cadastrar o cartão. Tente novamente em breve." — sem nenhuma escrita, sem tool call e sem erro persistido.
- Resultado esperado: o usuário consegue cadastrar um cartão pela conversa, com confirmação explícita antes da escrita, mensagens acionáveis em cada falha e cálculo correto do dia de fechamento inclusive para bancos fora da lista suportada; o agente nunca mais responde sucesso ou falha de cadastro sem executar a ferramenta de escrita.
- Fonte: transcrição de produção fornecida pelo usuário e investigação via SSH em `187.77.45.48` (Postgres `platform_runs`/`platform_messages`, logs de container e inspeção do binário deployado `ghcr.io/limateixeiratecnologia/mecontrola@sha256:e5d1ff02...`), confrontada com a base de código local (branch `main`).

## Regras de Negócio
- RN1 — O cadastro por chat é executado por uma nova ferramenta `create_card`, seguindo o padrão `tool.NewTool[I,O]` já usado pelas demais ferramentas em `internal/agents/application/tools/` (ex.: `resolve_card.go`, `update_card.go`). A ferramenta é adapter fino: valida o input contra o schema, mapeia para o comando e delega a `CardManager.CreateCard` (`internal/agents/infrastructure/binding/card_manager_adapter.go:58`), que já chama o usecase `CreateCard` (`internal/card/application/usecases/create_card.go:49`). É proibida qualquer regra de negócio, SQL direto ou branching de domínio na ferramenta (R-ADAPTER-001.2 e R-AGENT-WF-001.2).
- RN2 — Antes de qualquer escrita, é obrigatória a confirmação humana explícita usando o mecanismo do kernel (`workflow.SuspendAwaitingInput` + resume por merge-patch) e o padrão de estado de espera fechado exemplificado por `internal/agents/application/workflows/pending_entry_workflow.go` (`AwaitingSlotConfirmation`, `handleConfirmationResume`, TTL `PendingEntryStaleAfter = 35 * time.Minute`). Atenção: esse workflow existente é acoplado a transações — `handleConfirmationResume` recebe `interfaces.TransactionsLedger`, `categoryValidator` e `IdempotentWriter` (`pending_entry_workflow.go:353`) e `executeWithIdempotency` escreve pelo ledger de transações (`pending_entry_workflow.go:408,444`) — portanto o cadastro de cartão exige um estado de espera próprio (card-scoped), não o reuso literal deste workflow. O estado de espera é um tipo fechado (DMMF state-as-type, nunca string solta), persistido no `Snapshot` do kernel antes de a pergunta de confirmação ser enviada; o resume aplica merge-patch antes do parse (R-AGENT-WF-001.7 e contrato comportamental do addendum .7-A).
- RN3 — Semântica da confirmação: resposta afirmativa explícita ("sim", "confirmar", "ok", "pode") executa a criação; resposta negativa explícita ("não", "cancelar") descarta sem efeito; resposta ambígua re-pergunta uma única vez e, na segunda ambiguidade, cancela sem efeito; expiração de TTL avaliada no resume cancela sem efeito e devolve o texto do usuário ao fluxo normal. Após efetivar, cancelar ou expirar, o run é concluído — nunca permanece suspenso.
- RN4 — Slot-filling: apelido do cartão, banco e dia de vencimento são obrigatórios; o agente pergunta apenas o dado que ainda falta, espelhando a diretriz de perguntar somente o pendente já usada no fluxo de lançamento (`mecontrola_agent.go:62`, em `internal/agents`, camada application).
- RN5 — Cálculo do dia de fechamento: para banco reconhecido na tabela `mecontrola.banks` (hoje: banco-do-brasil, bradesco, c6-bank, caixa, inter, itau, nubank, santander), o `closing_day` é derivado por `PurchaseDayService.Decide` a partir do `daysBeforeDue` do banco (`internal/card/application/usecases/create_card.go:91-96`). Para banco não reconhecido, o agente pergunta ao usuário o dia de fechamento da fatura e usa o valor informado; é proibido usar o fallback silencioso de 7 dias que hoje ocorre para bancos desconhecidos.
- RN6 — Validações de domínio são reaproveitadas dos smart constructors existentes, sem duplicação: `due_day` entre 1 e 31 (`internal/card/application/dtos/input/create_card.go`, `CreateCard.Validate`), apelido entre 1 e 32 caracteres (`internal/card/domain/valueobjects/nickname.go`), banco não vazio (`NewBankCode`), `closing_day` entre 1 e 31 (`NewBillingCycle`). Cada falha de validação vira uma mensagem acionável específica ao usuário.
- RN7 — Unicidade: já existe um cartão ativo com o mesmo apelido para o usuário aciona `ErrNicknameConflict`, garantido pelo índice único parcial `cards_user_nickname_active_uniq_idx` sobre `(user_id, nickname) WHERE deleted_at IS NULL` (`migrations/000001_initial_schema.up.sql`). Nesse caso o agente informa que o apelido já está em uso e não cria duplicata.
- RN8 — Guardrail anti-alucinação: o agente nunca afirma "cadastrei" nem "não consegui cadastrar" sem ter invocado a ferramenta `create_card`; cada pedido de cadastro de cartão passa pela ferramenta (com confirmação) ou por um redirecionamento determinístico com orientação clara. Isso fecha diretamente o gap do incidente, em que a resposta de falha foi gerada pelo LLM sem tool call (mensagem `platform_messages` gerada ~2ms após o "Sim", `parts` vazio, nenhuma mensagem `role=tool`).
- RN9 — Idempotência: reenvio da mensagem de confirmação ou repetição do "Sim" não cria um segundo cartão. O usecase já contém o bloco de idempotência (`internal/card/application/usecases/create_card.go:115-132`), porém ele só é ativado quando um contexto de idempotência é injetado (`idempotency.FromContext`, `create_card.go:80`) — hoje esse bloco está dormente para cadastro de cartão, pois o único chamador (onboarding) não injeta contexto (comprovado em produção: 0 linhas em `idempotency_keys` e `agents_write_ledger` para o usuário/hora). Portanto a nova ferramenta DEVE injetar o contexto de idempotência (escopo `create_card`, chave derivada do `wamid` da mensagem de confirmação) para ativar o bloco existente e retornar a mesma resposta em replay.
- RN10 — Observabilidade e auditoria: toda execução de cadastro é um run auditável com erro real persistido — hoje o incidente ficou como falha silenciosa (coluna `error` do run vazia, nenhum span em Tempo, nenhuma métrica em Prometheus, nenhum registro em `idempotency_keys`/`agents_write_ledger`). O usecase já emite log estruturado `card.create.failed` (`internal/card/application/usecases/create_card.go:142`), mas NÃO emite métrica alguma (comprovado: não existe `agents_write_total{operation="create_card"}` em Prometheus; só há `operation="register_expense"`). Portanto a nova ferramenta DEVE (a) preencher a coluna de erro do run em qualquer falha e (b) emitir métrica de escrita seguindo o padrão de `agents_write_total` (`internal/agents/application/usecases/idempotent_write.go:35`), com cardinalidade controlada — labels `operation`/`outcome`, sem `user_id` (R-AGENT-WF-001.5 e R-TXN-004).
- RN11 — Escopo por usuário: o cartão é sempre criado para o `user_id` do principal autenticado da conversa; o agente nunca aceita `user_id` vindo do conteúdo da mensagem, preservando isolamento entre usuários (IDOR-safe).

## Critérios de Aceite
```gherkin
Cenário: Cadastro com banco reconhecido e confirmação afirmativa
  Dado que sou um usuário ativo no WhatsApp sem cartão com o apelido "Nu"
  E que "nubank" é um banco reconhecido na tabela de bancos
  Quando eu digito "cadastrar cartão Nu, banco Nubank, vencimento dia 10"
  Então o agente persiste o estado de espera e me pergunta a confirmação com apelido, banco e dia de vencimento
  E quando eu respondo "Sim"
  Então a ferramenta create_card é invocada e um cartão é criado para o meu user_id com o dia de fechamento derivado do banco
  E o agente confirma o cadastro com sucesso

Cenário: Banco não reconhecido pede o dia de fechamento antes de confirmar
  Dado que "XP" não é um banco reconhecido na tabela de bancos
  Quando eu digito "cadastrar cartão XP, banco XP, vencimento dia 1"
  Então o agente me pergunta o dia de fechamento da fatura em vez de usar um fallback silencioso de 7 dias
  E quando eu informo o dia de fechamento e respondo "Sim" na confirmação
  Então o cartão é criado com o closing_day igual ao valor que informei

Cenário: Slot-filling do dado pendente
  Dado que eu envio "cadastrar cartão Nu do Nubank" sem informar o dia de vencimento
  Quando o agente identifica que só falta o dia de vencimento
  Então o agente pergunta somente o dia de vencimento, sem repetir apelido nem banco

Cenário: Confirmação negada não cria cartão
  Dado que o agente já me apresentou a confirmação do cadastro
  Quando eu respondo "não"
  Então nenhum cartão é criado, o run é concluído e o agente informa que o cadastro foi cancelado

Cenário: Resposta ambígua re-pergunta uma vez e depois cancela
  Dado que o agente já me apresentou a confirmação do cadastro
  Quando eu respondo algo ambíguo pela primeira vez
  Então o agente re-pergunta a confirmação uma única vez
  E quando eu respondo algo ambíguo pela segunda vez
  Então o cadastro é cancelado sem efeito e o run é concluído

Cenário: Confirmação expirada por TTL
  Dado que o estado de confirmação do cadastro expirou pelo TTL
  Quando eu envio uma nova mensagem
  Então o cadastro é cancelado sem efeito e o meu texto segue para o fluxo normal do agente

Cenário: Apelido já existente é bloqueado com mensagem clara
  Dado que já tenho um cartão ativo com o apelido "Nu"
  Quando eu confirmo o cadastro de outro cartão com o apelido "Nu"
  Então o agente responde que o apelido já está em uso e nenhum cartão duplicado é criado

Cenário: Dia de vencimento inválido é rejeitado
  Dado que eu informo o dia de vencimento 32
  Quando o agente processa o pedido de cadastro
  Então o agente rejeita com uma mensagem acionável indicando que o dia deve estar entre 1 e 31, sem criar cartão

Cenário: Regressão do incidente — nunca responder sem tool call
  Dado que eu peço para cadastrar um cartão
  Quando o agente for responder sucesso ou falha do cadastro
  Então a resposta só ocorre após a invocação da ferramenta create_card
  E em qualquer falha o erro real fica persistido no run e no log estruturado, nunca como falha silenciosa com erro vazio

Cenário: Idempotência na confirmação repetida
  Dado que eu confirmei o cadastro e o cartão foi criado
  Quando o "Sim" é reenviado dentro da janela de idempotência
  Então nenhum segundo cartão é criado e a mesma resposta de sucesso é retornada
```

## Dados e Permissões
- Dados obrigatórios: apelido do cartão (1 a 32 caracteres), banco, dia de vencimento (1 a 31); dia de fechamento da fatura (1 a 31) somente quando o banco não é reconhecido na tabela `mecontrola.banks`.
- Perfis/permissões: usuário final autenticado do MeControla no WhatsApp (principal da conversa); o cadastro é sempre escopado ao `user_id` do principal, sem aceitar `user_id` do conteúdo da mensagem.

## Dependências
- Usecase, binding e tabela existentes: `CreateCard` (`internal/card/application/usecases/create_card.go`), `cardManagerAdapter.CreateCard` (`internal/agents/infrastructure/binding/card_manager_adapter.go:58`) e tabela `mecontrola.cards` com o índice único parcial (`migrations/000001_initial_schema.up.sql`).
- Mecanismo de confirmação/slot-filling do kernel (`workflow.SuspendAwaitingInput` + resume por merge-patch) e o padrão de estado de espera fechado exemplificado por `internal/agents/application/workflows/pending_entry_workflow.go`; como esse workflow é acoplado a transações (ledger/categoria/`IdempotentWriter` em `pending_entry_workflow.go:353`), é necessário um estado de espera próprio para cartão, não reuso literal.
- Injeção de contexto de idempotência (`idempotency.FromContext`) pela nova ferramenta para ativar o bloco existente em `internal/card/application/usecases/create_card.go:115-132`, hoje dormente no caminho conversacional.
- Nova métrica de escrita de cartão seguindo o padrão de `agents_write_total` (`internal/agents/application/usecases/idempotent_write.go:35`); hoje não há métrica de cadastro de cartão.
- Ampliação do tipo `NewCard` (`internal/agents/application/interfaces/types.go:122`) e do input `CreateCard` para transportar um dia de fechamento explícito quando o banco não é reconhecido.
- Sinal de "banco não reconhecido" no leitor `BankDaysReader.DaysBeforeDue` (hoje retorna fallback silencioso de 7 dias em `internal/card/application/usecases/create_card.go:91`), necessário para decidir quando perguntar o dia de fechamento; a mesma correção beneficia o cadastro no onboarding (`internal/agents/application/workflows/onboarding_workflow.go:681`).
- Registro da nova ferramenta `create_card` no conjunto de ferramentas do agente diário e ajuste das instruções em `mecontrola_agent.go` (em `internal/agents`, camada application) para descrever a capacidade e proibir respostas de cadastro sem tool call.

## Fora de Escopo
- Edição e exclusão de cartão pela conversa, que já existem como `update_card` e soft delete (`internal/agents/application/tools/update_card.go`, `cardManagerAdapter.SoftDeleteCard`).
- Gestão de fatura e parcelas do cartão.
- Cadastro de cartão pelo aplicativo (fora do canal WhatsApp).
- Expandir a lista de bancos suportados na tabela `mecontrola.banks`; a decisão adotada é perguntar o dia de fechamento para bancos não reconhecidos, não cadastrar novos bancos.

## Evidências
- Entrada: transcrição de produção fornecida pelo usuário (pedido de cadastro do cartão "XP" e a resposta de falha) e investigação via SSH em `187.77.45.48`, triangulada em quatro sinks de observabilidade:
  - Postgres (dados): run `0a42d7e2` em `platform_runs` com `status=succeeded`, `outcome=routed` e coluna de erro vazia; `platform_messages` com o "Sim" às 18:32:42.231255 e a resposta de falha às 18:32:42.233615 (~2ms), `parts` vazio e nenhuma mensagem `role=tool`; nenhuma linha de cartão criada além do "Cartão Nu" (Nubank) já existente; nenhum evento no outbox.
  - Tempo (traces): nenhum span `card.usecase.create` nem `agents.binding.card_manager.create_card` na janela 18:32:00-18:34:00 UTC; a janela contém apenas `GET /healthz`, `whatsapp.ratelimit.cleanup`, `onboarding.repository.magic_token.count_paid_unconsumed` e `budgets.usecase.run_pending_events_reaper`.
  - Prometheus (métricas): `agents_write_total` só possui `operation="register_expense"`, sem `operation="create_card"`; não há métrica de escrita de cartão.
  - Postgres (idempotência): 0 linhas em `idempotency_keys` e em `agents_write_ledger` para o usuário e para a hora.
  - Binário deployado `ghcr.io/limateixeiratecnologia/mecontrola@sha256:e5d1ff02...` sem ferramenta de criação de cartão no agente diário.
- Base de código: ausência de ferramenta de criação em `internal/agents/application/tools/` (só existem `get_card.go`, `list_cards.go`, `resolve_card.go`, `update_card.go`, `count_cards.go`, `query_card_invoice.go`); `CardManager.CreateCard` chamado apenas por `internal/agents/application/workflows/onboarding_workflow.go:681`; usecase e derivação do fechamento em `internal/card/application/usecases/create_card.go:49-96`; fallback silencioso para banco desconhecido em `create_card.go:91`; idempotência em `create_card.go:115-132`; índice único parcial em `migrations/000001_initial_schema.up.sql`; padrão de confirmação em `internal/agents/application/workflows/pending_entry_workflow.go`; instruções do agente em `mecontrola_agent.go:62` e `:80` (em `internal/agents`, camada application).
- Inferências: a resposta de falha foi gerada pelo LLM sem qualquer execução de backend. A conclusão não depende de um único sinal — é sustentada pela concordância dos quatro sinks (sem span em Tempo, sem métrica de escrita de cartão em Prometheus, sem linha em `idempotency_keys`/`agents_write_ledger`, sem mensagem `role=tool` e sem linha de cartão no Postgres), além do intervalo de ~2ms entre "Sim" e a resposta.
- Não evidenciado: não existe erro de usecase oculto a recuperar — a triangulação prova que o usecase `CreateCard` nunca foi invocado (nenhum span/métrica/registro de idempotência), então não há "string exata de erro" pendente; a lacuna real é de capacidade e de instrumentação, corrigida por RN8 (guardrail + tool) e RN10 (persistir erro do run + métrica de escrita).

## Notas de Validação
- A história cobre fluxo feliz (banco reconhecido), fluxos alternativos (banco não reconhecido, slot-filling, cancelamento, ambiguidade, TTL, idempotência) e fluxos de erro (apelido duplicado, dia inválido, regressão do incidente sem tool call), atendendo aos critérios de qualidade de história.
- O root cause foi confrontado contra codebase, tracing (Tempo), métricas (Prometheus), logs de container e Postgres; os quatro sinks concordam que nenhuma lógica de cadastro executou, eliminando falso positivo sobre a causa.
- Distinção explícita entre o que já existe e o que precisa ser construído, para evitar falso positivo de evidência: já existem o usecase `CreateCard`, o binding, a tabela `cards` e o mecanismo de suspend/resume do kernel; precisam ser construídos a ferramenta `create_card`, o estado de espera card-scoped (RN2, pois o gate atual é acoplado a transações), a injeção de idempotência (RN9, hoje dormente) e a métrica de escrita de cartão (RN10, hoje inexistente).
- Todas as afirmações técnicas estão ancoradas em caminho e, quando aplicável, linha da base de código local (branch `main`) ou em evidência direta da investigação de produção; nenhuma capacidade é atribuída à base de código sem verificação.
- As duas decisões de escopo (habilitar cadastro por chat e perguntar o dia de fechamento para bancos não reconhecidos) foram confirmadas explicitamente pelo usuário e estão refletidas em RN1, RN2 e RN5.
