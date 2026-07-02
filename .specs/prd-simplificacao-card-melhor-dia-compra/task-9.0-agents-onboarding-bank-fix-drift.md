# Tarefa 9.0: agents onboarding — coletar `bank`, corrigir drift `ClosingDay=DueDay`, remover `LimitCents`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ajustar o onboarding de cartão via WhatsApp em `internal/agents`: o schema de extração LLM e o prompt
passam a coletar o **banco emissor** além de apelido e vencimento; `DecideCardEntry` valida o banco; os
tipos `NewCard`/`Card` ganham `Bank` e perdem `LimitCents`; o adapter `card_manager_adapter` corrige o
drift (envia `DueDay` corretamente, não como `ClosingDay`) e repassa `Bank`. Banco desconhecido segue com
fallback de 7 dias, sem atrito.

<requirements>
- RF-16(a): `cardExtract`/`cardSchema` + prompt coletam `bank`; system message de extração inclui `bank`.
- RF-16(b): `card_manager_adapter.CreateCard` envia `DueDay: in.DueDay` (corrige drift `ClosingDay: in.DueDay`) e `Bank: in.Bank`; remove `LimitCents`.
- RF-16(c): `interfaces.NewCard`/`Card` ganham `Bank`; `LimitCents` removido; `ClosingDay` reflete o derivado retornado pelo card.
- RF-16(d): banco fora da tabela → segue o cadastro com fallback 7 (sem interromper nem pedir confirmação).
- `DecideCardEntry(nickname, bank string, dueDay int)` valida `bank` não-vazio.
</requirements>

## Subtarefas

- [ ] 9.1 Editar `application/workflows/onboarding_workflow.go`: `cardExtract{+Bank}`, `cardSchema` (+`bank` em properties/required), prompt (pedir banco) e system message de extração (+`bank`).
- [ ] 9.2 `DecideCardEntry(nickname, bank string, dueDay int) error`: validar `bank` não-vazio (além de nickname/dueDay); ajustar a chamada `cards.CreateCard(NewCard{..., Bank: extract.Bank})`.
- [ ] 9.3 Editar `application/interfaces/types.go`: `NewCard{+Bank}`; `Card{+Bank, -LimitCents}`.
- [ ] 9.4 Editar `infrastructure/binding/card_manager_adapter.go`: `CreateCard` mapeia `Nickname`, `Bank`, `DueDay` (ptr) corretamente; remover `Name`/`ClosingDay`/`LimitCents` hardcoded; `ListCards` sem `LimitCents`, com `Bank`.

## Detalhes de Implementação

Ver `techspec.md` §"Pontos de Integração" (RF-16) e o mapeamento do adapter. Depende da nova assinatura
de `cardinput.CreateCard` (`Nickname, Bank, DueDay`) entregue na tarefa 4.0. O onboarding é um workflow
do consumidor `internal/agents` (substrato Mastra) — manter tipos fechados de fase e adapter fino.

## Critérios de Sucesso

- Onboarding pergunta e extrai `bank`; `DecideCardEntry` rejeita banco vazio.
- Cartão criado via WhatsApp tem `bank` correto e `due_day` = o informado (não mais mapeado como fechamento).
- `NewCard`/`Card` sem `LimitCents`; adapter compila com a nova `cardinput.CreateCard`.
- Banco desconhecido cadastra com fallback 7 sem erro/pergunta extra.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — alteração do workflow de onboarding (schema de extração, prompt, `DecideCardEntry`) e do adapter de cartão no consumidor `internal/agents` sobre o substrato Mastra.

## Testes da Tarefa

- [ ] Testes unitários: teste de `DecideCardEntry` (banco vazio → erro); teste do adapter (`DueDay` correto, `Bank` repassado, sem `LimitCents`).
- [ ] Testes de integração: fluxo de onboarding de cartão (se houver harness) com banco conhecido e desconhecido.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/interfaces/types.go`
- `internal/agents/infrastructure/binding/card_manager_adapter.go`
