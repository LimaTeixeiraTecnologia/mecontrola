# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** `CardUpdateState` dedicado com confirmação de-para e correção do payload de `due_day`
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Solicitante do produto, time de plataforma/agentes
- **Relacionados:** PRD `.specs/prd-editar-cartao-conversacional/prd.md` (RF-05, RF-10, RF-18, RF-19), techspec `.specs/prd-editar-cartao-conversacional/techspec.md`, ADR-001, ADR-002

## Contexto

Dois problemas concretos no estado atual da edição:

1. **Defeito de payload de `due_day`.** Quando a confirmação é acionada por mudança de vencimento, o `CardUpdate` serializado no `ConfirmState.UpdatePayload` **omite `DueDay`** (`update_card.go:118-122`), embora a confirmação só ocorra justamente quando `due_day` muda. Ao efetivar, `executeUpdateCard` (`destructive_confirm_workflow.go:363-369`) desserializa um payload sem `DueDay`, e o novo vencimento pode não ser persistido.

2. **Falta de clareza na confirmação.** O gate compartilhado mostra apenas uma nota de impacto genérica, sem informar o que muda em relação ao estado atual. O PRD decidiu (RF-10) exibir o **de-para** (valor atual → novo) por campo alterado.

O PRD também decidiu edição multi-campo numa única confirmação (RF-05) e permitir alteração de vencimento com parcelas em aberto após aviso (RF-19).

## Decisão

Modelar um estado fechado dedicado `CardUpdateState` que carrega **tanto os valores atuais quanto os novos**, e usá-lo para (a) montar a confirmação de-para e (b) construir o payload de escrita completo — eliminando o defeito de `due_day`.

Campos do estado (tipo fechado `CardUpdateStatus` = Active/Completed/Cancelled/Expired):
- Identidade/lock: `UserID`, `CardID`, `ExpectedVersion` (ADR-002).
- Valores atuais (de-para): `CurrentNickname`, `CurrentBank`, `CurrentDueDay`.
- Valores novos (apenas os informados): `NewNickname *string`, `NewBank *string`, `NewDueDay *int`, `NewClosingDay *int`.
- Controle de confirmação: `Awaiting`, `MessageID`, `IncomingMessageID`, `ProcessedMessageID`, `ConfirmReprompt`, `SuspendedAt`, `ResumeText`, `ResponseText`, `Expired`.

Pergunta de confirmação de-para (`buildCardUpdateQuestion`), determinística, uma linha por campo alterado:
- `Apelido: {CurrentNickname} → {NewNickname}`
- `Banco: {CurrentBank} → {NewBank}`
- `Vencimento: dia {CurrentDueDay} → dia {NewDueDay}` e, quando o vencimento muda, acrescentar `A alteração do dia de vencimento pode impactar parcelas em aberto.` (RF-19: permite, apenas avisa).
- Encerrar com `Responda *sim* para confirmar ou *não* para cancelar.`

Regras de montagem na tool `update_card`:
- Só entram no de-para os campos **informados e diferentes** do atual; se nada difere, retorna `no_changes` sem iniciar workflow.
- Banco novo não reconhecido sem `closingDay` → outcome `needs_closing` (espelha a criação), pedindo o dia de fechamento; com `closingDay` informado, segue.
- `closing_day` não é editável isoladamente (RF-07); só é aceito para compor o ciclo de um banco não reconhecido.

Plumbing do fechamento informado (RF-17), fim a fim, para que o valor do usuário não seja substituído pelo `fallbackDaysBeforeDue`:
- `CardUpdateState.NewClosingDay *int` (tool coleta no caso `needs_closing`).
- `executeUpdateCard` inclui `ClosingDay` no `interfaces.CardUpdate`.
- `interfaces.CardUpdate.ClosingDay *int` (ADR-002/task 2.0) e `cardinput.UpdateCard.ClosingDay *int` (task 1.0) propagam ao módulo card.
- `resolveUpdate`, quando `ClosingDay != nil`, constrói o ciclo via `NewBillingCycle(closingDay, dueDay)` **sem** chamar `DaysBeforeDue` (espelha `create_card.go`).

Escrita (`executeUpdateCard`): o `CardUpdate` é montado a partir do estado com **todos** os campos novos presentes (incluindo `NewDueDay`), `ExpectedVersion` e identidade — corrigindo o defeito.

Decisão de confirmação: reutilizar o tipo fechado `CardConfirmAction` (já existente em `card_create_decisions.go`) e uma função pura `DecideCardUpdateConfirmation` (Accept/Cancel/Reprompt/Expire/Replay), sem IO, testável sem mock.

## Alternativas Consideradas

- **Reusar `ConfirmState` (do destructive-confirm) e só adicionar `DueDay` ao payload.**
  - Vantagens: menor mudança imediata.
  - Desvantagens: `ConfirmState` é genérico (serve delete/edit de várias entidades) e não guarda valores atuais para o de-para; forçaria campos específicos de cartão num estado compartilhado; mantém o acoplamento que o ADR-001 remove.
  - Motivo da rejeição: viola o isolamento de semântica e não oferece o de-para de forma limpa.
- **Buscar os valores atuais só no momento de renderizar a pergunta (sem guardá-los no estado).**
  - Vantagens: estado menor.
  - Desvantagens: exigiria IO adicional no passo de suspensão e no resume; o snapshot deixaria de ser autossuficiente; a `ExpectedVersion` precisa mesmo ser capturada uma única vez no início.
  - Motivo da rejeição: o snapshot como fonte única de verdade (merge-patch no resume) fica mais simples e determinístico guardando os valores atuais uma vez.

## Consequências

### Benefícios Esperados

- Corrige o defeito de `due_day` (novo vencimento sempre persiste).
- Confirmação clara com de-para, reduzindo edição do cartão errado.
- Estado fechado autossuficiente no snapshot; resume por merge-patch sem side-store.
- Edição multi-campo natural (todos os campos novos no mesmo estado).

### Trade-offs e Custos

- Estado com mais campos (atual + novo) que o `ConfirmState` genérico.
- Necessário snapshot dos valores atuais no início da confirmação (uma leitura via `GetCard`, já necessária para a versão — ADR-002).

### Riscos e Mitigações

- Risco: divergência entre valores atuais capturados e o estado real no commit. Mitigação: o lock otimista (`ExpectedVersion`, ADR-002) aborta se o cartão mudou; o de-para é meramente informativo.
- Risco: de-para exibir campo não alterado. Mitigação: incluir no de-para apenas campos informados e diferentes do atual; teste unitário de montagem.

## Plano de Implementação

1. `card_update_state.go` (`CardUpdateStatus`, `CardUpdateState`).
2. `card_update_decisions.go` (`DecideCardUpdateConfirmation`, `buildCardUpdateQuestion` de-para, TTL/reprompt).
3. Consumo no workflow (`executeUpdateCard` monta payload completo) e na tool (montagem do estado, `no_changes`/`needs_closing`).
4. Testes puros de decisão e de montagem do de-para; teste que prova a persistência do novo `due_day`.
Concluído quando: teste específico do defeito de `due_day` passa (o valor gravado é o novo); de-para correto por campo; build/vet/lint/race verdes.

## Monitoramento e Validação

- Teste de regressão dedicado ao `due_day` (persistência do novo valor).
- Caso golden/real-LLM de alteração de vencimento verifica a presença do aviso de impacto.
- Critério de sucesso: nenhum caso de vencimento confirmado que não persista.

## Impacto em Documentação e Operação

- Documentar o formato de-para da confirmação de edição.
- Sem impacto em runbooks de operação além do já previsto em ADR-001.

## Revisão Futura

- Revisar se novos campos de cartão (futuros) exigirem generalizar o de-para para uma renderização por lista de campos.
