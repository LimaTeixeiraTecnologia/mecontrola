# ADR-005 — Consolidação da identificação do cartão em um único campo (`nickname`)

## Metadados

- **Título:** Consolidar `name` + `nickname` em `nickname`; dropar coluna/VO `name`
- **Data:** 2026-07-01
- **Status:** Aceita
- **Decisores:** JailtonJunior (owner), time de plataforma
- **Relacionados:** PRD (RF-01, RF-02), techspec.md

## Contexto

Hoje o cartão tem dois campos de identificação: `name` (VO `CardName`, 1-64 runas, coluna
`cards.name`, CHECK `cards_name_len_chk`) e `nickname` (VO `Nickname`, 1-32 runas, coluna
`cards.nickname`, índice único parcial `cards_user_nickname_active_uniq_idx` por `(user_id, nickname)`).
O pedido do usuário trata "nome do cartão/apelido" como **um único conceito** (RF-02). O adapter de
onboarding já envia `Name = Nickname` (`card_manager_adapter.go`), evidenciando redundância. Precisamos
escolher qual campo sobrevive.

## Decisão

Consolidar em **`nickname`** e **dropar** `name`. Concretamente: remover o VO `CardName`
(`card_name.go`) e o erro `ErrInvalidCardName`; remover `Card.Name` e `NewCardInput.Name`; remover
`name` de DTOs (`create_card`, `update_card`, `output.Card`), do mapper, das queries/scan do repositório
e do OpenAPI; migration `000002` dropa a coluna `cards.name` e o CHECK `cards_name_len_chk`. O índice
único parcial permanece em `(user_id, nickname)` — a unicidade por usuário do apelido é preservada. O
limite de tamanho passa a ser o de `Nickname` (1-32 runas).

## Alternativas Consideradas

- **Manter `nickname`, dropar `name` (escolhida).** Vantagens: `nickname` já tem unicidade por usuário
  (índice existente) e é o campo coletado no onboarding. Desvantagens: reduz o comprimento máximo de
  64→32 runas.
- **Manter `name`, dropar `nickname`.** Desvantagens: exigiria migrar o índice de unicidade para `name`
  e realinhar o onboarding (que usa `nickname`). Mais mudança, sem ganho. Rejeitada.
- **Manter os dois campos.** Contradiz RF-02 (conceito único) e mantém a redundância evidenciada pelo
  drift do onboarding. Rejeitada.

## Consequências

### Benefícios Esperados

- Um só campo de identificação; contrato e onboarding coerentes; menos VOs/DTOs.
- Unicidade por usuário preservada sem migração de índice.

### Trade-offs e Custos

- Comprimento máximo do identificador cai para 32 runas. Sem cartões em produção ⇒ sem impacto de dados.

### Riscos e Mitigações

- **Decisão:** o payload/evento `card.invoice_due.v1` **renomeia** a chave `card_name` → `card_nickname`
  (struct `CardName` → `CardNickname`), alimentada por `nickname`. Sem cartões/uso em produção e consumidor
  no mesmo repo ⇒ renomeação sem versionamento de evento; elimina o campo semanticamente enganoso (regra
  HARD "zero campo morto/enganoso"). Producer, consumer e `NotifyInvoiceDueInput` mudam juntos.
- **Rollback:** `.down.sql` recria `cards.name` (com CHECK) — sem dados a restaurar.

## Plano de Implementação

1. Migration `000002`: `DROP CONSTRAINT cards_name_len_chk; DROP COLUMN name;`.
2. Domínio: remover `CardName`/`ErrInvalidCardName`; ajustar `Card`/hydrate/deciders.
3. Application/infra: remover `name` de DTOs, mapper, queries, handlers, OpenAPI.
4. Produtor/consumidor: `card_name` do evento alimentado por `nickname`.

## Monitoramento e Validação

- Testes de repositório/e2e sem `name`; contrato OpenAPI revalidado.
- Sucesso: grep por `CardName`/`ErrInvalidCardName`/`\.Name` no módulo card retorna vazio.

## Impacto em Documentação e Operação

- OpenAPI; runbook de card se existir; exemplos de request.

## Revisão Futura

- Revisitar se surgir necessidade de um "nome de exibição" separado do "apelido único".
