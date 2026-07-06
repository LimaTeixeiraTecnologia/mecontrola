# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Confirmação humana obrigatória antes de toda escrita financeira da conversa
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia / Produto MeControla
- **Relacionados:** `prd.md` (spec-version 3), `techspec.md`, `adr-001`, `adr-003`

## Contexto

A política anterior tratava lançamentos do agente como "report-only", sem gate de confirmação, priorizando idempotência e rastreabilidade em vez de controle. O PRD `spec-version 3` reverte essa premissa: toda escrita financeira originada da conversa (registro, edição, recorrência) passa a exigir aceite humano explícito antes de persistir, inclusive quando o lançamento já está totalmente especificado e sem ambiguidade categorial. O repositório já possui um contrato semântico de confirmação validado em `destructive-confirm` (aceite/cancelamento estrito, reprompt único, TTL, idempotência pela mensagem original).

## Decisão

Introduzir o estado fechado `AwaitingSlotConfirmation` como gate terminal obrigatório da pendência `pending-entry`. Toda operação transita para este estado após os demais slots estarem preenchidos e válidos; a persistência só ocorre no resume que carrega o aceite explícito. A decisão de confirmação é a função pura `DecideConfirmation(state, msg, now)`, sem IO e sem LLM. O contrato semântico do `destructive-confirm` é **reutilizado**, mas o workflow destrutivo NÃO é reaproveitado — evita-se misturar confirmação de escrita normal com operações destrutivas/sensíveis.

Lançamentos inequívocos abrem a pendência diretamente em `AwaitingSlotConfirmation`; nenhuma tool de escrita persiste de forma síncrona.

## Alternativas Consideradas

- Manter escrita report-only sem confirmação: revogada pelo PRD `spec-version 3`.
- Confirmar apenas quando houve clarificação de slot: rejeitada por não cumprir "sempre confirmar".
- Rotear registros pelo próprio workflow `destructive-confirm`: rejeitada por misturar confirmação de escrita normal com operações destrutivas, aumentando risco de regressão.
- Confirmação inline sem estado durável no caminho feliz: rejeitada por reduzir auditabilidade do aceite.

## Consequências

### Benefícios Esperados

- Zero escrita sem aceite humano explícito comprovável no Run auditável (M-07 = 0).
- Reutilização de contrato de confirmação já validado, com semântica estrita e determinística.
- Uniformidade: registro, edição e recorrência compartilham o mesmo gate e as mesmas defesas categoriais.

### Trade-offs e Custos

- Um turno adicional de confirmação no caminho feliz, inclusive para lançamentos inequívocos.
- Necessidade de revisar fluxos e critérios de aceite do PRD para incluir o turno de confirmação.
- `ConfirmRepromptCount` separado do `RepromptCount` de coleta de slot.

### Riscos e Mitigações

- Risco de contagem de perguntas violar M-05. Mitigação: o gate é passo único e final, explicitamente fora da métrica de coleta de dados (RF-41).
- Risco de escrita antes do aceite por ordem incorreta de passos. Mitigação: harness verifica ordem confirm→write no Run auditável (CA-13).
- Risco de ambiguidade infinita na confirmação. Mitigação: reprompt único e depois cancelamento sem efeito (RF-39/CA-14).

## Plano de Implementação

1. Adicionar `DecideConfirmation` puro e `ConfirmRepromptCount` no estado.
2. Tornar `AwaitingSlotConfirmation` transição terminal obrigatória antes de qualquer escrita.
3. Fazer `register_attempt` abrir a pendência sempre (clarify ou confirmação), sem escrita síncrona.
4. Cobrir CA-13/CA-14 e M-07 no harness determinístico.

## Monitoramento e Validação

Monitorar `agents_pending_entry_slot_total{slot="confirmation",outcome}` e `agents_pending_entry_write_total{outcome}`. A decisão é válida quando 100% das escritas do harness ocorrem após aceite explícito e M-07 = 0.

## Impacto em Documentação e Operação

Atualizar runbook de conversas pendentes com o passo de confirmação e revogar a documentação da política report-only anterior.

## Revisão Futura

Revisar se surgir necessidade de confirmação assíncrona multi-dispositivo ou aprovação por terceiros, hoje explicitamente fora de escopo.
